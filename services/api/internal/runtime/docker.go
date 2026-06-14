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

type Runner interface {
	EnsureRunning(ctx context.Context, spec ContainerSpec) error
	Stop(ctx context.Context, containerName string) error
	Remove(ctx context.Context, containerName string) error
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
	dockerBin string
}

func NewDockerRunner(dockerBin string) Runner {
	if strings.TrimSpace(dockerBin) == "" {
		dockerBin = "docker"
	}
	return &dockerRunner{dockerBin: dockerBin}
}

func DefaultContainerName(agentID string) string {
	return "agentforge-hermes-" + agentID
}

func (r *dockerRunner) EnsureRunning(ctx context.Context, spec ContainerSpec) error {
	if strings.TrimSpace(spec.AgentID) == "" {
		return errors.New("agent id is required")
	}
	containerName := DefaultContainerName(spec.AgentID)
	homePath, err := filepath.Abs(spec.HermesHome)
	if err != nil {
		return err
	}

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

	args := []string{
		"run",
		"-d",
		"--name", containerName,
		"--restart", "unless-stopped",
		"-v", homePath + ":/opt/data",
		"-e", "HERMES_HOME=/opt/data",
		"--memory=" + spec.Memory,
		"--cpus=" + spec.CPUs,
		spec.Image,
		"gateway",
		"run",
	}
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
		return fmt.Errorf("docker rm failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (r *dockerRunner) Inspect(ctx context.Context, containerName string) (ContainerStatus, error) {
	output, err := exec.CommandContext(ctx, r.dockerBin, "inspect", "--format", "{{json .State}}", containerName).CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if strings.Contains(trimmed, "No such object") || strings.Contains(trimmed, "No such container") {
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
