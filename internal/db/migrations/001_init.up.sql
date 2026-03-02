-- Migration 001: Core tables — users, agents, sources
-- Uses UUIDs (gen_random_uuid()) as primary keys throughout.
-- Requires pgcrypto extension (available in all standard Postgres installs).

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ---------------------------------------------------------------------------
-- users
-- ---------------------------------------------------------------------------
CREATE TABLE users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT        NOT NULL UNIQUE,
    email         TEXT        NOT NULL UNIQUE,
    role          TEXT        NOT NULL DEFAULT 'viewer'
                              CHECK (role IN ('admin', 'operator', 'viewer')),
    password_hash TEXT,                          -- NULL for OIDC-only accounts
    oidc_sub      TEXT        UNIQUE,            -- NULL for local accounts
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_oidc_sub ON users (oidc_sub) WHERE oidc_sub IS NOT NULL;

-- ---------------------------------------------------------------------------
-- agents
-- ---------------------------------------------------------------------------
CREATE TABLE agents (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT        NOT NULL UNIQUE,
    hostname        TEXT        NOT NULL,
    ip_address      TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'pending_approval'
                                CHECK (status IN (
                                    'pending_approval', 'idle', 'busy',
                                    'offline', 'disabled'
                                )),
    tags            TEXT[]      NOT NULL DEFAULT '{}',
    gpu_vendor      TEXT        NOT NULL DEFAULT '',
    gpu_model       TEXT        NOT NULL DEFAULT '',
    gpu_enabled     BOOLEAN     NOT NULL DEFAULT FALSE,
    agent_version   TEXT        NOT NULL DEFAULT '',
    os_version      TEXT        NOT NULL DEFAULT '',
    cpu_count       INT         NOT NULL DEFAULT 0,
    ram_mib         BIGINT      NOT NULL DEFAULT 0,
    -- Capabilities flags
    nvenc           BOOLEAN     NOT NULL DEFAULT FALSE,
    qsv             BOOLEAN     NOT NULL DEFAULT FALSE,
    amf             BOOLEAN     NOT NULL DEFAULT FALSE,
    -- API key used by this agent (hashed with bcrypt on the server)
    api_key_hash    TEXT,
    last_heartbeat  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_agents_status ON agents (status);
CREATE INDEX idx_agents_tags   ON agents USING gin (tags);

-- ---------------------------------------------------------------------------
-- sources
-- ---------------------------------------------------------------------------
CREATE TABLE sources (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    filename     TEXT        NOT NULL,
    unc_path     TEXT        NOT NULL UNIQUE,
    size_bytes   BIGINT      NOT NULL DEFAULT 0,
    detected_by  UUID        REFERENCES agents (id) ON DELETE SET NULL,
    state        TEXT        NOT NULL DEFAULT 'detected'
                             CHECK (state IN (
                                 'detected', 'scanning', 'ready',
                                 'encoding', 'done'
                             )),
    vmaf_score   FLOAT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sources_state ON sources (state);
