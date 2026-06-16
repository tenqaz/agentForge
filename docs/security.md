# Security Checklist

## API and Frontend

- Secrets must not be returned to the frontend.
- Provider API keys must not appear in JSON responses.
- `WEIXIN_TOKEN` and `bot_token` must not appear in JSON responses.
- Password hashes must not appear in JSON responses.

## Runtime Isolation

- Each Agent has its own host-side `hermes-home`.
- Each Hermes container mounts only that Agent's `hermes-home`.
- Runtime container names are generated from internal Agent IDs, not user input.

## Weixin Policy

- Private-message policy is `WEIXIN_DM_POLICY=allowlist`.
- Group policy is `WEIXIN_GROUP_POLICY=disabled`.
- The confirmed扫码 user ID is written into `WEIXIN_ALLOWED_USERS`.

## Persistence Expectations

- Deleting and recreating the runtime container must not delete:
  - `config.yaml`
  - `.env`
  - `sessions/`
  - `weixin/accounts/{account_id}.json`

## Admin Template Surface

- Admins can add complete skills by uploading a ZIP archive and delete complete skills.
- MVP does not expose a skill edit endpoint.
- MVP does not expose a skill replace endpoint.
