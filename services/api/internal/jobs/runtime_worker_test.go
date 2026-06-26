package jobs_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentforge.local/services/api/internal/agents"
	"agentforge.local/services/api/internal/jobs"
	"agentforge.local/services/api/internal/runtime"
	"agentforge.local/services/api/internal/templates"

	_ "modernc.org/sqlite"
)

func TestRuntimeWorkerProvisionAgentTransitionsToRunningAndRecordsEvents(t *testing.T) {
	database := newRuntimeWorkerTestDB(t)
	ctx := context.Background()
	homeRoot := t.TempDir()
	template := seedWorkerTemplateFiles(t, homeRoot)
	agentID := insertRuntimeWorkerAgent(t, database, homeRoot, agents.StatusCreating)
	jobID := insertRuntimeWorkerJob(t, database, agentID, jobs.TypeProvisionAgent)

	worker := jobs.NewRuntimeWorker(jobs.RuntimeWorkerDependencies{
		Database:       database,
		RuntimeJobs:    jobs.NewRuntimeRepository(database),
		HomeBuilder:    runtime.NewHomeBuilder(),
		Runner:         &stubRunner{},
		TemplateLoader: stubTemplateLoader{template: template},
		Provider: runtime.ProviderConfig{
			DefaultModel: "deepseek-v4-flash",
			Provider:     "custom",
			BaseURL:      "https://api.deepseek.com",
			APIKey:       "secret-api-key",
			APIMode:      "chat_completions",
		},
		HermesImage:  "nousresearch/hermes-agent:v2026.6.5",
		HermesMemory: "500m",
		HermesCPUs:   "0.5",
	})

	if err := worker.ProcessJob(ctx, jobID); err != nil {
		t.Fatalf("ProcessJob returned error: %v", err)
	}

	agent, err := agents.NewRepository(database).Get(ctx, agentID)
	if err != nil {
		t.Fatalf("Get agent: %v", err)
	}
	if agent.Status != agents.StatusRunning {
		t.Fatalf("agent status = %s, want %s", agent.Status, agents.StatusRunning)
	}
	if agent.RuntimeID != runtime.DefaultContainerName(agentID) {
		t.Fatalf("agent runtime id = %q, want %q", agent.RuntimeID, runtime.DefaultContainerName(agentID))
	}

	job, err := jobs.NewRuntimeRepository(database).GetByID(ctx, agentID, jobID)
	if err != nil {
		t.Fatalf("Get job: %v", err)
	}
	if job.Status != jobs.StatusSucceeded {
		t.Fatalf("job status = %s, want %s", job.Status, jobs.StatusSucceeded)
	}

	events := listRuntimeEvents(t, database, agentID)
	if got := strings.Join(events, ","); got != "provisioning,starting,running" {
		t.Fatalf("event sequence = %q, want provisioning,starting,running", got)
	}

	mustReadContains(t, filepath.Join(homeRoot, "agents", agentID, "hermes-home", "SOUL.md"), "Soul contents")
	mustReadContains(t, filepath.Join(homeRoot, "agents", agentID, "hermes-home", "memories", "USER.md"), "User memory")
}

