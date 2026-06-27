-- Add sleeping/waking to agents status CHECK constraint.
-- SQLite doesn't support ALTER TABLE ... ALTER CHECK, so we recreate the table.

CREATE TABLE agents_new (
    id TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL,
    template_id TEXT NOT NULL,
    template_version INTEGER NOT NULL DEFAULT 1,
    name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'creating' CHECK (status IN ('creating', 'provisioning', 'starting', 'running', 'stopped', 'error', 'sleeping', 'waking')),
    runtime_id TEXT NOT NULL DEFAULT '',
    hermes_home_path TEXT NOT NULL DEFAULT '',
    last_error_code TEXT NOT NULL DEFAULT '',
    last_error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (owner_user_id) REFERENCES users(id),
    FOREIGN KEY (template_id) REFERENCES agent_templates(id)
);

INSERT INTO agents_new SELECT * FROM agents;

DROP TABLE agents;
ALTER TABLE agents_new RENAME TO agents;

CREATE INDEX idx_agents_owner ON agents(owner_user_id);
CREATE INDEX idx_agents_template ON agents(template_id);
CREATE INDEX idx_agents_status ON agents(status);
CREATE UNIQUE INDEX idx_agents_runtime ON agents(runtime_id) WHERE runtime_id != '';
