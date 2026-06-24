package jobs_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentforge.local/services/api/internal/channels"
	"agentforge.local/services/api/internal/jobs"
	"agentforge.local/services/api/internal/runtime"
	"agentforge.local/services/api/internal/weixin"

	_ "modernc.org/sqlite"
)

func TestChannelWorkerConnectWeixinTransitionsToConnected(t *testing.T) {
	database := newChannelWorkerTestDB(t)
	ctx := context.Background()
	agentID, channelID := insertChannelWorkerAgent(t, database, t.TempDir(), "running")
	jobID := insertChannelWorkerJob(t, database, channelID, jobs.TypeConnectWeixin)

	client := &stubWeixinClient{
		qrResponse: weixin.QRCodeResponse{
			QRCode:             "qr-1",
			QRCodeImageContent: "data:image/png;base64,abc",
		},
		statuses: []weixin.QRStatusResponse{
			{Status: weixin.StatusWait},
			{Status: weixin.StatusScanned},
			{
				Status:      weixin.StatusConfirmed,
				ILinkBotID:  "bot-1",
				BotToken:    "bot-token-1",
				BaseURL:     "https://weixin.example.com",
				ILinkUserID: "user-1",
			},
		},
	}
	runner := &stubChannelRunner{
		status: runtime.ContainerStatus{Exists: true, Running: true, Status: "running"},
	}
	worker := jobs.NewChannelWorker(jobs.ChannelWorkerDependencies{
		Database:           database,
		ChannelJobs:        jobs.NewChannelRepository(database),
		Channels:           channels.NewRepository(database),
		WeixinClient:       client,
		Runner:             runner,
		PollInterval:       time.Millisecond,
		MaxRefreshAttempts: 1,
	})

	if err := worker.ProcessJob(ctx, jobID); err != nil {
		t.Fatalf("ProcessJob returned error: %v", err)
	}

	channel, err := channels.NewRepository(database).GetByID(ctx, channelID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if channel.Status != channels.StatusConnected || channel.ExternalAccountID != "bot-1" {
		t.Fatalf("channel = %#v", channel)
	}

	job, err := jobs.NewChannelRepository(database).GetByID(ctx, channelID, jobID)
	if err != nil {
		t.Fatalf("GetByID job returned error: %v", err)
	}
	if job.Status != jobs.StatusSucceeded {
		t.Fatalf("job = %#v", job)
	}

	session, err := channels.NewRepository(database).GetActivePairingSession(ctx, channelID)
	if err != nil {
		t.Fatalf("GetActivePairingSession returned error: %v", err)
	}
	if session.Status != channels.PairingStatusConnected {
		t.Fatalf("session = %#v", session)
	}

	homePath := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(channel.CreatedAt))), "unused")
	_ = homePath
	agentHome := loadChannelWorkerHomePath(t, database, agentID)
	mustReadFileContains(t, filepath.Join(agentHome, ".env"), "WEIXIN_ALLOWED_USERS=user-1")
	mustReadFileContains(t, filepath.Join(agentHome, ".env"), "WEIXIN_DM_POLICY=allowlist")
	mustReadFileContains(t, filepath.Join(agentHome, ".env"), "WEIXIN_GROUP_POLICY=disabled")

	accountFile := filepath.Join(agentHome, "weixin", "accounts", "bot-1.json")
	data, err := os.ReadFile(accountFile)
	if err != nil {
		t.Fatalf("read account file: %v", err)
	}
	var account map[string]string
	if err := json.Unmarshal(data, &account); err != nil {
		t.Fatalf("unmarshal account file: %v", err)
	}
	want := map[string]string{
		"account_id": "bot-1",
		"token":      "bot-token-1",
		"base_url":   "https://weixin.example.com",
		"user_id":    "user-1",
	}
	if len(account) != len(want) {
		t.Fatalf("account = %#v, want %#v", account, want)
	}
	for key, value := range want {
		if account[key] != value {
			t.Fatalf("account[%q] = %q, want %q", key, account[key], value)
		}
	}
	if runner.stopCount != 1 || runner.ensureCount != 1 {
		t.Fatalf("runner stop/ensure counts = %d/%d, want 1/1", runner.stopCount, runner.ensureCount)
	}
}