func TestRuntimeWorkerProvisionAgentNoopsWhenAgentAlreadyRunning(t *testing.T) {
	database := newRuntimeWorkerTestDB(t)
	ctx := context.Background()
	homeRoot := t.TempDir()
	agentID := insertRuntimeWorkerAgent(t, database, homeRoot, agents.StatusRunning)
	if _, err := database.ExecContext(ctx, `
		UPDATE agents
		SET runtime_id = ?
		WHERE id = ?;
	`, runtime.DefaultContainerName(agentID), agentID); err != nil {
		t.Fatalf("seed runtime id: %v", err)
	}
	jobID := insertRuntimeWorkerJob(t, database, agentID, jobs.TypeProvisionAgent)

	worker := jobs.NewRuntimeWorker(jobs.RuntimeWorkerDependencies{
		Database:    database,
		RuntimeJobs: jobs.NewRuntimeRepository(database),
		HomeBuilder: runtime.NewHomeBuilder(),
		Runner: &stubRunner{
			status: runtime.ContainerStatus{Exists: true, Running: true, Status: "running"},
		},
		HermesImage:  "nousresearch/hermes-agent:v2026.6.5",
		HermesMemory: "500m",
		HermesCPUs:   "0.5",
	})

	if err := worker.ProcessJob(ctx, jobID); err != nil {
		t.Fatalf("ProcessJob returned error: %v", err)
	}

	agent, err := agents.NewRepository(database).Get(ctx, agentID)
	if err != nil {
		t.Fatalf("Get agent: %v", err)
	}
	if agent.Status != agents.StatusRunning {
		t.Fatalf("agent status = %s, want %s", agent.Status, agents.StatusRunning)
	}

	job, err := jobs.NewRuntimeRepository(database).GetByID(ctx, agentID, jobID)
	if err != nil {
		t.Fatalf("Get job: %v", err)
	}
	if job.Status != jobs.StatusSucceeded {
		t.Fatalf("job status = %s, want %s", job.Status, jobs.StatusSucceeded)
	}

	events := listRuntimeEvents(t, database, agentID)
	if len(events) != 0 {
		t.Fatalf("unexpected runtime events = %#v", events)
	}
}

