package agents

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"agentforge.local/services/api/internal/jobs"

	_ "modernc.org/sqlite"
)

func TestStatusCanTransition(t *testing.T) {
	t.Parallel()

	cases := []struct {
		from Status
		to   Status
		want bool
	}{
		{StatusCreating, StatusProvisioning, true},
		{StatusCreating, StatusError, true},
		{StatusCreating, StatusRunning, false},
		{StatusCreating, StatusCreating, false},
		{StatusProvisioning, StatusStarting, true},
		{StatusProvisioning, StatusError, true},
		{StatusProvisioning, StatusStopped, false},
		{StatusProvisioning, StatusProvisioning, false},
		{StatusStarting, StatusRunning, true},
		{StatusStarting, StatusError, true},
		{StatusStarting, StatusProvisioning, false},
		{StatusStarting, StatusStarting, false},
		{StatusRunning, StatusStopped, true},
		{StatusRunning, StatusError, true},
		{StatusRunning, StatusCreating, false},
		{StatusRunning, StatusRunning, false},
		{StatusStopped, StatusStarting, true},
		{StatusStopped, StatusRunning, false},
		{StatusStopped, StatusStopped, false},
		{StatusError, StatusProvisioning, true},
		{StatusError, StatusStarting, true},
		{StatusError, StatusRunning, false},
		{StatusError, StatusError, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.from)+"_to_"+string(tc.to), func(t *testing.T) {
			t.Parallel()
			if got := tc.from.CanTransitionTo(tc.to); got != tc.want {
				t.Fatalf("%s.CanTransitionTo(%s) = %t, want %t", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestRepositoryTransitionStatusRejectsInvalidTransition(t *testing.T) {
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	ctx := context.Background()
	insertAgentFixture(t, database, "agent-1", "user-1", StatusCreating)

	_, err := repository.TransitionStatus(ctx, "agent-1", StatusRunning, "", "", "")
	if !errors.Is(err, ErrInvalidStateTransition) {
		t.Fatalf("TransitionStatus error = %v, want ErrInvalidStateTransition", err)
	}
}

func TestRepositoryTransitionStatusClearsErrorOnRecovery(t *testing.T) {
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	ctx := context.Background()
	insertAgentFixture(t, database, "agent-1", "user-1", StatusError)
	if _, err := database.ExecContext(ctx, `
		UPDATE agents
		SET last_error_code = 'boot_failed', last_error_message = 'runtime crashed'
		WHERE id = 'agent-1';
	`); err != nil {
		t.Fatalf("seed error fields: %v", err)
	}

	agent, err := repository.TransitionStatus(ctx, "agent-1", StatusStarting, "", "", "runtime-1")
	if err != nil {
		t.Fatalf("TransitionStatus returned error: %v", err)
	}
	if agent.Status != StatusStarting || agent.LastErrorCode != "" || agent.LastErrorMessage != "" || agent.RuntimeID != "runtime-1" {
		t.Fatalf("recovered agent = %#v", agent)
	}
}

func TestServiceCreateCreatesAgentAndProvisionJob(t *testing.T) {
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	jobRepository := jobs.NewRuntimeRepository(database)
	service := NewService(database, repository, jobRepository, t.TempDir())
	ctx := context.Background()

	created, err := service.Create(ctx, CreateParams{
		OwnerUserID: "user-1",
		TemplateID:  "template-1",
		Name:        "Support Agent",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.Status != StatusCreating {
		t.Fatalf("created status = %s, want %s", created.Status, StatusCreating)
	}
	if created.TemplateVersion != 3 {
		t.Fatalf("created template version = %d, want 3", created.TemplateVersion)
	}
	expectedHome := filepath.Join(service.dataDir, "agents", created.ID, "hermes-home")
	if created.HermesHomePath != expectedHome {
		t.Fatalf("created hermes home path = %q, want %q", created.HermesHomePath, expectedHome)
	}

	stored, err := repository.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if stored.Name != "Support Agent" || stored.OwnerUserID != "user-1" {
		t.Fatalf("stored agent = %#v", stored)
	}

	runtimeJobs, err := jobRepository.ListByAgent(ctx, created.ID)
	if err != nil {
		t.Fatalf("ListByAgent returned error: %v", err)
	}
	if len(runtimeJobs) != 1 {
		t.Fatalf("runtime job count = %d, want 1", len(runtimeJobs))
	}
	if runtimeJobs[0].Type != jobs.TypeProvisionAgent || runtimeJobs[0].Status != jobs.StatusQueued {
		t.Fatalf("runtime job = %#v", runtimeJobs[0])
	}
}

func TestServiceCreateRejectsNonPublishedTemplates(t *testing.T) {
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	jobRepository := jobs.NewRuntimeRepository(database)
	service := NewService(database, repository, jobRepository, t.TempDir())
	ctx := context.Background()

	for _, status := range []string{"draft", "archived"} {
		if _, err := database.ExecContext(ctx, `
			INSERT INTO agent_templates (
				id, name, description, status, version, template_path, content_checksum,
				soul_md_path, user_md_path, skills_path, created_by
			) VALUES (?, 'Hidden template', '', ?, 1, '/tmp/hidden', 'checksum', '/tmp/hidden/SOUL.md',
				'/tmp/hidden/USER.md', '/tmp/hidden/skills', 'admin-1');
		`, "template-"+status, status); err != nil {
			t.Fatalf("insert %s template: %v", status, err)
		}

		_, err := service.Create(ctx, CreateParams{
			OwnerUserID: "user-1",
			TemplateID:  "template-" + status,
			Name:        "Support Agent",
		})
		if !errors.Is(err, ErrTemplateNotFound) {
			t.Fatalf("Create with %s template error = %v, want ErrTemplateNotFound", status, err)
		}
	}
}

func TestServiceCreateRuntimeJobRejectsUnavailableRuntime(t *testing.T) {
	database := newAgentsTestDB(t)
	repository := NewRepository(database)
	jobRepository := jobs.NewRuntimeRepository(database)
	service := NewService(database, repository, jobRepository, t.TempDir())
	ctx := context.Background()
	insertAgentFixture(t, database, "agent-1", "user-1", StatusCreating)

	if _, err := service.CreateRuntimeJob(ctx, "agent-1", jobs.TypeRestartRuntime); !errors.Is(err, ErrRuntimeUnavailable) {
		t.Fatalf("CreateRuntimeJob without runtime error = %v, want ErrRuntimeUnavailable", err)
	}

	if _, err := database.ExecContext(ctx, `
		UPDATE agents
		SET status = 'running', runtime_id = 'runtime-1'
		WHERE id = 'agent-1';
	`); err != nil {
		t.Fatalf("seed runtime: %v", err)
	}

	job, err := service.CreateRuntimeJob(ctx, "agent-1", jobs.TypeRestartRuntime)
	if err != nil {
		t.Fatalf("CreateRuntimeJob returned error: %v", err)
	}
	if job.Type != jobs.TypeRestartRuntime || job.Status != jobs.StatusQueued {
		t.Fatalf("CreateRuntimeJob result = %#v", job)
	}
}

func newAgentsTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite", "file:agents-test-"+t.Name()+"?mode=memory&cache=shared")
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
			soul_md_path TEXT NOT NULL,
			user_md_path TEXT NOT NULL,
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
		VALUES ('user-1', 'user@example.com', 'unused', 'user'),
		       ('admin-1', 'admin@example.com', 'unused', 'admin');
		INSERT INTO agent_templates (
			id, name, description, status, version, template_path, content_checksum,
			soul_md_path, user_md_path, skills_path, created_by
		) VALUES (
			'template-1', 'Support', 'Published template', 'published', 3,
			'/tmp/template-1', 'checksum', '/tmp/template-1/SOUL.md',
			'/tmp/template-1/USER.md', '/tmp/template-1/skills', 'admin-1'
		);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}

	return database
}

func insertAgentFixture(t *testing.T, database *sql.DB, agentID, ownerUserID string, status Status) {
	t.Helper()

	_, err := database.Exec(`
		INSERT INTO agents (
			id, owner_user_id, template_id, template_version, name, status, hermes_home_path
		) VALUES (?, ?, 'template-1', 3, 'Fixture Agent', ?, ?);
	`, agentID, ownerUserID, status, "/tmp/"+agentID)
	if err != nil {
		t.Fatalf("insert agent fixture: %v", err)
	}
}
