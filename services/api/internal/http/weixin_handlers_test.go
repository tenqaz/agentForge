package http

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"agentforge.local/services/api/internal/agents"
	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/channels"
	"agentforge.local/services/api/internal/jobs"

	_ "modernc.org/sqlite"
)

func TestWeixinRoutesRequireRunningAgentForConfiguration(t *testing.T) {
	router, manager, database, _ := newWeixinTestRouter(t)
	userCookie := sessionCookieFor(t, manager, auth.User{ID: "user-1", Email: "user@example.com", Role: auth.RoleUser})
	agentID := insertWeixinHTTPAgent(t, database, "user-1", agents.StatusCreating)

	putRecorder := doJSON(t, router, http.MethodPut, "/api/agents/"+agentID+"/channels/weixin", `{}`, userCookie)
	if putRecorder.Code != http.StatusConflict {
		t.Fatalf("PUT status = %d, want 409, body = %s", putRecorder.Code, putRecorder.Body.String())
	}
	if !bytes.Contains(putRecorder.Body.Bytes(), []byte("agent_not_running")) {
		t.Fatalf("PUT body = %s, want agent_not_running", putRecorder.Body.String())
	}

	postRecorder := doJSON(t, router, http.MethodPost, "/api/agents/"+agentID+"/channels/weixin/pairing-sessions", `{}`, userCookie)
	if postRecorder.Code != http.StatusConflict {
		t.Fatalf("POST status = %d, want 409, body = %s", postRecorder.Code, postRecorder.Body.String())
	}
	if !bytes.Contains(postRecorder.Body.Bytes(), []byte("agent_not_running")) {
		t.Fatalf("POST body = %s, want agent_not_running", postRecorder.Body.String())
	}
}

func TestWeixinRoutesExposePairingSessionsWithoutSecrets(t *testing.T) {
	router, manager, database, _ := newWeixinTestRouter(t)
	userCookie := sessionCookieFor(t, manager, auth.User{ID: "user-1", Email: "user@example.com", Role: auth.RoleUser})
	agentID := insertWeixinHTTPAgent(t, database, "user-1", agents.StatusRunning)

	putRecorder := doJSON(t, router, http.MethodPut, "/api/agents/"+agentID+"/channels/weixin", `{}`, userCookie)
	if putRecorder.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", putRecorder.Code, putRecorder.Body.String())
	}

	postRecorder := doJSON(t, router, http.MethodPost, "/api/agents/"+agentID+"/channels/weixin/pairing-sessions", `{}`, userCookie)
	if postRecorder.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, body = %s", postRecorder.Code, postRecorder.Body.String())
	}
	created := decodePairingSessionResponse(t, postRecorder.Body.Bytes()).Session

	secondRecorder := doJSON(t, router, http.MethodPost, "/api/agents/"+agentID+"/channels/weixin/pairing-sessions", `{}`, userCookie)
	if secondRecorder.Code != http.StatusOK {
		t.Fatalf("second POST status = %d, body = %s", secondRecorder.Code, secondRecorder.Body.String())
	}
	reused := decodePairingSessionResponse(t, secondRecorder.Body.Bytes()).Session
	if reused.ID != created.ID {
		t.Fatalf("reused session id = %q, want %q", reused.ID, created.ID)
	}

	channelID := loadWeixinHTTPChannelID(t, database, agentID)
	// After the rename: qr_image_path column now stores the scannable URL
	// directly (not a file path), so we seed it with the liteapp URL text.
	if _, err := database.Exec(`
		UPDATE channel_pairing_sessions
		SET qr_payload = 'qr-payload', qr_image_path = ?, expires_at = ?
		WHERE id = ?;
	`, "https://liteapp.weixin.qq.com/q/test?qrcode=abc123", time.Now().Add(5*time.Minute).UTC().Format(time.RFC3339), created.ID); err != nil {
		t.Fatalf("seed pairing session content: %v", err)
	}
	if _, err := database.Exec(`
		UPDATE agent_channels
		SET status = 'qr_pending'
		WHERE id = ?;
	`, channelID); err != nil {
		t.Fatalf("seed channel status: %v", err)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID+"/channels/weixin/pairing-sessions", nil)
	listRequest.AddCookie(userCookie)
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	list := decodePairingSessionsResponse(t, listRecorder.Body.Bytes()).Sessions
	if len(list) != 1 || list[0].QRPayload != "qr-payload" || list[0].QRPayloadURL != "https://liteapp.weixin.qq.com/q/test?qrcode=abc123" {
		t.Fatalf("list sessions = %#v", list)
	}

	detailRecorder := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID+"/channels/weixin/pairing-sessions/"+created.ID, nil)
	detailRequest.AddCookie(userCookie)
	router.ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("detail status = %d, body = %s", detailRecorder.Code, detailRecorder.Body.String())
	}

	channelRecorder := httptest.NewRecorder()
	channelRequest := httptest.NewRequest(http.MethodGet, "/api/agents/"+agentID+"/channels/weixin", nil)
	channelRequest.AddCookie(userCookie)
	router.ServeHTTP(channelRecorder, channelRequest)
	if channelRecorder.Code != http.StatusOK {
		t.Fatalf("channel status = %d, body = %s", channelRecorder.Code, channelRecorder.Body.String())
	}
	if bytes.Contains(channelRecorder.Body.Bytes(), []byte("bot_token")) || bytes.Contains(channelRecorder.Body.Bytes(), []byte("WEIXIN_TOKEN")) {
		t.Fatalf("channel response leaked secret: %s", channelRecorder.Body.String())
	}
	if bytes.Contains(detailRecorder.Body.Bytes(), []byte("bot_token")) || bytes.Contains(detailRecorder.Body.Bytes(), []byte("WEIXIN_TOKEN")) {
		t.Fatalf("pairing detail leaked secret: %s", detailRecorder.Body.String())
	}
}

