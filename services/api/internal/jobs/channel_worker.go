package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"agentforge.local/services/api/internal/channels"
	"agentforge.local/services/api/internal/runtime"
	"agentforge.local/services/api/internal/weixin"
)

type ChannelWorkerDependencies struct {
	Database           *sql.DB
	ChannelJobs        *ChannelRepository
	Channels           *channels.Repository
	WeixinClient       weixin.Client
	Runner             runtime.Runner
	PollInterval       time.Duration
	MaxRefreshAttempts int
	HermesImage        string
	HermesMemory       string
	HermesCPUs         string
}

type ChannelWorker struct {
	database           *sql.DB
	channelJobs        *ChannelRepository
	channels           *channels.Repository
	weixinClient       weixin.Client
	runner             runtime.Runner
	pollInterval       time.Duration
	maxRefreshAttempts int
	hermesImage        string
	hermesMemory       string
	hermesCPUs         string
}

func NewChannelWorker(deps ChannelWorkerDependencies) *ChannelWorker {
	interval := deps.PollInterval
	if interval <= 0 {
		// 1.5s matches the cadence the Hermes weixin adapter uses for QR
		// polling; 200ms (the prior default) was unnecessarily aggressive
		// against the iLink gateway.
		interval = 1500 * time.Millisecond
	}
	maxAttempts := deps.MaxRefreshAttempts
	if maxAttempts <= 0 {
		// Hermes' qr_login allows up to 3 refreshes before giving up; the
		// prior default of 1 meant a single expiry killed the session
		// before the user had a realistic chance to scan.
		maxAttempts = 3
	}
	return &ChannelWorker{
		database:           deps.Database,
		channelJobs:        deps.ChannelJobs,
		channels:           deps.Channels,
		weixinClient:       deps.WeixinClient,
		runner:             deps.Runner,
		pollInterval:       interval,
		maxRefreshAttempts: maxAttempts,
		hermesImage:        deps.HermesImage,
		hermesMemory:       deps.HermesMemory,
		hermesCPUs:         deps.HermesCPUs,
	}
}

