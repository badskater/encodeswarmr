-- 031_api_key_rate_limit.down.sql
ALTER TABLE api_keys DROP COLUMN IF EXISTS rate_limit;
