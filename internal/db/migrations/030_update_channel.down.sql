-- 030_update_channel.down.sql
DROP TABLE IF EXISTS upgrade_binaries;
ALTER TABLE agents DROP COLUMN IF EXISTS update_channel;
