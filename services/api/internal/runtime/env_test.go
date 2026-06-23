package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAgentEnvWritesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hermes-home", ".env")
	if err := WriteAgentEnv(path); err != nil {
		t.Fatalf("WriteAgentEnv returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	want := "WEIXIN_DM_POLICY=allowlist\n" +
		"WEIXIN_GROUP_POLICY=disabled\n" +
		"WEIXIN_GROUP_ALLOWED_USERS=\n"
	if string(data) != want {
		t.Fatalf(".env = %q, want %q", string(data), want)
	}
}

func TestWriteAgentEnvConnectedWritesConnectionValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hermes-home", ".env")
	if err := WriteAgentEnvConnected(path, "account-1", "token-1", "https://weixin.example.com", "user-1"); err != nil {
		t.Fatalf("WriteAgentEnvConnected returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	want := "WEIXIN_ACCOUNT_ID=account-1\n" +
		"WEIXIN_TOKEN=token-1\n" +
		"WEIXIN_BASE_URL=https://weixin.example.com\n" +
		"WEIXIN_DM_POLICY=allowlist\n" +
		"WEIXIN_GROUP_POLICY=disabled\n" +
		"WEIXIN_ALLOWED_USERS=user-1\n" +
		"WEIXIN_HOME_CHANNEL=user-1\n" +
		"WEIXIN_GROUP_ALLOWED_USERS=\n"
	if string(data) != want {
		t.Fatalf(".env = %q, want %q", string(data), want)
	}
}
