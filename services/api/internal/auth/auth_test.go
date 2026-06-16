package auth

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

	user, err = repo.FindUserByID(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("FindUserByID returned error: %v", err)
	}
	if user.ID != "user-1" || user.Email != "USER@example.com" || user.Role != RoleUser {
		t.Fatalf("unexpected user by ID: %#v", user)
	}
	if user.PasswordHash != "" {
		t.Fatal("FindUserByID exposed password hash on User")
	}

	_, err = repo.FindUserByEmail(context.Background(), "missing@example.com")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("FindUserByEmail missing user error = %v, want ErrUserNotFound", err)
	}
	_, err = repo.FindUserByID(context.Background(), "missing-user")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("FindUserByID missing user error = %v, want ErrUserNotFound", err)
	}
}

func TestCreateUser_NormalizesEmailAndHashesPassword(t *testing.T) {
	database := newAuthTestDB(t)
	repo := NewRepository(database)

	user, err := repo.CreateUser(context.Background(), CreateUserParams{
		Email:    "  USER@Example.com ",
		Password: "abc12345",
		Role:     RoleUser,
	})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	if user.Email != "user@example.com" || user.Role != RoleUser {
		t.Fatalf("unexpected user: %#v", user)
	}
	if user.PasswordHash != "" {
		t.Fatalf("CreateUser exposed password hash: %#v", user)
	}

	hash, err := repo.PasswordHashForUser(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("PasswordHashForUser returned error: %v", err)
	}
	if !CheckPassword(hash, "abc12345") {
		t.Fatal("stored password hash does not verify")
	}
}

func TestCreateUser_RejectsInvalidEmailWeakPasswordAndDuplicateEmail(t *testing.T) {
	database := newAuthTestDB(t)
	repo := NewRepository(database)

	if _, err := repo.CreateUser(context.Background(), CreateUserParams{
		Email:    "bad-email",
		Password: "abc12345",
		Role:     RoleUser,
	}); !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("invalid email error = %v, want ErrInvalidEmail", err)
	}

	if _, err := repo.CreateUser(context.Background(), CreateUserParams{
		Email:    "user@example.com",
		Password: "password",
		Role:     RoleUser,
	}); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("weak password error = %v, want ErrInvalidPassword", err)
	}

	_, err := repo.CreateUser(context.Background(), CreateUserParams{
		Email:    "user@example.com",
		Password: "abc12345",
		Role:     RoleUser,
	})
	if err != nil {
		t.Fatalf("first CreateUser returned error: %v", err)
	}

	_, err = repo.CreateUser(context.Background(), CreateUserParams{
		Email:    "USER@example.com",
		Password: "xyz12345",
		Role:     RoleUser,
	})
	if !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("duplicate email error = %v, want ErrEmailAlreadyExists", err)
	}
}

func TestCreateUser_RejectsLegacyNormalizedEmailCollision(t *testing.T) {
	database := newAuthTestDB(t)
	hash, err := HashPassword("abc12345")
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

	repo := NewRepository(database)
	_, err = repo.CreateUser(context.Background(), CreateUserParams{
		Email:    "user@example.com",
		Password: "xyz12345",
		Role:     RoleUser,
	})
	if !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("CreateUser error = %v, want ErrEmailAlreadyExists", err)
	}
}

func TestFindUserByEmail_NormalizesLookupAfterCreateUser(t *testing.T) {
	database := newAuthTestDB(t)
	repo := NewRepository(database)

	created, err := repo.CreateUser(context.Background(), CreateUserParams{
		Email:    "user@example.com",
		Password: "abc12345",
		Role:     RoleUser,
	})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	user, err := repo.FindUserByEmail(context.Background(), "  USER@example.com ")
	if err != nil {
		t.Fatalf("FindUserByEmail returned error: %v", err)
	}
	if user.ID != created.ID || user.Email != "user@example.com" || user.Role != RoleUser {
		t.Fatalf("unexpected user: %#v", user)
	}
	if user.PasswordHash != "" {
		t.Fatalf("FindUserByEmail exposed password hash: %#v", user)
	}
}

