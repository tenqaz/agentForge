package tests

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"agentforge.local/services/api/internal/agents"
	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/channels"
	"agentforge.local/services/api/internal/db"
	httpapi "agentforge.local/services/api/internal/http"
	"agentforge.local/services/api/internal/jobs"
	"agentforge.local/services/api/internal/runtime"
	"agentforge.local/services/api/internal/templates"
	"agentforge.local/services/api/internal/weixin"

	_ "modernc.org/sqlite"
)

func TestMVPIntegrationAdminPublishesUserCreatesAgentAndWeixinConnects(t *testing.T) {
	ctx := context.Background()
	fixture := newMVPFixture(t)

	adminCookie := loginAndGetCookie(t, fixture.router, "admin@example.com", "secret-password")
	userCookie := loginAndGetCookie(t, fixture.router, "user@example.com", "secret-password")

	createTemplate := doMultipartTemplateCreate(t, fixture.router, adminCookie, "Support Concierge", "Handles private support requests.", "# Soul\nCalm, direct, reliable.", "# User\nAnswer concisely.", []multipartSkillFile{
		{name: "triage.zip", content: makeSkillArchive(t, map[string]string{
			"SKILL.md": "---\nname: Triage\ndescription: Escalate billing issues.\n---\n# SKILL\nEscalate billing issues.\n",
		})},
	})
	if createTemplate.Code != http.StatusCreated {
		t.Fatalf("create template status = %d, body = %s", createTemplate.Code, createTemplate.Body.String())
	}
	templateID := decodeTemplateID(t, createTemplate.Body.Bytes())

	adminDetail := doAuthedRequest(t, fixture.router, http.MethodGet, "/api/admin/templates/"+templateID, "", adminCookie)
	if adminDetail.Code != http.StatusOK || !bytes.Contains(adminDetail.Body.Bytes(), []byte(templateID)) {
		t.Fatalf("admin detail status/body = %d / %s", adminDetail.Code, adminDetail.Body.String())
	}

	publish := doJSON(t, fixture.router, http.MethodPut, "/api/admin/templates/"+templateID+"/publication", `{}`, adminCookie)
	if publish.Code != http.StatusOK {
		t.Fatalf("publish template status = %d, body = %s", publish.Code, publish.Body.String())
	}

	publicList := httptest.NewRecorder()
	fixture.router.ServeHTTP(publicList, httptest.NewRequest(http.MethodGet, "/api/templates", nil))
	if publicList.Code != http.StatusOK || !bytes.Contains(publicList.Body.Bytes(), []byte(templateID)) {
		t.Fatalf("public list status/body = %d / %s", publicList.Code, publicList.Body.String())
	}

	createAgent := doJSON(t, fixture.router, http.MethodPost, "/api/agents", `{"templateId":"`+templateID+`","name":"Support Concierge Agent"}`, userCookie)
	if createAgent.Code != http.StatusCreated {
		t.Fatalf("create agent status = %d, body = %s", createAgent.Code, createAgent.Body.String())
	}
	agentID := decodeAgentID(t, createAgent.Body.Bytes())

	runtimeJobID := mustLoadSingleRuntimeJobID(t, fixture.database, agentID)
	if err := fixture.runtimeWorker.ProcessJob(ctx, runtimeJobID); err != nil {
		t.Fatalf("runtime worker process job: %v", err)
	}

	runtimeResponse := doAuthedRequest(t, fixture.router, http.MethodGet, "/api/agents/"+agentID+"/runtime", "", userCookie)
	if runtimeResponse.Code != http.StatusOK {
		t.Fatalf("get runtime status = %d, body = %s", runtimeResponse.Code, runtimeResponse.Body.String())
	}
	if !bytes.Contains(runtimeResponse.Body.Bytes(), []byte(`"status":"running"`)) {
		t.Fatalf("runtime body = %s", runtimeResponse.Body.String())
	}

	ensureChannel := doJSON(t, fixture.router, http.MethodPut, "/api/agents/"+agentID+"/channels/weixin", `{}`, userCookie)
	if ensureChannel.Code != http.StatusOK {
		t.Fatalf("ensure channel status = %d, body = %s", ensureChannel.Code, ensureChannel.Body.String())
	}
	createPairing := doJSON(t, fixture.router, http.MethodPost, "/api/agents/"+agentID+"/channels/weixin/pairing-sessions", `{}`, userCookie)
	if createPairing.Code != http.StatusCreated {
		t.Fatalf("create pairing status = %d, body = %s", createPairing.Code, createPairing.Body.String())
	}

	channelJobID := mustLoadSingleChannelJobID(t, fixture.database, agentID)
	if err := fixture.channelWorker.ProcessJob(ctx, channelJobID); err != nil {
		t.Fatalf("channel worker process job: %v", err)
	}

	sessionResponse := doAuthedRequest(t, fixture.router, http.MethodGet, "/api/agents/"+agentID+"/channels/weixin/pairing-sessions", "", userCookie)
	if sessionResponse.Code != http.StatusOK {
		t.Fatalf("list pairing sessions status = %d, body = %s", sessionResponse.Code, sessionResponse.Body.String())
	}
	if !bytes.Contains(sessionResponse.Body.Bytes(), []byte(`"status":"connected"`)) {
		t.Fatalf("pairing sessions body = %s", sessionResponse.Body.String())
	}

	channelResponse := doAuthedRequest(t, fixture.router, http.MethodGet, "/api/agents/"+agentID+"/channels/weixin", "", userCookie)
	if channelResponse.Code != http.StatusOK {
		t.Fatalf("get channel status = %d, body = %s", channelResponse.Code, channelResponse.Body.String())
	}
	if !bytes.Contains(channelResponse.Body.Bytes(), []byte(`"externalAccountId":"wx-bot-1"`)) {
		t.Fatalf("channel response = %s", channelResponse.Body.String())
	}
	if bytes.Contains(channelResponse.Body.Bytes(), []byte("bot-token-1")) {
		t.Fatalf("channel response leaked token: %s", channelResponse.Body.String())
	}

	agentHome := filepath.Join(fixture.dataDir, "agents", agentID, "hermes-home")
	assertFileContains(t, filepath.Join(agentHome, ".env"), "WEIXIN_DM_POLICY=allowlist")
	assertFileContains(t, filepath.Join(agentHome, ".env"), "WEIXIN_GROUP_POLICY=disabled")
	assertFileContains(t, filepath.Join(agentHome, ".env"), "WEIXIN_ALLOWED_USERS=wx-user-1")
}

