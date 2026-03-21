-- 021_agent_upgrade_flag.down.sql
ALTER TABLE agents DROP COLUMN IF EXISTS upgrade_requested;
