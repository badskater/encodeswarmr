-- 021_agent_upgrade_flag.up.sql
-- Adds upgrade_requested flag to agents for server-initiated push upgrade.

ALTER TABLE agents
    ADD COLUMN IF NOT EXISTS upgrade_requested BOOL NOT NULL DEFAULT false;
