-- Migration 035: add rate_limit column to api_keys table
-- rate_limit is the maximum requests per minute allowed for this key.
-- 0 means "use the global default" (no per-key override).

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS rate_limit INT NOT NULL DEFAULT 0;
