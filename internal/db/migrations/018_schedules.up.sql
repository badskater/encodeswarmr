-- 017_schedules.up.sql
-- Job scheduling table for recurring cron-based encode triggers.

CREATE TABLE IF NOT EXISTS schedules (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT        NOT NULL,
    cron_expr     TEXT        NOT NULL,
    job_template  JSONB       NOT NULL,
    enabled       BOOL        NOT NULL DEFAULT true,
    last_run_at   TIMESTAMPTZ,
    next_run_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
