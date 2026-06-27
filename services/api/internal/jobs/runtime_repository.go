package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Type string
type Status string

const (
	TypeProvisionAgent Type = "provision_agent"
	TypeStartRuntime   Type = "start_runtime"
	TypeStopRuntime    Type = "stop_runtime"
	TypeRestartRuntime Type = "restart_runtime"
)

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

var (
	ErrNotFound     = errors.New("runtime job not found")
	ErrConflict     = errors.New("runtime job conflict")
	ErrInvalidInput = errors.New("invalid runtime job input")
)

type RuntimeJob struct {
	ID               string  `json:"id"`
	AgentID          string  `json:"agentId"`
	Type             Type    `json:"type"`
	Status           Status  `json:"status"`
	Priority         int     `json:"priority"`
	AttemptCount     int     `json:"attemptCount"`
	MaxAttempts      int     `json:"maxAttempts"`
	LockedBy         string  `json:"lockedBy"`
	LockedUntil      *string `json:"lockedUntil,omitempty"`
	IdempotencyKey   string  `json:"idempotencyKey"`
	LastErrorCode    string  `json:"lastErrorCode"`
	LastErrorMessage string  `json:"lastErrorMessage"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
	StartedAt        *string `json:"startedAt,omitempty"`
	FinishedAt       *string `json:"finishedAt,omitempty"`
}

type rowScanner interface {
	Scan(dest ...any) error
}

type queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type RuntimeRepository struct {
	database *sql.DB
}

func NewRuntimeRepository(database *sql.DB) *RuntimeRepository {
	return &RuntimeRepository{database: database}
}

func (r *RuntimeRepository) CreateQueued(ctx context.Context, job RuntimeJob) (RuntimeJob, error) {
	return r.createQueued(ctx, r.database, job)
}

func (r *RuntimeRepository) CreateQueuedTx(ctx context.Context, tx *sql.Tx, job RuntimeJob) (RuntimeJob, error) {
	return r.createQueued(ctx, tx, job)
}

func (r *RuntimeRepository) GetByID(ctx context.Context, agentID, jobID string) (RuntimeJob, error) {
	row := r.database.QueryRowContext(ctx, `
		SELECT id, agent_id, type, status, priority, attempt_count, max_attempts, locked_by,
		       locked_until, idempotency_key, last_error_code, last_error_message,
		       created_at, updated_at, started_at, finished_at
		FROM runtime_jobs
		WHERE agent_id = ? AND id = ?;
	`, agentID, jobID)
	job, err := scanRuntimeJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return RuntimeJob{}, ErrNotFound
	}
	if err != nil {
		return RuntimeJob{}, err
	}
	return job, nil
}

func (r *RuntimeRepository) ListByAgent(ctx context.Context, agentID string) ([]RuntimeJob, error) {
	rows, err := r.database.QueryContext(ctx, `
		SELECT id, agent_id, type, status, priority, attempt_count, max_attempts, locked_by,
		       locked_until, idempotency_key, last_error_code, last_error_message,
		       created_at, updated_at, started_at, finished_at
		FROM runtime_jobs
		WHERE agent_id = ?
		ORDER BY created_at DESC, id ASC;
	`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // deferred close

	var runtimeJobs []RuntimeJob
	for rows.Next() {
		job, err := scanRuntimeJob(rows)
		if err != nil {
			return nil, err
		}
		runtimeJobs = append(runtimeJobs, job)
	}
	return runtimeJobs, rows.Err()
}

func (r *RuntimeRepository) ClaimNextQueued(ctx context.Context, workerID string, lockedUntil time.Time) (RuntimeJob, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return RuntimeJob{}, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		UPDATE runtime_jobs
		SET status = 'running',
		    locked_by = ?,
		    locked_until = ?,
		    started_at = COALESCE(started_at, CURRENT_TIMESTAMP),
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = (
		    SELECT id FROM runtime_jobs
		    WHERE status = 'queued'
		      AND (locked_until IS NULL OR locked_until < CURRENT_TIMESTAMP)
		    ORDER BY priority DESC, created_at ASC
		    LIMIT 1
		)
		RETURNING id, agent_id, type, status, priority, attempt_count, max_attempts, locked_by,
		          locked_until, idempotency_key, last_error_code, last_error_message,
		          created_at, updated_at, started_at, finished_at;
	`, workerID, lockedUntil.UTC().Format("2006-01-02 15:04:05"))
	job, err := scanRuntimeJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return RuntimeJob{}, ErrNotFound
	}
	if err != nil {
		return RuntimeJob{}, err
	}

	if err := tx.Commit(); err != nil {
		return RuntimeJob{}, err
	}
	return job, nil
}