func (w *ChannelWorker) ProcessJob(ctx context.Context, jobID string) error {
	job, channel, agent, err := w.loadJobContext(ctx, jobID)
	if err != nil {
		return fmt.Errorf("load channel job context: %w", err)
	}
	if job.Type != TypeConnectWeixin {
		return fmt.Errorf("%w: unsupported channel job type: %q", ErrInvalidInput, job.Type)
	}
	if agent.Status != "running" {
		_ = w.fail(ctx, job.ID, channel.ID, "agent_not_running", "agent is not running", channels.StatusNotConfigured)
		return errors.New("agent not running")
	}

	session, err := w.ensureSession(ctx, channel.ID)
	if err != nil {
		return fmt.Errorf("ensure pairing session: %w", err)
	}
	if _, err := w.channels.TransitionStatus(ctx, channel.ID, channels.StatusQRPending, "", "", ""); err != nil && !errors.Is(err, channels.ErrInvalidStateTransition) && !errors.Is(err, channels.ErrConflict) {
		return fmt.Errorf("transition channel to qr pending: %w", err)
	}

	qr, err := w.weixinClient.GetBotQRCode(ctx, weixin.QRCodeRequest{BotType: 3})
	if err != nil {
		return w.fail(ctx, job.ID, channel.ID, "qr_request_failed", fmt.Sprintf("request qr code: %v", err), channels.StatusError)
	}
	payloadURL, err := w.writeQRPayload(agent.HermesHomePath, session.ID, qr.QRCodeImageContent)
	if err != nil {
		return w.fail(ctx, job.ID, channel.ID, "qr_image_write_failed", fmt.Sprintf("write qr payload: %v", err), channels.StatusError)
	}
	session.QRPayload = qr.QRCode
	session.QRPayloadURL = payloadURL
	session.AttemptCount = 1
	if session.ExpiresAt == "" {
		session.ExpiresAt = time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339)
	}
	if _, err := w.updateSession(ctx, session); err != nil {
		return fmt.Errorf("update pairing session: %w", err)
	}

	// redirectBaseURL tracks the redirect_host returned by a
	// "scaned_but_redirect" status. It is scoped to this single pairing
	// session so that one agent's redirect cannot bleed into another's
	// (the weixin.Client is shared across agents).
	var redirectBaseURL string
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		status, err := w.weixinClient.GetQRCodeStatus(ctx, weixin.QRStatusRequest{
			QRCode:          qr.QRCode,
			BaseURLOverride: redirectBaseURL,
		})
		if err != nil {
			return w.fail(ctx, job.ID, channel.ID, "qr_status_failed", fmt.Sprintf("get qr status: %v", err), channels.StatusError)
		}
		switch status.Status {
		case weixin.StatusWait, weixin.StatusScanned:
			time.Sleep(w.pollInterval)
			continue
		case weixin.StatusScannedButRedirect:
			if host := weixin.NormalizeRedirectHost(status.RedirectHost); host != "" {
				redirectBaseURL = host
			}
			time.Sleep(w.pollInterval)
			continue
		case weixin.StatusExpired:
			attemptsUsed := session.AttemptCount
			session.AttemptCount = attemptsUsed + 1
			session.Status = channels.PairingStatusExpired
			session.LastErrorCode = "qr_expired"
			session.LastErrorMessage = "qr expired"
			if _, err := w.updateSession(ctx, session); err != nil {
				return fmt.Errorf("update expired pairing session: %w", err)
			}
			if attemptsUsed < w.maxRefreshAttempts {
				nextSession, err := w.channels.CreatePairingSession(ctx, channel.ID, time.Now().Add(5*time.Minute).UTC().Format(time.RFC3339))
				if err != nil {
					return fmt.Errorf("create refreshed pairing session: %w", err)
				}
				session = nextSession
				session.AttemptCount = 1
				qr, err = w.weixinClient.GetBotQRCode(ctx, weixin.QRCodeRequest{BotType: 3})
				if err != nil {
					return w.fail(ctx, job.ID, channel.ID, "qr_request_failed", fmt.Sprintf("request qr code: %v", err), channels.StatusError)
				}
				payloadURL, err = w.writeQRPayload(agent.HermesHomePath, session.ID, qr.QRCodeImageContent)
				if err != nil {
					return w.fail(ctx, job.ID, channel.ID, "qr_image_write_failed", fmt.Sprintf("write qr payload: %v", err), channels.StatusError)
				}
				session.QRPayload = qr.QRCode
				session.QRPayloadURL = payloadURL
				if _, err := w.updateSession(ctx, session); err != nil {
					return fmt.Errorf("update refreshed pairing session: %w", err)
				}
				// A new qrcode token starts a fresh negotiation; any
				// redirect from the previous one no longer applies.
				redirectBaseURL = ""
				continue
			}
			return w.fail(ctx, job.ID, channel.ID, "qr_expired", "qr expired", channels.StatusNotConfigured)
		case weixin.StatusConfirmed:
			if err := w.writeConfirmedCredentials(agent.HermesHomePath, status); err != nil {
				return w.fail(ctx, job.ID, channel.ID, "credential_write_failed", fmt.Sprintf("write confirmed credentials: %v", err), channels.StatusError)
			}
			if err := w.runner.Stop(ctx, agent.RuntimeID); err != nil && !errors.Is(err, runtime.ErrContainerNotFound) {
				return w.fail(ctx, job.ID, channel.ID, "runtime_restart_failed", fmt.Sprintf("stop runtime: %v", err), channels.StatusError)
			}
			if err := w.runner.EnsureRunning(ctx, runtime.ContainerSpec{
				AgentID:    agent.ID,
				HermesHome: agent.HermesHomePath,
				Image:      w.hermesImage,
				Memory:     w.hermesMemory,
				CPUs:       w.hermesCPUs,
			}); err != nil {
				return w.fail(ctx, job.ID, channel.ID, "runtime_restart_failed", fmt.Sprintf("ensure runtime running: %v", err), channels.StatusError)
			}
			session.Status = channels.PairingStatusConnected
			session.LastErrorCode = ""
			session.LastErrorMessage = ""
			if _, err := w.updateSession(ctx, session); err != nil {
				return fmt.Errorf("mark connected pairing session: %w", err)
			}
			if _, err := w.channels.TransitionStatus(ctx, channel.ID, channels.StatusConnected, status.ILinkBotID, "", ""); err != nil {
				return fmt.Errorf("transition channel to connected: %w", err)
			}
			return w.channelJobs.MarkSucceeded(ctx, job.ID)
		default:
			return w.fail(ctx, job.ID, channel.ID, "unknown_qr_status", status.Status, channels.StatusError)
		}
	}
}

type channelWorkerJob struct {
	ID             string
	AgentChannelID string
	Type           ChannelJobType
}

