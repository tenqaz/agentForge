package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agentforge.local/services/api/internal/auth"
)

func TestRegistrationRouteCreatesUserWithoutSessionCookie(t *testing.T) {
	database := newHTTPTestDB(t)
	router := NewRouter(Dependencies{
		AuthRepository: auth.NewRepository(database),
		SessionManager: auth.NewSessionManager("test-secret", false),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(`{"email":"  USER@example.com ","password":"abc12345"}`))
	router.ServeHTTP(recorder, request)

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
}

func TestRegistrationRouteRejectsDuplicateEmail(t *testing.T) {
	database := newHTTPTestDB(t)
	hash, err := auth.HashPassword("abc12345")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO users (id, email, password_hash, role)
		VALUES ('legacy-user', ' USER@Example.com ', ?, 'user');
	`, hash)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	router := NewRouter(Dependencies{
		AuthRepository: auth.NewRepository(database),
		SessionManager: auth.NewSessionManager("test-secret", false),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(`{"email":"user@example.com","password":"xyz12345"}`))
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("duplicate registration status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.String() != "{\"error\":\"email_already_exists\"}\n" {
		t.Fatalf("duplicate registration body = %q", recorder.Body.String())
	}
	if len(recorder.Result().Cookies()) != 0 {
		t.Fatalf("duplicate registration set cookies: %#v", recorder.Result().Cookies())
	}
}

func TestRegistrationRouteRejectsWeakPassword(t *testing.T) {
	database := newHTTPTestDB(t)
	router := NewRouter(Dependencies{
		AuthRepository: auth.NewRepository(database),
		SessionManager: auth.NewSessionManager("test-secret", false),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(`{"email":"user@example.com","password":"password"}`))
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("weak password status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.String() != "{\"error\":\"invalid_password\"}\n" {
		t.Fatalf("weak password body = %q", recorder.Body.String())
	}
	if len(recorder.Result().Cookies()) != 0 {
		t.Fatalf("weak password set cookies: %#v", recorder.Result().Cookies())
	}
}

func TestRegistrationRouteRejectsInvalidAndTrailingJSON(t *testing.T) {
	database := newHTTPTestDB(t)
	router := NewRouter(Dependencies{
		AuthRepository: auth.NewRepository(database),
		SessionManager: auth.NewSessionManager("test-secret", false),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(`{"email":"user@example.com","password":"abc12345"`))
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.String() != "{\"error\":\"invalid_json\"}\n" {
		t.Fatalf("invalid JSON body = %q", recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(`{"email":"user@example.com","password":"abc12345"} {}`))
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("trailing JSON status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.String() != "{\"error\":\"invalid_json\"}\n" {
		t.Fatalf("trailing JSON body = %q", recorder.Body.String())
	}
	if len(recorder.Result().Cookies()) != 0 {
		t.Fatalf("trailing JSON set cookies: %#v", recorder.Result().Cookies())
	}
}
