package runtime

import (
	"path/filepath"
	"strings"
)

func WriteAgentEnv(path string) error {
	return writeEnv(path, []string{
		"WEIXIN_DM_POLICY=allowlist",
		"WEIXIN_GROUP_POLICY=disabled",
		"WEIXIN_GROUP_ALLOWED_USERS=",
	})
}

func WriteAgentEnvConnected(path, accountID, botToken, baseURL, ilinkUserID string) error {
	return writeEnv(path, []string{
		"WEIXIN_ACCOUNT_ID=" + accountID,
		"WEIXIN_TOKEN=" + botToken,
		"WEIXIN_BASE_URL=" + baseURL,
		"WEIXIN_DM_POLICY=allowlist",
		"WEIXIN_GROUP_POLICY=disabled",
		"WEIXIN_ALLOWED_USERS=" + ilinkUserID,
		"WEIXIN_HOME_CHANNEL=" + ilinkUserID,
		"WEIXIN_GROUP_ALLOWED_USERS=",
	})
}

func writeEnv(path string, lines []string) error {
	content := strings.Join(lines, "\n") + "\n"
	return writeTextFile(filepath.Clean(path), content)
}
