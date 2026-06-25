package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"agentforge.local/services/api/internal/runtime"
	"agentforge.local/services/api/internal/templates"
	"github.com/google/uuid"
)

const (
	agentStatusCreating     = "creating"
	agentStatusProvisioning = "provisioning"
	agentStatusStarting     = "starting"
	agentStatusRunning      = "running"
	agentStatusStopped      = "stopped"
	agentStatusError        = "error"
)

type TemplateLoader interface {
	LoadPublishedTemplate(ctx context.Context, templateID string, version int) (templates.Template, error)
}

type RuntimeWorkerDependencies struct {
	Database       *sql.DB
	RuntimeJobs    *RuntimeRepository
	HomeBuilder    runtime.HomeBuilder
	Runner         runtime.Runner
	TemplateLoader TemplateLoader
	Provider       runtime.ProviderConfig
	HermesImage    string
	HermesMemory   string
	HermesCPUs     string
}

type RuntimeWorker struct {
	database       *sql.DB
	runtimeJobs    *RuntimeRepository
	homeBuilder    runtime.HomeBuilder
	runner         runtime.Runner
	templateLoader TemplateLoader
	provider       runtime.ProviderConfig
	hermesImage    string
	hermesMemory   string
	hermesCPUs     string
}

type runtimeJobRecord struct {
	ID      string
	AgentID string
	Type    Type
	Status  Status
}

type runtimeAgentRecord struct {
	ID              string
	TemplateID      string
	TemplateVersion int
	Status          string
	RuntimeID       string
	HermesHomePath  string
}

func NewRuntimeWorker(deps RuntimeWorkerDependencies) *RuntimeWorker {
	runtimeJobs := deps.RuntimeJobs
	if runtimeJobs == nil && deps.Database != nil {
		runtimeJobs = NewRuntimeRepository(deps.Database)
	}
	homeBuilder := deps.HomeBuilder
	if homeBuilder == nil {
		homeBuilder = runtime.NewHomeBuilder()
	}
	return &RuntimeWorker{
		database:       deps.Database,
		runtimeJobs:    runtimeJobs,
		homeBuilder:    homeBuilder,
		runner:         deps.Runner,
		templateLoader: deps.TemplateLoader,
		provider:       deps.Provider,
		hermesImage:    deps.HermesImage,
		hermesMemory:   deps.HermesMemory,
		hermesCPUs:     deps.HermesCPUs,
	}
}

