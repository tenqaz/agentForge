package http

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"agentforge.local/services/api/internal/agents"
	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/jobs"

	_ "modernc.org/sqlite"
)

func TestAgentRoutesCreateListDetailRuntimeAndJobs(t *testing.T) {
	router, manager, database, dataDir := newAgentTestRouter(t)
	userCookie := sessionCookieFor(t, manager, auth.User{ID: "user-1", Email: "user@example.com", Role: auth.RoleUser})
	ctx := t.Context()

	createRecorder := doJSON(t, router, http.MethodPost, "/api/agents", `{"templateId":"template-1","name":"Support Agent"}`, userCookie)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	created := decodeAgentResponse(t, createRecorder.Body.Bytes()).Agent
	if created.Status != agents.StatusCreating {
		t.Fatalf("created agent = %#v", created)
	}
	expectedHome := filepath.Join(dataDir, "agents", created.ID, "hermes-home")
	var storedHome string
	if err := database.QueryRow(`SELECT hermes_home_path FROM agents WHERE id = ?`, created.ID).Scan(&storedHome); err != nil {
		t.Fatalf("load hermes_home_path: %v", err)
	}
	if storedHome != expectedHome {
		t.Fatalf("stored hermes_home_path = %q, want %q", storedHome, expectedHome)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	listRequest.AddCookie(userCookie)
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	var listResponse struct {
		Agents []agentDTO `json:"agents"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(listResponse.Agents) != 1 || listResponse.Agents[0].ID != created.ID {
		t.Fatalf("list response = %#v", listResponse.Agents)
	}

	detailRecorder := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/agents/"+created.ID, nil)
	detailRequest.AddCookie(userCookie)
	router.ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("detail status = %d, body = %s", detailRecorder.Code, detailRecorder.Body.String())
	}
	detail := decodeAgentResponse(t, detailRecorder.Body.Bytes()).Agent
	if detail.ID != created.ID || detail.TemplateVersion != 2 {
		t.Fatalf("detail agent = %#v", detail)
	}
	if bytes.Contains(detailRecorder.Body.Bytes(), []byte("hermesHomePath")) {
		t.Fatalf("detail leaked hermes_home_path: %s", detailRecorder.Body.String())
	}

	runtimeRecorder := httptest.NewRecorder()
	runtimeRequest := httptest.NewRequest(http.MethodGet, "/api/agents/"+created.ID+"/runtime", nil)
	runtimeRequest.AddCookie(userCookie)
	router.ServeHTTP(runtimeRecorder, runtimeRequest)
	if runtimeRecorder.Code != http.StatusOK {
		t.Fatalf("runtime status = %d, body = %s", runtimeRecorder.Code, runtimeRecorder.Body.String())
	}
	runtime := decodeRuntimeResponse(t, runtimeRecorder.Body.Bytes()).Runtime
	if runtime.AgentID != created.ID || runtime.Status != agents.StatusCreating {
		t.Fatalf("runtime response = %#v", runtime)
	}

	jobsRecorder := httptest.NewRecorder()
	jobsRequest := httptest.NewRequest(http.MethodGet, "/api/agents/"+created.ID+"/runtime-jobs", nil)
	jobsRequest.AddCookie(userCookie)
	router.ServeHTTP(jobsRecorder, jobsRequest)
	if jobsRecorder.Code != http.StatusOK {
		t.Fatalf("runtime jobs status = %d, body = %s", jobsRecorder.Code, jobsRecorder.Body.String())
	}
	var jobsResponse struct {
		Jobs []runtimeJobDTO `json:"jobs"`
	}
	if err := json.Unmarshal(jobsRecorder.Body.Bytes(), &jobsResponse); err != nil {
		t.Fatalf("unmarshal jobs response: %v", err)
	}
	if len(jobsResponse.Jobs) != 1 || jobsResponse.Jobs[0].Type != jobs.TypeProvisionAgent {
		t.Fatalf("jobs response = %#v", jobsResponse.Jobs)
	}

	jobRecorder := httptest.NewRecorder()
	jobRequest := httptest.NewRequest(http.MethodGet, "/api/agents/"+created.ID+"/runtime-jobs/"+jobsResponse.Jobs[0].ID, nil)
	jobRequest.AddCookie(userCookie)
	router.ServeHTTP(jobRecorder, jobRequest)
	if jobRecorder.Code != http.StatusOK {
		t.Fatalf("job detail status = %d, body = %s", jobRecorder.Code, jobRecorder.Body.String())
	}

	if _, err := database.ExecContext(ctx, `
		UPDATE runtime_jobs
		SET status = 'succeeded', finished_at = datetime('now'), updated_at = datetime('now')
		WHERE id = ?;
	`, jobsResponse.Jobs[0].ID); err != nil {
		t.Fatalf("complete provision job: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		UPDATE agents
		SET status = 'running', runtime_id = 'runtime-1', updated_at = datetime('now')
		WHERE id = ?;
	`, created.ID); err != nil {
		t.Fatalf("activate runtime: %v", err)
	}

	restartRecorder := doJSON(t, router, http.MethodPost, "/api/agents/"+created.ID+"/runtime-jobs", `{"type":"restart_runtime"}`, userCookie)
	if restartRecorder.Code != http.StatusCreated {
		t.Fatalf("restart status = %d, body = %s", restartRecorder.Code, restartRecorder.Body.String())
	}
	restart := decodeRuntimeJobResponse(t, restartRecorder.Body.Bytes()).Job
	if restart.Type != jobs.TypeRestartRuntime || restart.Status != jobs.StatusQueued {
		t.Fatalf("restart job = %#v", restart)
	}
}

func TestAgentRoutesEnforceOwnershipAndAuthentication(t *testing.T) {
	router, manager, database, _ := newAgentTestRouter(t)
	adminCookie := sessionCookieFor(t, manager, auth.User{ID: "admin-1", Email: "admin@example.com", Role: auth.RoleAdmin})
	userCookie := sessionCookieFor(t, manager, auth.User{ID: "user-1", Email: "user@example.com", Role: auth.RoleUser})
	otherUserCookie := sessionCookieFor(t, manager, auth.User{ID: "user-2", Email: "user2@example.com", Role: auth.RoleUser})
	agentID := insertAgentHTTPFixture(t, database, "user-1", agents.StatusRunning)

	missingSession := doJSON(t, router, http.MethodPost, "/api/agents", `{"templateId":"template-1","name":"Support Agent"}`, nil)
	if missingSession.Code != http.StatusUnauthorized {
		t.Fatalf("missing session status = %d, want 401", missingSession.Code)
	}

	forbiddenRecorder := httptest.NewRecorder()
	forbiddenRequest := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID, nil)
	forbiddenRequest.AddCookie(otherUserCookie)
	router.ServeHTTP(forbiddenRecorder, forbiddenRequest)
	if forbiddenRecorder.Code != http.StatusForbidden {
		t.Fatalf("forbidden detail status = %d, want 403", forbiddenRecorder.Code)
	}

	adminRecorder := httptest.NewRecorder()
	adminRequest := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID, nil)
	adminRequest.AddCookie(adminCookie)
	router.ServeHTTP(adminRecorder, adminRequest)
	if adminRecorder.Code != http.StatusOK {
		t.Fatalf("admin detail status = %d, body = %s", adminRecorder.Code, adminRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	listRequest.AddCookie(userCookie)
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	var listResponse struct {
		Agents []agentDTO `json:"agents"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(listResponse.Agents) != 1 || listResponse.Agents[0].OwnerUserID != "user-1" {
		t.Fatalf("list response = %#v", listResponse.Agents)
	}
}

func TestCreateRuntimeJobReturnsConflictWhenAgentAlreadyHasActiveJob(t *testing.T) {
	router, manager, database, _ := newAgentTestRouter(t)
	userCookie := sessionCookieFor(t, manager, auth.User{ID: "user-1", Email: "user@example.com", Role: auth.RoleUser})
	agentID := insertAgentHTTPFixture(t, database, "user-1", agents.StatusRunning)
	if _, err := database.Exec(`UPDATE agents SET runtime_id = 'runtime-1' WHERE id = ?`, agentID); err != nil {
		t.Fatalf("seed runtime id: %v", err)
	}
	insertRuntimeJobHTTPFixture(t, database, "job-active", agentID, jobs.TypeRestartRuntime, jobs.StatusQueued)

	recorder := doJSON(t, router, http.MethodPost, "/api/agents/"+agentID+"/runtime-jobs", `{"type":"restart_runtime"}`, userCookie)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("create runtime job status = %d, want 409, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestCreateRuntimeJobRejectsUnavailableRuntime(t *testing.T) {
	router, manager, database, _ := newAgentTestRouter(t)
	userCookie := sessionCookieFor(t, manager, auth.User{ID: "user-1", Email: "user@example.com", Role: auth.RoleUser})
	agentID := insertAgentHTTPFixture(t, database, "user-1", agents.StatusCreating)

	recorder := doJSON(t, router, http.MethodPost, "/api/agents/"+agentID+"/runtime-jobs", `{"type":"restart_runtime"}`, userCookie)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("create runtime job status = %d, want 409, body = %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("runtime_unavailable")) {
		t.Fatalf("unexpected response body = %s", recorder.Body.String())
	}
}

func TestCreateAgentRejectsNonPublishedTemplate(t *testing.T) {
	router, manager, database, _ := newAgentTestRouter(t)
	userCookie := sessionCookieFor(t, manager, auth.User{ID: "user-1", Email: "user@example.com", Role: auth.RoleUser})

	for _, status := range []string{"draft", "archived"} {
		templateID := "template-" + status
		_, err := database.Exec(`
			INSERT INTO agent_templates (
				id, name, description, status, version, template_path, content_checksum,
				soul_md_path, user_md_path, soul_content, user_content, skills_path, created_by
			) VALUES (?, 'Hidden template', '', ?, 1, '/tmp/hidden', 'checksum', '/tmp/hidden/SOUL.md',
				'/tmp/hidden/USER.md', '', '', '/tmp/hidden/skills', 'admin-1');
		`, templateID, status)
		if err != nil {
			t.Fatalf("insert %s template: %v", status, err)
		}

		recorder := doJSON(t, router, http.MethodPost, "/api/agents", `{"templateId":"`+templateID+`","name":"Support Agent"}`, userCookie)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("create with %s template status = %d, want 404, body = %s", status, recorder.Code, recorder.Body.String())
		}
	}
}

func newAgentTestRouter(t *testing.T) (http.Handler, *auth.SessionManager, *sql.DB, string) {
	t.Helper()

	database := newAgentHTTPTestDB(t)
	manager := auth.NewSessionManager("test-secret", false)
	agentRepository := agents.NewRepository(database)
	jobRepository := jobs.NewRuntimeRepository(database)
	dataDir := testDataDir(t)
	router := NewRouter(Dependencies{
		AuthRepository:       auth.NewRepository(database),
		SessionManager:       manager,
		AgentService:         agents.NewService(database, agentRepository, jobRepository, nil, dataDir),
		RuntimeJobRepository: jobRepository,
	})
	return router, manager, database, dataDir
}

func testDataDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "var")
}

func newAgentHTTPTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite", "file:agent-http-test-"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	_, err = database.Exec(`
		PRAGMA foreign_keys = ON;
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		CREATE TABLE agent_templates (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
			version INTEGER NOT NULL DEFAULT 1,
			template_path TEXT NOT NULL,
			content_checksum TEXT NOT NULL,
			soul_md_path TEXT NOT NULL DEFAULT '',
			user_md_path TEXT NOT NULL DEFAULT '',
			soul_content TEXT NOT NULL DEFAULT '',
			user_content TEXT NOT NULL DEFAULT '',
			skills_path TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			published_at TEXT,
			FOREIGN KEY (created_by) REFERENCES users(id)
		);
		CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			owner_user_id TEXT NOT NULL,
			template_id TEXT NOT NULL,
			template_version INTEGER NOT NULL,
			name TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'creating' CHECK (status IN ('creating', 'provisioning', 'starting', 'running', 'stopped', 'error')),
			runtime_id TEXT NOT NULL DEFAULT '',
			hermes_home_path TEXT NOT NULL UNIQUE,
			last_error_code TEXT NOT NULL DEFAULT '',
			last_error_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (owner_user_id) REFERENCES users(id),
			FOREIGN KEY (template_id) REFERENCES agent_templates(id)
		);
		CREATE TABLE runtime_jobs (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			type TEXT NOT NULL CHECK (type IN ('provision_agent', 'start_runtime', 'stop_runtime', 'restart_runtime')),
			status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
			priority INTEGER NOT NULL DEFAULT 0,
			attempt_count INTEGER NOT NULL DEFAULT 0,
			max_attempts INTEGER NOT NULL DEFAULT 3,
			locked_by TEXT NOT NULL DEFAULT '',
			locked_until TEXT,
			idempotency_key TEXT NOT NULL DEFAULT '',
			payload_json TEXT NOT NULL DEFAULT '{}',
			result_json TEXT NOT NULL DEFAULT '{}',
			last_error_code TEXT NOT NULL DEFAULT '',
			last_error_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			started_at TEXT,
			finished_at TEXT,
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		);
		CREATE UNIQUE INDEX idx_runtime_jobs_one_active
		ON runtime_jobs(agent_id)
		WHERE status IN ('queued', 'running');
		INSERT INTO users (id, email, password_hash, role)
		VALUES ('admin-1', 'admin@example.com', 'unused', 'admin'),
		       ('user-1', 'user@example.com', 'unused', 'user'),
		       ('user-2', 'user2@example.com', 'unused', 'user');
		INSERT INTO agent_templates (
			id, name, description, status, version, template_path, content_checksum,
			soul_md_path, user_md_path, soul_content, user_content, skills_path, created_by
		) VALUES (
			'template-1', 'Support', 'Published template', 'published', 2,
			'/tmp/template-1', 'checksum', '/tmp/template-1/SOUL.md',
			'/tmp/template-1/USER.md', '', '', '/tmp/template-1/skills', 'admin-1'
		);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}

	return database
}

