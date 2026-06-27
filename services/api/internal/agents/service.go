package agents

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agentforge.local/services/api/internal/jobs"
	"agentforge.local/services/api/internal/runtime"
	"github.com/google/uuid"
)

type Service struct {
	database    *sql.DB
	repository  *Repository
	runtimeJobs *jobs.RuntimeRepository
	runner      runtime.Runner
	dataDir     string
	runnerMode  string

	// Global defaults used when recreating containers (Wake).
	hermesImage  string
	hermesMemory string
	hermesCPUs   string
}

func NewService(database *sql.DB, repository *Repository, runtimeJobs *jobs.RuntimeRepository, runner runtime.Runner, dataDir, runnerMode, hermesImage, hermesMemory, hermesCPUs string) *Service {
	return &Service{
		database:    database,
		repository:  repository,
		runtimeJobs: runtimeJobs,
		runner:      runner,
		dataDir:     dataDir,
		runnerMode:  runnerMode,
		hermesImage: hermesImage,
		hermesMemory: hermesMemory,
		hermesCPUs:  hermesCPUs,
	}
}

func (s *Service) Create(ctx context.Context, params CreateParams) (Agent, error) {
	params.OwnerUserID = strings.TrimSpace(params.OwnerUserID)
	params.TemplateID = strings.TrimSpace(params.TemplateID)
	params.Name = strings.TrimSpace(params.Name)
	if params.OwnerUserID == "" {
		return Agent{}, fmt.Errorf("%w: owner user ID cannot be empty", ErrInvalidInput)
	}
	if params.TemplateID == "" {
		return Agent{}, fmt.Errorf("%w: template ID cannot be empty", ErrInvalidInput)
	}
	if params.Name == "" {
		return Agent{}, fmt.Errorf("%w: agent name cannot be empty", ErrInvalidInput)
	}

	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return Agent{}, fmt.Errorf("begin agent create transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	templateVersion, err := s.repository.TemplateVersion(ctx, tx, params.TemplateID)
	if err != nil {
		return Agent{}, fmt.Errorf("load template version for agent create: %w", err)
	}

	agentID := uuid.NewString()
	// In ECI mode the agent directory maps directly to NAS /hermes-home/{agentID}
	// (bind-mounted via /mnt/nas/hermes-home:/data/agents). In Docker mode we
	// keep the legacy hermes-home subdirectory for backward compatibility.
	homePath := filepath.Join(s.dataDir, "agents", agentID, "hermes-home")
	if s.runnerMode == "eci" {
		homePath = filepath.Join(s.dataDir, "agents", agentID)
	}
	created, err := s.repository.Create(ctx, tx, Agent{
		ID:              agentID,
		OwnerUserID:     params.OwnerUserID,
		TemplateID:      params.TemplateID,
		TemplateVersion: templateVersion,
		Name:            params.Name,
		Status:          StatusCreating,
		HermesHomePath:  homePath,
	})
	if err != nil {
		return Agent{}, fmt.Errorf("create agent: %w", err)
	}

	if _, err := s.runtimeJobs.CreateQueuedTx(ctx, tx, jobs.RuntimeJob{
		ID:      uuid.NewString(),
		AgentID: created.ID,
		Type:    jobs.TypeProvisionAgent,
	}); err != nil {
		return Agent{}, fmt.Errorf("create provision job for agent: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Agent{}, fmt.Errorf("commit agent create transaction: %w", err)
	}
	return created, nil
}

func (s *Service) Get(ctx context.Context, id string) (Agent, error) {
	agent, err := s.repository.Get(ctx, id)
	if err != nil {
		return Agent{}, fmt.Errorf("get agent: %w", err)
	}
	return agent, nil
}

func (s *Service) List(ctx context.Context) ([]Agent, error) {
	agents, err := s.repository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	return agents, nil
}

func (s *Service) ListByOwner(ctx context.Context, ownerUserID string) ([]Agent, error) {
	agents, err := s.repository.ListByOwner(ctx, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("list agents by owner: %w", err)
	}
	return agents, nil
}

func (s *Service) Runtime(ctx context.Context, id string) (Runtime, error) {
	agent, err := s.repository.Get(ctx, id)
	if err != nil {
		return Runtime{}, fmt.Errorf("get agent runtime: %w", err)
	}
	return Runtime{
		AgentID:          agent.ID,
		RuntimeID:        agent.RuntimeID,
		Status:           agent.Status,
		LastErrorCode:    agent.LastErrorCode,
		LastErrorMessage: agent.LastErrorMessage,
		UpdatedAt:        agent.UpdatedAt,
	}, nil
}

func (s *Service) CreateRuntimeJob(ctx context.Context, agentID string, jobType jobs.Type) (jobs.RuntimeJob, error) {
	if jobType != jobs.TypeRestartRuntime {
		return jobs.RuntimeJob{}, fmt.Errorf("%w: unsupported job type: %q", jobs.ErrInvalidInput, jobType)
	}
	agent, err := s.repository.Get(ctx, agentID)
	if err != nil {
		return jobs.RuntimeJob{}, fmt.Errorf("get agent for runtime job: %w", err)
	}
	if !agent.Status.CanRestartRuntime() || strings.TrimSpace(agent.RuntimeID) == "" {
		return jobs.RuntimeJob{}, ErrRuntimeUnavailable
	}
	job, err := s.runtimeJobs.CreateQueued(ctx, jobs.RuntimeJob{
		AgentID: agent.ID,
		Type:    jobType,
	})
	if err != nil {
		return jobs.RuntimeJob{}, fmt.Errorf("create runtime job: %w", err)
	}
	return job, nil
}

// Delete cleans up an agent's container, hermes-home directory, and
// database row in that order. Each external side-effect stage is
// idempotent, so a partially-completed deletion can be retried safely
// (the agent will be in StatusError, which CanDelete allows).
//
// This method follows the single-handling rule: it never logs; the HTTP
// handler is the sole logging point.
func (s *Service) Delete(ctx context.Context, agentID string) error {
	agent, err := s.repository.Get(ctx, agentID)
	if err != nil {
		return fmt.Errorf("get agent for delete: %w", err)
	}
	if !agent.Status.CanDelete() {
		return fmt.Errorf("%w: status=%s", ErrCannotDelete, agent.Status)
	}
	hasUnfinished, err := s.runtimeJobs.HasUnfinishedByAgent(ctx, agentID)
	if err != nil {
		return fmt.Errorf("check unfinished jobs: %w", err)
	}
	if hasUnfinished {
		return ErrHasUnfinishedJobs
	}

	// Destroy the ECI/Docker container first and wait until it is fully
	// gone. For ECI this polls until the container group disappears, so
	// the NFS mount is released before we clean up files below.
	containerName := runtime.DefaultContainerName(agentID)
	if err := s.runner.Destroy(ctx, containerName); err != nil {
		return s.failWith(ctx, agentID, DeleteFailureRemove,
			fmt.Errorf("destroy container: %w", err))
	}

	if err := runtime.DestroyHome(agent.HermesHomePath); err != nil {
		return s.failWith(ctx, agentID, DeleteFailureHome,
			fmt.Errorf("destroy hermes home: %w", err))
	}

	if err := s.repository.Delete(ctx, agentID); err != nil {
		return fmt.Errorf("delete agent from database: %w", err)
	}
	return nil
}

// failWith records the deletion failure on the agent row and returns the
// original error. If recording itself fails, both errors are joined so
// neither is lost.
func (s *Service) failWith(ctx context.Context, agentID, code string, original error) error {
	msg := original.Error()
	if markErr := s.repository.MarkDeleteFailed(ctx, agentID, code, msg); markErr != nil {
		return errors.Join(original, fmt.Errorf("mark agent delete failed: %w", markErr))
	}
	return original
}

// Sleep destroys the agent's container and transitions its status from
// running to sleeping. If destroying the container fails, the agent stays
// in running so IdleDetector can retry on the next tick — it is never
// forced into error by a sleep failure.
func (s *Service) Sleep(ctx context.Context, agentID string) error {
	agent, err := s.repository.Get(ctx, agentID)
	if err != nil {
		return fmt.Errorf("sleep: get agent: %w", err)
	}
	if agent.Status != StatusRunning {
		return fmt.Errorf("sleep: agent not running, current status: %s", agent.Status)
	}

	containerName := runtime.DefaultContainerName(agentID)
	if err := s.runner.Destroy(ctx, containerName); err != nil {
		return fmt.Errorf("sleep: destroy container: %w", err)
	}

	// Clean up heartbeat so a stale file doesn't fool the next Wake.
	if err := os.Remove(filepath.Join(agent.HermesHomePath, ".heartbeat")); err != nil && !os.IsNotExist(err) {
		slog.Warn("sleep: remove heartbeat", "agent", agentID, "error", err)
	}

	if _, err := s.repository.TransitionStatus(ctx, agentID, StatusSleeping, "", "", ""); err != nil {
		return fmt.Errorf("sleep: transition to sleeping: %w", err)
	}
	return nil
}

// Wake creates a container for a sleeping agent, waits for the gateway to
// confirm startup (heartbeat file appears), then transitions the agent to
// running. If any step fails, the agent goes back to sleeping for a retry.
func (s *Service) Wake(ctx context.Context, agentID string) error {
	agent, err := s.repository.Get(ctx, agentID)
	if err != nil {
		return fmt.Errorf("wake: get agent: %w", err)
	}
	if agent.Status != StatusSleeping {
		return fmt.Errorf("wake: agent not sleeping, current status: %s", agent.Status)
	}

	// sleeping → waking
	if _, err := s.repository.TransitionStatus(ctx, agentID, StatusWaking, "", "", ""); err != nil {
		return fmt.Errorf("wake: transition to waking: %w", err)
	}

	// Spin up the container.
	if err := s.runner.EnsureRunning(ctx, runtime.ContainerSpec{
		AgentID:    agentID,
		HermesHome: agent.HermesHomePath,
		Image:      s.hermesImage,
		Memory:     s.hermesMemory,
		CPUs:       s.hermesCPUs,
	}); err != nil {
		// Start failed — go back to sleeping so SleepPoller retries.
		if _, rollbackErr := s.repository.TransitionStatus(ctx, agentID, StatusSleeping, "", "", ""); rollbackErr != nil {
			slog.Error("wake: rollback to sleeping failed", "agent", agentID, "error", rollbackErr)
		}
		return fmt.Errorf("wake: ensure running: %w", err)
	}

	// Wait for the gateway to confirm it has started. The heartbeat
	// background loop in the container command touches .heartbeat every
	// 30s, so we wait up to 60s for the first touch.
	heartbeatPath := filepath.Join(agent.HermesHomePath, ".heartbeat")
	if err := waitForHeartbeat(ctx, heartbeatPath, 60*time.Second); err != nil {
		if _, rollbackErr := s.repository.TransitionStatus(ctx, agentID, StatusSleeping, "", "", ""); rollbackErr != nil {
			slog.Error("wake: rollback to sleeping failed", "agent", agentID, "error", rollbackErr)
		}
		return fmt.Errorf("wake: heartbeat wait: %w", err)
	}

	// waking → running — now visible to IdleDetector.
	if _, err := s.repository.TransitionStatus(ctx, agentID, StatusRunning, "", "", ""); err != nil {
		return fmt.Errorf("wake: transition to running: %w", err)
	}
	return nil
}

// waitForHeartbeat blocks until the heartbeat file exists or the timeout
// expires, polling every 500ms.
func waitForHeartbeat(ctx context.Context, path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for heartbeat file %q", path)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}
