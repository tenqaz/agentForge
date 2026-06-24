package db

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAppliesSQLitePragmas(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "agentforge.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	var journalMode string
	if err := database.QueryRowContext(ctx, "PRAGMA journal_mode;").Scan(&journalMode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if strings.ToLower(journalMode) != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}

	var foreignKeys int
	if err := database.QueryRowContext(ctx, "PRAGMA foreign_keys;").Scan(&foreignKeys); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}

	var busyTimeout int
	if err := database.QueryRowContext(ctx, "PRAGMA busy_timeout;").Scan(&busyTimeout); err != nil {
		t.Fatalf("query busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("busy_timeout = %d, want 5000", busyTimeout)
	}
}

func TestMigrateIsIdempotentAndEnforcesForeignKeys(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "agentforge.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	migrationsDir := filepath.Join("..", "..", "migrations")
	if err := Migrate(ctx, database, migrationsDir); err != nil {
		t.Fatalf("first Migrate() error = %v", err)
	}
	if err := Migrate(ctx, database, migrationsDir); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO agents (id, owner_user_id, template_id, template_version, name, status, hermes_home_path)
		VALUES ('agent_missing_fk', 'missing_user', 'missing_template', 1, 'Bad Agent', 'creating', '/tmp/agent');
	`)
	if !isConstraintError(err) {
		t.Fatalf("insert orphan agent error = %v, want foreign key constraint", err)
	}
}

func TestMigrateEnforcesSpecStatusConstraints(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "agentforge.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	if err := Migrate(ctx, database, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	insertSpecFixture(t, ctx, database)

	if _, err := database.ExecContext(ctx, `
		INSERT INTO agents (id, owner_user_id, template_id, template_version, name, status, hermes_home_path)
		VALUES ('agent_valid_creating', 'user_1', 'template_1', 1, 'Valid Agent', 'creating', '/tmp/agent_valid_creating');
	`); err != nil {
		t.Fatalf("insert agent with status creating: %v", err)
	}
	_, err = database.ExecContext(ctx, `
		INSERT INTO agents (id, owner_user_id, template_id, template_version, name, status, hermes_home_path)
		VALUES ('agent_invalid_failed', 'user_1', 'template_1', 1, 'Invalid Agent', 'failed', '/tmp/agent_invalid_failed');
	`)
	if !isConstraintError(err) {
		t.Fatalf("insert agent with status failed error = %v, want constraint", err)
	}

	if _, err := database.ExecContext(ctx, `
		INSERT INTO agent_channels (id, agent_id, channel_type, status)
		VALUES ('channel_valid_qr', 'agent_1', 'weixin', 'qr_pending');
	`); err != nil {
		t.Fatalf("insert channel with status qr_pending: %v", err)
	}
	_, err = database.ExecContext(ctx, `
		INSERT INTO agent_channels (id, agent_id, channel_type, status)
		VALUES ('channel_invalid_pairing', 'agent_1', 'weixin', 'pairing');
	`)
	if !isConstraintError(err) {
		t.Fatalf("insert channel with status pairing error = %v, want constraint", err)
	}

	if _, err := database.ExecContext(ctx, `
		INSERT INTO channel_jobs (id, agent_channel_id, pairing_session_id, type, status)
		VALUES ('channel_job_valid_connect', 'channel_1', 'pairing_1', 'connect_weixin', 'queued');
	`); err != nil {
		t.Fatalf("insert channel job with type connect_weixin: %v", err)
	}
	_, err = database.ExecContext(ctx, `
		INSERT INTO channel_jobs (id, agent_channel_id, pairing_session_id, type, status)
		VALUES ('channel_job_invalid_pair', 'channel_1', 'pairing_1', 'pair_channel', 'queued');
	`)
	if !isConstraintError(err) {
		t.Fatalf("insert channel job with type pair_channel error = %v, want constraint", err)
	}
}

func insertSpecFixture(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()

	statements := []string{
		`INSERT INTO users (id, email, password_hash, role)
		 VALUES ('user_1', 'user@example.test', 'hash', 'user');`,
		`INSERT INTO agent_templates (
			id, name, description, status, version, template_path, content_checksum,
			soul_content, user_content, skills_path, created_by
		 )
		 VALUES (
			'template_1', 'Template', 'Description', 'published', 1, '/tmp/templates/template_1',
			'checksum',
			'', '', '/tmp/templates/template_1/skills', 'user_1'
		 );`,
		`INSERT INTO agents (id, owner_user_id, template_id, template_version, name, status, hermes_home_path)
		 VALUES ('agent_1', 'user_1', 'template_1', 1, 'Agent', 'running', '/tmp/agent_1');`,
		`INSERT INTO agent_channels (id, agent_id, channel_type, status)
		 VALUES ('channel_1', 'agent_1', 'weixin', 'not_configured');`,
		`INSERT INTO channel_pairing_sessions (id, agent_channel_id, status, qr_payload, expires_at)
		 VALUES ('pairing_1', 'channel_1', 'pending', 'payload', datetime('now', '+5 minutes'));`,
	}
	for _, statement := range statements {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			t.Fatalf("insert fixture: %v\n%s", err, statement)
		}
	}
}

func isConstraintError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "constraint") || strings.Contains(msg, "foreign key")
}
