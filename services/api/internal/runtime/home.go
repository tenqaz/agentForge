package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"agentforge.local/services/api/internal/templates"
)

const (
	ErrCodeCopyTemplateFailed   = "copy_template_failed"
	ErrCodeConfigWriteFailed    = "config_write_failed"
	ErrCodeContainerStartFailed = "container_start_failed"
)

type ProviderConfig struct {
	DefaultModel string `json:"defaultModel"`
	Provider     string `json:"provider"`
	BaseURL      string `json:"baseURL"`
	APIKey       string `json:"-"`
	APIMode      string `json:"apiMode"`
}

func (c ProviderConfig) String() string {
	return fmt.Sprintf("ProviderConfig{DefaultModel:%q, Provider:%q, BaseURL:%q, APIKey:[redacted], APIMode:%q}", c.DefaultModel, c.Provider, c.BaseURL, c.APIMode)
}

func (c ProviderConfig) GoString() string {
	return c.String()
}

type HomeSpec struct {
	AgentID  string
	HomePath string
	Template templates.Template
	Provider ProviderConfig
}

type HomeResult struct {
	HomePath   string `json:"homePath"`
	ConfigPath string `json:"configPath"`
	EnvPath    string `json:"envPath"`
	SoulPath   string `json:"soulPath"`
	UserPath   string `json:"userPath"`
	SkillsPath string `json:"skillsPath"`
}

type HomeBuilder interface {
	Provision(ctx context.Context, spec HomeSpec) (HomeResult, error)
}

type homeBuilder struct{}

type ProvisionError struct {
	Code    string
	Message string
	Err     error
}

func NewHomeBuilder() HomeBuilder {
	return homeBuilder{}
}

func NewProvisionError(code, message string, err error) *ProvisionError {
	return &ProvisionError{Code: code, Message: message, Err: err}
}

func (e *ProvisionError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return e.Message
	}
	if strings.TrimSpace(e.Message) == "" {
		return e.Err.Error()
	}
	return e.Message + ": " + e.Err.Error()
}

func (e *ProvisionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func AsProvisionError(err error, target **ProvisionError) bool {
	return errors.As(err, target)
}

func (homeBuilder) Provision(ctx context.Context, spec HomeSpec) (HomeResult, error) {
	select {
	case <-ctx.Done():
		return HomeResult{}, ctx.Err()
	default:
	}

	homePath, err := filepath.Abs(spec.HomePath)
	if err != nil {
		return HomeResult{}, NewProvisionError(ErrCodeConfigWriteFailed, "failed to resolve Hermes home path", err)
	}

	result := HomeResult{
		HomePath:   homePath,
		ConfigPath: filepath.Join(homePath, "config.yaml"),
		EnvPath:    filepath.Join(homePath, ".env"),
		SoulPath:   filepath.Join(homePath, "SOUL.md"),
		UserPath:   filepath.Join(homePath, "memories", "USER.md"),
		SkillsPath: filepath.Join(homePath, "skills"),
	}

	if err := ensureHomeLayout(homePath); err != nil {
		return HomeResult{}, NewProvisionError(ErrCodeConfigWriteFailed, "failed to create Hermes home layout", err)
	}

	if err := copyFile(spec.Template.SoulMDPath, result.SoulPath); err != nil {
		return HomeResult{}, NewProvisionError(ErrCodeCopyTemplateFailed, "failed to copy SOUL.md", err)
	}
	if err := copyFile(spec.Template.UserMDPath, result.UserPath); err != nil {
		return HomeResult{}, NewProvisionError(ErrCodeCopyTemplateFailed, "failed to copy USER.md", err)
	}
	if err := copyDir(spec.Template.SkillsPath, result.SkillsPath); err != nil {
		return HomeResult{}, NewProvisionError(ErrCodeCopyTemplateFailed, "failed to copy skills", err)
	}

	if err := writeConfig(result.ConfigPath, spec.Provider); err != nil {
		return HomeResult{}, NewProvisionError(ErrCodeConfigWriteFailed, "failed to write Hermes config", err)
	}
	if _, err := os.Stat(result.EnvPath); errors.Is(err, os.ErrNotExist) {
		if err := WriteAgentEnv(result.EnvPath); err != nil {
			return HomeResult{}, NewProvisionError(ErrCodeConfigWriteFailed, "failed to write Hermes env", err)
		}
	} else if err != nil {
		return HomeResult{}, NewProvisionError(ErrCodeConfigWriteFailed, "failed to inspect Hermes env", err)
	}

	return result, nil
}

func ensureHomeLayout(homePath string) error {
	dirs := []string{
		homePath,
		filepath.Join(homePath, "memories"),
		filepath.Join(homePath, "skills"),
		filepath.Join(homePath, "sessions"),
		filepath.Join(homePath, "logs"),
		filepath.Join(homePath, "weixin", "accounts"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func writeConfig(path string, provider ProviderConfig) error {
	content := strings.Join([]string{
		"model:",
		"  default: " + provider.DefaultModel,
		"  provider: " + provider.Provider,
		"  base_url: " + provider.BaseURL,
		"  api_key: " + provider.APIKey,
		"  api_mode: " + provider.APIMode,
		"",
	}, "\n")
	return writeTextFile(path, content)
}

func writeTextFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func copyFile(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	output, err := os.Create(target)
	if err != nil {
		return err
	}
	defer output.Close()

	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	return output.Close()
}

func copyDir(source, target string) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(source, entry.Name())
		targetPath := filepath.Join(target, entry.Name())
		if entry.IsDir() {
			if err := copyDir(sourcePath, targetPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(sourcePath, targetPath); err != nil {
			return err
		}
	}
	return nil
}

// DestroyHome removes the agent's hermes-home directory. It is idempotent:
// a missing directory is treated as success. The path is validated to be
// under {dataDir}/agents/{agentID} and to be at least three levels deep,
// refusing root or other shallow paths to avoid accidental destruction.
func DestroyHome(homePath string) error {
	trimmed := strings.TrimSpace(homePath)
	if trimmed == "" {
		return errors.New("hermes home path is empty")
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return fmt.Errorf("resolve hermes home path: %w", err)
	}
	cleaned := filepath.Clean(abs)
	// Validate: parent must be "agents" directory, preventing accidental
	// destruction of non-agent paths. The agent dir itself contains all
	// Hermes home files (SOUL.md, .env, skills/, etc.).
	parent := filepath.Dir(cleaned)
	if filepath.Base(parent) != "agents" {
		return fmt.Errorf("refuse to destroy path not under agents/: %s", cleaned)
	}
	grandparent := filepath.Dir(parent)
	separator := string(filepath.Separator)
	if grandparent == separator || grandparent == "." || grandparent == filepath.VolumeName(grandparent)+separator {
		return fmt.Errorf("refuse to destroy shallow path: %s", cleaned)
	}
	if err := os.RemoveAll(cleaned); err != nil {
		return fmt.Errorf("remove hermes home: %w", err)
	}
	return nil
}
