package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"agentforge.local/services/api/internal/channels"
	"github.com/google/uuid"
)

type ChannelJobType string

const (
	TypeConnectWeixin    ChannelJobType = "connect_weixin"
	TypeDisconnectWeixin ChannelJobType = "disconnect_weixin"
	TypeRefreshWeixin    ChannelJobType = "refresh_weixin_pairing"
)

type ChannelJob struct {
	ID               string         `json:"id"`
	AgentChannelID   string         `json:"agentChannelId"`
	PairingSessionID *string        `json:"pairingSessionId,omitempty"`
	Type             ChannelJobType `json:"type"`
	Status           Status         `json:"status"`
	Priority         int            `json:"priority"`
	AttemptCount     int            `json:"attemptCount"`
	MaxAttempts      int            `json:"maxAttempts"`
	LockedBy         string         `json:"lockedBy"`
	LockedUntil      *string        `json:"lockedUntil,omitempty"`
	LastErrorCode    string         `json:"lastErrorCode"`
	LastErrorMessage string         `json:"lastErrorMessage"`
	CreatedAt        string         `json:"createdAt"`
	UpdatedAt        string         `json:"updatedAt"`
	StartedAt        *string        `json:"startedAt,omitempty"`
	FinishedAt       *string        `json:"finishedAt,omitempty"`
}

type ChannelRepository struct {
	database *sql.DB
}

func NewChannelRepository(database *sql.DB) *ChannelRepository {
	return &ChannelRepository{database: database}
}

func (r *ChannelRepository) CreateQueued(ctx context.Context, job ChannelJob) (ChannelJob, error) {
	if strings.TrimSpace(job.AgentChannelID) == "" {
		return ChannelJob{}, fmt.Errorf("%w: agent channel ID cannot be empty", ErrInvalidInput)
	}
	if !isValidChannelJobType(job.Type) {
		return ChannelJob{}, fmt.Errorf("%w: invalid channel job type: %q", ErrInvalidInput, job.Type)
	}
	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	if job.MaxAttempts == 0 {
		job.MaxAttempts = 3
	}
	_, err := r.database.ExecContext(ctx, `
		INSERT INTO channel_jobs (id, agent_channel_id, pairing_session_id, type, status, priority, attempt_count, max_attempts, locked_by)
		VALUES (?, ?, ?, ?, 'queued', ?, 0, ?, '');
	`, job.ID, job.AgentChannelID, job.PairingSessionID, job.Type, job.Priority, job.MaxAttempts)
	if err != nil {
		if isUniqueConstraint(err) {
			return ChannelJob{}, ErrConflict
		}
		return ChannelJob{}, err
	}
	return r.GetByID(ctx, job.AgentChannelID, job.ID)
}

func (r *ChannelRepository) GetByID(ctx context.Context, channelID, jobID string) (ChannelJob, error) {
	row := r.database.QueryRowContext(ctx, `
		SELECT id, agent_channel_id, pairing_session_id, type, status, priority, attempt_count, max_attempts, locked_by,
		       locked_until, last_error_code, last_error_message, created_at, updated_at, started_at, finished_at
		FROM channel_jobs
		WHERE agent_channel_id = ? AND id = ?;
	`, channelID, jobID)
	job, err := scanChannelJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ChannelJob{}, ErrNotFound
	}
	return job, err
}