func TestChannelWorkerConnectWeixinMarksExpiredQRCode(t *testing.T) {
	database := newChannelWorkerTestDB(t)
	ctx := context.Background()
	_, channelID := insertChannelWorkerAgent(t, database, t.TempDir(), "running")
	jobID := insertChannelWorkerJob(t, database, channelID, jobs.TypeConnectWeixin)

	worker := jobs.NewChannelWorker(jobs.ChannelWorkerDependencies{
		Database:           database,
		ChannelJobs:        jobs.NewChannelRepository(database),
		Channels:           channels.NewRepository(database),
		WeixinClient: &stubWeixinClient{
			qrResponse: weixin.QRCodeResponse{QRCode: "qr-1", QRCodeImageContent: "data:image/png;base64,abc"},
			statuses:   []weixin.QRStatusResponse{{Status: weixin.StatusExpired}},
		},
		Runner:             &stubChannelRunner{},
		PollInterval:       time.Millisecond,
		MaxRefreshAttempts: 1,
	})

	err := worker.ProcessJob(ctx, jobID)
	if err == nil {
		t.Fatal("ProcessJob error = nil, want error")
	}

	channel, getErr := channels.NewRepository(database).GetByID(ctx, channelID)
	if getErr != nil {
		t.Fatalf("GetByID returned error: %v", getErr)
	}
	if channel.Status != channels.StatusNotConfigured || channel.LastErrorCode != "qr_expired" {
		t.Fatalf("channel = %#v", channel)
	}

	job, getErr := jobs.NewChannelRepository(database).GetByID(ctx, channelID, jobID)
	if getErr != nil {
		t.Fatalf("GetByID job returned error: %v", getErr)
	}
	if job.Status != jobs.StatusFailed || job.LastErrorCode != "qr_expired" {
		t.Fatalf("job = %#v", job)
	}
}

