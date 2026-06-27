package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/verification"
	"github.com/gin-gonic/gin"
)

type testVerificationMailer struct {
	lastEmail string
	lastCode  string
	err       error
}

func (m *testVerificationMailer) SendRegistrationCode(_ context.Context, email, code string) error {
	m.lastEmail = email
	m.lastCode = code
	return m.err
}

// newRegistrationTestRouter 装配带验证码服务的注册路由，返回路由与捕获验证码的 mailer。
func newRegistrationTestRouter(t *testing.T, database *sql.DB) (*gin.Engine, *testVerificationMailer) {
	t.Helper()
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	mailer := &testVerificationMailer{}
	service := verification.NewService(
		verification.NewMemoryStore(func() time.Time { return now }),
		mailer,
		func() time.Time { return now },
	)
	router := NewRouter(Dependencies{
		AuthRepository:      auth.NewRepository(database),
		SessionManager:      auth.NewSessionManager("test-secret", false),
		VerificationService: service,
	})
	return router, mailer
}

func sendTestEmailCode(t *testing.T, router *gin.Engine, email string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/registration/email-codes",
		bytes.NewBufferString(fmt.Sprintf(`{"email":%q}`, email))))
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("send code status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func registerRequest(email, password, code string) *http.Request {
	body := fmt.Sprintf(`{"email":%q,"password":%q,"emailCode":%q}`, email, password, code)
	return httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(body))
}

func TestRegistrationRouteSendsEmailCodeAndReturnsAccepted(t *testing.T) {
	database := newHTTPTestDB(t)
	router, mailer := newRegistrationTestRouter(t, database)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/registration/email-codes",
		bytes.NewBufferString(`{"email":" USER@example.com "}`)))

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("send code status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Retry-After"); got != "" {
		t.Fatalf("Retry-After = %q, want empty on success", got)
	}
	if mailer.lastEmail != "user@example.com" {
		t.Fatalf("mailer lastEmail = %q", mailer.lastEmail)
	}
}

func TestRegistrationRouteSendCodeRejectsInvalidEmail(t *testing.T) {
	database := newHTTPTestDB(t)
	router, _ := newRegistrationTestRouter(t, database)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/registration/email-codes",
		bytes.NewBufferString(`{"email":"not-an-email"}`)))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("send code status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var errResp map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if errResp["error"] != "invalid_email" {
		t.Fatalf("error code = %q", errResp["error"])
	}
}

func TestRegistrationRouteSendCodeReturnsEmailSendFailedWithoutLeakingDetails(t *testing.T) {
	database := newHTTPTestDB(t)
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	mailer := &testVerificationMailer{err: errors.New("brevo status 400: sender not verified")}
	service := verification.NewService(
		verification.NewMemoryStore(func() time.Time { return now }),
		mailer,
		func() time.Time { return now },
	)
	router := NewRouter(Dependencies{
		AuthRepository:      auth.NewRepository(database),
		SessionManager:      auth.NewSessionManager("test-secret", false),
		VerificationService: service,
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/registration/email-codes",
		bytes.NewBufferString(`{"email":"user@example.com"}`)))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("send code status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var errResp map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if errResp["error"] != "email_send_failed" {
		t.Fatalf("error code = %q, want email_send_failed", errResp["error"])
	}
	// 底层 Brevo 错误细节不得泄漏到响应体。
	if bytes.Contains(recorder.Body.Bytes(), []byte("sender not verified")) {
		t.Fatalf("response leaked internal error: %s", recorder.Body.String())
	}
}

func TestRegistrationRouteSendCodeRejectsAlreadyRegisteredEmail(t *testing.T) {
	database := newHTTPTestDB(t)
	hash, err := auth.HashPassword("abc12345")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO users (id, email, password_hash, role)
		VALUES ('legacy-user', 'user@example.com', ?, 'user');
	`, hash)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	router, mailer := newRegistrationTestRouter(t, database)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/registration/email-codes",
		bytes.NewBufferString(`{"email":"user@example.com"}`)))

	if recorder.Code != http.StatusConflict {
		t.Fatalf("send code status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var errResp map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if errResp["error"] != "email_already_exists" {
		t.Fatalf("error code = %q", errResp["error"])
	}
	if mailer.lastCode != "" {
		t.Fatalf("mailer should not be called for registered email, lastCode = %q", mailer.lastCode)
	}
}

func TestRegistrationRouteRejectsMissingEmailCode(t *testing.T) {
	database := newHTTPTestDB(t)
	router, _ := newRegistrationTestRouter(t, database)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, registerRequest("user@example.com", "abc12345", ""))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("registration status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var errResp map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if errResp["error"] != "email_code_required" {
		t.Fatalf("error code = %q", errResp["error"])
	}
}

func TestRegistrationRouteCreatesUserWithoutSessionCookie(t *testing.T) {
	database := newHTTPTestDB(t)
	router, mailer := newRegistrationTestRouter(t, database)

	sendTestEmailCode(t, router, "  USER@example.com ")
	code := mailer.lastCode

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, registerRequest("user@example.com", "abc12345", code))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("registration status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if len(recorder.Result().Cookies()) != 0 {
		t.Fatalf("registration set cookies: %#v", recorder.Result().Cookies())
	}
	var response struct {
		User auth.User `json:"user"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response %q: %v", recorder.Body.Bytes(), err)
	}
	if response.User.ID == "" || response.User.Email != "user@example.com" || response.User.Role != auth.RoleUser {
		t.Fatalf("response user = %#v", response.User)
	}
	if response.User.PasswordHash != "" {
		t.Fatalf("registration response exposed password hash: %#v", response.User)
	}

	// 注册成功后验证码被消费，再次使用同码注册应被拒绝。
	reuseRecorder := httptest.NewRecorder()
	router.ServeHTTP(reuseRecorder, registerRequest("user@example.com", "abc12345", code))
	if reuseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("reuse code status = %d, body = %s", reuseRecorder.Code, reuseRecorder.Body.String())
	}
	var reuseErr map[string]string
	if err := json.Unmarshal(reuseRecorder.Body.Bytes(), &reuseErr); err != nil {
		t.Fatalf("unmarshal reuse response: %v", err)
	}
	if reuseErr["error"] != "email_code_invalid" {
		t.Fatalf("reuse code error = %q, want email_code_invalid", reuseErr["error"])
	}
}