func (r *RuntimeRepository) ExtendLease(ctx context.Context, agentID, jobID, workerID string, lockedUntil time.Time) error {
	result, err := r.database.ExecContext(ctx, `
		UPDATE runtime_jobs
		SET locked_until = ?, updated_at = CURRENT_TIMESTAMP
		WHERE agent_id = ? AND id = ? AND status = 'running' AND locked_by = ?;
	`, lockedUntil.UTC().Format("2006-01-02 15:04:05"), agentID, jobID, workerID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *RuntimeRepository) MarkFailed(ctx context.Context, agentID, jobID, code, message string) error {
	result, err := r.database.ExecContext(ctx, `
		UPDATE runtime_jobs
		SET status = 'failed',
		    last_error_code = ?,
		    last_error_message = ?,
		    finished_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE agent_id = ? AND id = ?;
	`, code, message, agentID, jobID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *RuntimeRepository) createQueued(ctx context.Context, db queryer, job RuntimeJob) (RuntimeJob, error) {
	if strings.TrimSpace(job.AgentID) == "" {
		return RuntimeJob{}, fmt.Errorf("%w: agent ID cannot be empty", ErrInvalidInput)
	}
	if !isValidType(job.Type) {
		return RuntimeJob{}, fmt.Errorf("%w: invalid job type: %q", ErrInvalidInput, job.Type)
	}
	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	if job.MaxAttempts == 0 {
		job.MaxAttempts = 3
	}

	_, err := db.ExecContext(ctx, `
		INSERT INTO runtime_jobs (
			id, agent_id, type, status, priority, attempt_count, max_attempts, locked_by,
			locked_until, idempotency_key, payload_json, result_json, last_error_code,
			last_error_message
		) VALUES (?, ?, ?, 'queued', ?, 0, ?, '', NULL, ?, '{}', '{}', '', '');
	`, job.ID, job.AgentID, job.Type, job.Priority, job.MaxAttempts, job.IdempotencyKey)
	if err != nil {
		if isUniqueConstraint(err) {
			return RuntimeJob{}, ErrConflict
		}
		return RuntimeJob{}, err
	}

	row := db.QueryRowContext(ctx, `
		SELECT id, agent_id, type, status, priority, attempt_count, max_attempts, locked_by,
		       locked_until, idempotency_key, last_error_code, last_error_message,
		       created_at, updated_at, started_at, finished_at
		FROM runtime_jobs
		WHERE agent_id = ? AND id = ?;
	`, job.AgentID, job.ID)
	return scanRuntimeJob(row)
}

func scanRuntimeJob(scanner rowScanner) (RuntimeJob, error) {
	var job RuntimeJob
	var lockedUntil sql.NullString
	var startedAt sql.NullString
	var finishedAt sql.NullString
	err := scanner.Scan(
		&job.ID, &job.AgentID, &job.Type, &job.Status, &job.Priority, &job.AttemptCount,
		&job.MaxAttempts, &job.LockedBy, &lockedUntil, &job.IdempotencyKey,
		&job.LastErrorCode, &job.LastErrorMessage, &job.CreatedAt, &job.UpdatedAt,
		&startedAt, &finishedAt,
	)
	if err != nil {
		return RuntimeJob{}, err
	}
	if lockedUntil.Valid {
		job.LockedUntil = &lockedUntil.String
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.String
	}
	if finishedAt.Valid {
		job.FinishedAt = &finishedAt.String
	}
	return job, nil
}

func isValidType(jobType Type) bool {
	switch jobType {
	case TypeProvisionAgent, TypeStartRuntime, TypeStopRuntime, TypeRestartRuntime:
		return true
	default:
		return false
	}
}

func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}

// HasUnfinishedByAgent reports whether the agent has any runtime job that
// is still queued or running (i.e. not in a terminal state).
func (r *RuntimeRepository) HasUnfinishedByAgent(ctx context.Context, agentID string) (bool, error) {
	var count int
	err := r.database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM runtime_jobs
		WHERE agent_id = ? AND status IN (?, ?);
	`, agentID, StatusQueued, StatusRunning).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("count unfinished runtime jobs: %w", err)
	}
	return count > 0, nil
}