func insertAgentHTTPFixture(t *testing.T, database *sql.DB, ownerUserID string, status agents.Status) string {
	t.Helper()

	agentID := "agent-" + ownerUserID
	_, err := database.Exec(`
		INSERT INTO agents (
			id, owner_user_id, template_id, template_version, name, status, hermes_home_path
		) VALUES (?, ?, 'template-1', 2, 'Fixture Agent', ?, ?);
	`, agentID, ownerUserID, status, "/tmp/"+agentID)
	if err != nil {
		t.Fatalf("insert agent fixture: %v", err)
	}
	return agentID
}

func insertRuntimeJobHTTPFixture(t *testing.T, database *sql.DB, jobID, agentID string, jobType jobs.Type, status jobs.Status) {
	t.Helper()

	_, err := database.Exec(`
		INSERT INTO runtime_jobs (id, agent_id, type, status)
		VALUES (?, ?, ?, ?);
	`, jobID, agentID, jobType, status)
	if err != nil {
		t.Fatalf("insert runtime job fixture: %v", err)
	}
}

func decodeAgentResponse(t *testing.T, body []byte) struct {
	Agent agents.Agent `json:"agent"`
} {
	t.Helper()
	var response struct {
		Agent agents.Agent `json:"agent"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("unmarshal agent response %q: %v", body, err)
	}
	return response
}

func decodeRuntimeResponse(t *testing.T, body []byte) struct {
	Runtime agentRuntimeDTO `json:"runtime"`
} {
	t.Helper()
	var response struct {
		Runtime agentRuntimeDTO `json:"runtime"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("unmarshal runtime response %q: %v", body, err)
	}
	return response
}

func decodeRuntimeJobResponse(t *testing.T, body []byte) struct {
	Job jobs.RuntimeJob `json:"job"`
} {
	t.Helper()
	var response struct {
		Job jobs.RuntimeJob `json:"job"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("unmarshal runtime job response %q: %v", body, err)
	}
	return response
}
