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
		INSERT INTO agents (id, user_id, template_id, name, hermes_home, status)
		VALUES ('agent_missing_fk', 'missing_user', 'missing_template', 'Bad Agent', '/tmp/agent', 'provisioning');
	`)
	if !isConstraintError(err) {
		t.Fatalf("insert orphan agent error = %v, want foreign key constraint", err)
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