type mvpFixture struct {
	router        http.Handler
	database      *sql.DB
	dataDir       string
	runtimeWorker *jobs.RuntimeWorker
	channelWorker *jobs.ChannelWorker
	runner        *integrationRunner
}

func newMVPFixture(t *testing.T) mvpFixture {
	t.Helper()

	ctx := context.Background()
	dataDir := t.TempDir()
	database := openAndMigrateTestDB(t, filepath.Join(dataDir, "agentforge.db"))
	seedIntegrationUsers(t, database)

	authRepo := auth.NewRepository(database)
	sessionManager := auth.NewSessionManager("test-secret", false)
	templateRepo := templates.NewRepository(database)
	templateStore := templates.NewFileStore(dataDir)
	templateService := templates.NewService(templateRepo, templateStore)
	runtimeJobs := jobs.NewRuntimeRepository(database)
	agentRepo := agents.NewRepository(database)
	runner := &integrationRunner{}
	agentService := agents.NewService(database, agentRepo, runtimeJobs, runner, dataDir, "docker", "", "", "")
	channelRepo := channels.NewRepository(database)
	channelService := channels.NewService(database, channelRepo)
	channelJobs := jobs.NewChannelRepository(database)

	runtimeWorker := jobs.NewRuntimeWorker(jobs.RuntimeWorkerDependencies{
		Database:       database,
		RuntimeJobs:    runtimeJobs,
		Runner:         runner,
		TemplateLoader: templateService,
		Provider: runtime.ProviderConfig{
			DefaultModel: "deepseek-v4-flash",
			Provider:     "custom",
			BaseURL:      "https://api.deepseek.com",
			APIKey:       "secret-api-key",
			APIMode:      "chat_completions",
		},
		HermesImage:  "nousresearch/hermes-agent:v2026.6.5",
		HermesMemory: "500m",
		HermesCPUs:   "0.5",
	})
	channelWorker := jobs.NewChannelWorker(jobs.ChannelWorkerDependencies{
		Database:           database,
		ChannelJobs:        channelJobs,
		Channels:           channelRepo,
		WeixinClient:       &integrationWeixinClient{},
		Runner:             runner,
		PollInterval:       time.Millisecond,
		MaxRefreshAttempts: 1,
	})

	router := httpapi.NewRouter(httpapi.Dependencies{
		AuthRepository:       authRepo,
		SessionManager:       sessionManager,
		TemplateService:      templateService,
		AgentService:         agentService,
		RuntimeJobRepository: runtimeJobs,
		ChannelService:       channelService,
		ChannelRepository:    channelRepo,
		ChannelJobRepository: channelJobs,
	})

	t.Cleanup(func() {
		_ = database.Close()
	})
	if err := ctx.Err(); err != nil {
		t.Fatalf("fixture context error: %v", err)
	}

	return mvpFixture{
		router:        router,
		database:      database,
		dataDir:       dataDir,
		runtimeWorker: runtimeWorker,
		channelWorker: channelWorker,
		runner:        runner,
	}
}

