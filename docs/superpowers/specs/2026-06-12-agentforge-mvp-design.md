# AgentForge MVP Design

Date: 2026-06-12

## Summary

AgentForge MVP provides a closed loop for users to create an Agent from an administrator-managed template, wait for the Hermes runtime to start, connect the Agent to Weixin by scanning a QR code, and chat with the Agent in Weixin.

The MVP intentionally excludes user-uploaded skills, user editing of `SOUL.md` or `USER.md`, team workspaces, template marketplace features, and additional channels such as QQ, Telegram, or WeCom. Those can be added after the first Hermes + Weixin runtime path is stable.

## Confirmed Scope

- The frontend uses Next.js.
- The backend uses Go.
- SQLite is the metadata database and must run with WAL enabled.
- Hermes is reused as the Agent runtime.
- Each user-created Agent gets its own Hermes container and independent `HERMES_HOME`.
- The first channel is Weixin personal account QR-code login.
- The platform is a lightweight multi-user product.
- Only administrators can create and publish Agent templates.
- Ordinary users can create Agents only from published templates.
- Ordinary users cannot edit template-provided `SOUL.md`, `USER.md`, or skills.
- Ordinary users cannot upload or select custom skills in the MVP.
- Channel configuration is available only after the Agent runtime has started successfully.

## Hermes Constraints

Hermes documentation describes:

- `SOUL.md` for personality configuration.
- `USER.md` and memory-related files for persistent user context.
- A skills system based on skill directories and `SKILL.md`.
- A messaging gateway that supports Weixin, WeCom, QQ Bot, Telegram, and other platforms.
- `hermes gateway setup` as the interactive path for configuring messaging platforms.
- Gateway runtime behavior where platform adapters receive messages, route them through chat sessions, and dispatch them to the Agent.

AgentForge should treat Hermes as the runtime boundary. The Go backend controls lifecycle, filesystem preparation, container operations, and status tracking, but the Hermes process owns actual Agent execution, gateway operation, and chat handling.

## Architecture

The system has three main boundaries.

### Next.js Console

The console provides:

- Login and session UI.
- Template list for ordinary users.
- Agent creation from a template.
- Agent list and detail pages.
- Runtime status display.
- Weixin channel setup page.
- QR-code display and connection status.
- Administrator pages for template management.

The console does not directly manipulate Hermes files or containers. All runtime actions go through the Go API.

### Go API and Worker

The Go backend provides:

- Authentication and authorization.
- Role-based access control for `admin` and `user`.
- SQLite persistence with WAL enabled.
- Template metadata and version management.
- Agent instance creation.
- Hermes home directory preparation.
- Hermes container lifecycle management.
- Weixin channel setup orchestration.
- Runtime and channel status synchronization.
- Event logging for provisioning and troubleshooting.

Long-running operations must be performed by worker tasks, not request handlers. API calls create records, validate permissions, enqueue work, and return current state.

### Hermes Runtime

Each Agent has one independent Hermes container. The container receives:

- A dedicated `HERMES_HOME`.
- Template-provided `SOUL.md`, `USER.md`, Hermes config fragments, and skills.
- Runtime configuration needed for model/provider access.
- Gateway configuration for Weixin after the user starts channel setup.

The platform should not share Hermes home directories across Agents or users.

## Data Model

### `users`

Stores account identity and role.

Important fields:

- `id`
- `email`
- `password_hash` or external auth subject
- `role`: `admin` or `user`
- `created_at`
- `updated_at`

### `agent_templates`

Stores administrator-managed template metadata.

Important fields:

- `id`
- `name`
- `description`
- `status`: `draft`, `published`, `archived`
- `version`
- `template_path`
- `content_checksum`
- `created_by`
- `created_at`
- `updated_at`
- `published_at`

Published templates should be immutable. Editing a published template creates a new version so existing Agents remain reproducible.

### `template_skills`

Stores the skills included in a template.

Important fields:

- `id`
- `template_id`
- `skill_name`
- `skill_path`
- `checksum`
- `created_at`

Only administrators can modify this table through template management flows.

### `agents`

Stores user-created Agent instances.

Important fields:

- `id`
- `owner_user_id`
- `template_id`
- `template_version`
- `name`
- `status`: `creating`, `provisioning`, `starting`, `running`, `stopped`, `error`
- `runtime_id`
- `hermes_home_path`
- `last_error_code`
- `last_error_message`
- `created_at`
- `updated_at`

