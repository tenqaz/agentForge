package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, sqlitePath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(sqlitePath), 0o755); err != nil {
		return nil, err
	}

	database, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(1)

	if err := applyPragmas(ctx, database); err != nil {
		_ = database.Close()
		return nil, err
	}
	return database, nil
}

func applyPragmas(ctx context.Context, database *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA foreign_keys=ON;",
	}
	for _, pragma := range pragmas {
		if _, err := database.ExecContext(ctx, pragma); err != nil {
			return err
		}
	}
	return nil
}