func openAndMigrateTestDB(t *testing.T, sqlitePath string) *sql.DB {
	t.Helper()
	database, err := db.Open(context.Background(), sqlitePath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Migrate(context.Background(), database, filepath.Join("..", "migrations")); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return database
}

func seedIntegrationUsers(t *testing.T, database *sql.DB) {
	t.Helper()
	adminHash, err := auth.HashPassword("secret-password")
	if err != nil {
		t.Fatalf("hash admin password: %v", err)
	}
	userHash, err := auth.HashPassword("secret-password")
	if err != nil {
		t.Fatalf("hash user password: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO users (id, email, password_hash, role)
		VALUES ('admin-1', 'admin@example.com', ?, 'admin'),
		       ('user-1', 'user@example.com', ?, 'user');
	`, adminHash, userHash); err != nil {
		t.Fatalf("seed users: %v", err)
	}
}

type integrationRunner struct {
	mu          sync.Mutex
	stopErr     error
	removeErr   error
	stopCalls   int
	removeCalls int
}

func (r *integrationRunner) EnsureRunning(_ context.Context, _ runtime.ContainerSpec) error {
	return nil
}

func (r *integrationRunner) Stop(_ context.Context, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopCalls++
	return r.stopErr
}

func (r *integrationRunner) Remove(_ context.Context, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeCalls++
	return r.removeErr
}

func (r *integrationRunner) Inspect(_ context.Context, _ string) (runtime.ContainerStatus, error) {
	return runtime.ContainerStatus{Exists: true, Running: true, Status: "running"}, nil
}

func (r *integrationRunner) StopCalls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stopCalls
}

func (r *integrationRunner) RemoveCalls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.removeCalls
}

func (r *integrationRunner) SetStopError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopErr = err
}

type integrationWeixinClient struct {
	index int
}

func (c *integrationWeixinClient) GetBotQRCode(_ context.Context, _ weixin.QRCodeRequest) (weixin.QRCodeResponse, error) {
	return weixin.QRCodeResponse{
		QRCode:             "weixin://pairing/qr-1",
		QRCodeImageContent: "data:image/png;base64,abc",
	}, nil
}

func (c *integrationWeixinClient) GetQRCodeStatus(_ context.Context, _ weixin.QRStatusRequest) (weixin.QRStatusResponse, error) {
	statuses := []weixin.QRStatusResponse{
		{Status: weixin.StatusWait},
		{Status: weixin.StatusScanned},
		{
			Status:      weixin.StatusConfirmed,
			ILinkBotID:  "wx-bot-1",
			BotToken:    "bot-token-1",
			BaseURL:     "https://weixin.example.com",
			ILinkUserID: "wx-user-1",
		},
	}
	if c.index >= len(statuses) {
		return statuses[len(statuses)-1], nil
	}
	response := statuses[c.index]
	c.index++
	return response, nil
}

func loginAndGetCookie(t *testing.T, router http.Handler, email, password string) *http.Cookie {
	t.Helper()
	recorder := doAuthedRequest(t, router, http.MethodPost, "/api/sessions", `{"email":"`+email+`","password":"`+password+`"}`, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	return findCookie(t, recorder.Result().Cookies(), auth.SessionCookieName)
}

func doJSON(t *testing.T, router http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	return doAuthedRequest(t, router, method, path, body, cookie)
}

func doAuthedRequest(t *testing.T, router http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func decodeTemplateID(t *testing.T, body []byte) string {
	t.Helper()
	var response struct {
		Template struct {
			ID string `json:"id"`
		} `json:"template"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode template response %q: %v", body, err)
	}
	return response.Template.ID
}

func decodeAgentID(t *testing.T, body []byte) string {
	t.Helper()
	var response struct {
		Agent struct {
			ID string `json:"id"`
		} `json:"agent"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode agent response %q: %v", body, err)
	}
	return response.Agent.ID
}

func mustLoadSingleRuntimeJobID(t *testing.T, database *sql.DB, agentID string) string {
	t.Helper()
	var jobID string
	if err := database.QueryRow(`SELECT id FROM runtime_jobs WHERE agent_id = ? ORDER BY created_at ASC LIMIT 1`, agentID).Scan(&jobID); err != nil {
		t.Fatalf("load runtime job id: %v", err)
	}
	return jobID
}

func mustLoadSingleChannelJobID(t *testing.T, database *sql.DB, agentID string) string {
	t.Helper()
	var jobID string
	if err := database.QueryRow(`
		SELECT cj.id
		FROM channel_jobs cj
		JOIN agent_channels ac ON ac.id = cj.agent_channel_id
		WHERE ac.agent_id = ?
		ORDER BY cj.created_at ASC
		LIMIT 1
	`, agentID).Scan(&jobID); err != nil {
		t.Fatalf("load channel job id: %v", err)
	}
	return jobID
}

func assertStatus(t *testing.T, recorder *httptest.ResponseRecorder, want int) {
	t.Helper()
	if recorder.Code != want {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, want, recorder.Body.String())
	}
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s = %q, want substring %q", path, string(data), want)
	}
}

func findCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("cookie %q not found", name)
	return nil
}

type multipartSkillFile struct {
	name    string
	content []byte
}

func doMultipartTemplateCreate(t *testing.T, router http.Handler, cookie *http.Cookie, name, description, soulContent, userContent string, skills []multipartSkillFile) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for field, value := range map[string]string{
		"name":        name,
		"description": description,
		"soulContent": soulContent,
		"userContent": userContent,
	} {
		if err := writer.WriteField(field, value); err != nil {
			t.Fatalf("WriteField(%s): %v", field, err)
		}
	}
	for _, skill := range skills {
		part, err := writer.CreateFormFile("skillZips", skill.name)
		if err != nil {
			t.Fatalf("CreateFormFile(%s): %v", skill.name, err)
		}
		if _, err := part.Write(skill.content); err != nil {
			t.Fatalf("Write skill archive %s: %v", skill.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/admin/templates", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if cookie != nil {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func makeSkillArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("Create zip entry %s: %v", name, err)
		}
		if _, err := io.Copy(entry, strings.NewReader(content)); err != nil {
			t.Fatalf("Write zip entry %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close zip writer: %v", err)
	}
	return buffer.Bytes()
}
