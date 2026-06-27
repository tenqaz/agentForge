package config

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"agentforge.local/services/api/internal/weixin"
)

const (
	defaultHTTPAddr      = ":8080"
	defaultPublicBaseURL = "http://localhost:8080"
	defaultDataDir       = "../../var"
	defaultSessionSecret = "dev-change-me"
	defaultHermesImage   = "nousresearch/hermes-agent:v2026.6.5"
	defaultHermesMemory  = "500m"
	defaultHermesCPUs    = "0.5"
	defaultDockerBin     = "docker"
)

type Config struct {
	HTTPAddr      string
	PublicBaseURL string
	DataDir       string
	SQLitePath    string
	SessionSecret string
	HermesImage   string
	HermesMemory  string
	HermesCPUs    string
	DockerBin     string
	// DockerAgentsVolume is the Docker named volume that stores agent data
	// under /data/agents/. Used by the Docker runner to share data with
	// Hermes containers (avoids bind-mount issues on macOS).
	DockerAgentsVolume string
	WeixinBaseURL      string
	ModelDefault       string
	ModelProvider      string
	ModelBaseURL       string
	ModelAPIKey        string
	ModelAPIMode       string

	// ECI (Alibaba Cloud Elastic Container Instance) settings.
	// When RunnerMode is "eci", these must be configured.
	RunnerMode         string // "docker" (default) or "eci"
	ECIRegion          string
	ECIAccessKeyID     string
	ECIAccessKeySecret string
	ECISecurityGroupID string
	ECIVSwitchID       string
	ECIImageCacheID    string // optional, accelerates cold starts
	ECIEIPInstanceID   string // optional, EIP to bind to container group
	ECINASHost         string // NAS 挂载点地址
	ECINASPath         string // NAS 上根路径，默认 "/"
	ECINASFileSystemID string // NAS 文件系统 ID

	// Auto sleep / wake settings.
	AutoSleepEnabled        bool
	IdleTimeoutMinutes      int
	SleepPollIntervalSec    int
	IdleCheckIntervalSec    int
	IdleHeartbeatMisses     int
	WakeHeartbeatTimeoutSec int

	// Brevo 事务邮件配置，用于发送注册验证码邮件。全部可选；未配置 API key 或
	// 发件人邮箱时，发码端点在运行时返回 email_send_failed（500），不影响启动。
	BrevoAPIKey      string
	BrevoSenderEmail string
	BrevoSenderName  string
	BrevoBaseURL     string
}

