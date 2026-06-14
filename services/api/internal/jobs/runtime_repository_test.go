package jobs

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestCreateQueuedRejectsSecondActiveJobForAgent(t *testing.T) {
	database := newRuntimeJobsTestDB(t)
	repository := NewRuntimeRepository(database)
	ctx := context.Background()

	first, err := repository.CreateQueued(ctx, RuntimeJob{
		ID:      "job-1",
		AgentID: "agent-1",
		Type:    TypeRestartRuntime,
	})
	if err != nil {
		t.Fatalf("CreateQueued first returned error: %v", err)
	}
	if first.Status != StatusQueued {
		t.Fatalf("first job status = %s, want %s", first.Status, StatusQueued)
	}

	_, err = repository.CreateQueued(ctx, RuntimeJob{
		ID:      "job-2",
		AgentID: "agent-1",
		Type:    TypeStopRuntime,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("CreateQueued second error = %v, want ErrConflict", err)
	}
}

func TestClaimNextQueuedClaimsHighestPriorityOldestJob(t *testing.T) {
	database := newRuntimeJobsTestDB(t)
	repository := NewRuntimeRepository(database)
	ctx := context.Background()

	insertRuntimeJobFixture(t, database, "job-low", "agent-1", TypeRestartRuntime, StatusQueued, 1, "datetime('now')", "NULL")
	insertRuntimeJobFixture(t, database, "job-high-new", "agent-2", TypeRestartRuntime, StatusQueued, 5, "datetime('now', '+1 second')", "NULL")
	insertRuntimeJobFixture(t, database, "job-high-old", "agent-3", TypeRestartRuntime, StatusQueued, 5, "datetime('now')", "NULL")
	insertRuntimeJobFixture(t, database, "job-locked", "agent-4", TypeRestartRuntime, StatusQueued, 10, "datetime('now', '-1 second')", "datetime('now', '+10 minutes')")

	claimed, err := repository.ClaimNextQueued(ctx, "worker-1", time.Now().Add(5*time.Minute))
	if err != nil {
		t.Fatalf("ClaimNextQueued returned error: %v", err)
	}
	if claimed.ID != "job-high-old" {
		t.Fatalf("claimed job id = %s, want job-high-old", claimed.ID)
	}
	if claimed.Status != StatusRunning || claimed.LockedBy != "worker-1" || claimed.StartedAt == nil || claimed.LockedUntil == nil {
		t.Fatalf("claimed job = %#v", claimed)
	}

	stored, err := repository.GetByID(ctx, claimed.AgentID, claimed.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if stored.Status != StatusRunning || stored.LockedBy != "worker-1" {
		t.Fatalf("stored claimed job = %#v", stored)
	}
}

func TestClaimNextQueuedReturnsNotFoundWhenQueueEmpty(t *testing.T) {
	database := newRuntimeJobsTestDB(t)
	repository := NewRuntimeRepository(database)
	ctx := context.Background()

	_, err := repository.ClaimNextQueued(ctx, "worker-1", time.Now().Add(time.Minute))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ClaimNextQueued error = %v, want ErrNotFound", err)
	}
}

func newRuntimeJobsTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite", "file:runtime-jobs-test-"+t.Name()+"?mode=memory&cache=shared")
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
		VALUES ('user-1', 'user@example.com', 'unused', 'user');
		INSERT INTO agent_templates (
			id, name, description, status, version, template_path, content_checksum,
			soul_md_path, user_md_path, skills_path, created_by
		) VALUES (
			'template-1', 'Support', 'Published template', 'published', 1,
			'/tmp/template-1', 'checksum', '/tmp/template-1/SOUL.md',
			'/tmp/template-1/USER.md', '/tmp/template-1/skills', 'user-1'
		);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}

	for _, agentID := range []string{"agent-1", "agent-2", "agent-3", "agent-4"} {
		_, err = database.Exec(`
			INSERT INTO agents (
				id, owner_user_id, template_id, template_version, name, status, hermes_home_path
			) VALUES (?, 'user-1', 'template-1', 1, ?, 'running', ?);
		`, agentID, "Agent "+agentID, "/tmp/"+agentID)
		if err != nil {
			t.Fatalf("insert agent %s: %v", agentID, err)
		}
	}

	return database
}

func insertRuntimeJobFixture(t *testing.T, database *sql.DB, jobID, agentID string, jobType Type, status Status, priority int, createdAtExpr, lockedUntilExpr string) {
	t.Helper()

	if createdAtExpr == "" {
		createdAtExpr = "datetime('now')"
	}
	if lockedUntilExpr == "" {
		lockedUntilExpr = "NULL"
	}
	_, err := database.Exec(`
		INSERT INTO runtime_jobs (
			id, agent_id, type, status, priority, locked_until, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, `+lockedUntilExpr+`, `+createdAtExpr+`, `+createdAtExpr+`);
	`, jobID, agentID, jobType, status, priority)
	if err != nil {
		t.Fatalf("insert runtime job fixture: %v", err)
	}
}
