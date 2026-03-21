CREATE TABLE IF NOT EXISTS audit_log (
    id          BIGSERIAL PRIMARY KEY,
    user_id     UUID        REFERENCES users(id) ON DELETE SET NULL,
    username    TEXT        NOT NULL DEFAULT '',
    action      TEXT        NOT NULL,
    resource    TEXT        NOT NULL DEFAULT '',
    resource_id TEXT        NOT NULL DEFAULT '',
    detail      JSONB,
    ip_address  TEXT        NOT NULL DEFAULT '',
    logged_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_log_logged_at ON audit_log(logged_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log(action);
