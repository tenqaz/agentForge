package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// TestAgentDeleteHappyPath verifies the full delete flow:
// owner can delete a running agent, container Stop+Remove are invoked,
// hermes-home is wiped, DB row is gone, child runtime_jobs are cascaded.
func TestAgentDeleteHappyPath(t *testing.T) {
	ctx := context.Background()
	fixture := newMVPFixture(t)

	adminCookie := loginAndGetCookie(t, fixture.router, "admin@example.com", "secret-password")
	userCookie := loginAndGetCookie(t, fixture.router, "user@example.com", "secret-password")
	agentID, homePath := provisionRunningAgent(t, ctx, fixture, adminCookie, userCookie, "delete-happy-agent")

	// Verify hermes home was actually created by provision
	if _, err := os.Stat(homePath); err != nil {
		t.Fatalf("hermes home should exist after provision, stat err = %v", err)
	}

	stopBefore := fixture.runner.StopCalls()
	removeBefore := fixture.runner.RemoveCalls()

	deleteResp := doAuthedRequest(t, fixture.router, http.MethodDelete, "/api/agents/"+agentID, "", userCookie)
	if deleteResp.Code != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, body = %s", deleteResp.Code, deleteResp.Body.String())
	}

	// GET should now be 404
	getResp := doAuthedRequest(t, fixture.router, http.MethodGet, "/api/agents/"+agentID, "", userCookie)
	if getResp.Code != http.StatusNotFound {
		t.Fatalf("GET after delete status = %d, want 404", getResp.Code)
	}

	// DB rows are gone (CASCADE)
	var agentCount, jobCount int
	if err := fixture.database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM agents WHERE id = ?;`, agentID).Scan(&agentCount); err != nil {
		t.Fatalf("count agents: %v", err)
	}
	if agentCount != 0 {
		t.Errorf("agents row count = %d, want 0", agentCount)
	}
	if err := fixture.database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM runtime_jobs WHERE agent_id = ?;`, agentID).Scan(&jobCount); err != nil {
		t.Fatalf("count runtime_jobs: %v", err)
	}
	if jobCount != 0 {
		t.Errorf("runtime_jobs count = %d, want 0 (CASCADE)", jobCount)
	}

	// hermes home directory removed
	if _, err := os.Stat(homePath); !os.IsNotExist(err) {
		t.Errorf("hermes home still exists, stat err = %v", err)
	}

	// runner.Stop + runner.Remove were both called once
	if got := fixture.runner.StopCalls() - stopBefore; got != 1 {
		t.Errorf("runner.Stop calls (delta) = %d, want 1", got)
	}
	if got := fixture.runner.RemoveCalls() - removeBefore; got != 1 {
		t.Errorf("runner.Remove calls (delta) = %d, want 1", got)
	}
}

