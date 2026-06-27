package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerRunnerEnsureRunningBuildsExpectedCommand(t *testing.T) {
	ctx := context.Background()
	workdir := t.TempDir()
	logPath := filepath.Join(workdir, "docker-args.log")
	stubPath := filepath.Join(workdir, "docker")
	script := "#!/bin/sh\nprintf '%s\n' \"$@\" >" + shellQuote(logPath) + "\nif [ \"$1\" = \"inspect\" ]; then\n  printf 'Error: No such object: %s' \"$4\" >&2\n  exit 1\nfi\nif [ \"$1\" = \"run\" ]; then\n  printf 'container-123'\nfi\n"
	if err := os.WriteFile(stubPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub docker: %v", err)
	}

	runner := NewDockerRunner(stubPath, "")
	homePath := filepath.Join(workdir, "relative", "..", "hermes-home")
	spec := ContainerSpec{
		AgentID:       "agent-internal-id",
		ContainerName: "Friendly Agent Name",
		HermesHome:    homePath,
		Image:         "nousresearch/hermes-agent:v2026.6.5",
		Memory:        "500m",
		CPUs:          "0.5",
	}

	if err := runner.EnsureRunning(ctx, spec); err != nil {
		t.Fatalf("EnsureRunning returned error: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read docker args log: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(data)), "\n")
	joined := strings.Join(args, " ")
	absoluteHome, err := filepath.Abs(homePath)
	if err != nil {
		t.Fatalf("abs home path: %v", err)
	}

	if !strings.Contains(joined, "--name agentforge-hermes-agent-internal-id") {
		t.Fatalf("command missing internal id container name: %s", joined)
	}
	if strings.Contains(joined, "Friendly Agent Name") {
		t.Fatalf("command used user input name: %s", joined)
	}
	if !strings.Contains(joined, "-v "+absoluteHome+":/opt/data") {
		t.Fatalf("command missing absolute volume mount: %s", joined)
	}
	if !strings.Contains(joined, "-e HERMES_HOME=/opt/data") {
		t.Fatalf("command missing HERMES_HOME env: %s", joined)
	}
	if !strings.Contains(joined, "nousresearch/hermes-agent:v2026.6.5 gateway run") {
		t.Fatalf("command missing image and gateway run suffix: %s", joined)
	}
}

func TestDockerRunnerInspectParsesRunningState(t *testing.T) {
	ctx := context.Background()
	workdir := t.TempDir()
	stubPath := filepath.Join(workdir, "docker")
	script := "#!/bin/sh\nif [ \"$1\" = \"inspect\" ]; then\n  printf '{\"Running\":true,\"Status\":\"running\"}'\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(stubPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub docker: %v", err)
	}

	runner := NewDockerRunner(stubPath, "")
	status, err := runner.Inspect(ctx, "agentforge-hermes-agent-1")
	if err != nil {
		t.Fatalf("Inspect returned error: %v", err)
	}
	if !status.Exists || !status.Running || status.Status != "running" {
		t.Fatalf("Inspect status = %#v", status)
	}
}

func TestDockerRunnerEnsureRunningStartsExistingStoppedContainer(t *testing.T) {
	ctx := context.Background()
	workdir := t.TempDir()
	logPath := filepath.Join(workdir, "docker-args.log")
	stubPath := filepath.Join(workdir, "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" >>" + shellQuote(logPath) + "\n" +
		"if [ \"$1\" = \"inspect\" ]; then\n" +
		"  printf '{\"Running\":false,\"Status\":\"exited\"}'\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"start\" ]; then\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 1\n"
	if err := os.WriteFile(stubPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub docker: %v", err)
	}

	runner := NewDockerRunner(stubPath, "")
	if err := runner.EnsureRunning(ctx, ContainerSpec{
		AgentID:    "agent-internal-id",
		HermesHome: filepath.Join(workdir, "hermes-home"),
		Image:      "nousresearch/hermes-agent:v2026.6.5",
		Memory:     "500m",
		CPUs:       "0.5",
	}); err != nil {
		t.Fatalf("EnsureRunning returned error: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read docker args log: %v", err)
	}
	logged := string(data)
	if !strings.Contains(logged, "inspect\n--format\n{{json .State}}\nagentforge-hermes-agent-internal-id") {
		t.Fatalf("missing inspect call: %s", logged)
	}
	if !strings.Contains(logged, "start\nagentforge-hermes-agent-internal-id") {
		t.Fatalf("missing start call: %s", logged)
	}
	if strings.Contains(logged, "\nrun\n") {
		t.Fatalf("unexpected docker run call: %s", logged)
	}
}

func TestDockerRunnerEnsureRunningNoopsWhenContainerAlreadyRunning(t *testing.T) {
	ctx := context.Background()
	workdir := t.TempDir()
	logPath := filepath.Join(workdir, "docker-args.log")
	stubPath := filepath.Join(workdir, "docker")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" >>" + shellQuote(logPath) + "\n" +
		"if [ \"$1\" = \"inspect\" ]; then\n" +
		"  printf '{\"Running\":true,\"Status\":\"running\"}'\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 1\n"
	if err := os.WriteFile(stubPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub docker: %v", err)
	}

	runner := NewDockerRunner(stubPath, "")
	if err := runner.EnsureRunning(ctx, ContainerSpec{
		AgentID:    "agent-internal-id",
		HermesHome: filepath.Join(workdir, "hermes-home"),
		Image:      "nousresearch/hermes-agent:v2026.6.5",
		Memory:     "500m",
		CPUs:       "0.5",
	}); err != nil {
		t.Fatalf("EnsureRunning returned error: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read docker args log: %v", err)
	}
	logged := string(data)
	if !strings.Contains(logged, "inspect\n--format\n{{json .State}}\nagentforge-hermes-agent-internal-id") {
		t.Fatalf("missing inspect call: %s", logged)
	}
	if strings.Contains(logged, "\nstart\n") || strings.Contains(logged, "\nrun\n") {
		t.Fatalf("unexpected start/run call: %s", logged)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func TestDockerRunnerRemoveReturnsNotFoundWhenContainerMissing(t *testing.T) {
	ctx := context.Background()
	workdir := t.TempDir()
	stubPath := filepath.Join(workdir, "docker")
	script := "#!/bin/sh\nprintf 'Error: No such container: %s' \"$3\" >&2\nexit 1\n"
	if err := os.WriteFile(stubPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub docker: %v", err)
	}
	runner := NewDockerRunner(stubPath, "")

	err := runner.Remove(ctx, "agentforge-hermes-missing")
	if !errors.Is(err, ErrContainerNotFound) {
		t.Fatalf("Remove err = %v, want ErrContainerNotFound", err)
	}
}

func TestDockerRunnerRemoveWrapsOtherErrors(t *testing.T) {
	ctx := context.Background()
	workdir := t.TempDir()
	stubPath := filepath.Join(workdir, "docker")
	script := "#!/bin/sh\nprintf 'Error: permission denied' >&2\nexit 1\n"
	if err := os.WriteFile(stubPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub docker: %v", err)
	}
	runner := NewDockerRunner(stubPath, "")

	err := runner.Remove(ctx, "agentforge-hermes-x")
	if err == nil {
		t.Fatal("Remove returned nil, want error")
	}
	if errors.Is(err, ErrContainerNotFound) {
		t.Fatalf("Remove err = %v, did not expect ErrContainerNotFound", err)
	}
}