func (r *ChannelRepository) ClaimNextQueued(ctx context.Context, workerID string, lockedUntil time.Time) (ChannelJob, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return ChannelJob{}, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		UPDATE channel_jobs
		SET status = 'running',
		    locked_by = ?,
		    locked_until = ?,
		    started_at = COALESCE(started_at, CURRENT_TIMESTAMP),
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = (
		    SELECT id FROM channel_jobs
		    WHERE status = 'queued'
		      AND (locked_until IS NULL OR locked_until < CURRENT_TIMESTAMP)
		    ORDER BY priority DESC, created_at ASC
		    LIMIT 1
		)
		RETURNING id, agent_channel_id, pairing_session_id, type, status, priority, attempt_count, max_attempts, locked_by,
		          locked_until, last_error_code, last_error_message, created_at, updated_at, started_at, finished_at;
	`, workerID, lockedUntil.UTC().Format("2006-01-02 15:04:05"))
	job, err := scanChannelJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ChannelJob{}, ErrNotFound
	}
	if err != nil {
		return ChannelJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return ChannelJob{}, err
	}
	return job, nil
}

func (r *ChannelRepository) ExtendLease(ctx context.Context, channelID, jobID, workerID string, lockedUntil time.Time) error {
	result, err := r.database.ExecContext(ctx, `
		UPDATE channel_jobs
		SET locked_until = ?, updated_at = CURRENT_TIMESTAMP
		WHERE agent_channel_id = ? AND id = ? AND status = 'running' AND locked_by = ?;
	`, lockedUntil.UTC().Format("2006-01-02 15:04:05"), channelID, jobID, workerID)
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

func (r *ChannelRepository) CreateOrReuseConnectJob(ctx context.Context, channelID string, expiresAt time.Time) (channels.PairingSession, ChannelJob, bool, error) {
	session, err := r.getActivePairingSession(ctx, channelID)
	if err == nil {
		job, err := r.getActiveJobByChannel(ctx, channelID)
		if errors.Is(err, ErrNotFound) {
			job, err = r.CreateQueued(ctx, ChannelJob{
				AgentChannelID: channelID,
				PairingSessionID: &session.ID,
				Type:           TypeConnectWeixin,
			})
			return session, job, true, err
		}
		return session, job, false, err
	}
	if !errors.Is(err, ErrNotFound) {
		return channels.PairingSession{}, ChannelJob{}, false, err
	}

	session, err = r.createPairingSession(ctx, channelID, expiresAt)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			session, err = r.getActivePairingSession(ctx, channelID)
			if err != nil {
				return channels.PairingSession{}, ChannelJob{}, false, err
			}
			job, err := r.getActiveJobByChannel(ctx, channelID)
			return session, job, false, err
		}
		return channels.PairingSession{}, ChannelJob{}, false, err
	}
	job, err := r.CreateQueued(ctx, ChannelJob{
		AgentChannelID: channelID,
		PairingSessionID: &session.ID,
		Type:           TypeConnectWeixin,
	})
	return session, job, true, err
}

func (r *ChannelRepository) MarkSucceeded(ctx context.Context, jobID string) error {
	_, err := r.database.ExecContext(ctx, `
		UPDATE channel_jobs
		SET status = 'succeeded', last_error_code = '', last_error_message = '', finished_at = datetime('now'), updated_at = datetime('now')
		WHERE id = ?;
	`, jobID)
	return err
}

func (r *ChannelRepository) MarkFailed(ctx context.Context, jobID, code, message string) error {
	_, err := r.database.ExecContext(ctx, `
		UPDATE channel_jobs
		SET status = 'failed', last_error_code = ?, last_error_message = ?, finished_at = datetime('now'), updated_at = datetime('now')
		WHERE id = ?;
	`, code, message, jobID)
	return err
}

func scanChannelJob(scanner interface{ Scan(dest ...any) error }) (ChannelJob, error) {
	var job ChannelJob
	var pairingSessionID sql.NullString
	var lockedUntil sql.NullString
	var startedAt sql.NullString
	var finishedAt sql.NullString
	err := scanner.Scan(&job.ID, &job.AgentChannelID, &pairingSessionID, &job.Type, &job.Status, &job.Priority, &job.AttemptCount, &job.MaxAttempts, &job.LockedBy, &lockedUntil, &job.LastErrorCode, &job.LastErrorMessage, &job.CreatedAt, &job.UpdatedAt, &startedAt, &finishedAt)
	if err != nil {
		return ChannelJob{}, err
	}
	if pairingSessionID.Valid {
		job.PairingSessionID = &pairingSessionID.String
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

func isValidChannelJobType(jobType ChannelJobType) bool {
	switch jobType {
	case TypeConnectWeixin, TypeDisconnectWeixin, TypeRefreshWeixin:
		return true
	default:
		return false
	}
}

func (r *ChannelRepository) getActiveJobByChannel(ctx context.Context, channelID string) (ChannelJob, error) {
	row := r.database.QueryRowContext(ctx, `
		SELECT id, agent_channel_id, pairing_session_id, type, status, priority, attempt_count, max_attempts, locked_by,
		       locked_until, last_error_code, last_error_message, created_at, updated_at, started_at, finished_at
		FROM channel_jobs
		WHERE agent_channel_id = ? AND status IN ('queued', 'running')
		ORDER BY created_at ASC
		LIMIT 1;
	`, channelID)
	job, err := scanChannelJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ChannelJob{}, ErrNotFound
	}
	return job, err
}

func (r *ChannelRepository) getActivePairingSession(ctx context.Context, channelID string) (channels.PairingSession, error) {
	var session channels.PairingSession
	err := r.database.QueryRowContext(ctx, `
		SELECT id, agent_channel_id, status, qr_payload, qr_image_path, expires_at, attempt_count, last_error_code, last_error_message, created_at, updated_at
		FROM channel_pairing_sessions
		WHERE agent_channel_id = ? AND status = 'pending';
	`, channelID).Scan(&session.ID, &session.AgentChannelID, &session.Status, &session.QRPayload, &session.QRImagePath, &session.ExpiresAt, &session.AttemptCount, &session.LastErrorCode, &session.LastErrorMessage, &session.CreatedAt, &session.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return channels.PairingSession{}, ErrNotFound
	}
	return session, err
}

func (r *ChannelRepository) createPairingSession(ctx context.Context, channelID string, expiresAt time.Time) (channels.PairingSession, error) {
	id := uuid.NewString()
	_, err := r.database.ExecContext(ctx, `
		INSERT INTO channel_pairing_sessions (id, agent_channel_id, status, expires_at)
		VALUES (?, ?, 'pending', ?);
	`, id, channelID, expiresAt.UTC().Format(time.RFC3339))
	if err != nil {
		if isUniqueConstraint(err) {
			return channels.PairingSession{}, ErrConflict
		}
		return channels.PairingSession{}, err
	}
	var session channels.PairingSession
	err = r.database.QueryRowContext(ctx, `
		SELECT id, agent_channel_id, status, qr_payload, qr_image_path, expires_at, attempt_count, last_error_code, last_error_message, created_at, updated_at
		FROM channel_pairing_sessions
		WHERE id = ?;
	`, id).Scan(&session.ID, &session.AgentChannelID, &session.Status, &session.QRPayload, &session.QRImagePath, &session.ExpiresAt, &session.AttemptCount, &session.LastErrorCode, &session.LastErrorMessage, &session.CreatedAt, &session.UpdatedAt)
	return session, err
}
