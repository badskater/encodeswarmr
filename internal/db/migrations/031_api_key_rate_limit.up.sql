-- 031_api_key_rate_limit.up.sql
-- Adds per-API-key rate limit (requests per second).
-- Default 200 matches the global per-IP limiter.

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS rate_limit INT NOT NULL DEFAULT 200;
