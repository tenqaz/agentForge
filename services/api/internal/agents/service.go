package agents

import (
	"context"
	"database/sql"
	"strings"

	"agentforge.local/services/api/internal/jobs"
	"github.com/google/uuid"
)

type Service struct {
	database    *sql.DB
	repository  *Repository
	runtimeJobs *jobs.RuntimeRepository
}

func NewService(database *sql.DB, repository *Repository, runtimeJobs *jobs.RuntimeRepository) *Service {
	return &Service{
		database:    database,
		repository:  repository,
		runtimeJobs: runtimeJobs,
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
		return Agent{}, err
	}
	defer func() { _ = tx.Rollback() }()

	templateVersion, err := s.repository.TemplateVersion(ctx, tx, params.TemplateID)
	if err != nil {
		return Agent{}, err
	}

	agentID := uuid.NewString()
	created, err := s.repository.Create(ctx, tx, Agent{
		ID:              agentID,
		OwnerUserID:     params.OwnerUserID,
		TemplateID:      params.TemplateID,
		TemplateVersion: templateVersion,
		Name:            params.Name,
		Status:          StatusCreating,
		HermesHomePath:  "/var/lib/agentforge/hermes/" + agentID,
	})
	if err != nil {
		return Agent{}, err
	}

	if _, err := s.runtimeJobs.CreateQueuedTx(ctx, tx, jobs.RuntimeJob{
		ID:      uuid.NewString(),
		AgentID: created.ID,
		Type:    jobs.TypeProvisionAgent,
	}); err != nil {
		return Agent{}, err
	}

	if err := tx.Commit(); err != nil {
		return Agent{}, err
	}
	return created, nil
}

func (s *Service) Get(ctx context.Context, id string) (Agent, error) {
	return s.repository.Get(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]Agent, error) {
	return s.repository.List(ctx)
}

func (s *Service) ListByOwner(ctx context.Context, ownerUserID string) ([]Agent, error) {
	return s.repository.ListByOwner(ctx, ownerUserID)
}

func (s *Service) Runtime(ctx context.Context, id string) (Runtime, error) {
	agent, err := s.repository.Get(ctx, id)
	if err != nil {
		return Runtime{}, err
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
