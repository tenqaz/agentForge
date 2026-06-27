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

func TestLoadReadsBrevoSettings(t *testing.T) {
	t.Setenv("AGENTFORGE_BREVO_API_KEY", "test-key")
	t.Setenv("AGENTFORGE_BREVO_SENDER_EMAIL", "noreply@example.com")
	t.Setenv("AGENTFORGE_BREVO_SENDER_NAME", "AgentForge")
	t.Setenv("AGENTFORGE_BREVO_BASE_URL", "")
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
	if cfg.BrevoAPIKey != "test-key" {
		t.Fatalf("BrevoAPIKey = %q", cfg.BrevoAPIKey)
	}
	if cfg.BrevoSenderEmail != "noreply@example.com" {
		t.Fatalf("BrevoSenderEmail = %q", cfg.BrevoSenderEmail)
	}
	if cfg.BrevoSenderName != "AgentForge" {
		t.Fatalf("BrevoSenderName = %q", cfg.BrevoSenderName)
	}
	if cfg.BrevoBaseURL != "https://api.brevo.com" {
		t.Fatalf("BrevoBaseURL = %q, want default", cfg.BrevoBaseURL)
	}
}

func TestLoadReadsTurnstileConfig(t *testing.T) {
	t.Setenv("AGENTFORGE_TURNSTILE_SECRET", "")
	t.Setenv("AGENTFORGE_TURNSTILE_SITEKEY", "")
	t.Setenv("AGENTFORGE_TURNSTILE_VERIFY_URL", "")
	t.Setenv("AGENTFORGE_TURNSTILE_EXPECTED_HOSTNAME", "")

	dir := t.TempDir()
	t.Chdir(dir)
	dotEnv := []byte("AGENTFORGE_PUBLIC_BASE_URL=https://app.example.com\n" +
		"AGENTFORGE_TURNSTILE_SECRET=sec\n" +
		"AGENTFORGE_TURNSTILE_SITEKEY=site\n")
	if err := os.WriteFile(".env", dotEnv, 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.TurnstileSecret != "sec" {
		t.Fatalf("TurnstileSecret = %q, want sec", cfg.TurnstileSecret)
	}
	if cfg.TurnstileSitekey != "site" {
		t.Fatalf("TurnstileSitekey = %q, want site", cfg.TurnstileSitekey)
	}
	if cfg.TurnstileVerifyURL != "https://challenges.cloudflare.com/turnstile/v0/siteverify" {
		t.Fatalf("TurnstileVerifyURL = %q, want default", cfg.TurnstileVerifyURL)
	}
	// 未设 EXPECTED_HOSTNAME → 从 PublicBaseURL 推导 host。
	if cfg.TurnstileExpectedHostname != "app.example.com" {
		t.Fatalf("TurnstileExpectedHostname = %q, want app.example.com", cfg.TurnstileExpectedHostname)
	}
}

func TestLoadTurnstileExpectedHostnameNoneSkipsCheck(t *testing.T) {
	t.Setenv("AGENTFORGE_TURNSTILE_SECRET", "")
	t.Setenv("AGENTFORGE_TURNSTILE_SITEKEY", "")
	t.Setenv("AGENTFORGE_TURNSTILE_VERIFY_URL", "")
	t.Setenv("AGENTFORGE_TURNSTILE_EXPECTED_HOSTNAME", "none")

	dir := t.TempDir()
	t.Chdir(dir)
	dotEnv := []byte("AGENTFORGE_PUBLIC_BASE_URL=https://app.example.com\n")
	if err := os.WriteFile(".env", dotEnv, 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.TurnstileExpectedHostname != "none" {
		t.Fatalf("TurnstileExpectedHostname = %q, want none", cfg.TurnstileExpectedHostname)
	}
}