func TestRegistrationRouteRejectsWeakPassword(t *testing.T) {
	database := newHTTPTestDB(t)
	router, mailer := newRegistrationTestRouter(t, database)

	sendTestEmailCode(t, router, "user@example.com")
	code := mailer.lastCode

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, registerRequest("user@example.com", "password", code))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("weak password status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var errResp map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if errResp["error"] != "invalid_password" {
		t.Fatalf("weak password body = %q", recorder.Body.String())
	}
	if len(recorder.Result().Cookies()) != 0 {
		t.Fatalf("weak password set cookies: %#v", recorder.Result().Cookies())
	}
}

func TestRegistrationRouteRejectsDuplicateEmail(t *testing.T) {
	database := newHTTPTestDB(t)
	router, mailer := newRegistrationTestRouter(t, database)

	sendTestEmailCode(t, router, "user@example.com")
	code := mailer.lastCode

	// 模拟竞态：发码后、注册前邮箱被占用。
	hash, err := auth.HashPassword("abc12345")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO users (id, email, password_hash, role)
		VALUES ('legacy-user', 'user@example.com', ?, 'user');
	`, hash)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, registerRequest("user@example.com", "abc12345", code))

	if recorder.Code != http.StatusConflict {
		t.Fatalf("duplicate registration status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var errResp map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if errResp["error"] != "email_already_exists" {
		t.Fatalf("duplicate registration body = %q", recorder.Body.String())
	}

	// 用户创建失败时验证码不应被消费：再次用同码注册仍应因邮箱冲突返回 409，而非 email_code_invalid。
	reuseRecorder := httptest.NewRecorder()
	router.ServeHTTP(reuseRecorder, registerRequest("user@example.com", "abc12345", code))
	if reuseRecorder.Code != http.StatusConflict {
		t.Fatalf("reuse after failed create status = %d, body = %s", reuseRecorder.Code, reuseRecorder.Body.String())
	}
	var reuseErr map[string]string
	if err := json.Unmarshal(reuseRecorder.Body.Bytes(), &reuseErr); err != nil {
		t.Fatalf("unmarshal reuse response: %v", err)
	}
	if reuseErr["error"] != "email_already_exists" {
		t.Fatalf("reuse after failed create error = %q, want email_already_exists (code not consumed)", reuseErr["error"])
	}
}

func TestRegistrationRouteEmailCodeCooldownReturnsRetryAfter(t *testing.T) {
	database := newHTTPTestDB(t)
	router, _ := newRegistrationTestRouter(t, database)

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/api/registration/email-codes", bytes.NewBufferString(`{"email":"user@example.com"}`)))
	if first.Code != http.StatusAccepted {
		t.Fatalf("first send status = %d, body = %s", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	router.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/api/registration/email-codes", bytes.NewBufferString(`{"email":"user@example.com"}`)))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("cooldown status = %d, body = %s", second.Code, second.Body.String())
	}
	if retryAfter := second.Header().Get("Retry-After"); retryAfter != "60" {
		t.Fatalf("Retry-After = %q, want 60", retryAfter)
	}
	var errResp map[string]string
	if err := json.Unmarshal(second.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if errResp["error"] != "email_code_cooldown" {
		t.Fatalf("cooldown error = %q, want email_code_cooldown", errResp["error"])
	}
}

func TestRegistrationRouteEmailCodeRateLimitedReturnsRetryAfter(t *testing.T) {
	database := newHTTPTestDB(t)
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	mailer := &testVerificationMailer{}
	service := verification.NewService(
		verification.NewMemoryStore(func() time.Time { return now }),
		mailer,
		func() time.Time { return now },
	)
	router := NewRouter(Dependencies{
		AuthRepository:      auth.NewRepository(database),
		SessionManager:      auth.NewSessionManager("test-secret", false),
		VerificationService: service,
	})

	// 发送 5 次，每次推进 61 秒绕过冷却但仍处于 1 小时窗口内。
	for i := 0; i < 5; i++ {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/registration/email-codes", bytes.NewBufferString(`{"email":"user@example.com"}`)))
		if recorder.Code != http.StatusAccepted {
			t.Fatalf("send %d status = %d, body = %s", i+1, recorder.Code, recorder.Body.String())
		}
		now = now.Add(61 * time.Second)
	}

	sixth := httptest.NewRecorder()
	router.ServeHTTP(sixth, httptest.NewRequest(http.MethodPost, "/api/registration/email-codes", bytes.NewBufferString(`{"email":"user@example.com"}`)))
	if sixth.Code != http.StatusTooManyRequests {
		t.Fatalf("rate limited status = %d, body = %s", sixth.Code, sixth.Body.String())
	}
	if retryAfter := sixth.Header().Get("Retry-After"); retryAfter != "3600" {
		t.Fatalf("Retry-After = %q, want 3600", retryAfter)
	}
	var errResp map[string]string
	if err := json.Unmarshal(sixth.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if errResp["error"] != "email_code_rate_limited" {
		t.Fatalf("rate limited error = %q, want email_code_rate_limited", errResp["error"])
	}
}

func TestRegistrationRouteRejectsInvalidAndTrailingJSON(t *testing.T) {
	database := newHTTPTestDB(t)
	router, _ := newRegistrationTestRouter(t, database)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(`{"email":"user@example.com","password":"abc12345"`))
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var errResp map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if errResp["error"] != "invalid_json" {
		t.Fatalf("invalid JSON body = %q", recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(`{"email":"user@example.com","password":"abc12345"} {}`))
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("trailing JSON status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var errResp2 map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &errResp2); err != nil {
		t.Fatalf("unmarshal trailing error response: %v", err)
	}
	if errResp2["error"] != "invalid_json" {
		t.Fatalf("trailing JSON body = %q", recorder.Body.String())
	}
	if len(recorder.Result().Cookies()) != 0 {
		t.Fatalf("trailing JSON set cookies: %#v", recorder.Result().Cookies())
	}
}

func TestRouterWithNilAuthRepositoryDoesNotMountRegistrationRoute(t *testing.T) {
	router := NewRouter(Dependencies{
		SessionManager: auth.NewSessionManager("test-secret", false),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(`{"email":"user@example.com","password":"abc12345"}`))
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("nil auth registration status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestRegistrationRouteRejectsExpiredEmailCode(t *testing.T) {
	database := newHTTPTestDB(t)
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	mailer := &testVerificationMailer{}
	service := verification.NewService(
		verification.NewMemoryStore(func() time.Time { return now }),
		mailer,
		func() time.Time { return now },
	)
	router := NewRouter(Dependencies{
		AuthRepository:      auth.NewRepository(database),
		SessionManager:      auth.NewSessionManager("test-secret", false),
		VerificationService: service,
	})

	sendTestEmailCode(t, router, "user@example.com")
	code := mailer.lastCode

	// 推进时钟超过 CodeTTL，验证码过期。
	now = now.Add(verification.CodeTTL + time.Minute)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, registerRequest("user@example.com", "abc12345", code))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expired code status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var errResp map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if errResp["error"] != "email_code_expired" {
		t.Fatalf("expired code error = %q, want email_code_expired", errResp["error"])
	}
}

func TestRegistrationRouteRejectsExhaustedEmailCode(t *testing.T) {
	database := newHTTPTestDB(t)
	router, mailer := newRegistrationTestRouter(t, database)

	sendTestEmailCode(t, router, "user@example.com")
	code := mailer.lastCode

	// 连续失败 MaxVerifyAttempts 次后验证码被锁定。
	for i := 0; i < verification.MaxVerifyAttempts; i++ {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, registerRequest("user@example.com", "abc12345", "000000"))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d status = %d, body = %s", i+1, recorder.Code, recorder.Body.String())
		}
	}

	// 锁定后即使提交正确码也返回 email_code_attempts_exhausted。
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, registerRequest("user@example.com", "abc12345", code))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("exhausted status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var errResp map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if errResp["error"] != "email_code_attempts_exhausted" {
		t.Fatalf("exhausted error = %q, want email_code_attempts_exhausted", errResp["error"])
	}
}

func TestRegistrationRouteNormalizesStoredEmail(t *testing.T) {
	database := newHTTPTestDB(t)
	router, mailer := newRegistrationTestRouter(t, database)

	// 用带空格与大写的邮箱发码；服务内部归一化存储。
	sendTestEmailCode(t, router, "  USER@example.com ")
	code := mailer.lastCode

	// 注册时同样提交未归一化邮箱，建号应使用归一化后的地址。
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, registerRequest("  USER@example.com ", "abc12345", code))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("registration status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		User auth.User `json:"user"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.User.Email != "user@example.com" {
		t.Fatalf("stored email = %q, want normalized user@example.com", response.User.Email)
	}
}
