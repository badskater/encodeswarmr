-- 019_notification_prefs.up.sql
-- Per-user notification preference table.

CREATE TABLE IF NOT EXISTS notification_preferences (
    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    notify_on_job_complete   BOOL        NOT NULL DEFAULT true,
    notify_on_job_failed     BOOL        NOT NULL DEFAULT true,
    notify_on_agent_stale    BOOL        NOT NULL DEFAULT false,
    webhook_filter_user_only BOOL        NOT NULL DEFAULT false,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id)
);