func (w *RuntimeWorker) ProcessJob(ctx context.Context, jobID string) error {
	if w.database == nil || w.homeBuilder == nil || w.runner == nil {
		return errors.New("runtime worker dependencies are incomplete")
	}

	job, err := w.loadJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("load runtime job: %w", err)
	}
	switch job.Type {
	case TypeProvisionAgent:
		if err := w.processProvisionAgent(ctx, job); err != nil {
			return fmt.Errorf("process provision agent job: %w", err)
		}
		return nil
	case TypeRestartRuntime:
		if err := w.processRestartRuntime(ctx, job); err != nil {
			return fmt.Errorf("process restart runtime job: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("%w: unsupported job type: %q", ErrInvalidInput, job.Type)
	}
}

func (w *RuntimeWorker) processProvisionAgent(ctx context.Context, job runtimeJobRecord) error {
	agent, err := w.loadAgent(ctx, job.AgentID)
	if err != nil {
		return fmt.Errorf("load agent for provision: %w", err)
	}

	if agent.Status == agentStatusRunning {
		containerName := agent.RuntimeID
		if strings.TrimSpace(containerName) == "" {
			containerName = runtime.DefaultContainerName(agent.ID)
		}
		status, err := w.runner.Inspect(ctx, containerName)
		if err == nil && status.Running {
			return w.markJobSucceeded(ctx, job.ID)
		}
	}

	if agent.Status == agentStatusCreating || agent.Status == agentStatusError {
		if err := w.transitionAgent(ctx, agent.ID, agent.Status, agentStatusProvisioning, "", "", ""); err != nil {
			return fmt.Errorf("transition agent to provisioning: %w", err)
		}
		if err := w.recordEvent(ctx, agent.ID, "provisioning", agent.Status, agentStatusProvisioning, ""); err != nil {
			return fmt.Errorf("record provisioning event: %w", err)
		}
		agent.Status = agentStatusProvisioning
	} else if agent.Status != agentStatusProvisioning && agent.Status != agentStatusStarting {
		return ErrConflict
	}

	template, err := w.loadTemplate(ctx, agent)
	if err != nil {
		return w.failProvision(ctx, job, agent, runtime.ErrCodeCopyTemplateFailed, "failed to copy template files")
	}
	if _, err := w.homeBuilder.Provision(ctx, runtime.HomeSpec{
		AgentID:  agent.ID,
		HomePath: agent.HermesHomePath,
		Template: template,
		Provider: w.provider,
	}); err != nil {
		code := runtime.ErrCodeCopyTemplateFailed
		message := "failed to copy template files"
		var provisionErr *runtime.ProvisionError
		if runtime.AsProvisionError(err, &provisionErr) && provisionErr != nil {
			switch provisionErr.Code {
			case runtime.ErrCodeConfigWriteFailed:
				code = runtime.ErrCodeConfigWriteFailed
				message = "failed to write Hermes config"
			case runtime.ErrCodeCopyTemplateFailed:
				code = runtime.ErrCodeCopyTemplateFailed
			}
		}
		return w.failProvision(ctx, job, agent, code, message)
	}

	runtimeID := runtime.DefaultContainerName(agent.ID)
	if agent.Status == agentStatusProvisioning {
		if err := w.transitionAgent(ctx, agent.ID, agent.Status, agentStatusStarting, runtimeID, "", ""); err != nil {
			return fmt.Errorf("transition agent to starting: %w", err)
		}
		if err := w.recordEvent(ctx, agent.ID, "starting", agent.Status, agentStatusStarting, ""); err != nil {
			return fmt.Errorf("record starting event: %w", err)
		}
		agent.Status = agentStatusStarting
	}
	agent.RuntimeID = runtimeID

	if err := w.runner.EnsureRunning(ctx, runtime.ContainerSpec{
		AgentID:       agent.ID,
		ContainerName: runtimeID,
		HermesHome:    agent.HermesHomePath,
		Image:         w.hermesImage,
		Memory:        w.hermesMemory,
		CPUs:          w.hermesCPUs,
	}); err != nil {
		slog.Error("ECI/container start failed", "error", err, "agentID", agent.ID)
	return w.failProvision(ctx, job, agent, runtime.ErrCodeContainerStartFailed, err.Error())
	}

	if err := w.transitionAgent(ctx, agent.ID, agent.Status, agentStatusRunning, runtimeID, "", ""); err != nil {
		return fmt.Errorf("transition agent to running: %w", err)
	}
	if err := w.recordEvent(ctx, agent.ID, "running", agent.Status, agentStatusRunning, ""); err != nil {
		return fmt.Errorf("record running event: %w", err)
	}
	if err := w.markJobSucceeded(ctx, job.ID); err != nil {
		return fmt.Errorf("mark provision job succeeded: %w", err)
	}
	return nil
}

func (w *RuntimeWorker) processRestartRuntime(ctx context.Context, job runtimeJobRecord) error {
	agent, err := w.loadAgent(ctx, job.AgentID)
	if err != nil {
		return fmt.Errorf("load agent for restart: %w", err)
	}
	if agent.Status != agentStatusRunning && agent.Status != agentStatusStarting && agent.Status != agentStatusStopped && agent.Status != agentStatusError {
		return ErrConflict
	}
	containerName := agent.RuntimeID
	if strings.TrimSpace(containerName) == "" {
		containerName = runtime.DefaultContainerName(agent.ID)
	}
	if err := w.runner.Stop(ctx, containerName); err != nil && !errors.Is(err, runtime.ErrContainerNotFound) {
		return w.markJobFailed(ctx, job.ID, runtime.ErrCodeContainerStartFailed, "failed to restart Hermes container")
	}
	if err := w.runner.EnsureRunning(ctx, runtime.ContainerSpec{
		AgentID:       agent.ID,
		ContainerName: containerName,
		HermesHome:    agent.HermesHomePath,
		Image:         w.hermesImage,
		Memory:        w.hermesMemory,
		CPUs:          w.hermesCPUs,
	}); err != nil {
		return w.markJobFailed(ctx, job.ID, runtime.ErrCodeContainerStartFailed, "failed to restart Hermes container")
	}
	if err := w.transitionAgent(ctx, agent.ID, agent.Status, agentStatusRunning, containerName, "", ""); err != nil {
		if !errors.Is(err, ErrConflict) {
			return fmt.Errorf("transition restarted agent to running: %w", err)
		}
	}
	if err := w.recordEvent(ctx, agent.ID, "running", agent.Status, agentStatusRunning, ""); err != nil {
		return fmt.Errorf("record restarted running event: %w", err)
	}
	if err := w.markJobSucceeded(ctx, job.ID); err != nil {
		return fmt.Errorf("mark restart job succeeded: %w", err)
	}
	return nil
}

func (w *RuntimeWorker) failProvision(ctx context.Context, job runtimeJobRecord, agent runtimeAgentRecord, code, message string) error {
	if err := w.transitionAgent(ctx, agent.ID, agent.Status, agentStatusError, agent.RuntimeID, code, message); err != nil {
		return fmt.Errorf("transition agent to error: %w", err)
	}
	if err := w.recordEvent(ctx, agent.ID, code, agent.Status, agentStatusError, message); err != nil {
		return fmt.Errorf("record failed provision event: %w", err)
	}
	if err := w.markJobFailed(ctx, job.ID, code, message); err != nil {
		return fmt.Errorf("mark provision job failed: %w", err)
	}
	return fmt.Errorf("%s: %s", code, message)
}

func (w *RuntimeWorker) loadJob(ctx context.Context, jobID string) (runtimeJobRecord, error) {
	var job runtimeJobRecord
	err := w.database.QueryRowContext(ctx, `
		SELECT id, agent_id, type, status
		FROM runtime_jobs
		WHERE id = ?;
	`, jobID).Scan(&job.ID, &job.AgentID, &job.Type, &job.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return runtimeJobRecord{}, ErrNotFound
		}
		return runtimeJobRecord{}, err
	}
	return job, nil
}

