package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsDotEnvAndDerivesPaths(t *testing.T) {
	t.Setenv("AGENTFORGE_HTTP_ADDR", "")
	t.Setenv("AGENTFORGE_PUBLIC_BASE_URL", "")
	t.Setenv("AGENTFORGE_DATA_DIR", "")
	t.Setenv("AGENTFORGE_SESSION_SECRET", "")
	t.Setenv("AGENTFORGE_HERMES_IMAGE", "")
	t.Setenv("AGENTFORGE_HERMES_MEMORY", "")
	t.Setenv("AGENTFORGE_HERMES_CPUS", "")
	t.Setenv("AGENTFORGE_DOCKER_BIN", "podman")

	dir := t.TempDir()
	t.Chdir(dir)

	dotEnv := []byte("AGENTFORGE_HTTP_ADDR=:9090\n" +
		"AGENTFORGE_PUBLIC_BASE_URL=http://127.0.0.1:9090\n" +
		"AGENTFORGE_DATA_DIR=relative-data\n" +
		"AGENTFORGE_SESSION_SECRET=test-secret\n" +
		"AGENTFORGE_HERMES_IMAGE=example/hermes:test\n" +
		"AGENTFORGE_HERMES_MEMORY=1g\n" +
		"AGENTFORGE_HERMES_CPUS=1.5\n")
	if err := os.WriteFile(".env", dotEnv, 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	wantDataDir := filepath.Join(dir, "relative-data")
	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("HTTPAddr = %q, want :9090", cfg.HTTPAddr)
	}
	if cfg.PublicBaseURL != "http://127.0.0.1:9090" {
		t.Fatalf("PublicBaseURL = %q", cfg.PublicBaseURL)
	}
	if cfg.DataDir != wantDataDir {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, wantDataDir)
	}
	if cfg.SQLitePath != filepath.Join(wantDataDir, "agentforge.db") {
		t.Fatalf("SQLitePath = %q", cfg.SQLitePath)
	}
	if cfg.SessionSecret != "test-secret" {
		t.Fatalf("SessionSecret = %q", cfg.SessionSecret)
	}
	if cfg.HermesImage != "example/hermes:test" {
		t.Fatalf("HermesImage = %q", cfg.HermesImage)
	}
	if cfg.HermesMemory != "1g" {
		t.Fatalf("HermesMemory = %q", cfg.HermesMemory)
	}
	if cfg.HermesCPUs != "1.5" {
		t.Fatalf("HermesCPUs = %q", cfg.HermesCPUs)
	}
	if cfg.DockerBin != "docker" {
		t.Fatalf("DockerBin = %q, want docker", cfg.DockerBin)
	}
}

func TestLoadUsesDefaultsWithoutDotEnv(t *testing.T) {
	t.Setenv("AGENTFORGE_HTTP_ADDR", "")
	t.Setenv("AGENTFORGE_PUBLIC_BASE_URL", "")
	t.Setenv("AGENTFORGE_DATA_DIR", "")
	t.Setenv("AGENTFORGE_SESSION_SECRET", "")
	t.Setenv("AGENTFORGE_HERMES_IMAGE", "")
	t.Setenv("AGENTFORGE_HERMES_MEMORY", "")
	t.Setenv("AGENTFORGE_HERMES_CPUS", "")

	dir := t.TempDir()
	t.Chdir(dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	wantDataDir, err := filepath.Abs(filepath.Join(dir, "..", "..", "var"))
	if err != nil {
		t.Fatalf("abs default data dir: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.PublicBaseURL != "http://localhost:8080" {
		t.Fatalf("PublicBaseURL = %q", cfg.PublicBaseURL)
	}
	if cfg.DataDir != wantDataDir {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, wantDataDir)
	}
	if cfg.SQLitePath != filepath.Join(wantDataDir, "agentforge.db") {
		t.Fatalf("SQLitePath = %q", cfg.SQLitePath)
	}
	if cfg.SessionSecret != "dev-change-me" {
		t.Fatalf("SessionSecret = %q", cfg.SessionSecret)
	}
	if cfg.HermesImage != "nousresearch/hermes-agent:v2026.6.5" {
		t.Fatalf("HermesImage = %q", cfg.HermesImage)
	}
	if cfg.HermesMemory != "500m" {
		t.Fatalf("HermesMemory = %q", cfg.HermesMemory)
	}
	if cfg.HermesCPUs != "0.5" {
		t.Fatalf("HermesCPUs = %q", cfg.HermesCPUs)
	}
}
