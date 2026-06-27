# AgentForge MVP API

## Session

### `POST /api/sessions`

Create a cookie session with email and password.

Request:

```json
{
  "email": "user@example.com",
  "password": "secret-password",
  "turnstileToken": "<token-from-turnstile-widget>"
}
```

- `turnstileToken`: Turnstile widget response token (string, required when Turnstile is enabled)

Response `200`:

```json
{
  "user": {
    "id": "user-1",
    "email": "user@example.com",
    "role": "user"
  }
}
```

### `GET /api/session`

Return the current authenticated user from the session cookie.

### `DELETE /api/session`

Clear the session cookie.

## Registration

### `POST /api/registration/email-codes`

Send a registration verification code to an email address.

Request:

```json
{
  "email": "user@example.com",
  "turnstileToken": "<token-from-turnstile-widget>"
}
```

- `turnstileToken`: Turnstile widget response token (string, required when Turnstile is enabled)

Response `202 Accepted`:

```json
{ "ok": true }
```

Error codes:
- `email_already_exists` (409): email already registered
- `email_code_cooldown` (429): too many requests, cooldown period active
- `email_code_rate_limited` (429): rate limited
- `email_send_failed` (500): failed to send email

### `POST /api/users`

Create a new user with a valid email verification code.

Request:

```json
{
  "email": "user@example.com",
  "password": "secret-password",
  "emailCode": "123456",
  "turnstileToken": "<token-from-turnstile-widget>"
}
```

- `turnstileToken`: Turnstile widget response token (string, required when Turnstile is enabled)

Response `201 Created`:

```json
{
  "user": {
    "id": "user-1",
    "email": "user@example.com",
    "role": "user"
  }
}
```

Error codes:
- `email_code_required` (400): email code missing
- `email_code_expired` (400): email code expired
- `email_code_attempts_exhausted` (400): too many failed attempts
- `email_code_invalid` (400): invalid email code
- `email_already_exists` (409): email already registered
- `invalid_password` (400): invalid password

## Turnstile

### `GET /api/turnstile/config`

Public endpoint (no authentication required). Returns configuration for rendering the Turnstile widget in the frontend.

Response `200`:

```json
{ "sitekey": "<sitekey-or-empty>", "enabled": true }
```

- `enabled`: Whether the backend has Turnstile secret configured
- `sitekey`: Sitekey for the frontend (empty string if not configured)

This endpoint itself does not require Turnstile verification.

## Health

### `GET /api/health`

Return basic API health.

## Public Templates

### `GET /api/templates`

List published templates only.

### `GET /api/templates/{id}`

Get one published template.

Draft and archived templates are not visible on public routes.

## Admin Templates

All `/api/admin/templates*` routes require an admin session.

### `GET /api/admin/templates`

List active templates for admin management.

Archived templates are hidden from this default list.

### `GET /api/admin/templates/{id}`

Get one template for admin management.

### `POST /api/admin/templates`

Create a draft template.

Request:

```json
{
  "name": "Support Concierge",
  "description": "Handles private support requests."
}
```

### `PUT /api/admin/templates/{id}`

Update template metadata.

### `DELETE /api/admin/templates/{id}`

Archive a template.

### `GET /api/admin/templates/{id}/soul`

Read `SOUL.md`.

### `PUT /api/admin/templates/{id}/soul`

Write `SOUL.md`.

Request:

```json
{
  "content": "# Soul\nCalm and direct."
}
```

### `GET /api/admin/templates/{id}/user`

Read `USER.md`.

### `PUT /api/admin/templates/{id}/user`

Write `USER.md`.

### `GET /api/admin/templates/{id}/skills`

List skills for the template.

### `POST /api/admin/templates/{id}/skills`

Add one complete skill.

Request: `multipart/form-data` with a `file` field containing a ZIP archive.

The ZIP must contain exactly one top-level directory. That directory name becomes the skill name, and it must include `SKILL.md`.

### `GET /api/admin/templates/{id}/skills/{skillId}`

Read one skill and its `SKILL.md` content.

### `DELETE /api/admin/templates/{id}/skills/{skillId}`

Delete one complete skill.

There is intentionally no skill edit or skill replace endpoint in MVP.

### `PUT /api/admin/templates/{id}/publication`

Publish a template.

### `DELETE /api/admin/templates/{id}/publication`

Return a published template to a new draft and archive the published version.

## Agents

### `POST /api/agents`

Create an Agent from a published template and queue runtime provisioning.

Request:

```json
{
  "templateId": "template-1",
  "name": "Support Concierge Agent"
}
```

### `GET /api/agents`

List Agents.

- admins see all Agents
- normal users see only their own Agents

### `GET /api/agents/{id}`

Get one Agent.

### `GET /api/agents/{id}/runtime`

Get runtime state.

### `GET /api/agents/{id}/runtime-jobs`

List runtime jobs for one Agent.

### `GET /api/agents/{id}/runtime-jobs/{jobId}`

Get one runtime job.

### `POST /api/agents/{id}/runtime-jobs`

Create a runtime job. MVP only supports restart.

Request:

```json
{
  "type": "restart_runtime"
}
```

### `DELETE /api/agents/{id}`

物理删除 agent：停止并移除其 Docker 容器、删除 hermes-home 目录、删除数据库记录（关联的 runtime_jobs/agent_channels/agent_runtime_events 通过外键 CASCADE 自动清理）。

权限：owner 或 admin。

响应：

| 状态码 | error code | 说明 |
|------|------|------|
| 204 No Content | — | 删除成功 |
| 401 | unauthorized | 未登录 |
| 403 | forbidden | 不是 owner 也不是 admin |
| 404 | agent_not_found / not_found | agent 不存在 |
| 409 | agent_cannot_delete | 当前状态（provisioning/starting）不允许删除，请等状态稳定后重试 |
| 409 | agent_has_unfinished_jobs | 存在未完成的运行时任务，请稍后重试 |
| 500 | internal_error | 删除过程内部错误（agent 转 error 状态，可重试同一接口） |

注意：500 错误后 agent 进入 `error` 状态并写入 `last_error_code`：`delete_inspect_failed` / `delete_stop_failed` / `delete_remove_failed` / `delete_home_failed`。再次调用 DELETE 会从中断处接续完成清理（每一步都幂等）。

## Weixin Channel

### `GET /api/agents/{id}/channels/weixin`

Get channel state. If the channel has not been created yet, the response still returns a synthetic `not_configured` state.

### `PUT /api/agents/{id}/channels/weixin`

Ensure the Weixin channel exists. The Agent must already be `running`.

### `DELETE /api/agents/{id}/channels/weixin`

No-op delete for MVP.

### `GET /api/agents/{id}/channels/weixin/pairing-sessions`

List pairing sessions for the Agent's Weixin channel.

### `POST /api/agents/{id}/channels/weixin/pairing-sessions`

Create or reuse an active pairing session and queue channel work.

### `GET /api/agents/{id}/channels/weixin/pairing-sessions/{sessionId}`

Get one pairing session.

## Error Codes

Typical response codes:

- `401 unauthorized`
- `403 forbidden`
- `404 not_found`
- `409 conflict`
- `400 invalid_request`

Important 400 error codes:

- `turnstile_required`: Turnstile token is required but not provided
- `turnstile_invalid`: Turnstile token is invalid or expired

Important conflict cases:

- Agent runtime not available for restart
- Agent not running when ensuring Weixin channel
- duplicate skill name inside the same template

## Secrets Policy

API responses must not expose:

- provider API keys
- `WEIXIN_TOKEN`
- `bot_token`
- session password hashes
