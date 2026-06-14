package http

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agentforge.local/services/api/internal/auth"

	_ "modernc.org/sqlite"
)

func TestSessionRoutesLoginCurrentAndLogout(t *testing.T) {
	database := newHTTPTestDB(t)
	hash, err := auth.HashPassword("secret-password")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO users (id, email, password_hash, role)
		VALUES ('user-1', 'user@example.com', ?, 'user');
	`, hash)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	router := NewRouter(Dependencies{
		AuthRepository: auth.NewRepository(database),
		SessionManager: auth.NewSessionManager("test-secret"),
	})

	loginBody := bytes.NewBufferString(`{"email":"user@example.com","password":"secret-password"}`)
	loginRecorder := httptest.NewRecorder()
	router.ServeHTTP(loginRecorder, httptest.NewRequest(http.MethodPost, "/api/sessions", loginBody))
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginRecorder.Code, loginRecorder.Body.String())
	}
	sessionCookie := findCookie(t, loginRecorder.Result().Cookies(), auth.SessionCookieName)
	if !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie missing security attributes: %#v", sessionCookie)
	}
	assertUserResponse(t, loginRecorder.Body.Bytes(), "user-1", "user@example.com", auth.RoleUser)
	if bytes.Contains(loginRecorder.Body.Bytes(), []byte("password_hash")) {
		t.Fatal("login response exposed password_hash")
	}

	currentRequest := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	currentRequest.AddCookie(sessionCookie)
	currentRecorder := httptest.NewRecorder()
	router.ServeHTTP(currentRecorder, currentRequest)
	if currentRecorder.Code != http.StatusOK {
		t.Fatalf("current session status = %d, body = %s", currentRecorder.Code, currentRecorder.Body.String())
	}
	assertUserResponse(t, currentRecorder.Body.Bytes(), "user-1", "user@example.com", auth.RoleUser)

	logoutRequest := httptest.NewRequest(http.MethodDelete, "/api/session", nil)
	logoutRecorder := httptest.NewRecorder()
	router.ServeHTTP(logoutRecorder, logoutRequest)
	if logoutRecorder.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, body = %s", logoutRecorder.Code, logoutRecorder.Body.String())
	}
	clearCookie := findCookie(t, logoutRecorder.Result().Cookies(), auth.SessionCookieName)
	if clearCookie.MaxAge >= 0 {
		t.Fatalf("logout did not clear session cookie: %#v", clearCookie)
	}
}

func TestSessionRoutesRejectInvalidCredentialsAndMissingSession(t *testing.T) {
	database := newHTTPTestDB(t)
	hash, err := auth.HashPassword("secret-password")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO users (id, email, password_hash, role)
		VALUES ('user-1', 'user@example.com', ?, 'user');
	`, hash)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	router := NewRouter(Dependencies{
		AuthRepository: auth.NewRepository(database),
		SessionManager: auth.NewSessionManager("test-secret"),
	})

	loginRecorder := httptest.NewRecorder()
	router.ServeHTTP(loginRecorder, httptest.NewRequest(http.MethodPost, "/api/sessions", bytes.NewBufferString(`{"email":"user@example.com","password":"wrong"}`)))
	if loginRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("invalid login status = %d, want 401", loginRecorder.Code)
	}
	if len(loginRecorder.Result().Cookies()) != 0 {
		t.Fatal("invalid login set cookies")
	}

	currentRecorder := httptest.NewRecorder()
	router.ServeHTTP(currentRecorder, httptest.NewRequest(http.MethodGet, "/api/session", nil))
	if currentRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("missing session status = %d, want 401", currentRecorder.Code)
	}
}

func TestSessionMiddlewareAddsAuthenticatedUser(t *testing.T) {
	manager := auth.NewSessionManager("test-secret")
	user := auth.User{ID: "user-1", Email: "user@example.com", Role: auth.RoleUser}
	cookieRecorder := httptest.NewRecorder()
	if err := manager.SetSessionCookie(cookieRecorder, user); err != nil {
		t.Fatalf("SetSessionCookie returned error: %v", err)
	}

	var got auth.User
	handler := SessionMiddleware(manager)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = UserFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookieRecorder.Result().Cookies()[0])
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("middleware status = %d", recorder.Code)
	}
	if got.ID != user.ID || got.Email != user.Email || got.Role != user.Role {
		t.Fatalf("context user = %#v, want %#v", got, user)
	}
}

func newHTTPTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", "file:http-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	_, err = database.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
	`)
	if err != nil {
		t.Fatalf("create users table: %v", err)
	}
	return database
}

func findCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("cookie %q not found in %#v", name, cookies)
	return nil
}

func assertUserResponse(t *testing.T, body []byte, id, email string, role auth.Role) {
	t.Helper()
	var response struct {
		User auth.User `json:"user"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("unmarshal response %q: %v", body, err)
	}
	if response.User.ID != id || response.User.Email != email || response.User.Role != role {
		t.Fatalf("response user = %#v", response.User)
	}
}
