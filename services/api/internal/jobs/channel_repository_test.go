package jobs

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestChannelRepositoryCreateQueuedRejectsSecondActiveJobForChannel(t *testing.T) {
	database := newChannelJobsTestDB(t)
	repository := NewChannelRepository(database)
	ctx := context.Background()

	first, err := repository.CreateQueued(ctx, ChannelJob{
		ID:             "job-1",
		AgentChannelID: "channel-1",
		Type:           TypeConnectWeixin,
	})
	if err != nil {
		t.Fatalf("CreateQueued first returned error: %v", err)
	}
	if first.Status != StatusQueued {
		t.Fatalf("first job status = %s, want %s", first.Status, StatusQueued)
	}

	_, err = repository.CreateQueued(ctx, ChannelJob{
		ID:             "job-2",
		AgentChannelID: "channel-1",
		Type:           TypeDisconnectWeixin,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("CreateQueued second error = %v, want ErrConflict", err)
	}
}

func TestChannelRepositoryCreateOrReuseConnectJobReturnsCurrentActiveSession(t *testing.T) {
	database := newChannelJobsTestDB(t)
	repository := NewChannelRepository(database)
	ctx := context.Background()

	session1, job1, created, err := repository.CreateOrReuseConnectJob(ctx, "channel-1", time.Now().Add(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateOrReuseConnectJob first returned error: %v", err)
	}
	if !created || session1.Status != "pending" || job1.Type != TypeConnectWeixin {
		t.Fatalf("first session/job = %#v / %#v", session1, job1)
	}

	session2, job2, created, err := repository.CreateOrReuseConnectJob(ctx, "channel-1", time.Now().Add(10*time.Minute))
	if err != nil {
		t.Fatalf("CreateOrReuseConnectJob second returned error: %v", err)
	}
	if created {
		t.Fatal("second CreateOrReuseConnectJob created = true, want false")
	}
	if session2.ID != session1.ID || job2.ID != job1.ID {
		t.Fatalf("reused session/job = %#v / %#v, want %#v / %#v", session2, job2, session1, job1)
	}
}

func newChannelJobsTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite", "file:channel-jobs-test-"+t.Name()+"?mode=memory&cache=shared")
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
		CREATE TABLE agent_channels (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			channel_type TEXT NOT NULL DEFAULT 'weixin' CHECK (channel_type IN ('weixin')),
			status TEXT NOT NULL DEFAULT 'not_configured' CHECK (status IN ('not_configured', 'qr_pending', 'connected', 'error', 'disconnected')),
			external_account_id TEXT NOT NULL DEFAULT '',
			last_error_code TEXT NOT NULL DEFAULT '',
			last_error_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		);
		CREATE TABLE channel_pairing_sessions (
			id TEXT PRIMARY KEY,
			agent_channel_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'connected', 'expired', 'failed')),
			qr_payload TEXT NOT NULL DEFAULT '',
			qr_image_path TEXT NOT NULL DEFAULT '',
			expires_at TEXT NOT NULL,
			attempt_count INTEGER NOT NULL DEFAULT 0,
			last_error_code TEXT NOT NULL DEFAULT '',
			last_error_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (agent_channel_id) REFERENCES agent_channels(id) ON DELETE CASCADE
		);
		CREATE TABLE channel_jobs (
			id TEXT PRIMARY KEY,
			agent_channel_id TEXT NOT NULL,
			pairing_session_id TEXT,
			type TEXT NOT NULL CHECK (type IN ('connect_weixin', 'disconnect_weixin', 'refresh_weixin_pairing')),
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
			FOREIGN KEY (agent_channel_id) REFERENCES agent_channels(id) ON DELETE CASCADE,
			FOREIGN KEY (pairing_session_id) REFERENCES channel_pairing_sessions(id) ON DELETE SET NULL
		);
		CREATE UNIQUE INDEX idx_channel_jobs_one_active
		ON channel_jobs(agent_channel_id)
		WHERE status IN ('queued', 'running');
		CREATE UNIQUE INDEX idx_pairing_one_active
		ON channel_pairing_sessions(agent_channel_id)
		WHERE status = 'pending';
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
		INSERT INTO agents (
			id, owner_user_id, template_id, template_version, name, status, hermes_home_path
		) VALUES (
			'agent-1', 'user-1', 'template-1', 1, 'Fixture Agent', 'running', '/tmp/agent-1'
		);
		INSERT INTO agent_channels (id, agent_id, channel_type, status)
		VALUES ('channel-1', 'agent-1', 'weixin', 'not_configured');
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return database
}
