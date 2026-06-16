# Registration Feature Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add public email/password registration, redirect successful registrations to the login page without creating a session, and change the bootstrapped admin email to `admin@123.com`.

**Architecture:** Extend the existing auth repository with user-creation and validation logic, expose a new public HTTP registration endpoint, and add a minimal Next.js registration page wired through the existing API client. Keep session creation exclusive to `POST /api/sessions` so registration remains a separate flow.

**Tech Stack:** Go, `net/http`, SQLite, Next.js App Router, TypeScript, Vitest, Playwright

---

## File Structure

- Modify: `services/api/internal/auth/repository.go`
  - Add user creation params, validation helpers, conflict/validation errors, and update default admin email.
- Modify: `services/api/internal/http/router.go`
  - Extend the auth repository interface and register the public signup route.
- Create: `services/api/internal/http/registration_handlers.go`
  - Keep registration request parsing and HTTP status mapping separate from login handlers.
- Create: `services/api/internal/http/registration_handlers_test.go`
  - Focused route tests for signup success, validation failure, duplicate email, and “no session cookie”.
- Modify: `services/api/internal/auth/auth_test.go`
  - Add repository-level tests for signup normalization/validation and update default admin assertions.
- Modify: `web/lib/api/types.ts`
  - Add request/response types if needed by the new client helper.
- Modify: `web/lib/api/client.ts`
  - Add a `registerUser` helper alongside `getSession` and template helpers.
- Create: `web/app/register/actions.ts`
  - Keep the API call wrapper parallel to `web/app/login/actions.ts`.
- Create: `web/app/register/page.tsx`
  - New registration page with email/password form, success redirect, and error display.
- Modify: `web/app/login/page.tsx`
  - Add a visible link to the register page and optionally show a success message after redirect.
- Modify: `web/tests/api-client.test.ts`
  - Add client coverage for signup error parsing.
- Create: `web/tests/register-flow.spec.ts`
  - Playwright coverage for the public registration flow.

### Task 1: Extend the auth repository for registration

**Files:**
- Modify: `services/api/internal/auth/repository.go`
- Test: `services/api/internal/auth/auth_test.go`

- [ ] **Step 1: Write the failing repository tests for user creation and the new default admin email**

Add these tests to `services/api/internal/auth/auth_test.go`:

```go
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
```

Update the existing default-admin tests so they look up `admin@123.com` instead of `admin`:

```go
user, err := repo.FindUserByEmail(context.Background(), "admin@123.com")
if err != nil {
	t.Fatalf("FindUserByEmail returned error: %v", err)
}
if user.ID != "admin" || user.Email != "admin@123.com" || user.Role != RoleAdmin {
	t.Fatalf("unexpected user: %#v", user)
}
```

- [ ] **Step 2: Run the auth package tests and confirm the new tests fail for missing symbols**

Run:

```bash
cd /Users/zhengwenfeng/work/projs/AgentForge/services/api
go test ./internal/auth -run 'Test(CreateUser|EnsureDefaultAdmin)' -v
```

Expected: FAIL with undefined `CreateUser`, `CreateUserParams`, `ErrInvalidEmail`, `ErrInvalidPassword`, or `ErrEmailAlreadyExists`.

- [ ] **Step 3: Add minimal repository implementation and validation helpers**

Update `services/api/internal/auth/repository.go` with these additions:

```go
var (
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidEmail      = errors.New("invalid email")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrEmailAlreadyExists = errors.New("email already exists")
)

type CreateUserParams struct {
	Email    string
	Password string
	Role     Role
}

func (r *Repository) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	email := normalizeEmail(params.Email)
	if !isValidEmail(email) {
		return User{}, ErrInvalidEmail
	}
	if !isValidPassword(params.Password) {
		return User{}, ErrInvalidPassword
	}
	if params.Role == "" {
		params.Role = RoleUser
	}

	hash, err := HashPassword(params.Password)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}

	id := strings.ToLower(strings.ReplaceAll(strings.Split(email, "@")[0], ".", "-"))
	_, err = r.database.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, role)
		VALUES (?, ?, ?, ?);
	`, id, email, hash, params.Role)
	if isUniqueConstraint(err) {
		return User{}, ErrEmailAlreadyExists
	}
	if err != nil {
		return User{}, err
	}
	return r.FindUserByEmail(ctx, email)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
