-- 030_update_channel.up.sql
-- Adds release channel support to agents and binary uploads.

ALTER TABLE agents
    ADD COLUMN IF NOT EXISTS update_channel TEXT NOT NULL DEFAULT 'stable';

CREATE TABLE IF NOT EXISTS upgrade_binaries (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    channel      TEXT        NOT NULL DEFAULT 'stable',
    version      TEXT        NOT NULL,
    os           TEXT        NOT NULL,
    arch         TEXT        NOT NULL,
    filename     TEXT        NOT NULL,
    sha256       TEXT        NOT NULL DEFAULT '',
    uploaded_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (channel, os, arch)
);

CREATE INDEX IF NOT EXISTS idx_upgrade_binaries_channel ON upgrade_binaries (channel);
