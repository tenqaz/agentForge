package channels

import (
	"context"
	"database/sql"
	"errors"
)

type Service struct {
	database   *sql.DB
	repository *Repository
}

func NewService(database *sql.DB, repository *Repository) *Service {
	return &Service{database: database, repository: repository}
}

func (s *Service) EnsureWeixinChannel(ctx context.Context, agentID string) (Channel, error) {
	var status string
	err := s.database.QueryRowContext(ctx, `SELECT status FROM agents WHERE id = ?;`, agentID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return Channel{}, ErrNotFound
	}
	if err != nil {
		return Channel{}, err
	}
	if status != "running" {
		return Channel{}, ErrAgentNotRunning
	}
	channel, err := s.repository.GetByAgentID(ctx, agentID)
	if err == nil {
		return channel, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Channel{}, err
	}
	return s.repository.CreateWeixin(ctx, agentID)
}
