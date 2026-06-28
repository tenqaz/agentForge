package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

var ErrContainerNotFound = errors.New("container not found")

// isContainerNotFoundOutput reports whether docker CLI combined output
// indicates the target container/object does not exist. Matching is
// case-insensitive because docker phrasing varies by platform/version
// (e.g. "Error: No such container" on Linux, lowercase "no such object"
// on some Docker Desktop builds).
func isContainerNotFoundOutput(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "no such object") ||
		strings.Contains(lower, "no such container")
}

type Runner interface {
	EnsureRunning(ctx context.Context, spec ContainerSpec) error
	Stop(ctx context.Context, containerName string) error
	Remove(ctx context.Context, containerName string) error
	// Destroy forcefully stops and removes the container, blocking until it
	// is fully gone (important for async runtimes like ECI). After Destroy
	// returns without error, the container no longer exists and any mounted
	// storage may be safely cleaned up.
	Destroy(ctx context.Context, containerName string) error
	Inspect(ctx context.Context, containerName string) (ContainerStatus, error)
}

type ContainerSpec struct {
	AgentID       string
	ContainerName string
	HermesHome    string
	Image         string
	Memory        string
	CPUs          string
}

type ContainerStatus struct {
	Exists  bool
	Running bool
	Status  string
}

type dockerRunner struct {
	dockerBin   string
	agentsVolume string // Docker named volume for /data/agents
}

func NewDockerRunner(dockerBin, agentsVolume string) Runner {
	if strings.TrimSpace(dockerBin) == "" {
		dockerBin = "docker"
	}
	return &dockerRunner{dockerBin: dockerBin, agentsVolume: agentsVolume}
}

func DefaultContainerName(agentID string) string {
	return "agentforge-hermes-" + agentID
}

func (r *dockerRunner) EnsureRunning(ctx context.Context, spec ContainerSpec) error {
	if strings.TrimSpace(spec.AgentID) == "" {
		return errors.New("agent id is required")
	}
	containerName := DefaultContainerName(spec.AgentID)

	status, err := r.Inspect(ctx, containerName)
	if err == nil {
		if status.Running {
			return nil
		}
		output, startErr := exec.CommandContext(ctx, r.dockerBin, "start", containerName).CombinedOutput()
		if startErr != nil {
			return fmt.Errorf("docker start failed: %w: %s", startErr, strings.TrimSpace(string(output)))
		}
		return nil
	}
	if !errors.Is(err, ErrContainerNotFound) {
		return err
	}

	// Mount the same named volume the API uses for /data/agents so Hermes
	// can read SOUL.md, skills, and write logs/sessions. This avoids
	// bind-mounting a path that may not be shared (e.g. macOS Docker Desktop).
	hermesHome := "/opt/data"
	if strings.TrimSpace(r.agentsVolume) != "" {
		hermesHome = "/data/agents/" + spec.AgentID + "/hermes-home"
	}

	args := []string{
		"run",
		"-d",
		"--name", containerName,
		"--restart", "unless-stopped",
		"-e", "HERMES_HOME=" + hermesHome,
		"--memory=" + spec.Memory,
		"--cpus=" + spec.CPUs,
	}
	if r.agentsVolume != "" {
		args = append(args, "-v", r.agentsVolume+":/data/agents")
	} else {
		// Fallback: bind-mount from API container filesystem (legacy).
		homePath, err := filepath.Abs(spec.HermesHome)
		if err != nil {
			return err
		}
		args = append(args, "-v", homePath+":/opt/data")
	}
	args = append(args, spec.Image,
		"sh", "-c",
		// Clean stale gateway state and heartbeat on every boot, then
		// start a liveness heartbeat loop (touch .heartbeat every 30s)
		// before exec-ing the gateway. The heartbeat file is read by
		// agentForge's IdleDetector to confirm the gateway process is alive.
		"rm -f $HERMES_HOME/gateway_state.json $HERMES_HOME/gateway.lock $HERMES_HOME/gateway.pid $HERMES_HOME/.heartbeat; "+
			"(while true; do touch $HERMES_HOME/.heartbeat; sleep 30; done) & "+
			"exec /opt/hermes/docker/main-wrapper.sh gateway run",
	)

	output, err := exec.CommandContext(ctx, r.dockerBin, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker run failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (r *dockerRunner) Stop(ctx context.Context, containerName string) error {
	output, err := exec.CommandContext(ctx, r.dockerBin, "stop", containerName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker stop failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (r *dockerRunner) Remove(ctx context.Context, containerName string) error {
	output, err := exec.CommandContext(ctx, r.dockerBin, "rm", "-f", containerName).CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if isContainerNotFoundOutput(trimmed) {
			return ErrContainerNotFound
		}
		return fmt.Errorf("docker rm failed: %w: %s", err, trimmed)
	}
	return nil
}

// Destroy forcefully stops and removes the container. Docker rm -f is
// synchronous — no polling needed.
func (r *dockerRunner) Destroy(ctx context.Context, containerName string) error {
	output, err := exec.CommandContext(ctx, r.dockerBin, "rm", "-f", containerName).CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if isContainerNotFoundOutput(trimmed) {
			return nil // already gone
		}
		return fmt.Errorf("docker destroy failed: %w: %s", err, trimmed)
	}
	return nil
}

func (r *dockerRunner) Inspect(ctx context.Context, containerName string) (ContainerStatus, error) {
	output, err := exec.CommandContext(ctx, r.dockerBin, "inspect", "--format", "{{json .State}}", containerName).CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if isContainerNotFoundOutput(trimmed) {
			return ContainerStatus{}, ErrContainerNotFound
		}
		return ContainerStatus{}, fmt.Errorf("docker inspect failed: %w: %s", err, trimmed)
	}
	var state struct {
		Running bool   `json:"Running"`
		Status  string `json:"Status"`
	}
	if err := json.Unmarshal(output, &state); err != nil {
		return ContainerStatus{}, err
	}
	return ContainerStatus{
		Exists:  true,
		Running: state.Running,
		Status:  state.Status,
	}, nil
}
