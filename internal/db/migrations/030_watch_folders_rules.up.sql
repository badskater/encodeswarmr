-- Add watch_folder and category columns to sources for watch folder tracking.
ALTER TABLE sources
    ADD COLUMN IF NOT EXISTS watch_folder TEXT,
    ADD COLUMN IF NOT EXISTS category     TEXT NOT NULL DEFAULT 'default';

-- Encoding rules engine: rules are evaluated when creating a job to suggest
-- (not auto-apply) the best template, audio codec, and priority.
CREATE TABLE IF NOT EXISTS encoding_rules (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    priority   INT         NOT NULL DEFAULT 100,
    conditions JSONB       NOT NULL DEFAULT '[]',
    actions    JSONB       NOT NULL DEFAULT '{}',
    enabled    BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
