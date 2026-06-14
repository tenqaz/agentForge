package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Migrate(ctx context.Context, database *sql.DB, migrationsDir string) error {
	if _, err := database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
	`); err != nil {
		return err
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)

	for _, file := range files {
		applied, err := migrationApplied(ctx, database, file)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := applyMigration(ctx, database, migrationsDir, file); err != nil {
			return err
		}
	}
	return nil
}

func migrationApplied(ctx context.Context, database *sql.DB, version string) (bool, error) {
	var exists int
	err := database.QueryRowContext(ctx, "SELECT 1 FROM schema_migrations WHERE version = ?;", version).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return false, nil
	}
	return false, err
}

func applyMigration(ctx context.Context, database *sql.DB, migrationsDir, file string) error {
	sqlBytes, err := os.ReadFile(filepath.Join(migrationsDir, file))
	if err != nil {
		return err
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES (?);", file); err != nil {
		return err
	}
	return tx.Commit()
}
