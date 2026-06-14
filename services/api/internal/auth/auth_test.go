package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "modernc.org/sqlite"
)

func TestHashPasswordAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword returned empty hash")
	}
	if hash == "correct horse battery staple" {
		t.Fatal("HashPassword returned plaintext password")
	}
	if !CheckPassword(hash, "correct horse battery staple") {
		t.Fatal("CheckPassword rejected the correct password")
	}
	if CheckPassword(hash, "wrong password") {
		t.Fatal("CheckPassword accepted the wrong password")
	}
}

func TestRepositoryFindsUsersByEmailWithoutPasswordHash(t *testing.T) {
	database := newAuthTestDB(t)
	hash, err := HashPassword("password")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO users (id, email, password_hash, role)
		VALUES ('user-1', 'USER@example.com', ?, 'user');
	`, hash)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := NewRepository(database)
	user, err := repo.FindUserByEmail(context.Background(), "USER@example.com")
	if err != nil {
		t.Fatalf("FindUserByEmail returned error: %v", err)
	}
	if user.ID != "user-1" || user.Email != "USER@example.com" || user.Role != RoleUser {
		t.Fatalf("unexpected user: %#v", user)
	}
	if user.PasswordHash != "" {
		t.Fatal("FindUserByEmail exposed password hash on User")
	}

	storedHash, err := repo.PasswordHashForUser(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("PasswordHashForUser returned error: %v", err)
	}
	if !CheckPassword(storedHash, "password") {
		t.Fatal("stored password hash does not verify")
	}

	_, err = repo.FindUserByEmail(context.Background(), "missing@example.com")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("FindUserByEmail missing user error = %v, want ErrUserNotFound", err)
	}
}

func TestSessionManagerCreatesParsesAndClearsSignedCookie(t *testing.T) {
	manager := NewSessionManager("test-secret")
	user := User{ID: "user-1", Email: "user@example.com", Role: RoleUser}

	recorder := httptest.NewRecorder()
	if err := manager.SetSessionCookie(recorder, user); err != nil {
		t.Fatalf("SetSessionCookie returned error: %v", err)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != SessionCookieName {
		t.Fatalf("cookie name = %q, want %q", cookie.Name, SessionCookieName)
	}
	if !cookie.HttpOnly {
		t.Fatal("cookie is not HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie SameSite = %v, want Lax", cookie.SameSite)
	}
	if cookie.Value == "" || cookie.Value == user.ID {
		t.Fatalf("cookie value is not a signed token: %q", cookie.Value)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	request.AddCookie(cookie)
	claims, err := manager.ParseRequest(request)
	if err != nil {
		t.Fatalf("ParseRequest returned error: %v", err)
	}
	if claims.User.ID != user.ID || claims.User.Email != user.Email || claims.User.Role != user.Role {
		t.Fatalf("unexpected claims user: %#v", claims.User)
	}

	tampered := *cookie
	tampered.Value = cookie.Value + "tampered"
	request = httptest.NewRequest(http.MethodGet, "/api/session", nil)
	request.AddCookie(&tampered)
	if _, err := manager.ParseRequest(request); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("tampered cookie error = %v, want ErrInvalidSession", err)
	}

	recorder = httptest.NewRecorder()
	manager.ClearSessionCookie(recorder)
	clearCookie := recorder.Result().Cookies()[0]
	if clearCookie.Name != SessionCookieName || clearCookie.MaxAge >= 0 {
		t.Fatalf("unexpected clear cookie: %#v", clearCookie)
	}
	if !clearCookie.HttpOnly || clearCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("clear cookie security attributes missing: %#v", clearCookie)
	}
}

func TestRBACRules(t *testing.T) {
	admin := User{ID: "admin-1", Role: RoleAdmin}
	user := User{ID: "user-1", Role: RoleUser}

	if err := RequireAdmin(admin); err != nil {
		t.Fatalf("RequireAdmin rejected admin: %v", err)
	}
	if err := RequireAdmin(user); !errors.Is(err, ErrForbidden) {
		t.Fatalf("RequireAdmin user error = %v, want ErrForbidden", err)
	}

	if err := RequireAgentOwner(user, "user-1"); err != nil {
		t.Fatalf("RequireAgentOwner rejected owner: %v", err)
	}
	if err := RequireAgentOwner(user, "other-user"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("RequireAgentOwner non-owner error = %v, want ErrForbidden", err)
	}
	if err := RequireAgentOwner(admin, "other-user"); err != nil {
		t.Fatalf("RequireAgentOwner rejected admin: %v", err)
	}

	if !CanViewTemplate(admin, "draft") || !CanViewTemplate(admin, "archived") {
		t.Fatal("admin should view all templates")
	}
	if !CanViewTemplate(user, "published") {
		t.Fatal("user should view published templates")
	}
	if CanViewTemplate(user, "draft") || CanViewTemplate(user, "archived") {
		t.Fatal("user should not view unpublished templates")
	}
}

func newAuthTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", "file:auth-test?mode=memory&cache=shared")
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
