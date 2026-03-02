-- Migration 005: webhooks and delivery log

-- ---------------------------------------------------------------------------
-- webhooks
-- ---------------------------------------------------------------------------
CREATE TABLE webhooks (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL UNIQUE,
    -- "discord", "teams", "slack"
    provider   TEXT        NOT NULL CHECK (provider IN ('discord', 'teams', 'slack')),
    url        TEXT        NOT NULL,
    -- HMAC-SHA256 secret (optional, stored as bcrypt hash)
    secret_hash TEXT,
    -- Array of event names this webhook subscribes to,
    -- e.g. '{job.completed,job.failed}'
    events     TEXT[]      NOT NULL DEFAULT '{}',
    enabled    BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_webhooks_events ON webhooks USING gin (events);

-- ---------------------------------------------------------------------------
-- webhook_deliveries — audit trail for every outbound POST attempt
-- ---------------------------------------------------------------------------
CREATE TABLE webhook_deliveries (
    id            BIGSERIAL   PRIMARY KEY,
    webhook_id    UUID        NOT NULL REFERENCES webhooks (id) ON DELETE CASCADE,
    event         TEXT        NOT NULL,
    payload       JSONB       NOT NULL,
    response_code INT,
    success       BOOLEAN     NOT NULL DEFAULT FALSE,
    attempt       INT         NOT NULL DEFAULT 1,
    error_msg     TEXT,
    delivered_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_webhook_deliveries_webhook_id ON webhook_deliveries (webhook_id, delivered_at DESC);
