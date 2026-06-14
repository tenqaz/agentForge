package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentforge.local/services/api/internal/templates"
)

func TestProvisionHermesHomeCopiesTemplateAndPreservesSessions(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	templateRoot := filepath.Join(root, "templates", "template-1", "versions", "3")
	template := templates.Template{
		ID:           "template-1",
		Version:      3,
		SoulMDPath:   filepath.Join(templateRoot, "SOUL.md"),
		UserMDPath:   filepath.Join(templateRoot, "USER.md"),
		SkillsPath:   filepath.Join(templateRoot, "skills"),
		TemplatePath: templateRoot,
	}
	if err := os.MkdirAll(filepath.Join(template.SkillsPath, "faq"), 0o755); err != nil {
		t.Fatalf("mkdir skills: %v", err)
	}
	if err := os.WriteFile(template.SoulMDPath, []byte("Soul contents"), 0o644); err != nil {
		t.Fatalf("write SOUL.md: %v", err)
	}
	if err := os.WriteFile(template.UserMDPath, []byte("User memory"), 0o644); err != nil {
		t.Fatalf("write USER.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(template.SkillsPath, "faq", "SKILL.md"), []byte("# FAQ"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	homePath := filepath.Join(root, "agents", "agent-internal-id", "hermes-home")
	if err := os.MkdirAll(filepath.Join(homePath, "sessions"), 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionMarker := filepath.Join(homePath, "sessions", "existing.session")
	if err := os.WriteFile(sessionMarker, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("write existing session: %v", err)
	}

	builder := NewHomeBuilder()
	spec := HomeSpec{
		AgentID:  "agent-internal-id",
		HomePath: homePath,
		Template: template,
		Provider: ProviderConfig{
			DefaultModel: "deepseek-v4-flash",
			Provider:     "custom",
			BaseURL:      "https://api.deepseek.com",
			APIKey:       "secret-api-key",
			APIMode:      "chat_completions",
		},
	}

	result, err := builder.Provision(ctx, spec)
	if err != nil {
		t.Fatalf("Provision returned error: %v", err)
	}
	if result.HomePath != homePath {
		t.Fatalf("HomePath = %q, want %q", result.HomePath, homePath)
	}

	mustContainFile(t, filepath.Join(homePath, "SOUL.md"), "Soul contents")
	mustContainFile(t, filepath.Join(homePath, "memories", "USER.md"), "User memory")
	mustContainFile(t, filepath.Join(homePath, "skills", "faq", "SKILL.md"), "# FAQ")
	mustContainFile(t, filepath.Join(homePath, "config.yaml"), "api_key: secret-api-key")
	mustContainFile(t, filepath.Join(homePath, ".env"), "WEIXIN_DM_POLICY=allowlist")
	mustContainFile(t, sessionMarker, "keep me")

	if _, err := os.Stat(filepath.Join(homePath, "logs")); err != nil {
		t.Fatalf("logs dir stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(homePath, "weixin", "accounts")); err != nil {
		t.Fatalf("weixin/accounts stat error: %v", err)
	}

	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal result: %v", err)
	}
	if strings.Contains(string(payload), "secret-api-key") {
		t.Fatalf("result JSON leaked api key: %s", string(payload))
	}

	var logs bytes.Buffer
	logger := log.New(&logs, "", 0)
	logger.Printf("provisioned %v", spec.Provider)
	if strings.Contains(logs.String(), "secret-api-key") {
		t.Fatalf("provider String leaked api key: %q", logs.String())
	}

	if err := os.WriteFile(filepath.Join(homePath, "sessions", "new.session"), []byte("new"), 0o644); err != nil {
		t.Fatalf("write new session: %v", err)
	}
	if _, err := builder.Provision(ctx, spec); err != nil {
		t.Fatalf("second Provision returned error: %v", err)
	}
	mustContainFile(t, filepath.Join(homePath, "sessions", "existing.session"), "keep me")
	mustContainFile(t, filepath.Join(homePath, "sessions", "new.session"), "new")
}

func TestProvisionHermesHomeReturnsCopyTemplateFailed(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	builder := NewHomeBuilder()
	_, err := builder.Provision(ctx, HomeSpec{
		AgentID:  "agent-1",
		HomePath: filepath.Join(root, "agents", "agent-1", "hermes-home"),
		Template: templates.Template{
			ID:         "template-1",
			Version:    1,
			SoulMDPath: filepath.Join(root, "missing", "SOUL.md"),
			UserMDPath: filepath.Join(root, "missing", "USER.md"),
			SkillsPath: filepath.Join(root, "missing", "skills"),
		},
		Provider: ProviderConfig{
			DefaultModel: "deepseek-v4-flash",
			Provider:     "custom",
			BaseURL:      "https://api.deepseek.com",
			APIKey:       "secret-api-key",
			APIMode:      "chat_completions",
		},
	})
	if err == nil {
		t.Fatal("Provision error = nil, want error")
	}
	var provisionErr *ProvisionError
	if !AsProvisionError(err, &provisionErr) {
		t.Fatalf("Provision error type = %T, want *ProvisionError", err)
	}
	if provisionErr.Code != ErrCodeCopyTemplateFailed {
		t.Fatalf("Provision error code = %q, want %q", provisionErr.Code, ErrCodeCopyTemplateFailed)
	}
}

func mustContainFile(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s = %q, want substring %q", path, string(data), want)
	}
}
