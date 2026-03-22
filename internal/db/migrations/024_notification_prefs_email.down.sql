-- 024_notification_prefs_email.down.sql

ALTER TABLE notification_preferences
    DROP COLUMN IF EXISTS email_address,
    DROP COLUMN IF EXISTS notify_email;
