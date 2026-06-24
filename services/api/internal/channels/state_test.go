package channels

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

func TestStatusCanTransition(t *testing.T) {
	t.Parallel()

	cases := []struct {
		from Status
		to   Status
		want bool
	}{
		{StatusNotConfigured, StatusQRPending, true},
		{StatusQRPending, StatusConnected, true},
		{StatusQRPending, StatusError, true},
		{StatusQRPending, StatusNotConfigured, true},
		{StatusConnected, StatusDisconnected, true},
		{StatusDisconnected, StatusQRPending, true},
		{StatusError, StatusQRPending, true},
		{StatusConnected, StatusQRPending, false},
		{StatusNotConfigured, StatusConnected, false},
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
	database := newChannelsTestDB(t)
	repository := NewRepository(database)
	ctx := context.Background()
	insertAgentFixture(t, database, "agent-1", "running")
	channelID := insertChannelFixture(t, database, "agent-1", StatusConnected)

	_, err := repository.TransitionStatus(ctx, channelID, StatusQRPending, "", "", "")
	if !errors.Is(err, ErrInvalidStateTransition) {
		t.Fatalf("TransitionStatus error = %v, want ErrInvalidStateTransition", err)
	}
}

func TestServiceEnsureWeixinChannelRequiresRunningAgent(t *testing.T) {
	database := newChannelsTestDB(t)
	service := NewService(database, NewRepository(database))
	ctx := context.Background()
	insertAgentFixture(t, database, "agent-creating", "creating")

	_, err := service.EnsureWeixinChannel(ctx, "agent-creating")
	if !errors.Is(err, ErrAgentNotRunning) {
		t.Fatalf("EnsureWeixinChannel error = %v, want ErrAgentNotRunning", err)
	}
}

func TestServiceEnsureWeixinChannelCreatesRecordForRunningAgent(t *testing.T) {
	database := newChannelsTestDB(t)
	service := NewService(database, NewRepository(database))
	ctx := context.Background()
	insertAgentFixture(t, database, "agent-running", "running")

	channel, err := service.EnsureWeixinChannel(ctx, "agent-running")
	if err != nil {
		t.Fatalf("EnsureWeixinChannel returned error: %v", err)
	}
	if channel.AgentID != "agent-running" || channel.Status != StatusNotConfigured || channel.ChannelType != TypeWeixin {
		t.Fatalf("channel = %#v", channel)
	}
}

func newChannelsTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite", "file:channels-test-"+t.Name()+"?mode=memory&cache=shared")
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
		INSERT INTO users (id, email, password_hash, role)
		VALUES ('user-1', 'user@example.com', 'unused', 'user');
		INSERT INTO agent_templates (
			id, name, description, status, version, template_path, content_checksum,
			soul_content, user_content, skills_path, created_by
		) VALUES (
			'template-1', 'Support', 'Published template', 'published', 1,
			'/tmp/template-1', 'checksum', '', '', '/tmp/template-1/skills', 'user-1'
		);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return database
}

func insertAgentFixture(t *testing.T, database *sql.DB, agentID, status string) {
	t.Helper()
	_, err := database.Exec(`
		INSERT INTO agents (
			id, owner_user_id, template_id, template_version, name, status, hermes_home_path
		) VALUES (?, 'user-1', 'template-1', 1, 'Fixture Agent', ?, ?);
	`, agentID, status, "/tmp/"+agentID)
	if err != nil {
		t.Fatalf("insert agent fixture: %v", err)
	}
}

func insertChannelFixture(t *testing.T, database *sql.DB, agentID string, status Status) string {
	t.Helper()
	channelID := "channel-" + agentID
	_, err := database.Exec(`
		INSERT INTO agent_channels (id, agent_id, channel_type, status)
		VALUES (?, ?, 'weixin', ?);
	`, channelID, agentID, status)
	if err != nil {
		t.Fatalf("insert channel fixture: %v", err)
	}
	return channelID
}
