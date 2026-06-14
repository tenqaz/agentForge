package agents

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

type queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Repository struct {
	database *sql.DB
}

func NewRepository(database *sql.DB) *Repository {
	return &Repository{database: database}
}

func (r *Repository) Create(ctx context.Context, db queryer, agent Agent) (Agent, error) {
	_, err := db.ExecContext(ctx, `
		INSERT INTO agents (
			id, owner_user_id, template_id, template_version, name, status, runtime_id,
			hermes_home_path, last_error_code, last_error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`, agent.ID, agent.OwnerUserID, agent.TemplateID, agent.TemplateVersion, agent.Name, agent.Status,
		agent.RuntimeID, agent.HermesHomePath, agent.LastErrorCode, agent.LastErrorMessage)
	if err != nil {
		if isUniqueConstraint(err) {
			return Agent{}, ErrConflict
		}
		return Agent{}, err
	}
	return r.get(ctx, db, agent.ID)
}

func (r *Repository) Get(ctx context.Context, id string) (Agent, error) {
	return r.get(ctx, r.database, id)
}

func (r *Repository) List(ctx context.Context) ([]Agent, error) {
	return r.list(ctx, r.database, "")
}

func (r *Repository) ListByOwner(ctx context.Context, ownerUserID string) ([]Agent, error) {
	return r.list(ctx, r.database, ownerUserID)
}

func (r *Repository) TemplateVersion(ctx context.Context, db queryer, templateID string) (int, error) {
	var version int
	err := db.QueryRowContext(ctx, `
		SELECT version
		FROM agent_templates
		WHERE id = ?;
	`, templateID).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrTemplateNotFound
	}
	if err != nil {
		return 0, err
	}
	return version, nil
}

func (r *Repository) TransitionStatus(ctx context.Context, id string, next Status, lastErrorCode, lastErrorMessage, runtimeID string) (Agent, error) {
	agent, err := r.Get(ctx, id)
	if err != nil {
		return Agent{}, err
	}
	if !agent.Status.CanTransitionTo(next) {
		return Agent{}, ErrInvalidStateTransition
	}
	if runtimeID == "" {
		runtimeID = agent.RuntimeID
	}
	if next != StatusError {
		lastErrorCode = ""
		lastErrorMessage = ""
	}

	result, err := r.database.ExecContext(ctx, `
		UPDATE agents
		SET status = ?, runtime_id = ?, last_error_code = ?, last_error_message = ?, updated_at = datetime('now')
		WHERE id = ?;
	`, next, runtimeID, lastErrorCode, lastErrorMessage, id)
	if err != nil {
		return Agent{}, err
	}
	if err := requireAffected(result); err != nil {
		return Agent{}, err
	}
	return r.Get(ctx, id)
}

func (r *Repository) get(ctx context.Context, db queryer, id string) (Agent, error) {
	var agent Agent
	err := db.QueryRowContext(ctx, `
		SELECT id, owner_user_id, template_id, template_version, name, status, runtime_id,
		       hermes_home_path, last_error_code, last_error_message, created_at, updated_at
		FROM agents
		WHERE id = ?;
	`, id).Scan(
		&agent.ID, &agent.OwnerUserID, &agent.TemplateID, &agent.TemplateVersion, &agent.Name,
		&agent.Status, &agent.RuntimeID, &agent.HermesHomePath, &agent.LastErrorCode,
		&agent.LastErrorMessage, &agent.CreatedAt, &agent.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Agent{}, ErrNotFound
	}
	if err != nil {
		return Agent{}, err
	}
	return agent, nil
}

func (r *Repository) list(ctx context.Context, db queryer, ownerUserID string) ([]Agent, error) {
	query := `
		SELECT id, owner_user_id, template_id, template_version, name, status, runtime_id,
		       hermes_home_path, last_error_code, last_error_message, created_at, updated_at
		FROM agents`
	var args []any
	if ownerUserID != "" {
		query += " WHERE owner_user_id = ?"
		args = append(args, ownerUserID)
	}
	query += " ORDER BY created_at DESC, id ASC;"

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		var agent Agent
		if err := rows.Scan(
			&agent.ID, &agent.OwnerUserID, &agent.TemplateID, &agent.TemplateVersion, &agent.Name,
			&agent.Status, &agent.RuntimeID, &agent.HermesHomePath, &agent.LastErrorCode,
			&agent.LastErrorMessage, &agent.CreatedAt, &agent.UpdatedAt,
		); err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	return agents, rows.Err()
}

func requireAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}