```

Add the password/email helpers in the same file:

```go
func isValidEmail(email string) bool {
	if email == "" {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}

func isValidPassword(password string) bool {
	if len(password) < 8 {
		return false
	}
	hasLetter := false
	hasDigit := false
	for _, r := range password {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	return hasLetter && hasDigit
}
```

Update `EnsureDefaultAdmin` to use the new email:

```go
_, err := r.FindUserByEmail(ctx, "admin@123.com")
// ...
_, err = r.database.ExecContext(ctx, `
	INSERT INTO users (id, email, password_hash, role)
	VALUES (?, ?, ?, ?);
`, "admin", "admin@123.com", hash, RoleAdmin)
```

Implementation note: if the repository already has a preferred ID generation helper elsewhere, use that instead of the placeholder local-part conversion shown above. The important contract is “unique enough for tests plus hidden from the API”.

- [ ] **Step 4: Re-run the auth package tests and get them green**

Run:

```bash
cd /Users/zhengwenfeng/work/projs/AgentForge/services/api
go test ./internal/auth -run 'Test(CreateUser|EnsureDefaultAdmin)' -v
```

Expected: PASS for the new `CreateUser` tests and the updated default-admin tests.

- [ ] **Step 5: Commit the repository changes**

Run:

```bash
cd /Users/zhengwenfeng/work/projs/AgentForge
git add services/api/internal/auth/repository.go services/api/internal/auth/auth_test.go
git commit -m "feat: add auth repository registration support"
```

### Task 2: Add the public registration endpoint

**Files:**
- Modify: `services/api/internal/http/router.go`
- Create: `services/api/internal/http/registration_handlers.go`
- Test: `services/api/internal/http/registration_handlers_test.go`

- [ ] **Step 1: Write the failing HTTP route tests**

Create `services/api/internal/http/registration_handlers_test.go` with:

```go
func TestRegistrationRouteCreatesUserWithoutSessionCookie(t *testing.T) {
	database := newHTTPTestDB(t)
	router := NewRouter(Dependencies{
		AuthRepository: auth.NewRepository(database),
		SessionManager: auth.NewSessionManager("test-secret", false),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(`{"email":"new@example.com","password":"abc12345"}`))
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("registration status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if len(recorder.Result().Cookies()) != 0 {
		t.Fatalf("registration unexpectedly set cookies: %#v", recorder.Result().Cookies())
	}
	var response struct {
		User auth.User `json:"user"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.User.ID == "" || response.User.Email != "new@example.com" || response.User.Role != auth.RoleUser {
		t.Fatalf("unexpected user response: %#v", response.User)
	}
}

func TestRegistrationRouteRejectsDuplicateEmailAndWeakPassword(t *testing.T) {
	database := newHTTPTestDB(t)
	repo := auth.NewRepository(database)
	if _, err := repo.CreateUser(context.Background(), auth.CreateUserParams{
		Email: "user@example.com", Password: "abc12345", Role: auth.RoleUser,
	}); err != nil {
		t.Fatalf("seed CreateUser returned error: %v", err)
	}
	router := NewRouter(Dependencies{
		AuthRepository: repo,
		SessionManager: auth.NewSessionManager("test-secret", false),
	})

	duplicate := httptest.NewRecorder()
	router.ServeHTTP(duplicate, httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(`{"email":"USER@example.com","password":"abc12345"}`)))
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, body = %s", duplicate.Code, duplicate.Body.String())
	}

	weak := httptest.NewRecorder()
	router.ServeHTTP(weak, httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(`{"email":"weak@example.com","password":"password"}`)))
	if weak.Code != http.StatusBadRequest {
		t.Fatalf("weak password status = %d, body = %s", weak.Code, weak.Body.String())
	}
}
```

- [ ] **Step 2: Run the HTTP tests and confirm the route is missing**

Run:

```bash
cd /Users/zhengwenfeng/work/projs/AgentForge/services/api
go test ./internal/http -run 'TestRegistrationRoute' -v
```

Expected: FAIL with `404`, missing route wiring, or missing repository methods on the HTTP interface.

- [ ] **Step 3: Implement a dedicated registration handler and wire it into the router**

Create `services/api/internal/http/registration_handlers.go`:

```go
package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"agentforge.local/services/api/internal/auth"
)

type RegistrationHandlers struct {
	authRepository AuthRepository
}

func NewRegistrationHandlers(authRepository AuthRepository) *RegistrationHandlers {
	return &RegistrationHandlers{authRepository: authRepository}
}

func (h *RegistrationHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	user, err := h.authRepository.CreateUser(r.Context(), auth.CreateUserParams{
		Email:    request.Email,
		Password: request.Password,
		Role:     auth.RoleUser,
	})
	switch {
	case err == nil:
		writeJSON(w, http.StatusCreated, userResponse{User: user})
	case errors.Is(err, auth.ErrInvalidEmail):
		writeError(w, http.StatusBadRequest, "invalid_email")
	case errors.Is(err, auth.ErrInvalidPassword):
		writeError(w, http.StatusBadRequest, "invalid_password")
	case errors.Is(err, auth.ErrEmailAlreadyExists):
		writeError(w, http.StatusConflict, "email_already_exists")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error")
	}
}
```

Update the `AuthRepository` interface in `services/api/internal/http/router.go`:

```go
type AuthRepository interface {
	FindUserByEmail(ctx context.Context, email string) (auth.User, error)
	FindUserByID(ctx context.Context, userID string) (auth.User, error)
	PasswordHashForUser(ctx context.Context, userID string) (string, error)
	CreateUser(ctx context.Context, params auth.CreateUserParams) (auth.User, error)
}
```

Wire the route in `NewRouter`:

```go
registrationHandlers := NewRegistrationHandlers(deps.AuthRepository)
mux.HandleFunc("POST /api/users", registrationHandlers.Create)
sessionHandlers := NewSessionHandlers(deps.AuthRepository, deps.SessionManager)
```

- [ ] **Step 4: Run the HTTP tests and the full auth+http backend slice**

Run:

```bash
cd /Users/zhengwenfeng/work/projs/AgentForge/services/api
go test ./internal/http -run 'Test(RegistrationRoute|SessionRoutes)' -v
go test ./internal/auth ./internal/http
```

Expected: PASS for registration and existing session tests; no session cookie should be emitted by `POST /api/users`.

- [ ] **Step 5: Commit the HTTP layer changes**

Run:

```bash
cd /Users/zhengwenfeng/work/projs/AgentForge
git add services/api/internal/http/router.go services/api/internal/http/registration_handlers.go services/api/internal/http/registration_handlers_test.go
git commit -m "feat: add public registration endpoint"
```

### Task 3: Add frontend registration flow

**Files:**
- Modify: `web/lib/api/client.ts`
- Modify: `web/lib/api/types.ts`
- Create: `web/app/register/actions.ts`
- Create: `web/app/register/page.tsx`
- Modify: `web/app/login/page.tsx`
- Test: `web/tests/api-client.test.ts`

- [ ] **Step 1: Write the failing frontend tests**

Add a new API client test in `web/tests/api-client.test.ts`:

```ts
it("maps signup conflict responses to a stable error code", async () => {
  const fetchImpl = vi.fn(async () =>
    new Response(JSON.stringify({ error: "email_already_exists" }), {
      status: 409,
      headers: { "content-type": "application/json" },
    }),
  );
  const client = createApiClient({ fetchImpl });

  const response = await registerUser(client, {
    email: "user@example.com",
    password: "abc12345",
  });

  expect(response.ok).toBe(false);
  if (response.ok) {
    throw new Error("expected error response");
  }
  expect(response.error.code).toBe("email_already_exists");
  expect(response.error.message).toContain("email already exists");
});
```

- [ ] **Step 2: Run the web unit tests and confirm the helper does not exist yet**

Run:

```bash
cd /Users/zhengwenfeng/work/projs/AgentForge/web
npm test -- --run web/tests/api-client.test.ts
```

Expected: FAIL with undefined `registerUser`.

- [ ] **Step 3: Add the API helper and register page action wrapper**

Update `web/lib/api/client.ts`:

```ts
export async function registerUser(
  client: ApiClient,
  input: { email: string; password: string },
) {
  return client.post<UserResponse, { email: string; password: string }>(
    "/api/users",
    input,
  );
}
```

Create `web/app/register/actions.ts`:

```ts
import type { ApiClient } from "@/lib/api";
import type { UserResponse } from "@/lib/api";

export async function registerWithPassword(
  apiClient: ApiClient,
  email: string,
  password: string,
) {
  return apiClient.post<UserResponse, { email: string; password: string }>(
    "/api/users",
    {
      email,
      password,
    },
  );
}
```

- [ ] **Step 4: Build the register page and login-page links**

Create `web/app/register/page.tsx` using the same page structure as the login page:

```tsx
"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, useSyncExternalStore, type FormEvent } from "react";

import { registerWithPassword } from "@/app/register/actions";
import { useApiClient } from "@/components/app-shell";
import { apiErrorMessage } from "@/lib/api";

export default function RegisterPage() {
  const apiClient = useApiClient();
  const router = useRouter();
  const hydrated = useSyncExternalStore(
    () => () => undefined,
    () => true,
    () => false,
  );
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError("");

    const response = await registerWithPassword(apiClient, email.trim(), password);
    if (!response.ok) {
      setError(apiErrorMessage(response.error.code, response.error.message));
      setPending(false);
      return;
    }

    router.push("/login?registered=1");
    router.refresh();
  }

  return (
    <section className="mx-auto max-w-2xl">
      <div className="panel rounded-[2rem] p-8 sm:p-10">
        <p className="eyebrow">Access</p>
        <h1 className="mt-5 text-4xl font-semibold tracking-tight text-stone-950">
          Create your account.
        </h1>
        <form className="mt-8 grid gap-5" onSubmit={(event) => void handleSubmit(event)}>
          {/* email + password inputs matching login page classes */}
          {error ? <div className="rounded-[1.25rem] border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div> : null}
          <button disabled={!hydrated || pending} type="submit">
            {pending ? "Creating Account..." : "Create Account"}
          </button>
        </form>
        <p className="mt-6 text-sm text-stone-600">
          Already have an account? <Link href="/login">Sign in</Link>
        </p>
      </div>
    </section>
  );
}
```

Update `web/app/login/page.tsx` to add:

```tsx
import Link from "next/link";
import { useSearchParams } from "next/navigation";

const searchParams = useSearchParams();
const registered = searchParams.get("registered") === "1";
```

Render the success notice and the register link:

```tsx
{registered ? (
  <div className="rounded-[1.25rem] border border-emerald-300 bg-emerald-50 px-4 py-3 text-sm text-emerald-800">
    Registration successful. Please sign in.
  </div>
) : null}

<p className="mt-6 text-sm text-stone-600">
  Need an account? <Link href="/register">Create one</Link>
</p>
```

- [ ] **Step 5: Run web unit tests**

Run:

```bash
cd /Users/zhengwenfeng/work/projs/AgentForge/web
npm test -- --run web/tests/api-client.test.ts
```

Expected: PASS, including the new signup error-mapping test.

- [ ] **Step 6: Commit the frontend registration changes**

Run:

```bash
cd /Users/zhengwenfeng/work/projs/AgentForge
git add web/lib/api/client.ts web/lib/api/types.ts web/app/register/actions.ts web/app/register/page.tsx web/app/login/page.tsx web/tests/api-client.test.ts
git commit -m "feat: add registration page flow"
```

### Task 4: Verify the browser flow end-to-end

**Files:**
- Create: `web/tests/register-flow.spec.ts`

- [ ] **Step 1: Write the Playwright signup flow test**

Create `web/tests/register-flow.spec.ts`:

```ts
import { expect, test } from "@playwright/test";

test("visitor can register and is redirected to login without being signed in", async ({ page }) => {
  let registered = false;

  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;

    if (pathname === "/api/session" && request.method() === "GET") {
      await route.fulfill({
        status: 401,
        contentType: "application/json",
        body: JSON.stringify({ error: "unauthorized" }),
      });
      return;
    }

    if (pathname === "/api/users" && request.method() === "POST") {
      registered = true;
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          user: { id: "new-user", email: "new@example.com", role: "user" },
        }),
      });
      return;
    }

    if (pathname === "/api/sessions" && request.method() === "POST") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: { id: "new-user", email: "new@example.com", role: "user" },
        }),
      });
      return;
    }

    await route.fulfill({ status: 404, body: `Unhandled ${request.method()} ${pathname}` });
  });

  await page.goto("/register");
  await page.getByLabel("Email").fill("new@example.com");
  await page.getByLabel("Password").fill("abc12345");
  await page.getByRole("button", { name: "Create Account" }).click();

  await expect(page).toHaveURL(/\/login\?registered=1$/);
  await expect(page.getByText("Registration successful. Please sign in.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Sign In" })).toBeVisible();
  expect(registered).toBe(true);
});
```

- [ ] **Step 2: Run the new Playwright spec**

Run:

```bash
cd /Users/zhengwenfeng/work/projs/AgentForge/web
npm run test:e2e -- register-flow.spec.ts
```

Expected: PASS and the redirect target remains `/login?registered=1`, confirming registration did not create a session.

- [ ] **Step 3: Run the focused backend and frontend verification set**

Run:

```bash
cd /Users/zhengwenfeng/work/projs/AgentForge/services/api
go test ./internal/auth ./internal/http

cd /Users/zhengwenfeng/work/projs/AgentForge/web
npm test -- --run web/tests/api-client.test.ts
npm run test:e2e -- register-flow.spec.ts
```

Expected: all commands PASS.

- [ ] **Step 4: Commit the verification test**

Run:

```bash
cd /Users/zhengwenfeng/work/projs/AgentForge
git add web/tests/register-flow.spec.ts
git commit -m "test: cover registration flow"
```

### Task 5: Final review and branch hygiene

**Files:**
- Review: `services/api/internal/auth/repository.go`
- Review: `services/api/internal/http/registration_handlers.go`
- Review: `web/app/register/page.tsx`
- Review: `web/app/login/page.tsx`

- [ ] **Step 1: Confirm spec coverage manually**

Check these requirements against the code before merging:

```text
- public POST /api/users exists
- email is normalized to lowercase
- password requires >= 8 chars with at least one letter and one digit
- duplicate email returns 409/email_already_exists
- registration does not set a session cookie
- successful registration redirects to /login?registered=1
- default admin email is admin@123.com
```

- [ ] **Step 2: Inspect the final diff**

Run:

```bash
cd /Users/zhengwenfeng/work/projs/AgentForge
git diff --stat -- services/api/internal/auth/repository.go services/api/internal/http/router.go services/api/internal/http/registration_handlers.go web/app/register/page.tsx web/app/login/page.tsx web/app/register/actions.ts web/lib/api/client.ts web/tests/register-flow.spec.ts
git diff -- services/api/internal/auth/repository.go services/api/internal/http/router.go services/api/internal/http/registration_handlers.go web/app/register/page.tsx web/app/login/page.tsx
```

Expected: only registration-related files changed; no unrelated auth/session behavior drift.

- [ ] **Step 3: Write a concise implementation summary for handoff**

Use this template in the final status update:

```text
Implemented public registration across repository, HTTP, and Next.js UI.
Verified backend auth/http tests, frontend API client tests, and the Playwright register-flow spec.
Default admin bootstrap now uses admin@123.com with the existing admin password.
```

## Self-Review

- Spec coverage check: repository validation/default admin, HTTP status mapping, frontend register page, login redirect/success message, and verification are each mapped to a dedicated task.
- Placeholder scan: no `TODO`, `TBD`, or “similar to previous task” references remain.
- Type consistency check: plan uses `CreateUser`, `CreateUserParams`, `registerUser`, and `registerWithPassword` consistently across repository, router, and frontend tasks.
