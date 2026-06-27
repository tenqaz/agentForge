package channels

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
)

type Repository struct {
	database *sql.DB
}

func NewRepository(database *sql.DB) *Repository {
	return &Repository{database: database}
}

func (r *Repository) GetByID(ctx context.Context, id string) (Channel, error) {
	var channel Channel
	err := r.database.QueryRowContext(ctx, `
		SELECT id, agent_id, channel_type, status, external_account_id, last_error_code, last_error_message, created_at, updated_at
		FROM agent_channels
		WHERE id = ?;
	`, id).Scan(&channel.ID, &channel.AgentID, &channel.ChannelType, &channel.Status, &channel.ExternalAccountID, &channel.LastErrorCode, &channel.LastErrorMessage, &channel.CreatedAt, &channel.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Channel{}, ErrNotFound
	}
	return channel, err
}

func (r *Repository) GetByAgentID(ctx context.Context, agentID string) (Channel, error) {
	var channel Channel
	err := r.database.QueryRowContext(ctx, `
		SELECT id, agent_id, channel_type, status, external_account_id, last_error_code, last_error_message, created_at, updated_at
		FROM agent_channels
		WHERE agent_id = ? AND channel_type = 'weixin';
	`, agentID).Scan(&channel.ID, &channel.AgentID, &channel.ChannelType, &channel.Status, &channel.ExternalAccountID, &channel.LastErrorCode, &channel.LastErrorMessage, &channel.CreatedAt, &channel.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Channel{}, ErrNotFound
	}
	return channel, err
}

func (r *Repository) CreateWeixin(ctx context.Context, agentID string) (Channel, error) {
	id := uuid.NewString()
	_, err := r.database.ExecContext(ctx, `
		INSERT INTO agent_channels (id, agent_id, channel_type, status)
		VALUES (?, ?, 'weixin', 'not_configured');
	`, id, agentID)
	if err != nil {
		return Channel{}, err
	}
	return r.GetByID(ctx, id)
}

func (r *Repository) TransitionStatus(ctx context.Context, id string, next Status, externalAccountID, lastErrorCode, lastErrorMessage string) (Channel, error) {
	channel, err := r.GetByID(ctx, id)
	if err != nil {
		return Channel{}, err
	}
	if !channel.Status.CanTransitionTo(next) {
		return Channel{}, ErrInvalidStateTransition
	}
	if next != StatusConnected {
		externalAccountID = channel.ExternalAccountID
	}
	if next != StatusError && next != StatusNotConfigured {
		lastErrorCode = ""
		lastErrorMessage = ""
	}
	result, err := r.database.ExecContext(ctx, `
		UPDATE agent_channels
		SET status = ?, external_account_id = ?, last_error_code = ?, last_error_message = ?, updated_at = datetime('now')
		WHERE id = ? AND status = ?;
	`, next, externalAccountID, lastErrorCode, lastErrorMessage, id, channel.Status)
	if err != nil {
		return Channel{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Channel{}, err
	}
	if affected == 0 {
		return Channel{}, ErrConflict
	}
	return r.GetByID(ctx, id)
}

func (r *Repository) SetPairingSession(ctx context.Context, session PairingSession) (PairingSession, error) {
	// NOTE: the qr_image_path column is a misnomer kept for compatibility
	// — it actually stores the scannable liteapp URL, mapped to
	// PairingSession.QRPayloadURL.
	_, err := r.database.ExecContext(ctx, `
		UPDATE channel_pairing_sessions
		SET status = ?, qr_payload = ?, qr_image_path = ?, expires_at = ?, attempt_count = ?, last_error_code = ?, last_error_message = ?, updated_at = datetime('now')
		WHERE id = ?;
	`, session.Status, session.QRPayload, session.QRPayloadURL, session.ExpiresAt, session.AttemptCount, session.LastErrorCode, session.LastErrorMessage, session.ID)
	if err != nil {
		return PairingSession{}, err
	}
	return r.GetPairingSessionByID(ctx, session.AgentChannelID, session.ID)
}

func (r *Repository) GetActivePairingSession(ctx context.Context, channelID string) (PairingSession, error) {
	var session PairingSession
	err := r.database.QueryRowContext(ctx, `
		SELECT id, agent_channel_id, status, qr_payload, qr_image_path, expires_at, attempt_count, last_error_code, last_error_message, created_at, updated_at
		FROM channel_pairing_sessions
		WHERE agent_channel_id = ? AND status IN ('pending', 'connected')
		ORDER BY CASE status WHEN 'pending' THEN 0 ELSE 1 END, created_at DESC, id DESC
		LIMIT 1;
	`, channelID).Scan(&session.ID, &session.AgentChannelID, &session.Status, &session.QRPayload, &session.QRPayloadURL, &session.ExpiresAt, &session.AttemptCount, &session.LastErrorCode, &session.LastErrorMessage, &session.CreatedAt, &session.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PairingSession{}, ErrNotFound
	}
	return session, err
}

func (r *Repository) ListPairingSessions(ctx context.Context, channelID string) ([]PairingSession, error) {
	rows, err := r.database.QueryContext(ctx, `
		SELECT id, agent_channel_id, status, qr_payload, qr_image_path, expires_at, attempt_count, last_error_code, last_error_message, created_at, updated_at
		FROM channel_pairing_sessions
		WHERE agent_channel_id = ?
		ORDER BY created_at DESC, id ASC;
	`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // deferred close
	var sessions []PairingSession
	for rows.Next() {
		var session PairingSession
		if err := rows.Scan(&session.ID, &session.AgentChannelID, &session.Status, &session.QRPayload, &session.QRPayloadURL, &session.ExpiresAt, &session.AttemptCount, &session.LastErrorCode, &session.LastErrorMessage, &session.CreatedAt, &session.UpdatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (r *Repository) GetPairingSessionByID(ctx context.Context, channelID, sessionID string) (PairingSession, error) {
	var session PairingSession
	err := r.database.QueryRowContext(ctx, `
		SELECT id, agent_channel_id, status, qr_payload, qr_image_path, expires_at, attempt_count, last_error_code, last_error_message, created_at, updated_at
		FROM channel_pairing_sessions
		WHERE agent_channel_id = ? AND id = ?;
	`, channelID, sessionID).Scan(&session.ID, &session.AgentChannelID, &session.Status, &session.QRPayload, &session.QRPayloadURL, &session.ExpiresAt, &session.AttemptCount, &session.LastErrorCode, &session.LastErrorMessage, &session.CreatedAt, &session.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PairingSession{}, ErrNotFound
	}
	return session, err
}

func (r *Repository) CreatePairingSession(ctx context.Context, channelID, expiresAt string) (PairingSession, error) {
	id := uuid.NewString()
	_, err := r.database.ExecContext(ctx, `
		INSERT INTO channel_pairing_sessions (id, agent_channel_id, status, expires_at)
		VALUES (?, ?, 'pending', ?);
	`, id, channelID, expiresAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return PairingSession{}, ErrConflict
		}
		return PairingSession{}, err
	}
	return r.GetPairingSessionByID(ctx, channelID, id)
}