func TestFindUserByEmail_NormalizesLookupForExistingNormalizedRecord(t *testing.T) {
	database := newAuthTestDB(t)
	hash, err := HashPassword("abc12345")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO users (id, email, password_hash, role)
		VALUES ('user-2', 'user@example.com', ?, 'user');
	`, hash)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := NewRepository(database)
	user, err := repo.FindUserByEmail(context.Background(), " USER@EXAMPLE.COM ")
	if err != nil {
		t.Fatalf("FindUserByEmail returned error: %v", err)
	}
	if user.ID != "user-2" || user.Email != "user@example.com" || user.Role != RoleUser {
		t.Fatalf("unexpected user: %#v", user)
	}
	if user.PasswordHash != "" {
		t.Fatalf("FindUserByEmail exposed password hash: %#v", user)
	}
}

func TestFindUserByEmail_PrefersExactLegacyMatchWhenNormalizedDuplicateExists(t *testing.T) {
	database := newAuthTestDB(t)
	hash, err := HashPassword("abc12345")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO users (id, email, password_hash, role)
		VALUES
			('legacy-user', ' USER@Example.com ', ?, 'user'),
			('normalized-user', 'user@example.com', ?, 'user');
	`, hash, hash)
	if err != nil {
		t.Fatalf("insert users: %v", err)
	}

	repo := NewRepository(database)
	user, err := repo.FindUserByEmail(context.Background(), " USER@Example.com ")
	if err != nil {
		t.Fatalf("FindUserByEmail returned error: %v", err)
	}
	if user.ID != "legacy-user" || user.Email != " USER@Example.com " {
		t.Fatalf("unexpected exact-match user: %#v", user)
	}
}

func TestFindUserByEmail_FallsBackToNormalizedMatchDeterministically(t *testing.T) {
	database := newAuthTestDB(t)
	hash, err := HashPassword("abc12345")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO users (id, email, password_hash, role)
		VALUES
			('legacy-user', ' USER@Example.com ', ?, 'user'),
			('normalized-user', 'user@example.com', ?, 'user');
	`, hash, hash)
	if err != nil {
		t.Fatalf("insert users: %v", err)
	}

	repo := NewRepository(database)
	user, err := repo.FindUserByEmail(context.Background(), "USER@example.com")
	if err != nil {
		t.Fatalf("FindUserByEmail returned error: %v", err)
	}
	if user.ID != "normalized-user" || user.Email != "user@example.com" {
		t.Fatalf("unexpected normalized-match user: %#v", user)
	}
}

func TestFindUserByEmail_FindsLegacyOnlyStoredEmailViaNormalizedInput(t *testing.T) {
	database := newAuthTestDB(t)
	hash, err := HashPassword("abc12345")
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

	repo := NewRepository(database)
	user, err := repo.FindUserByEmail(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("FindUserByEmail returned error: %v", err)
	}
	if user.ID != "legacy-user" || user.Email != " USER@Example.com " || user.Role != RoleUser {
		t.Fatalf("unexpected legacy-match user: %#v", user)
	}
}

func TestFindUserByEmail_RejectsAmbiguousLegacyNormalizedDuplicates(t *testing.T) {
	database := newAuthTestDB(t)
	hash, err := HashPassword("abc12345")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO users (id, email, password_hash, role)
		VALUES
			('legacy-user-1', ' USER@Example.com ', ?, 'user'),
			('legacy-user-2', 'user@example.com  ', ?, 'user');
	`, hash, hash)
	if err != nil {
		t.Fatalf("insert users: %v", err)
	}

	repo := NewRepository(database)
	_, err = repo.FindUserByEmail(context.Background(), "user@example.com")
	if !errors.Is(err, ErrEmailLookupAmbiguous) {
		t.Fatalf("FindUserByEmail error = %v, want ErrEmailLookupAmbiguous", err)
	}
}

