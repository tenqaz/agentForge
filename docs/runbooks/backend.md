# Backend Runbook

## Runtime Prerequisites

- Docker must be installed and the API process must be allowed to run `docker`.
- The API server and worker can run inside the same Go process for MVP.
- The runtime container mount shape is:

```bash
-v {host_hermes_home}:/opt/data -e HERMES_HOME=/opt/data
```

## Environment

Required or commonly used backend variables:

```bash
AGENTFORGE_HTTP_ADDR=:8080
AGENTFORGE_SQLITE_PATH=./var/agentforge.db
AGENTFORGE_DATA_DIR=./var
AGENTFORGE_SESSION_SECRET=replace-me
AGENTFORGE_DOCKER_BIN=docker
AGENTFORGE_HERMES_IMAGE=nousresearch/hermes-agent:v2026.6.5
AGENTFORGE_HERMES_MEMORY=500m
AGENTFORGE_HERMES_CPUS=0.5
AGENTFORGE_WEIXIN_BASE_URL=https://example-weixin-gateway
AGENTFORGE_WEIXIN_API_KEY=replace-me
```

## SQLite Files

The SQLite deployment footprint is not just one file. Keep these together:

- `var/agentforge.db`
- `var/agentforge.db-wal`
- `var/agentforge.db-shm`

## Backup Scope

Back up the whole `var/` directory, not only the database file.

Reason:

- `var/agentforge.db*` stores API state.
- `var/templates/` stores template files.
- `var/agents/{agent_id}/hermes-home/` stores `config.yaml`, `.env`, `sessions/`, and `weixin/accounts/*.json`.

## Running Locally

Start the backend from `services/api`:

```bash
go run ./cmd/agentforge-api
```

The process will:

- open SQLite
- run migrations
- start HTTP
- run the runtime job worker loop
- run the channel job worker loop

## Inspect Runtime Jobs

Queued and running runtime jobs:

```bash
sqlite3 var/agentforge.db "
select id, agent_id, type, status, attempt_count, last_error_code, updated_at
from runtime_jobs
order by created_at desc;
"
```

Queued and running channel jobs:

```bash
sqlite3 var/agentforge.db "
select id, agent_channel_id, pairing_session_id, type, status, attempt_count, last_error_code, updated_at
from channel_jobs
order by created_at desc;
"
```

## Retry Via REST

Restart a runtime:

```bash
curl -X POST http://127.0.0.1:8080/api/agents/{agent_id}/runtime-jobs \
  -H 'Content-Type: application/json' \
  -H 'Cookie: agentforge_session=...' \
  -d '{"type":"restart_runtime"}'
```

Re-create or reuse a Weixin pairing session:

```bash
curl -X POST http://127.0.0.1:8080/api/agents/{agent_id}/channels/weixin/pairing-sessions \
  -H 'Content-Type: application/json' \
  -H 'Cookie: agentforge_session=...' \
  -d '{}'
```

## Operational Notes

- Runtime provisioning is asynchronous. `POST /api/agents` only creates the Agent row and a queued runtime job.
- Weixin pairing is asynchronous. Creating a pairing session only queues channel work.
- If a Hermes container is deleted, restarting the runtime should preserve host-side `hermes-home` contents.
