CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE agent_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    version INTEGER NOT NULL DEFAULT 1,
    template_path TEXT NOT NULL,
    content_checksum TEXT NOT NULL,
    soul_content TEXT NOT NULL DEFAULT '',
    user_content TEXT NOT NULL DEFAULT '',
    skills_path TEXT NOT NULL,
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    published_at TEXT,
    FOREIGN KEY (created_by) REFERENCES users(id)
);

CREATE TABLE template_skills (
    id TEXT PRIMARY KEY,
    template_id TEXT NOT NULL,
    skill_name TEXT NOT NULL,
    skill_path TEXT NOT NULL,
    checksum TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (template_id) REFERENCES agent_templates(id) ON DELETE CASCADE,
    UNIQUE (template_id, skill_name)
);

CREATE TABLE agents (
    id TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL,
    template_id TEXT NOT NULL,
    template_version INTEGER NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'creating' CHECK (status IN ('creating', 'provisioning', 'starting', 'running', 'stopped', 'error')),
    runtime_id TEXT NOT NULL DEFAULT '',
    hermes_home_path TEXT NOT NULL UNIQUE,
    last_error_code TEXT NOT NULL DEFAULT '',
    last_error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (owner_user_id) REFERENCES users(id),
    FOREIGN KEY (template_id) REFERENCES agent_templates(id)
);

CREATE TABLE agent_runtime_events (
    id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    status_before TEXT NOT NULL DEFAULT '',
    status_after TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

CREATE TABLE agent_channels (
    id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL,
    channel_type TEXT NOT NULL DEFAULT 'weixin' CHECK (channel_type IN ('weixin')),
    status TEXT NOT NULL DEFAULT 'not_configured' CHECK (status IN ('not_configured', 'qr_pending', 'connected', 'error', 'disconnected')),
    external_account_id TEXT NOT NULL DEFAULT '',
    last_error_code TEXT NOT NULL DEFAULT '',
    last_error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

CREATE TABLE channel_pairing_sessions (
    id TEXT PRIMARY KEY,
    agent_channel_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'connected', 'expired', 'failed')),
    qr_payload TEXT NOT NULL DEFAULT '',
    qr_image_path TEXT NOT NULL DEFAULT '',
    expires_at TEXT NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_error_code TEXT NOT NULL DEFAULT '',
    last_error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (agent_channel_id) REFERENCES agent_channels(id) ON DELETE CASCADE
);

CREATE TABLE runtime_jobs (
    id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('provision_agent', 'start_runtime', 'stop_runtime', 'restart_runtime')),
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    priority INTEGER NOT NULL DEFAULT 0,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    locked_by TEXT NOT NULL DEFAULT '',
    locked_until TEXT,
    idempotency_key TEXT NOT NULL DEFAULT '',
    payload_json TEXT NOT NULL DEFAULT '{}',
    result_json TEXT NOT NULL DEFAULT '{}',
    last_error_code TEXT NOT NULL DEFAULT '',
    last_error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    started_at TEXT,
    finished_at TEXT,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

CREATE TABLE channel_jobs (
    id TEXT PRIMARY KEY,
    agent_channel_id TEXT NOT NULL,
    pairing_session_id TEXT,
    type TEXT NOT NULL CHECK (type IN ('connect_weixin', 'disconnect_weixin', 'refresh_weixin_pairing')),
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    priority INTEGER NOT NULL DEFAULT 0,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    locked_by TEXT NOT NULL DEFAULT '',
    locked_until TEXT,
    idempotency_key TEXT NOT NULL DEFAULT '',
    payload_json TEXT NOT NULL DEFAULT '{}',
    result_json TEXT NOT NULL DEFAULT '{}',
    last_error_code TEXT NOT NULL DEFAULT '',
    last_error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    started_at TEXT,
    finished_at TEXT,
    FOREIGN KEY (agent_channel_id) REFERENCES agent_channels(id) ON DELETE CASCADE,
    FOREIGN KEY (pairing_session_id) REFERENCES channel_pairing_sessions(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX idx_runtime_jobs_one_active
ON runtime_jobs(agent_id)
WHERE status IN ('queued', 'running');

CREATE UNIQUE INDEX idx_channel_jobs_one_active
ON channel_jobs(agent_channel_id)
WHERE status IN ('queued', 'running');

CREATE UNIQUE INDEX idx_pairing_one_active
ON channel_pairing_sessions(agent_channel_id)
WHERE status = 'pending';