func TestSessionManagerCreatesParsesAndClearsSignedCookie(t *testing.T) {
	manager := NewSessionManager("test-secret", true)
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
	if !cookie.Secure {
		t.Fatal("cookie Secure is false")
	}
	if cookie.Value == "" || cookie.Value == user.ID {
		t.Fatalf("cookie value is not a signed token: %q", cookie.Value)
	}
	payload := decodeSessionPayload(t, cookie.Value)
	if payload["user_id"] != user.ID {
		t.Fatalf("payload user_id = %#v, want %q", payload["user_id"], user.ID)
	}
	if _, ok := payload["user"]; ok {
		t.Fatalf("payload stores full user claims: %#v", payload)
	}
	if _, ok := payload["role"]; ok {
		t.Fatalf("payload stores role claim: %#v", payload)
	}
	if _, ok := payload["email"]; ok {
		t.Fatalf("payload stores email claim: %#v", payload)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	request.AddCookie(cookie)
	claims, err := manager.ParseRequest(request)
	if err != nil {
		t.Fatalf("ParseRequest returned error: %v", err)
	}
	if claims.UserID != user.ID {
		t.Fatalf("claims user ID = %q, want %q", claims.UserID, user.ID)
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
	if !clearCookie.Secure {
		t.Fatal("clear cookie Secure is false")
	}
}

func TestSessionManagerSupportsInsecureDevelopmentCookies(t *testing.T) {
	manager := NewSessionManager("test-secret", false)

	recorder := httptest.NewRecorder()
	if err := manager.SetSessionCookie(recorder, User{ID: "user-1"}); err != nil {
		t.Fatalf("SetSessionCookie returned error: %v", err)
	}
	cookie := recorder.Result().Cookies()[0]
	if cookie.Secure {
		t.Fatal("cookie Secure is true for insecure manager")
	}

	recorder = httptest.NewRecorder()
	manager.ClearSessionCookie(recorder)
	clearCookie := recorder.Result().Cookies()[0]
	if clearCookie.Secure {
		t.Fatal("clear cookie Secure is true for insecure manager")
	}
}

func TestSessionManagerRejectsExpiredTokens(t *testing.T) {
	manager := NewSessionManager("test-secret", false)
	token, err := manager.sign(SessionClaims{
		UserID:    "user-1",
		ExpiresAt: time.Now().UTC().Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("sign returned error: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	if _, err := manager.ParseRequest(request); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expired token error = %v, want ErrInvalidSession", err)
	}
}

func TestEnsureDefaultAdmin_FirstTime(t *testing.T) {
	database := newAuthTestDB(t)
	repo := NewRepository(database)

	if err := repo.EnsureDefaultAdmin(context.Background()); err != nil {
		t.Fatalf("EnsureDefaultAdmin returned error: %v", err)
	}

	user, err := repo.FindUserByEmail(context.Background(), "admin@123.com")
	if err != nil {
		t.Fatalf("FindUserByEmail returned error: %v", err)
	}
	if user.ID != "admin" || user.Email != "admin@123.com" || user.Role != RoleAdmin {
		t.Fatalf("unexpected user: %#v", user)
	}

	hash, err := repo.PasswordHashForUser(context.Background(), "admin")
	if err != nil {
		t.Fatalf("PasswordHashForUser returned error: %v", err)
	}
	if !CheckPassword(hash, "admin") {
		t.Fatal("default admin password does not verify")
	}
}

func TestEnsureDefaultAdmin_Idempotent(t *testing.T) {
	database := newAuthTestDB(t)
	repo := NewRepository(database)

	if err := repo.EnsureDefaultAdmin(context.Background()); err != nil {
		t.Fatalf("first EnsureDefaultAdmin returned error: %v", err)
	}
	if err := repo.EnsureDefaultAdmin(context.Background()); err != nil {
		t.Fatalf("second EnsureDefaultAdmin returned error: %v", err)
	}

	var count int
	err := database.QueryRow(`SELECT COUNT(*) FROM users WHERE email = 'admin@123.com'`).Scan(&count)
	if err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("got %d admin users, want 1", count)
	}
}

func TestEnsureDefaultAdmin_AlreadyExists(t *testing.T) {
	database := newAuthTestDB(t)
	repo := NewRepository(database)

	customHash, err := HashPassword("custom-password")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO users (id, email, password_hash, role)
		VALUES ('custom-admin', 'admin@123.com', ?, 'admin');
	`, customHash)
	if err != nil {
		t.Fatalf("insert custom admin: %v", err)
	}

	if err := repo.EnsureDefaultAdmin(context.Background()); err != nil {
		t.Fatalf("EnsureDefaultAdmin returned error: %v", err)
	}

	hash, err := repo.PasswordHashForUser(context.Background(), "custom-admin")
	if err != nil {
		t.Fatalf("PasswordHashForUser returned error: %v", err)
	}
	if !CheckPassword(hash, "custom-password") {
		t.Fatal("existing admin password was modified")
	}
}

func TestEnsureDefaultAdmin_UpgradesLegacySeededAdmin(t *testing.T) {
	database := newAuthTestDB(t)
	repo := NewRepository(database)

	legacyHash, err := HashPassword("admin")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO users (id, email, password_hash, role)
		VALUES ('admin', 'admin', ?, 'admin');
	`, legacyHash)
	if err != nil {
		t.Fatalf("insert legacy admin: %v", err)
	}

	if err := repo.EnsureDefaultAdmin(context.Background()); err != nil {
		t.Fatalf("EnsureDefaultAdmin returned error: %v", err)
	}

	var count int
	err = database.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("got %d users, want 1", count)
	}

	user, err := repo.FindUserByEmail(context.Background(), "admin@123.com")
	if err != nil {
		t.Fatalf("FindUserByEmail returned error: %v", err)
	}
	if user.ID != "admin" || user.Email != "admin@123.com" || user.Role != RoleAdmin {
		t.Fatalf("unexpected upgraded admin after ensure: %#v", user)
	}

	hash, err := repo.PasswordHashForUser(context.Background(), "admin")
	if err != nil {
		t.Fatalf("PasswordHashForUser returned error: %v", err)
	}
	if !CheckPassword(hash, "admin") {
		t.Fatal("legacy admin password does not verify after upgrade")
	}

	_, err = repo.FindUserByEmail(context.Background(), "admin")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("FindUserByEmail(admin) error = %v, want ErrUserNotFound", err)
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
	database, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "-")))
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

func decodeSessionPayload(t *testing.T, token string) map[string]any {
	t.Helper()
	encodedPayload, _, ok := strings.Cut(token, ".")
	if !ok {
		t.Fatalf("session token missing signature separator: %q", token)
	}
	payload, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal payload %q: %v", payload, err)
	}
	return decoded
}