### `agent_runtime_events`

Stores runtime lifecycle events.

Important fields:

- `id`
- `agent_id`
- `event_type`
- `status_before`
- `status_after`
- `message`
- `metadata_json`
- `created_at`

This table is used for support, debugging, and summarized user-facing logs.

### `agent_channels`

Stores channel configuration and status.

Important fields:

- `id`
- `agent_id`
- `channel_type`: `weixin` in the MVP
- `status`: `not_configured`, `qr_pending`, `connected`, `error`, `disconnected`
- `external_account_id`
- `last_error_code`
- `last_error_message`
- `created_at`
- `updated_at`

### `channel_pairing_sessions`

Stores QR-code setup sessions.

Important fields:

- `id`
- `agent_channel_id`
- `status`: `pending`, `connected`, `expired`, `failed`
- `qr_payload`
- `qr_image_path`
- `expires_at`
- `attempt_count`
- `last_error_code`
- `last_error_message`
- `created_at`
- `updated_at`

QR payloads and image paths may be returned to the frontend. Provider keys, gateway tokens, Weixin credentials, and Hermes secrets must never be returned to the frontend.

## State Machines

### Agent Runtime

Allowed progression:

```text
creating -> provisioning -> starting -> running
```

Failure progression:

```text
creating -> error
provisioning -> error
starting -> error
running -> stopped
running -> error
stopped -> starting
error -> provisioning
error -> starting
```

The retry target depends on the failed phase. The backend records the failed phase in `last_error_code` and `agent_runtime_events`.

### Weixin Channel

Allowed progression:

```text
not_configured -> qr_pending -> connected
qr_pending -> error
qr_pending -> not_configured
connected -> disconnected
disconnected -> qr_pending
error -> qr_pending
```

The API must reject channel setup unless the Agent is `running`.

## User Flows

### Administrator Template Flow

1. Admin logs in.
2. Admin creates a draft Agent template.
3. Admin uploads or edits the template package containing `SOUL.md`, `USER.md`, Hermes config fragments, and skills.
4. Backend validates required files and skill structure.
5. Admin publishes the template.
6. Published template appears in the ordinary user template list.

### Ordinary User Agent Flow

1. User logs in.
2. User views published templates.
3. User chooses a template and enters an Agent name.
4. API creates an `agents` record with status `creating`.
5. API enqueues `ProvisionAgent`.
6. Worker copies the template package into a new `HERMES_HOME`.
7. Worker creates the Hermes container.
8. Worker starts the container.
9. Agent status becomes `running`.
10. Console enables Weixin setup.

### Weixin Setup Flow

1. User opens an Agent detail page.
2. Console confirms Agent status is `running`.
3. User starts Weixin setup.
4. API creates or reuses a valid `channel_pairing_sessions` record.
5. Worker runs the Hermes Weixin gateway setup path inside the Agent runtime.
6. Backend stores QR payload or image reference.
7. Console displays the QR code.
8. User scans the QR code with Weixin.
9. Worker observes successful connection.
10. Channel status becomes `connected`.
11. User chats with the Agent in Weixin.

## API Surface

### Authentication

- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/me`

### Templates

- `GET /api/templates`
- `GET /api/templates/{id}`
- `POST /api/admin/templates`
- `PUT /api/admin/templates/{id}`
- `POST /api/admin/templates/{id}/publish`
- `POST /api/admin/templates/{id}/archive`

### Agents

- `POST /api/agents`
- `GET /api/agents`
- `GET /api/agents/{id}`
- `POST /api/agents/{id}/start`
- `POST /api/agents/{id}/stop`
- `POST /api/agents/{id}/retry`

### Weixin Channel

- `POST /api/agents/{id}/channels/weixin/setup`
- `GET /api/agents/{id}/channels/weixin`
- `POST /api/agents/{id}/channels/weixin/retry`

The channel endpoints return current state. They do not block until QR scan completion.

## Worker Tasks

### `ProvisionAgent`

Responsibilities:

- Acquire an Agent-level runtime lock.
- Move Agent status from `creating` to `provisioning`.
- Create the Agent `HERMES_HOME`.
- Copy template files and skills.
- Write Hermes configuration.
- Create the Hermes container.
- Start the Hermes container.
- Move Agent status to `running`.
- Record events at each major step.

The task must be idempotent. Re-running it should detect existing filesystem and container state and continue from the last safe point.

### `SetupWeixinChannel`

Responsibilities:

- Verify Agent is `running`.
- Acquire an Agent channel lock.
- Create or reuse an unexpired pairing session.
- Run the Hermes Weixin gateway setup path or equivalent noninteractive adapter.
- Store QR payload or image reference.
- Update channel status to `qr_pending`.
- Observe connection success, expiration, or failure.
- Update channel status to `connected`, `not_configured`, or `error`.

Repeated setup requests should return the current unexpired pairing session instead of creating competing QR sessions.

## Concurrency

SQLite must use WAL mode and a busy timeout.

The backend must prevent concurrent runtime operations for the same Agent. Examples:

- Start and stop cannot run at the same time.
- Provision and retry cannot run at the same time.
- Weixin setup and channel retry cannot run at the same time.

This can be implemented with a database-backed lock table or transactional task status checks. The implementation plan should choose the simplest reliable option for the first version.

## Security

Authorization rules:

- Admins can manage templates.
- Ordinary users can list only published templates.
- Ordinary users can create Agents only for themselves.
- Ordinary users can access only their own Agents, channels, and user-facing event summaries.
- Ordinary users cannot modify template files or skills.

Runtime isolation:

- Each Hermes container mounts only its own `HERMES_HOME`.
- Containers must not mount a shared user home directory.
- Container names, paths, and runtime IDs should derive from internal IDs, not user-controlled strings.

Secret handling:

- Provider API keys, Weixin credentials, gateway tokens, and Hermes secrets must be stored server-side only.
- Secrets must not be returned in API responses, logs, or frontend state.
- User-facing errors should expose short error summaries and a stable error code.

## Error Handling

Agent errors should record the failed phase:

- `copy_template_failed`
- `config_write_failed`
- `container_create_failed`
- `container_start_failed`
- `container_healthcheck_failed`

Weixin errors should record channel-specific causes:

- `agent_not_running`
- `qr_generation_failed`
- `qr_expired`
- `scan_rejected`
- `gateway_disconnected`
- `setup_timeout`

Agent creation failure does not create a connected channel. Weixin channel failure does not delete or roll back the Agent.

## Testing Strategy

### Backend Unit Tests

Cover:

- RBAC rules.
- Template publish rules.
- Agent state machine transitions.
- Weixin channel state machine transitions.
- SQLite repository behavior.
- Runtime lock behavior.

### Worker Integration Tests

Use a fake Hermes runner instead of a real container by default.

Cover:

- Successful `ProvisionAgent`.
- Failed template copy.
- Failed container start.
- Retry after provision failure.
- Successful `SetupWeixinChannel`.
- QR expiration.
- Duplicate setup request reuses active pairing session.

### Frontend E2E Tests

Cover:

- User logs in.
- User chooses a published template.
- User creates an Agent.
- Agent progresses to `running`.
- Weixin setup becomes available only after `running`.
- User sees QR code.
- Simulated pairing changes status to `connected`.

### Manual or Optional External Tests

Real Hermes + Weixin QR login should be an optional test gated by environment variables and manual account availability. It should not run in default CI.

## Open Follow-Ups After MVP

- User-uploaded skills.
- Template variables instead of direct `SOUL.md` or `USER.md` editing.
- QQ, Telegram, WeCom, and other channels.
- Template marketplace and ratings.
- Organization/team support.
- Agent sharing.
- Runtime metrics and cost tracking.
- Noninteractive Hermes gateway setup adapters if the interactive CLI path is insufficient for automation.

## Acceptance Criteria

- Admin can create and publish an Agent template.
- Ordinary user can see published templates.
- Ordinary user can create an Agent from a template.
- Backend creates a dedicated Hermes home for the Agent.
- Backend starts a dedicated Hermes container for the Agent.
- Console shows Agent lifecycle status.
- Weixin setup is disabled before Agent reaches `running`.
- User can start Weixin setup after Agent reaches `running`.
- Console displays a QR code for Weixin setup.
- Successful scan marks channel as `connected`.
- User can chat with the Agent from Weixin.
- Failed Agent provisioning and failed Weixin setup produce recoverable states and user-facing error messages.
