# AgentForge MVP API

## Session

### `POST /api/sessions`

Create a cookie session with email and password.

Request:

```json
{
  "email": "user@example.com",
  "password": "secret-password"
}
```

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

List templates for admin management.

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

Request:

```json
{
  "skillName": "triage",
  "skillMD": "# SKILL\nEscalate billing issues."
}
```

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
