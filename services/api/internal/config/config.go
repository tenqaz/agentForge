package config

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

	return Config{
		HTTPAddr:      value("AGENTFORGE_HTTP_ADDR", dotEnv, defaultHTTPAddr),
		PublicBaseURL: value("AGENTFORGE_PUBLIC_BASE_URL", dotEnv, defaultPublicBaseURL),
		DataDir:       dataDir,
		SQLitePath:    filepath.Join(dataDir, "agentforge.db"),
		SessionSecret: value("AGENTFORGE_SESSION_SECRET", dotEnv, defaultSessionSecret),
		HermesImage:   value("AGENTFORGE_HERMES_IMAGE", dotEnv, defaultHermesImage),
		HermesMemory:  value("AGENTFORGE_HERMES_MEMORY", dotEnv, defaultHermesMemory),
		HermesCPUs:    value("AGENTFORGE_HERMES_CPUS", dotEnv, defaultHermesCPUs),
		DockerBin:     defaultDockerBin,
	}, nil
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
