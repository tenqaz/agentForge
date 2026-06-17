package agents

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"agentforge.local/services/api/internal/jobs"
	"github.com/google/uuid"
)

type Service struct {
	database    *sql.DB
	repository  *Repository
	runtimeJobs *jobs.RuntimeRepository
	dataDir     string
}

func NewService(database *sql.DB, repository *Repository, runtimeJobs *jobs.RuntimeRepository, dataDir string) *Service {
	return &Service{
		database:    database,
		repository:  repository,
		runtimeJobs: runtimeJobs,
		dataDir:     dataDir,
	}
}

func (s *Service) Create(ctx context.Context, params CreateParams) (Agent, error) {
	params.OwnerUserID = strings.TrimSpace(params.OwnerUserID)
	params.TemplateID = strings.TrimSpace(params.TemplateID)
	params.Name = strings.TrimSpace(params.Name)
	if params.OwnerUserID == "" || params.TemplateID == "" || params.Name == "" {
		return Agent{}, ErrInvalidInput
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
	created, err := s.repository.Create(ctx, tx, Agent{
		ID:              agentID,
		OwnerUserID:     params.OwnerUserID,
		TemplateID:      params.TemplateID,
		TemplateVersion: templateVersion,
		Name:            params.Name,
		Status:          StatusCreating,
		HermesHomePath:  filepath.Join(s.dataDir, "agents", agentID, "hermes-home"),
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
		return jobs.RuntimeJob{}, jobs.ErrInvalidInput
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