func TestChannelWorkerConnectWeixinRefreshesExpiredQRCodeBeforeGivingUp(t *testing.T) {
	database := newChannelWorkerTestDB(t)
	ctx := context.Background()
	_, channelID := insertChannelWorkerAgent(t, database, t.TempDir(), "running")
	jobID := insertChannelWorkerJob(t, database, channelID, jobs.TypeConnectWeixin)

	worker := jobs.NewChannelWorker(jobs.ChannelWorkerDependencies{
		Database:           database,
		ChannelJobs:        jobs.NewChannelRepository(database),
		Channels:           channels.NewRepository(database),
		WeixinClient: &stubWeixinClient{
			qrResponse: weixin.QRCodeResponse{QRCode: "qr-1", QRCodeImageContent: "data:image/png;base64,abc"},
			statuses: []weixin.QRStatusResponse{
				{Status: weixin.StatusExpired},
				{Status: weixin.StatusWait},
				{Status: weixin.StatusConfirmed, ILinkBotID: "bot-2", BotToken: "token-2", BaseURL: "https://weixin.example.com", ILinkUserID: "user-1"},
			},
		},
		Runner:             &stubChannelRunner{},
		PollInterval:       time.Millisecond,
		MaxRefreshAttempts: 2,
	})

	if err := worker.ProcessJob(ctx, jobID); err != nil {
		t.Fatalf("ProcessJob returned error: %v", err)
	}

	channel, err := channels.NewRepository(database).GetByID(ctx, channelID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if channel.Status != channels.StatusConnected || channel.ExternalAccountID != "bot-2" {
		t.Fatalf("channel = %#v", channel)
	}
}

func TestChannelWorkerConnectWeixinRejectsNonRunningAgent(t *testing.T) {
	database := newChannelWorkerTestDB(t)
	ctx := context.Background()
	_, channelID := insertChannelWorkerAgent(t, database, t.TempDir(), "creating")
	jobID := insertChannelWorkerJob(t, database, channelID, jobs.TypeConnectWeixin)

	worker := jobs.NewChannelWorker(jobs.ChannelWorkerDependencies{
		Database:           database,
		ChannelJobs:        jobs.NewChannelRepository(database),
		Channels:           channels.NewRepository(database),
		WeixinClient:       &stubWeixinClient{},
		Runner:             &stubChannelRunner{},
		PollInterval:       time.Millisecond,
		MaxRefreshAttempts: 1,
	})

	err := worker.ProcessJob(ctx, jobID)
	if err == nil {
		t.Fatal("ProcessJob error = nil, want error")
	}

	job, getErr := jobs.NewChannelRepository(database).GetByID(ctx, channelID, jobID)
	if getErr != nil {
		t.Fatalf("GetByID job returned error: %v", getErr)
	}
	if job.Status != jobs.StatusFailed || job.LastErrorCode != "agent_not_running" {
		t.Fatalf("job = %#v", job)
	}
}

type stubWeixinClient struct {
	qrResponse weixin.QRCodeResponse
	qrErr      error
	statuses   []weixin.QRStatusResponse
	statusErr  error
	index      int
}

func (s *stubWeixinClient) GetBotQRCode(_ context.Context, _ weixin.QRCodeRequest) (weixin.QRCodeResponse, error) {
	if s.qrErr != nil {
		return weixin.QRCodeResponse{}, s.qrErr
	}
	return s.qrResponse, nil
}

func (s *stubWeixinClient) GetQRCodeStatus(_ context.Context, _ weixin.QRStatusRequest) (weixin.QRStatusResponse, error) {
	if s.statusErr != nil {
		return weixin.QRStatusResponse{}, s.statusErr
	}
	if s.index >= len(s.statuses) {
		return weixin.QRStatusResponse{}, errors.New("status sequence exhausted")
	}
	response := s.statuses[s.index]
	s.index++
	return response, nil
}

type stubChannelRunner struct {
	status      runtime.ContainerStatus
	inspectErr  error
	ensureErr   error
	stopErr     error
	ensureCount int
	stopCount   int
}

func (s *stubChannelRunner) EnsureRunning(_ context.Context, _ runtime.ContainerSpec) error {
	s.ensureCount++
	return s.ensureErr
}

func (s *stubChannelRunner) Stop(_ context.Context, _ string) error {
	s.stopCount++
	return s.stopErr
}

func (s *stubChannelRunner) Remove(_ context.Context, _ string) error {
	return nil
}

func (s *stubChannelRunner) Inspect(_ context.Context, _ string) (runtime.ContainerStatus, error) {
	return s.status, s.inspectErr
}

func newChannelWorkerTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite", "file:channel-worker-test-"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	_, err = database.Exec(`
		PRAGMA foreign_keys = ON;
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		CREATE TABLE agent_templates (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
			version INTEGER NOT NULL DEFAULT 1,
			template_path TEXT NOT NULL,
			content_checksum TEXT NOT NULL,
			soul_md_path TEXT NOT NULL DEFAULT '',
			user_md_path TEXT NOT NULL DEFAULT '',
			soul_content TEXT NOT NULL DEFAULT '',
			user_content TEXT NOT NULL DEFAULT '',
			skills_path TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			published_at TEXT,
			FOREIGN KEY (created_by) REFERENCES users(id)
		);
		CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			owner_user_id TEXT NOT NULL,
			template_id TEXT NOT NULL,
			template_version INTEGER NOT NULL,
			name TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'creating' CHECK (status IN ('creating', 'provisioning', 'starting', 'running', 'stopped', 'error')),
			runtime_id TEXT NOT NULL DEFAULT '',
			hermes_home_path TEXT NOT NULL UNIQUE,
			last_error_code TEXT NOT NULL DEFAULT '',
			last_error_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (owner_user_id) REFERENCES users(id),
			FOREIGN KEY (template_id) REFERENCES agent_templates(id)
		);
		CREATE TABLE agent_channels (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			channel_type TEXT NOT NULL DEFAULT 'weixin' CHECK (channel_type IN ('weixin')),
			status TEXT NOT NULL DEFAULT 'not_configured' CHECK (status IN ('not_configured', 'qr_pending', 'connected', 'error', 'disconnected')),
			external_account_id TEXT NOT NULL DEFAULT '',
			last_error_code TEXT NOT NULL DEFAULT '',
			last_error_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		);
		CREATE TABLE channel_pairing_sessions (
			id TEXT PRIMARY KEY,
			agent_channel_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'connected', 'expired', 'failed')),
			qr_payload TEXT NOT NULL DEFAULT '',
			qr_image_path TEXT NOT NULL DEFAULT '',
			expires_at TEXT NOT NULL,
			attempt_count INTEGER NOT NULL DEFAULT 0,
			last_error_code TEXT NOT NULL DEFAULT '',
			last_error_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (agent_channel_id) REFERENCES agent_channels(id) ON DELETE CASCADE
		);
		CREATE TABLE channel_jobs (
			id TEXT PRIMARY KEY,
			agent_channel_id TEXT NOT NULL,
			pairing_session_id TEXT,
			type TEXT NOT NULL CHECK (type IN ('connect_weixin', 'disconnect_weixin', 'refresh_weixin_pairing')),
			status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
			priority INTEGER NOT NULL DEFAULT 0,
			attempt_count INTEGER NOT NULL DEFAULT 0,
			max_attempts INTEGER NOT NULL DEFAULT 3,
			locked_by TEXT NOT NULL DEFAULT '',
			locked_until TEXT,
			idempotency_key TEXT NOT NULL DEFAULT '',
			payload_json TEXT NOT NULL DEFAULT '{}',
			result_json TEXT NOT NULL DEFAULT '{}',
			last_error_code TEXT NOT NULL DEFAULT '',
			last_error_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			started_at TEXT,
			finished_at TEXT,
			FOREIGN KEY (agent_channel_id) REFERENCES agent_channels(id) ON DELETE CASCADE,
			FOREIGN KEY (pairing_session_id) REFERENCES channel_pairing_sessions(id) ON DELETE SET NULL
		);
		CREATE UNIQUE INDEX idx_channel_jobs_one_active
		ON channel_jobs(agent_channel_id)
		WHERE status IN ('queued', 'running');
		CREATE UNIQUE INDEX idx_pairing_one_active
		ON channel_pairing_sessions(agent_channel_id)
		WHERE status = 'pending';
		INSERT INTO users (id, email, password_hash, role)
		VALUES ('user-1', 'user@example.com', 'unused', 'user');
		INSERT INTO agent_templates (
			id, name, description, status, version, template_path, content_checksum,
			soul_md_path, user_md_path, soul_content, user_content, skills_path, created_by
		) VALUES (
			'template-1', 'Support', 'Published template', 'published', 1,
			'/tmp/template-1', 'checksum', '/tmp/template-1/SOUL.md',
			'/tmp/template-1/USER.md', '', '', '/tmp/template-1/skills', 'user-1'
		);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return database
}

