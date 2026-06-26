package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"agentforge.local/services/api/internal/agents"
	"agentforge.local/services/api/internal/jobs"
	"agentforge.local/services/api/internal/runtime"
	"agentforge.local/services/api/internal/templates"
)

func TestRuntimePersistenceRestartKeepsSessionsAndWeixinAccountFiles(t *testing.T) {
	ctx := context.Background()
	fixture := newMVPFixture(t)

	templateService := templates.NewService(
		templates.NewRepository(fixture.database),
		templates.NewFileStore(fixture.dataDir),
	)
	createdTemplate, err := templateService.Create(ctx, "admin-1", "Persistence Template", "Checks restart persistence.")
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	if _, err := templateService.PutSoul(ctx, createdTemplate.ID, "# Soul\nPersistent"); err != nil {
		t.Fatalf("put soul: %v", err)
	}
	if _, err := templateService.PutUser(ctx, createdTemplate.ID, "# User\nPersistent"); err != nil {
		t.Fatalf("put user: %v", err)
	}
	publishedTemplate, err := templateService.Publish(ctx, createdTemplate.ID)
	if err != nil {
		t.Fatalf("publish template: %v", err)
	}

	agentService := agents.NewService(
		fixture.database,
		agents.NewRepository(fixture.database),
		jobs.NewRuntimeRepository(fixture.database),
		nil,
		fixture.dataDir,
		"docker",
	)
	createdAgent, err := agentService.Create(ctx, agents.CreateParams{
		OwnerUserID: "user-1",
		TemplateID:  publishedTemplate.ID,
		Name:        "Persistence Agent",
	})
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}

	provisionJobID := mustLoadSingleRuntimeJobID(t, fixture.database, createdAgent.ID)
	if err := fixture.runtimeWorker.ProcessJob(ctx, provisionJobID); err != nil {
		t.Fatalf("provision runtime: %v", err)
	}

	agentHome := filepath.Join(fixture.dataDir, "agents", createdAgent.ID, "hermes-home")
	sessionPath := filepath.Join(agentHome, "sessions", "sticky.session")
	accountPath := filepath.Join(agentHome, "weixin", "accounts", "wx-bot-1.json")
	writeTestFile(t, sessionPath, "keep-session")
	writeTestFile(t, accountPath, `{"account_id":"wx-bot-1"}`)

	restartJob, err := jobs.NewRuntimeRepository(fixture.database).CreateQueued(ctx, jobs.RuntimeJob{
		AgentID: createdAgent.ID,
		Type:    jobs.TypeRestartRuntime,
	})
	if err != nil {
		t.Fatalf("create restart job: %v", err)
	}
	if err := fixture.runtimeWorker.ProcessJob(ctx, restartJob.ID); err != nil {
		t.Fatalf("restart runtime: %v", err)
	}

	assertFileContains(t, sessionPath, "keep-session")
	assertFileContains(t, accountPath, `"account_id":"wx-bot-1"`)

	runtimeState, err := agentService.Runtime(ctx, createdAgent.ID)
	if err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	if runtimeState.Status != agents.StatusRunning {
		t.Fatalf("runtime status = %s, want %s", runtimeState.Status, agents.StatusRunning)
	}
	if runtimeState.RuntimeID != runtime.DefaultContainerName(createdAgent.ID) {
		t.Fatalf("runtime id = %q, want %q", runtimeState.RuntimeID, runtime.DefaultContainerName(createdAgent.ID))
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