func (w *RuntimeWorker) loadAgent(ctx context.Context, agentID string) (runtimeAgentRecord, error) {
	var agent runtimeAgentRecord
	err := w.database.QueryRowContext(ctx, `
		SELECT id, template_id, template_version, status, runtime_id, hermes_home_path
		FROM agents
		WHERE id = ?;
	`, agentID).Scan(&agent.ID, &agent.TemplateID, &agent.TemplateVersion, &agent.Status, &agent.RuntimeID, &agent.HermesHomePath)
	if err != nil {
		return runtimeAgentRecord{}, err
	}
	return agent, nil
}

func (w *RuntimeWorker) loadTemplate(ctx context.Context, agent runtimeAgentRecord) (templates.Template, error) {
	if w.templateLoader != nil {
		return w.templateLoader.LoadPublishedTemplate(ctx, agent.TemplateID, agent.TemplateVersion)
	}
	dataDir, err := dataDirFromHermesHome(agent.HermesHomePath, agent.ID)
	if err != nil {
		return templates.Template{}, err
	}
	paths := templates.NewFileStore(dataDir).Paths(agent.TemplateID, agent.TemplateVersion)
	return templates.Template{
		ID:           agent.TemplateID,
		Version:      agent.TemplateVersion,
		TemplatePath: paths.TemplatePath,
		SoulMDPath:   paths.SoulMDPath,
		UserMDPath:   paths.UserMDPath,
		SkillsPath:   paths.SkillsPath,
	}, nil
}

func (w *RuntimeWorker) transitionAgent(ctx context.Context, agentID, currentStatus, nextStatus, runtimeID, errorCode, errorMessage string) error {
	result, err := w.database.ExecContext(ctx, `
		UPDATE agents
		SET status = ?,
		    runtime_id = ?,
		    last_error_code = ?,
		    last_error_message = ?,
		    updated_at = datetime('now')
		WHERE id = ? AND status = ?;
	`, nextStatus, runtimeID, errorCode, errorMessage, agentID, currentStatus)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrConflict
	}
	return nil
}

func (w *RuntimeWorker) recordEvent(ctx context.Context, agentID, eventType, before, after, message string) error {
	metadata, err := json.Marshal(map[string]string{})
	if err != nil {
		return err
	}
	_, err = w.database.ExecContext(ctx, `
		INSERT INTO agent_runtime_events (
			id, agent_id, event_type, status_before, status_after, message, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?);
	`, uuid.NewString(), agentID, eventType, before, after, message, string(metadata))
	return err
}

func (w *RuntimeWorker) markJobSucceeded(ctx context.Context, jobID string) error {
	_, err := w.database.ExecContext(ctx, `
		UPDATE runtime_jobs
		SET status = 'succeeded',
		    last_error_code = '',
		    last_error_message = '',
		    finished_at = datetime('now'),
		    updated_at = datetime('now')
		WHERE id = ?;
	`, jobID)
	return err
}

func (w *RuntimeWorker) markJobFailed(ctx context.Context, jobID, errorCode, errorMessage string) error {
	_, err := w.database.ExecContext(ctx, `
		UPDATE runtime_jobs
		SET status = 'failed',
		    last_error_code = ?,
		    last_error_message = ?,
		    finished_at = datetime('now'),
		    updated_at = datetime('now')
		WHERE id = ?;
	`, errorCode, strings.TrimSpace(errorMessage), jobID)
	return err
}

func dataDirFromHermesHome(homePath, agentID string) (string, error) {
	cleanHome := filepath.Clean(homePath)
	expectedSuffix := filepath.Join("agents", agentID, "hermes-home")
	if !strings.HasSuffix(cleanHome, expectedSuffix) {
		return "", fmt.Errorf("invalid hermes home path %q", homePath)
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(cleanHome))), nil
}