func Load() (Config, error) {
	dotEnv, err := readDotEnv(".env")
	if err != nil {
		return Config{}, err
	}

	dataDir, err := filepath.Abs(value("AGENTFORGE_DATA_DIR", dotEnv, defaultDataDir))
	if err != nil {
		return Config{}, err
	}

	weixinBaseURL := value("AGENTFORGE_WEIXIN_BASE_URL", dotEnv, weixin.DefaultBaseURL)
	if err := validateAbsoluteHTTPURL(weixinBaseURL); err != nil {
		return Config{}, fmt.Errorf("AGENTFORGE_WEIXIN_BASE_URL: %w", err)
	}

	return Config{
		HTTPAddr:           value("AGENTFORGE_HTTP_ADDR", dotEnv, defaultHTTPAddr),
		PublicBaseURL:      value("AGENTFORGE_PUBLIC_BASE_URL", dotEnv, defaultPublicBaseURL),
		DataDir:            dataDir,
		SQLitePath:         filepath.Join(dataDir, "agentforge.db"),
		SessionSecret:      value("AGENTFORGE_SESSION_SECRET", dotEnv, defaultSessionSecret),
		HermesImage:        value("AGENTFORGE_HERMES_IMAGE", dotEnv, defaultHermesImage),
		HermesMemory:       value("AGENTFORGE_HERMES_MEMORY", dotEnv, defaultHermesMemory),
		HermesCPUs:         value("AGENTFORGE_HERMES_CPUS", dotEnv, defaultHermesCPUs),
		DockerBin:          defaultDockerBin,
		DockerAgentsVolume: value("AGENTFORGE_DOCKER_AGENTS_VOLUME", dotEnv, "agentforge_agentforge-agents-data"),
		WeixinBaseURL:      weixinBaseURL,
		ModelDefault:       value("AGENTFORGE_MODEL_DEFAULT", dotEnv, ""),
		ModelProvider:      value("AGENTFORGE_MODEL_PROVIDER", dotEnv, ""),
		ModelBaseURL:       value("AGENTFORGE_MODEL_BASE_URL", dotEnv, ""),
		ModelAPIKey:        value("AGENTFORGE_MODEL_API_KEY", dotEnv, ""),
		ModelAPIMode:       value("AGENTFORGE_MODEL_API_MODE", dotEnv, ""),

		RunnerMode:         value("AGENTFORGE_RUNNER_MODE", dotEnv, "docker"),
		ECIRegion:          value("AGENTFORGE_ECI_REGION", dotEnv, ""),
		ECIAccessKeyID:     value("AGENTFORGE_ECI_ACCESS_KEY_ID", dotEnv, ""),
		ECIAccessKeySecret: value("AGENTFORGE_ECI_ACCESS_KEY_SECRET", dotEnv, ""),
		ECISecurityGroupID: value("AGENTFORGE_ECI_SECURITY_GROUP_ID", dotEnv, ""),
		ECIVSwitchID:       value("AGENTFORGE_ECI_VSWITCH_ID", dotEnv, ""),
		ECIImageCacheID:    value("AGENTFORGE_ECI_IMAGE_CACHE_ID", dotEnv, ""),
		ECIEIPInstanceID:   value("AGENTFORGE_ECI_EIP_INSTANCE_ID", dotEnv, ""),
		ECINASHost:         value("AGENTFORGE_ECI_NAS_HOST", dotEnv, ""),
		ECINASPath:         value("AGENTFORGE_ECI_NAS_PATH", dotEnv, "/"),
		ECINASFileSystemID: value("AGENTFORGE_ECI_NAS_FILE_SYSTEM_ID", dotEnv, ""),


			AutoSleepEnabled:        boolValue("AGENTFORGE_AUTO_SLEEP_ENABLED", dotEnv, false),
			IdleTimeoutMinutes:      intValue("AGENTFORGE_IDLE_TIMEOUT", dotEnv, 10),
			SleepPollIntervalSec:    intValue("AGENTFORGE_SLEEP_POLL_INTERVAL", dotEnv, 5),
			IdleCheckIntervalSec:    intValue("AGENTFORGE_IDLE_CHECK_INTERVAL", dotEnv, 60),
			IdleHeartbeatMisses:     intValue("AGENTFORGE_IDLE_HEARTBEAT_MISSES", dotEnv, 3),
			WakeHeartbeatTimeoutSec: intValue("AGENTFORGE_WAKE_HEARTBEAT_TIMEOUT", dotEnv, 60),

			BrevoAPIKey:      value("AGENTFORGE_BREVO_API_KEY", dotEnv, ""),
			BrevoSenderEmail: value("AGENTFORGE_BREVO_SENDER_EMAIL", dotEnv, ""),
			BrevoSenderName:  value("AGENTFORGE_BREVO_SENDER_NAME", dotEnv, ""),
			BrevoBaseURL:     value("AGENTFORGE_BREVO_BASE_URL", dotEnv, "https://api.brevo.com"),

	}, nil
}

// validateAbsoluteHTTPURL fails fast if a configured base URL is missing
// the http:// or https:// scheme. Without this check the API server
// would start successfully but every weixin gateway call would fail
// with `unsupported protocol scheme ""`.
func validateAbsoluteHTTPURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid url %q: %w", raw, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("must start with http:// or https:// (got %q)", raw)
	}
	if parsed.Host == "" {
		return fmt.Errorf("missing host in %q", raw)
	}
	return nil
}

func value(key string, dotEnv map[string]string, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	if v := strings.TrimSpace(dotEnv[key]); v != "" {
		return v
	}
	return fallback
}

func boolValue(key string, dotEnv map[string]string, fallback bool) bool {
	s := strings.ToLower(strings.TrimSpace(value(key, dotEnv, "")))
	if s == "" {
		return fallback
	}
	return s == "true" || s == "1" || s == "yes" || s == "on"
}

func intValue(key string, dotEnv map[string]string, fallback int) int {
	s := strings.TrimSpace(value(key, dotEnv, ""))
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return fallback
	}
	return v
}

func readDotEnv(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		values[key] = unquote(strings.TrimSpace(val))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func unquote(value string) string {
	if len(value) < 2 {
		return value
	}
	if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
		return value[1 : len(value)-1]
	}
	return value
}
