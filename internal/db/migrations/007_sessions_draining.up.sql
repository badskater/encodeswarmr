-- Migration 007: Add sessions table and draining agent status

-- ---------------------------------------------------------------------------
-- sessions
-- Sessions are created on successful login (local or OIDC) and deleted on
-- logout or expiry. The token is a random hex string hashed on the client.
-- ---------------------------------------------------------------------------
CREATE TABLE sessions (
    token      TEXT        PRIMARY KEY,
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_sessions_user_id    ON sessions (user_id);
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);

-- ---------------------------------------------------------------------------
-- Add 'draining' to agents status values.
-- The existing CHECK uses an IN list; PostgreSQL requires dropping and
-- re-adding the constraint to change it.
-- ---------------------------------------------------------------------------
ALTER TABLE agents
    DROP CONSTRAINT IF EXISTS agents_status_check;

ALTER TABLE agents
    ADD CONSTRAINT agents_status_check
    CHECK (status IN (
        'pending_approval', 'idle', 'busy',
        'offline', 'disabled', 'draining'
    ));
