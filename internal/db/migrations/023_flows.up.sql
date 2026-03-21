-- 023_flows.up.sql
-- Creates the flows table for the visual flow pipeline editor.

CREATE TABLE IF NOT EXISTS flows (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    graph       JSONB       NOT NULL DEFAULT '{"nodes":[],"edges":[]}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_flows_name ON flows (name);
