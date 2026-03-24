-- Migration 033: encoding_profiles table
-- Stores reusable encoding profile presets that can be loaded when creating jobs.

CREATE TABLE IF NOT EXISTS encoding_profiles (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL UNIQUE,
    description TEXT        NOT NULL DEFAULT '',
    -- container extension, e.g. "mkv", "mp4"
    container   TEXT        NOT NULL DEFAULT 'mkv',
    -- JSON object mirroring EncodeConfig fields (template_id, extra_vars, etc.)
    settings    JSONB       NOT NULL DEFAULT '{}',
    -- Optional JSON AudioConfig for jobs that include audio re-encoding.
    audio_config JSONB      NULL,
    created_by  TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