func TestRuntimeWorkerProvisionAgentPreservesExistingConnectedEnv(t *testing.T) {
	database := newRuntimeWorkerTestDB(t)
	ctx := context.Background()
	homeRoot := t.TempDir()
	template := seedWorkerTemplateFiles(t, homeRoot)
	agentID := insertRuntimeWorkerAgent(t, database, homeRoot, agents.StatusCreating)
	jobID := insertRuntimeWorkerJob(t, database, agentID, jobs.TypeProvisionAgent)
	homePath := filepath.Join(homeRoot, "agents", agentID, "hermes-home")
	if err := os.MkdirAll(homePath, 0o755); err != nil {
		t.Fatalf("mkdir hermes home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homePath, ".env"), []byte("WEIXIN_TOKEN=connected-token\nWEIXIN_ALLOWED_USERS=user-1\n"), 0o644); err != nil {
		t.Fatalf("seed connected env: %v", err)
	}

	worker := jobs.NewRuntimeWorker(jobs.RuntimeWorkerDependencies{
		Database:       database,
		RuntimeJobs:    jobs.NewRuntimeRepository(database),
		HomeBuilder:    runtime.NewHomeBuilder(),
		Runner:         &stubRunner{},
		TemplateLoader: stubTemplateLoader{template: template},
		Provider: runtime.ProviderConfig{
			DefaultModel: "deepseek-v4-flash",
			Provider:     "custom",
			BaseURL:      "https://api.deepseek.com",
			APIKey:       "secret-api-key",
			APIMode:      "chat_completions",
		},
		HermesImage:  "nousresearch/hermes-agent:v2026.6.5",
		HermesMemory: "500m",
		HermesCPUs:   "0.5",
	})

	if err := worker.ProcessJob(ctx, jobID); err != nil {
		t.Fatalf("ProcessJob returned error: %v", err)
	}

	mustReadContains(t, filepath.Join(homePath, ".env"), "WEIXIN_TOKEN=connected-token")
	mustReadContains(t, filepath.Join(homePath, ".env"), "WEIXIN_ALLOWED_USERS=user-1")
}

func TestRuntimeWorkerProvisionAgentRecordsCopyTemplateFailureAndPreservesSessions(t *testing.T) {
	database := newRuntimeWorkerTestDB(t)
	ctx := context.Background()
	homeRoot := t.TempDir()
	template := templates.Template{
		ID:           "template-1",
		Version:      3,
		TemplatePath: filepath.Join(homeRoot, "templates", "template-1", "versions", "3"),
		SkillsPath:   filepath.Join(homeRoot, "templates", "template-1", "versions", "3", "skills"),
	}
	agentID := insertRuntimeWorkerAgent(t, database, homeRoot, agents.StatusCreating)
	jobID := insertRuntimeWorkerJob(t, database, agentID, jobs.TypeProvisionAgent)
	homePath := filepath.Join(homeRoot, "agents", agentID, "hermes-home")
	if err := os.MkdirAll(filepath.Join(homePath, "sessions"), 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionFile := filepath.Join(homePath, "sessions", "sticky.session")
	if err := os.WriteFile(sessionFile, []byte("keep"), 0o644); err != nil {
		t.Fatalf("write sticky session: %v", err)
	}

	worker := jobs.NewRuntimeWorker(jobs.RuntimeWorkerDependencies{
		Database:       database,
		RuntimeJobs:    jobs.NewRuntimeRepository(database),
		HomeBuilder:    runtime.NewHomeBuilder(),
		Runner:         &stubRunner{},
		TemplateLoader: stubTemplateLoader{template: template},
		Provider: runtime.ProviderConfig{
			DefaultModel: "deepseek-v4-flash",
			Provider:     "custom",
			BaseURL:      "https://api.deepseek.com",
			APIKey:       "secret-api-key",
			APIMode:      "chat_completions",
		},
		HermesImage:  "nousresearch/hermes-agent:v2026.6.5",
		HermesMemory: "500m",
		HermesCPUs:   "0.5",
	})

	err := worker.ProcessJob(ctx, jobID)
	if err == nil {
		t.Fatal("ProcessJob error = nil, want error")
	}

	agent, getErr := agents.NewRepository(database).Get(ctx, agentID)
	if getErr != nil {
		t.Fatalf("Get agent: %v", getErr)
	}
	if agent.Status != agents.StatusError || agent.LastErrorCode != runtime.ErrCodeCopyTemplateFailed {
		t.Fatalf("agent after failure = %#v", agent)
	}

	job, getErr := jobs.NewRuntimeRepository(database).GetByID(ctx, agentID, jobID)
	if getErr != nil {
		t.Fatalf("Get job: %v", getErr)
	}
	if job.Status != jobs.StatusFailed || job.LastErrorCode != runtime.ErrCodeCopyTemplateFailed {
		t.Fatalf("job after failure = %#v", job)
	}

	events := listRuntimeEvents(t, database, agentID)
	if got := strings.Join(events, ","); got != "provisioning,copy_template_failed" {
		t.Fatalf("event sequence = %q, want provisioning,copy_template_failed", got)
	}
	mustReadContains(t, sessionFile, "keep")
}

func TestRuntimeWorkerProvisionAgentRecordsConfigWriteFailed(t *testing.T) {
	database := newRuntimeWorkerTestDB(t)
	ctx := context.Background()
	homeRoot := t.TempDir()
	template := seedWorkerTemplateFiles(t, homeRoot)
	agentID := insertRuntimeWorkerAgent(t, database, homeRoot, agents.StatusCreating)
	jobID := insertRuntimeWorkerJob(t, database, agentID, jobs.TypeProvisionAgent)

	worker := jobs.NewRuntimeWorker(jobs.RuntimeWorkerDependencies{
		Database:       database,
		RuntimeJobs:    jobs.NewRuntimeRepository(database),
		HomeBuilder:    stubHomeBuilder{err: runtime.NewProvisionError(runtime.ErrCodeConfigWriteFailed, "config write failed", errors.New("disk full"))},
		Runner:         &stubRunner{},
		TemplateLoader: stubTemplateLoader{template: template},
		Provider: runtime.ProviderConfig{
			DefaultModel: "deepseek-v4-flash",
			Provider:     "custom",
			BaseURL:      "https://api.deepseek.com",
			APIKey:       "secret-api-key",
			APIMode:      "chat_completions",
		},
		HermesImage:  "nousresearch/hermes-agent:v2026.6.5",
		HermesMemory: "500m",
		HermesCPUs:   "0.5",
	})

	err := worker.ProcessJob(ctx, jobID)
	if err == nil {
		t.Fatal("ProcessJob error = nil, want error")
	}

	events := listRuntimeEvents(t, database, agentID)
	if got := strings.Join(events, ","); got != "provisioning,config_write_failed" {
		t.Fatalf("event sequence = %q, want provisioning,config_write_failed", got)
	}
}

func TestRuntimeWorkerProvisionAgentRecordsContainerStartFailed(t *testing.T) {
	database := newRuntimeWorkerTestDB(t)
	ctx := context.Background()
	homeRoot := t.TempDir()
	template := seedWorkerTemplateFiles(t, homeRoot)
	agentID := insertRuntimeWorkerAgent(t, database, homeRoot, agents.StatusCreating)
	jobID := insertRuntimeWorkerJob(t, database, agentID, jobs.TypeProvisionAgent)

	worker := jobs.NewRuntimeWorker(jobs.RuntimeWorkerDependencies{
		Database:       database,
		RuntimeJobs:    jobs.NewRuntimeRepository(database),
		HomeBuilder:    runtime.NewHomeBuilder(),
		Runner:         &stubRunner{ensureErr: errors.New("docker failed")},
		TemplateLoader: stubTemplateLoader{template: template},
		Provider: runtime.ProviderConfig{
			DefaultModel: "deepseek-v4-flash",
			Provider:     "custom",
			BaseURL:      "https://api.deepseek.com",
			APIKey:       "secret-api-key",
			APIMode:      "chat_completions",
		},
		HermesImage:  "nousresearch/hermes-agent:v2026.6.5",
		HermesMemory: "500m",
		HermesCPUs:   "0.5",
	})

	err := worker.ProcessJob(ctx, jobID)
	if err == nil {
		t.Fatal("ProcessJob error = nil, want error")
	}

	events := listRuntimeEvents(t, database, agentID)
	if got := strings.Join(events, ","); got != "provisioning,starting,container_start_failed" {
		t.Fatalf("event sequence = %q, want provisioning,starting,container_start_failed", got)
	}
}

func TestRuntimeWorkerRestartRuntimeRestartsRunningAgent(t *testing.T) {
	database := newRuntimeWorkerTestDB(t)
	ctx := context.Background()
	homeRoot := t.TempDir()
	agentID := insertRuntimeWorkerAgent(t, database, homeRoot, agents.StatusRunning)
	if _, err := database.ExecContext(ctx, `
		UPDATE agents
		SET runtime_id = ?
		WHERE id = ?;
	`, runtime.DefaultContainerName(agentID), agentID); err != nil {
		t.Fatalf("seed runtime id: %v", err)
	}
	jobID := insertRuntimeWorkerJob(t, database, agentID, jobs.TypeRestartRuntime)
	runner := &stubRunner{}

	worker := jobs.NewRuntimeWorker(jobs.RuntimeWorkerDependencies{
		Database:     database,
		RuntimeJobs:  jobs.NewRuntimeRepository(database),
		HomeBuilder:  runtime.NewHomeBuilder(),
		Runner:       runner,
		HermesImage:  "nousresearch/hermes-agent:v2026.6.5",
		HermesMemory: "500m",
		HermesCPUs:   "0.5",
	})

	if err := worker.ProcessJob(ctx, jobID); err != nil {
		t.Fatalf("ProcessJob returned error: %v", err)
	}

	job, err := jobs.NewRuntimeRepository(database).GetByID(ctx, agentID, jobID)
	if err != nil {
		t.Fatalf("Get job: %v", err)
	}
	if job.Status != jobs.StatusSucceeded {
		t.Fatalf("job status = %s, want %s", job.Status, jobs.StatusSucceeded)
	}
	if runner.stopCount != 1 || runner.ensureCount != 1 {
		t.Fatalf("runner stop/ensure counts = %d/%d, want 1/1", runner.stopCount, runner.ensureCount)
	}
}

type stubRunner struct {
	ensureErr   error
	status      runtime.ContainerStatus
	inspectErr  error
	stopErr     error
	stopCount   int
	ensureCount int
}

func (s *stubRunner) EnsureRunning(_ context.Context, _ runtime.ContainerSpec) error {
	s.ensureCount++
	return s.ensureErr
}

func (s *stubRunner) Stop(_ context.Context, _ string) error {
	s.stopCount++
	return s.stopErr
}

func (s *stubRunner) Remove(_ context.Context, _ string) error {
	return nil
}

func (s *stubRunner) Inspect(_ context.Context, _ string) (runtime.ContainerStatus, error) {
	return s.status, s.inspectErr
}

func (s *stubRunner) Destroy(_ context.Context, _ string) error {
	return nil
}

type stubTemplateLoader struct {
	template templates.Template
	err      error
}

func (s stubTemplateLoader) LoadPublishedTemplate(_ context.Context, _ string, _ int) (templates.Template, error) {
	if s.err != nil {
		return templates.Template{}, s.err
	}
	return s.template, nil
}

type stubHomeBuilder struct {
	err error
}

func (s stubHomeBuilder) Provision(_ context.Context, _ runtime.HomeSpec) (runtime.HomeResult, error) {
	return runtime.HomeResult{}, s.err
}

func seedWorkerTemplateFiles(t *testing.T, root string) templates.Template {
	t.Helper()
	templateRoot := filepath.Join(root, "templates", "template-1", "versions", "3")
	template := templates.Template{
		ID:           "template-1",
		Version:      3,
		TemplatePath: templateRoot,
		SkillsPath:   filepath.Join(templateRoot, "skills"),
		SoulContent:  "Soul contents",
		UserContent:  "User memory",
	}
	if err := os.MkdirAll(filepath.Join(template.SkillsPath, "faq"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(template.SkillsPath, "faq", "SKILL.md"), []byte("# FAQ"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	return template
}

func newRuntimeWorkerTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite", "file:runtime-worker-test-"+t.Name()+"?mode=memory&cache=shared")
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
		CREATE TABLE agent_runtime_events (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			status_before TEXT NOT NULL DEFAULT '',
			status_after TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL DEFAULT '',
			metadata_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
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
		INSERT INTO users (id, email, password_hash, role)
		VALUES ('user-1', 'user@example.com', 'unused', 'user'),
		       ('admin-1', 'admin@example.com', 'unused', 'admin');
		INSERT INTO agent_templates (
			id, name, description, status, version, template_path, content_checksum,
			soul_content, user_content, skills_path, created_by
		) VALUES (
			'template-1', 'Support', 'Published template', 'published', 3,
			'/tmp/template-1', 'checksum', '', '', '/tmp/template-1/skills', 'admin-1'
		);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return database
}

func insertRuntimeWorkerAgent(t *testing.T, database *sql.DB, dataDir string, status agents.Status) string {
	t.Helper()
	agentID := "agent-worker-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-"))
	homePath := filepath.Join(dataDir, "agents", agentID, "hermes-home")
	_, err := database.Exec(`
		INSERT INTO agents (
			id, owner_user_id, template_id, template_version, name, status, hermes_home_path
		) VALUES (?, 'user-1', 'template-1', 3, 'Fixture Agent', ?, ?);
	`, agentID, status, homePath)
	if err != nil {
		t.Fatalf("insert agent fixture: %v", err)
	}
	return agentID
}

func insertRuntimeWorkerJob(t *testing.T, database *sql.DB, agentID string, jobType jobs.Type) string {
	t.Helper()
	jobID := "job-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-"))
	_, err := database.Exec(`
		INSERT INTO runtime_jobs (id, agent_id, type, status, locked_by)
		VALUES (?, ?, ?, 'queued', 'worker-1');
	`, jobID, agentID, jobType)
	if err != nil {
		t.Fatalf("insert runtime job: %v", err)
	}
	return jobID
}

func listRuntimeEvents(t *testing.T, database *sql.DB, agentID string) []string {
	t.Helper()
	rows, err := database.Query(`
		SELECT event_type
		FROM agent_runtime_events
		WHERE agent_id = ?
		ORDER BY created_at ASC, rowid ASC;
	`, agentID)
	if err != nil {
		t.Fatalf("query runtime events: %v", err)
	}
	defer rows.Close()
	var events []string
	for rows.Next() {
		var eventType string
		if err := rows.Scan(&eventType); err != nil {
			t.Fatalf("scan runtime event: %v", err)
		}
		events = append(events, eventType)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate runtime events: %v", err)
	}
	return events
}

func mustReadContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s = %q, want substring %q", path, string(data), want)
	}
}