type channelWorkerAgent struct {
	ID             string
	Status         string
	RuntimeID      string
	HermesHomePath string
}

func (w *ChannelWorker) loadJobContext(ctx context.Context, jobID string) (channelWorkerJob, channels.Channel, channelWorkerAgent, error) {
	var job channelWorkerJob
	err := w.database.QueryRowContext(ctx, `SELECT id, agent_channel_id, type FROM channel_jobs WHERE id = ?;`, jobID).Scan(&job.ID, &job.AgentChannelID, &job.Type)
	if err != nil {
		return channelWorkerJob{}, channels.Channel{}, channelWorkerAgent{}, fmt.Errorf("load channel job: %w", err)
	}
	channel, err := w.channels.GetByID(ctx, job.AgentChannelID)
	if err != nil {
		return channelWorkerJob{}, channels.Channel{}, channelWorkerAgent{}, fmt.Errorf("load channel by id: %w", err)
	}
	var agent channelWorkerAgent
	err = w.database.QueryRowContext(ctx, `SELECT id, status, runtime_id, hermes_home_path FROM agents WHERE id = ?;`, channel.AgentID).Scan(&agent.ID, &agent.Status, &agent.RuntimeID, &agent.HermesHomePath)
	if err != nil {
		return channelWorkerJob{}, channels.Channel{}, channelWorkerAgent{}, fmt.Errorf("load agent for channel job: %w", err)
	}
	return job, channel, agent, nil
}

func (w *ChannelWorker) ensureSession(ctx context.Context, channelID string) (channels.PairingSession, error) {
	session, err := w.channels.GetActivePairingSession(ctx, channelID)
	if err == nil {
		return session, nil
	}
	if !errors.Is(err, channels.ErrNotFound) {
		return channels.PairingSession{}, fmt.Errorf("get active pairing session: %w", err)
	}
	session, err = w.channels.CreatePairingSession(ctx, channelID, time.Now().Add(5*time.Minute).UTC().Format(time.RFC3339))
	if err != nil {
		return channels.PairingSession{}, fmt.Errorf("create pairing session: %w", err)
	}
	return session, nil
}

func (w *ChannelWorker) updateSession(ctx context.Context, session channels.PairingSession) (channels.PairingSession, error) {
	return w.channels.SetPairingSession(ctx, session)
}

// writeQRPayload persists the scannable liteapp URL Tencent's iLink
// gateway returned in qrcode_img_content. The on-disk file name still
// ends in .qr.txt for backward compatibility — the content is the URL,
// not image data, despite the legacy "QRImage" terminology used in the
// surrounding helpers.
func (w *ChannelWorker) writeQRPayload(homePath, sessionID, content string) (string, error) {
	path := filepath.Join(homePath, "weixin", "accounts", sessionID+".qr.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create qr payload directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write qr payload: %w", err)
	}
	return content, nil
}

func (w *ChannelWorker) writeConfirmedCredentials(homePath string, status weixin.QRStatusResponse) error {
	if err := runtime.WriteAgentEnvConnected(filepath.Join(homePath, ".env"), status.ILinkBotID, status.BotToken, status.BaseURL, status.ILinkUserID); err != nil {
		return fmt.Errorf("write connected env: %w", err)
	}
	accountFile := filepath.Join(homePath, "weixin", "accounts", status.ILinkBotID+".json")
	payload, err := json.Marshal(map[string]string{
		"account_id": status.ILinkBotID,
		"token":      status.BotToken,
		"base_url":   status.BaseURL,
		"user_id":    status.ILinkUserID,
	})
	if err != nil {
		return fmt.Errorf("marshal connected account payload: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(accountFile), 0o755); err != nil {
		return fmt.Errorf("create account directory: %w", err)
	}
	if err := os.WriteFile(accountFile, payload, 0o644); err != nil {
		return fmt.Errorf("write account file: %w", err)
	}
	return nil
}

func (w *ChannelWorker) fail(ctx context.Context, jobID, channelID, code, message string, nextStatus channels.Status) error {
	channel, err := w.channels.GetByID(ctx, channelID)
	if err == nil && channel.Status.CanTransitionTo(nextStatus) {
		_, _ = w.channels.TransitionStatus(ctx, channelID, nextStatus, "", code, message)
	}
	_ = w.channelJobs.MarkFailed(ctx, jobID, code, message)
	return fmt.Errorf("%s: %s", code, message)
}