// TestAgentDeleteRefusesProvisioning verifies that the API returns 409
// agent_cannot_delete when the agent is in an unstable state.
func TestAgentDeleteRefusesProvisioning(t *testing.T) {
	ctx := context.Background()
	fixture := newMVPFixture(t)
	adminCookie := loginAndGetCookie(t, fixture.router, "admin@example.com", "secret-password")
	userCookie := loginAndGetCookie(t, fixture.router, "user@example.com", "secret-password")

	templateID := publishSimpleTemplate(t, fixture, adminCookie, "Delete Refuse Template")
	createAgent := doAuthedRequest(t, fixture.router, http.MethodPost, "/api/agents",
		`{"templateId":"`+templateID+`","name":"refuse-agent"}`, userCookie)
	if createAgent.Code != http.StatusCreated {
		t.Fatalf("create agent status = %d, body = %s", createAgent.Code, createAgent.Body.String())
	}
	agentID := decodeAgentID(t, createAgent.Body.Bytes())

	// Force into provisioning to bypass white-list check.
	if _, err := fixture.database.ExecContext(ctx,
		`UPDATE agents SET status='provisioning' WHERE id=?;`, agentID); err != nil {
		t.Fatalf("force provisioning: %v", err)
	}

	resp := doAuthedRequest(t, fixture.router, http.MethodDelete, "/api/agents/"+agentID, "", userCookie)
	if resp.Code != http.StatusConflict {
		t.Fatalf("DELETE status = %d, want 409", resp.Code)
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Error != "agent_cannot_delete" {
		t.Errorf("error code = %q, want agent_cannot_delete", body.Error)
	}
}

// TestAgentDeleteRecoversFromStopFailure verifies the delete operation
// can be retried after a transient external failure: agent transitions
// to error with delete_stop_failed, then a second DELETE completes the
// cleanup.
func TestAgentDeleteRecoversFromStopFailure(t *testing.T) {
	ctx := context.Background()
	fixture := newMVPFixture(t)

	adminCookie := loginAndGetCookie(t, fixture.router, "admin@example.com", "secret-password")
	userCookie := loginAndGetCookie(t, fixture.router, "user@example.com", "secret-password")
	agentID, _ := provisionRunningAgent(t, ctx, fixture, adminCookie, userCookie, "delete-retry-agent")

	// First attempt: Stop fails.
	fixture.runner.SetStopError(errors.New("transient docker stop failure"))

	first := doAuthedRequest(t, fixture.router, http.MethodDelete, "/api/agents/"+agentID, "", userCookie)
	if first.Code != http.StatusInternalServerError {
		t.Fatalf("first DELETE status = %d, want 500, body = %s", first.Code, first.Body.String())
	}

	var status, lastErrorCode string
	if err := fixture.database.QueryRowContext(ctx,
		`SELECT status, last_error_code FROM agents WHERE id = ?;`, agentID).
		Scan(&status, &lastErrorCode); err != nil {
		t.Fatalf("fetch agent state after failed delete: %v", err)
	}
	if status != "error" || lastErrorCode != "delete_stop_failed" {
		t.Fatalf("agent state = (%s, %s), want (error, delete_stop_failed)", status, lastErrorCode)
	}

	// Second attempt: clear the error and retry.
	fixture.runner.SetStopError(nil)

	retry := doAuthedRequest(t, fixture.router, http.MethodDelete, "/api/agents/"+agentID, "", userCookie)
	if retry.Code != http.StatusNoContent {
		t.Fatalf("retry DELETE status = %d, want 204, body = %s", retry.Code, retry.Body.String())
	}

	var count int
	if err := fixture.database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM agents WHERE id = ?;`, agentID).Scan(&count); err != nil {
		t.Fatalf("count agents after retry: %v", err)
	}
	if count != 0 {
		t.Errorf("agents count after retry = %d, want 0", count)
	}
}

// provisionRunningAgent creates an admin-published template, then a user
// agent, runs the provision job, and returns the agentID and its hermes
// home path. It assumes the integrationRunner doesn't fail by default.
func provisionRunningAgent(
	t *testing.T,
	ctx context.Context,
	fixture mvpFixture,
	adminCookie, userCookie *http.Cookie,
	agentName string,
) (agentID, homePath string) {
	t.Helper()
	templateID := publishSimpleTemplate(t, fixture, adminCookie, "Delete IT Template "+agentName)

	createAgent := doAuthedRequest(t, fixture.router, http.MethodPost, "/api/agents",
		`{"templateId":"`+templateID+`","name":"`+agentName+`"}`, userCookie)
	if createAgent.Code != http.StatusCreated {
		t.Fatalf("create agent status = %d, body = %s", createAgent.Code, createAgent.Body.String())
	}
	agentID = decodeAgentID(t, createAgent.Body.Bytes())

	jobID := mustLoadSingleRuntimeJobID(t, fixture.database, agentID)
	if err := fixture.runtimeWorker.ProcessJob(ctx, jobID); err != nil {
		t.Fatalf("provision job failed: %v", err)
	}

	// Verify status is running
	var status string
	if err := fixture.database.QueryRowContext(ctx,
		`SELECT status FROM agents WHERE id = ?;`, agentID).Scan(&status); err != nil {
		t.Fatalf("fetch agent status: %v", err)
	}
	if status != "running" {
		t.Fatalf("agent status after provision = %s, want running", status)
	}

	homePath = filepath.Join(fixture.dataDir, "agents", agentID, "hermes-home")
	return agentID, homePath
}

// publishSimpleTemplate creates and publishes a minimal template for use
// in agent delete tests.
func publishSimpleTemplate(t *testing.T, fixture mvpFixture, adminCookie *http.Cookie, name string) string {
	t.Helper()

	createResp := doMultipartTemplateCreate(t, fixture.router, adminCookie,
		name, "for delete tests",
		"# Soul\nDelete-test soul.",
		"# User\nDelete-test user.",
		[]multipartSkillFile{
			{name: "noop.zip", content: makeSkillArchive(t, map[string]string{
				"SKILL.md": "---\nname: Noop\ndescription: Noop skill.\n---\n# SKILL\nNoop.\n",
			})},
		})
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create template status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	templateID := decodeTemplateID(t, createResp.Body.Bytes())

	publish := doAuthedRequest(t, fixture.router, http.MethodPut,
		"/api/admin/templates/"+templateID+"/publication", `{}`, adminCookie)
	if publish.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", publish.Code, publish.Body.String())
	}
	return templateID
}
