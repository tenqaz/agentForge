# Gin Migration Design

Date: 2026-06-17
Topic: Migrate `services/api` HTTP layer from `net/http` to Gin

## Goal

Migrate the backend HTTP stack in `services/api` from the current `net/http` + `ServeMux` implementation to Gin.

Primary goals:

- Adopt Gin's `Context`, router grouping, and middleware model across the entire HTTP layer.
- Use the migration to clean up handler structure, request binding, and error handling.
- Preserve application behavior and user-facing functionality, while allowing coordinated API response adjustments when they improve consistency.

Non-goals:

- Rewriting service, repository, database, runtime, or worker layers to depend on Gin.
- Changing product workflows or adding new end-user features.
- Broad refactoring outside the HTTP boundary that does not directly support this migration.

## Current State

The backend entrypoint in `services/api/cmd/agentforge-api/main.go` builds dependencies, creates an HTTP router through `internal/http.NewRouter`, and serves it through `http.Server`.

The current HTTP layer:

- Uses `http.NewServeMux()` and method-aware patterns such as `GET /api/health`.
- Registers routes from multiple handler groups: health, registration, session, templates, agents, and weixin channels.
- Uses plain `http.Handler` middleware for session resolution, panic recovery, and request ID injection.
- Implements JSON decoding manually in several handlers.
- Uses a shared JSON/error response helper pattern with error payloads shaped as `{ error, message?, requestId? }`.

The frontend depends on:

- Stable API paths under `/api/...`.
- Existing session, template, agent, runtime job, and weixin flows.
- Error-code-based handling in `web/lib/api/client.ts`.
- Playwright and API client tests that encode current path, method, and response expectations.

## Design Principles

1. Keep Gin confined to the HTTP adapter layer.
2. Preserve service-layer signatures that accept `context.Context`.
3. Prefer route and middleware consistency over minimal code churn.
4. Avoid gratuitous API redesign; only change HTTP behavior when it improves consistency and is practical to update in the frontend.
5. Keep route paths stable unless a change materially improves structure.

## Target Architecture

### HTTP Entry

`internal/http.NewRouter` will return a Gin engine instead of a generic `http.Handler`. The server bootstrap in `main.go` will continue to use `http.Server`, with the Gin engine set as the server handler.

This preserves:

- Existing startup flow
- Existing graceful shutdown behavior
- Existing dependency construction

### Layering Boundary

Gin will be used only in `services/api/internal/http`.

Business and persistence layers will remain unchanged:

- `internal/auth`
- `internal/agents`
- `internal/templates`
- `internal/channels`
- `internal/jobs`
- `internal/runtime`
- `internal/weixin`

Handlers will continue to call these services with `c.Request.Context()` or a derived `context.Context` from Gin's request context.

## Route Organization

### Engine Setup

The Gin engine will be assembled centrally in `router.go`:

- Create the engine with explicit middleware configuration.
- Install global middleware for request ID, panic recovery, logging-compatible metadata, and session hydration.
- Mount `/api` route groups by domain.

### Route Groups

Routes will be grouped by resource domain instead of individual `ServeMux` registration calls:

- `/api/health`
- `/api/users`
- `/api/sessions` and `/api/session`
- `/api/templates`
- `/api/admin/templates`
- `/api/agents`
- nested agent routes for runtime jobs and weixin channels

Each handler set will register against a `*gin.RouterGroup` or the engine directly, for example:

- `RegisterPublicRoutes`
- `RegisterAdminTemplateRoutes`
- `RegisterAgentRoutes`
- `RegisterWeixinRoutes`

Exact function naming can follow repository conventions, but the design requirement is to stop passing `*http.ServeMux` around and move all HTTP registration to Gin-native route groups.

## Handler Model

### Signatures

All HTTP handlers in `internal/http` will move from:

- `func (h *XHandlers) Method(w http.ResponseWriter, r *http.Request)`

to:

- `func (h *XHandlers) Method(c *gin.Context)`

### Parameter Access

All path parameter access will be moved from `r.PathValue(...)` to `c.Param(...)`.

### Request Binding

Manual JSON decoding will be replaced by shared Gin-aware request binding helpers.

Requirements for the binding layer:

- Consistent JSON error responses
- Explicit handling for malformed JSON
- Explicit handling for unexpected extra JSON content or unsupported body shapes when relevant
- Reusable helper for request DTO decoding across handlers

The helper may use Gin binding primitives internally, but it must preserve precise API behavior where existing tests rely on it. If the migration intentionally changes one of these semantics, the spec requires synchronizing backend tests, frontend client logic, and browser tests.

### Response Writing

Handlers will stop writing JSON directly through `http.ResponseWriter` and instead use shared Gin response helpers.

The response layer must standardize:

- JSON success responses
- no-content responses
- error responses
- request ID propagation into error payloads

## Middleware Design

### Request ID

The current `X-Request-ID` behavior will be preserved conceptually, but implemented as Gin middleware.

Requirements:

- Ensure each request has a request ID.
- Expose that request ID in the response header.
- Make it available to downstream handlers and error helpers.
- Keep `requestId` available in JSON error bodies.

### Panic Recovery

The current panic recovery middleware will be replaced by Gin middleware that:

- Recovers panics
- Logs panic context
- Returns a structured internal error response
- Includes request ID in the error payload when available

Gin's built-in recovery can be used as a base, but a custom wrapper is acceptable if needed to preserve current response semantics.

### Session Hydration and Auth Context

The current session middleware resolves the current authenticated user from the session cookie and repository.

After migration:

- Session parsing and authenticated user lookup will become Gin middleware and/or Gin helper functions.
- The resolved user will be stored in `gin.Context`.
- Authorization helpers will read from `gin.Context` instead of custom request context helpers.

This should support:

- authenticated-user requirements
- admin-only requirements
- resource ownership checks for agents and related sub-resources

## Authorization and Shared HTTP Helpers

The migration is a good point to consolidate duplicated HTTP-layer logic.

The design includes a shared helper layer for:

- getting the current authenticated user
- enforcing admin access
- authorizing agent access
- mapping domain/service errors to HTTP responses
- writing standardized JSON payloads

The goal is to reduce handler-specific branching and make route behavior easier to audit.

## Error Handling Strategy

### Stable Shape

The default error response shape remains:

```json
{
  "error": "code",
  "message": "optional human-readable message",
  "requestId": "optional request id"
}
```

This is stable because the frontend client and tests already depend on error-code-based behavior.

### Consolidation

The current pattern of `writeError`, `writeInternalError`, `writeAgentError`, `writeWeixinError`, and related helpers will be retained conceptually but reorganized around Gin-aware helpers.

Target outcomes:

- one consistent path for writing errors
- clear mapping from domain errors to HTTP status and code
- consistent logging metadata
- reduced duplication across handlers

### Allowed API Adjustments

HTTP behavior may be adjusted during migration when it improves consistency, but only if all impacted consumers are updated together.

Allowed examples:

- standardizing error messages
- standardizing bad-request handling
- aligning JSON response formatting across endpoints

Disallowed without explicit follow-up scope:

- changing core product workflows
- changing authentication model
- redesigning resource URLs for cosmetic reasons only

## Frontend Impact

The frontend is in scope for compatibility work required by this migration.

Affected areas may include:

- `web/lib/api/client.ts`
- API response typing in `web/lib/api/types.ts`
- server actions or components that assume specific response details
- Playwright tests under `web/tests`

Default compatibility rule:

- keep route paths unchanged unless there is a concrete reason to improve them
- update frontend parsing and tests for any intentional response or error-shape adjustments
- preserve all user-visible workflows

## Testing Strategy

### Backend

The migration must keep or restore full backend test coverage for the HTTP layer and existing integration flows.

Required backend verification:

- `go test ./...` in `services/api`
- HTTP handler tests in `services/api/internal/http`
- integration coverage in `services/api/tests`

Existing test helpers may need adaptation from generic `http.Handler` assumptions to Gin engine usage, but the test scenarios themselves should remain focused on behavior rather than framework details.

### Frontend

Required frontend verification should cover both API client behavior and user workflows affected by any API response changes.

Expected verification includes:

- unit tests around API client behavior
- relevant Playwright flows for registration, login, templates, agents, and weixin channel actions

Exact commands can be finalized in the implementation plan based on repository scripts.

## Migration Scope Breakdown

The implementation should cover at least these areas:

1. Add Gin dependency and wire it into the API service.
2. Replace `ServeMux` route assembly with Gin engine and route groups.
3. Convert all HTTP handlers to `gin.Context`.
4. Replace middleware implementations with Gin-native middleware.
5. Rework auth/session helper flow to use `gin.Context`.
6. Rework request binding and JSON response helpers.
7. Update backend tests to run against the Gin router.
8. Update frontend client code and UI tests for any intentional HTTP behavior changes.

## Risks

### Behavior Drift

Switching request binding and middleware stacks can accidentally change:

- JSON parse failure behavior
- header-writing timing
- cookie behavior
- path parameter extraction
- unauthorized vs forbidden responses

Mitigation:

- keep URL structure stable
- use existing tests as a behavioral safety net
- add targeted tests where Gin introduces ambiguity

### Over-coupling Business Logic to Gin

There is a risk of pulling Gin types into service or repository layers during migration.

Mitigation:

- enforce Gin usage only inside `internal/http`
- continue passing standard `context.Context` into domain services

### Frontend/Backend Contract Mismatch

Intentional API cleanup can break the frontend if changes are not synchronized.

Mitigation:

- treat frontend updates as part of the same migration
- update API client tests and flow tests in the same change set

## Success Criteria

The migration is complete when:

- all HTTP routing and middleware in `services/api` are Gin-native
- no `ServeMux`-based routing remains in the backend HTTP layer
- service and repository layers remain framework-agnostic
- backend tests pass
- affected frontend tests pass
- the application preserves existing user workflows

## Implementation Notes For Planning

The implementation plan should assume one cohesive migration rather than a prolonged mixed-framework state.

However, within that migration, work should still be sequenced to reduce risk:

1. establish Gin engine and shared middleware/helpers
2. convert route registration and handlers by domain
3. adapt backend tests
4. adapt frontend contract usage
5. run full verification

The plan should explicitly call out any endpoint semantics that are intentionally changed during migration so they can be reviewed before merge.
