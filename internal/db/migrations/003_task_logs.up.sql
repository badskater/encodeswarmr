-- Migration 003: task_logs — centralised log storage for SSE streaming

CREATE TABLE task_logs (
    id         BIGSERIAL   PRIMARY KEY,
    task_id    UUID        NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    job_id     UUID        NOT NULL REFERENCES jobs  (id) ON DELETE CASCADE,
    -- "stdout", "stderr", or "agent"
    stream     TEXT        NOT NULL DEFAULT 'stdout',
    -- "debug", "info", "warn", "error"
    level      TEXT        NOT NULL DEFAULT 'info',
    message    TEXT        NOT NULL,
    metadata   JSONB,
    logged_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Most queries either tail a task or fetch all logs for a job.
CREATE INDEX idx_task_logs_task_id ON task_logs (task_id, logged_at DESC);
CREATE INDEX idx_task_logs_job_id  ON task_logs (job_id,  logged_at DESC);
