package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	AgentID     string
	HomePath    string
	Template    templates.Template
	SoulContent string
	UserContent string
	Provider    ProviderConfig
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

	if err := writeTextFile(result.SoulPath, spec.SoulContent); err != nil {
		return HomeResult{}, NewProvisionError(ErrCodeCopyTemplateFailed, "failed to write SOUL.md", err)
	}
	if err := writeTextFile(result.UserPath, spec.UserContent); err != nil {
		return HomeResult{}, NewProvisionError(ErrCodeCopyTemplateFailed, "failed to write USER.md", err)
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
		// 0777 so the Hermes container user (non-root) can write runtime
		// files — session snapshots, gateway logs, weixin sync state.
		if err := os.MkdirAll(dir, 0o777); err != nil {
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
// under {dataDir}/agents/{agentID}[/hermes-home] and deep enough to avoid
// accidental destruction.
//
// On NFS, os.RemoveAll can fail with "directory not empty" even when the
// container that created the files is gone — NFS attribute caches and
// stray .nfs* files can delay cleanup. We work around this by walking the
// tree bottom-up and removing entries individually, then retrying the
// whole operation up to 3 times with a short sleep between attempts.
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
	// Support two path formats:
	//   Docker mode: {dataDir}/agents/{agentID}/hermes-home
	//   ECI mode:    {dataDir}/agents/{agentID}
	parent := filepath.Dir(cleaned)
	if filepath.Base(cleaned) == "hermes-home" {
		// Docker mode: parent is the agent ID dir, grandparent is agents/
		parent = filepath.Dir(parent)
	}
	if filepath.Base(parent) != "agents" {
		return fmt.Errorf("refuse to destroy path not under agents/: %s", cleaned)
	}
	grandparent := filepath.Dir(parent)
	separator := string(filepath.Separator)
	if grandparent == separator || grandparent == "." || grandparent == filepath.VolumeName(grandparent)+separator {
		return fmt.Errorf("refuse to destroy shallow path: %s", cleaned)
	}

	// On NFS, os.RemoveAll can transiently fail with ENOTEMPTY. Walk the
	// tree bottom-up to remove individual entries, then retry.
	const maxRetries = 3
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(500 * time.Millisecond)
		}
		lastErr = removeAllNFS(cleaned)
		if lastErr == nil {
			return nil
		}
	}
	return fmt.Errorf("remove hermes home: %w", lastErr)
}

// removeAllNFS walks the directory tree bottom-up, removing files and then
// directories. It tolerates ENOENT at any point (the entry may have been
// deleted by a concurrent NFS operation).
func removeAllNFS(root string) error {
	// Collect paths in bottom-up order so we delete children before parents.
	var paths []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// If we can't access a path, skip it — we'll retry.
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return err
	}

	// Delete bottom-up (reverse order).
	for i := len(paths) - 1; i >= 0; i-- {
		p := paths[i]
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			// ENOTEMPTY on a directory means a child appeared — continue
			// and let the outer retry loop handle it.
			if os.IsExist(err) {
				continue
			}
			return err
		}
	}
	return nil
}