func newWeixinTestRouter(t *testing.T) (http.Handler, *auth.SessionManager, *sql.DB, string) {
	t.Helper()

	database := newWeixinHTTPTestDB(t)
	manager := auth.NewSessionManager("test-secret", false)
	agentRepository := agents.NewRepository(database)
	runtimeJobRepository := jobs.NewRuntimeRepository(database)
	dataDir := filepath.Join(t.TempDir(), "var")
	channelRepository := channels.NewRepository(database)
	channelService := channels.NewService(database, channelRepository)
	router := NewRouter(Dependencies{
		AuthRepository:       auth.NewRepository(database),
		SessionManager:       manager,

		AgentService:         agents.NewService(database, agentRepository, runtimeJobRepository, nil, dataDir, "docker", "", "", ""),

		RuntimeJobRepository: runtimeJobRepository,
		ChannelService:       channelService,
		ChannelRepository:    channelRepository,
		ChannelJobRepository: jobs.NewChannelRepository(database),
	})
	return router, manager, database, dataDir
}

func newWeixinHTTPTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite", "file:weixin-http-test-"+t.Name()+"?mode=memory&cache=shared")
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
		CREATE TABLE runtime_jobs (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			type TEXT NOT NULL CHECK (type IN ('provision_agent', 'start_runtime', 'stop_runtime', 'restart_runtime')),
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
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		);
		CREATE UNIQUE INDEX idx_runtime_jobs_one_active
		ON runtime_jobs(agent_id)
		WHERE status IN ('queued', 'running');
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
		VALUES ('admin-1', 'admin@example.com', 'unused', 'admin'),
		       ('user-1', 'user@example.com', 'unused', 'user');
		INSERT INTO agent_templates (
			id, name, description, status, version, template_path, content_checksum,
			soul_content, user_content, skills_path, created_by
		) VALUES (
			'template-1', 'Support', 'Published template', 'published', 1,
			'/tmp/template-1', 'checksum', '', '', '/tmp/template-1/skills', 'admin-1'
		);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}

	return database
}

func insertWeixinHTTPAgent(t *testing.T, database *sql.DB, ownerUserID string, status agents.Status) string {
	t.Helper()
	agentID := "agent-" + ownerUserID + "-" + string(status)
	_, err := database.Exec(`
		INSERT INTO agents (
			id, owner_user_id, template_id, template_version, name, status, hermes_home_path
		) VALUES (?, ?, 'template-1', 1, 'Fixture Agent', ?, ?);
	`, agentID, ownerUserID, status, filepath.Join(t.TempDir(), agentID, "hermes-home"))
	if err != nil {
		t.Fatalf("insert agent fixture: %v", err)
	}
	return agentID
}

func loadWeixinHTTPChannelID(t *testing.T, database *sql.DB, agentID string) string {
	t.Helper()
	var channelID string
	if err := database.QueryRow(`SELECT id FROM agent_channels WHERE agent_id = ?`, agentID).Scan(&channelID); err != nil {
		t.Fatalf("load channel id: %v", err)
	}
	return channelID
}

func decodePairingSessionResponse(t *testing.T, body []byte) struct {
	Session pairingSessionDTO `json:"session"`
} {
	t.Helper()
	var response struct {
		Session pairingSessionDTO `json:"session"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("unmarshal pairing session response %q: %v", body, err)
	}
	return response
}

func decodePairingSessionsResponse(t *testing.T, body []byte) struct {
	Sessions []pairingSessionDTO `json:"sessions"`
} {
	t.Helper()
	var response struct {
		Sessions []pairingSessionDTO `json:"sessions"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("unmarshal pairing sessions response %q: %v", body, err)
	}
	return response
}
