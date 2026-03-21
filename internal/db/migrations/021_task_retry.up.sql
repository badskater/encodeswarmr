-- 020_task_retry.up.sql
-- Adds automatic retry support to tasks and max_retries to jobs.

ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS retry_count INT         NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS retry_after TIMESTAMPTZ;

ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS max_retries INT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_tasks_retry_after
    ON tasks (retry_after) WHERE status = 'pending' AND retry_after IS NOT NULL;