func insertChannelWorkerAgent(t *testing.T, database *sql.DB, root, status string) (string, string) {
	t.Helper()
	agentID := "agent-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-"))
	homePath := filepath.Join(root, "agents", agentID, "hermes-home")
	if err := os.MkdirAll(filepath.Join(homePath, "weixin", "accounts"), 0o755); err != nil {
		t.Fatalf("mkdir hermes home: %v", err)
	}
	_, err := database.Exec(`
		INSERT INTO agents (
			id, owner_user_id, template_id, template_version, name, status, runtime_id, hermes_home_path
		) VALUES (?, 'user-1', 'template-1', 1, 'Fixture Agent', ?, ?, ?);
	`, agentID, status, runtime.DefaultContainerName(agentID), homePath)
	if err != nil {
		t.Fatalf("insert agent fixture: %v", err)
	}
	channelID := "channel-" + agentID
	_, err = database.Exec(`
		INSERT INTO agent_channels (id, agent_id, channel_type, status)
		VALUES (?, ?, 'weixin', 'not_configured');
	`, channelID, agentID)
	if err != nil {
		t.Fatalf("insert channel fixture: %v", err)
	}
	return agentID, channelID
}

func insertChannelWorkerJob(t *testing.T, database *sql.DB, channelID string, jobType jobs.ChannelJobType) string {
	t.Helper()
	jobID := "job-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-"))
	_, err := database.Exec(`
		INSERT INTO channel_jobs (id, agent_channel_id, type, status, locked_by)
		VALUES (?, ?, ?, 'queued', 'worker-1');
	`, jobID, channelID, jobType)
	if err != nil {
		t.Fatalf("insert channel job: %v", err)
	}
	return jobID
}

func loadChannelWorkerHomePath(t *testing.T, database *sql.DB, agentID string) string {
	t.Helper()
	var homePath string
	if err := database.QueryRow(`SELECT hermes_home_path FROM agents WHERE id = ?`, agentID).Scan(&homePath); err != nil {
		t.Fatalf("load hermes home path: %v", err)
	}
	return homePath
}

func mustReadFileContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s = %q, want substring %q", path, string(data), want)
	}
}
