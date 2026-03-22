-- 024_notification_prefs_email.up.sql
-- Adds email notification fields to the notification_preferences table.

ALTER TABLE notification_preferences
    ADD COLUMN IF NOT EXISTS email_address TEXT    NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS notify_email  BOOL    NOT NULL DEFAULT false;
