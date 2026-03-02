-- Migration 002: jobs and tasks tables

-- ---------------------------------------------------------------------------
-- jobs
-- ---------------------------------------------------------------------------
CREATE TABLE jobs (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id    UUID        NOT NULL REFERENCES sources (id) ON DELETE RESTRICT,
    status       TEXT        NOT NULL DEFAULT 'queued'
                             CHECK (status IN (
                                 'queued', 'assigned', 'running',
                                 'completed', 'failed', 'cancelled'
                             )),
    job_type     TEXT        NOT NULL DEFAULT 'encode'
                             CHECK (job_type IN ('encode', 'analysis', 'audio')),
    priority     INT         NOT NULL DEFAULT 5,
    target_tags  TEXT[]      NOT NULL DEFAULT '{}',
    -- Progress counters (denormalised for fast dashboard queries)
    tasks_total     INT      NOT NULL DEFAULT 0,
    tasks_pending   INT      NOT NULL DEFAULT 0,
    tasks_running   INT      NOT NULL DEFAULT 0,
    tasks_completed INT      NOT NULL DEFAULT 0,
    tasks_failed    INT      NOT NULL DEFAULT 0,
    -- Timestamps
    completed_at TIMESTAMPTZ,
    failed_at    TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_jobs_status      ON jobs (status);
CREATE INDEX idx_jobs_priority    ON jobs (priority DESC, created_at ASC);
CREATE INDEX idx_jobs_source_id   ON jobs (source_id);

-- ---------------------------------------------------------------------------
-- tasks
-- ---------------------------------------------------------------------------
CREATE TABLE tasks (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id       UUID        NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    chunk_index  INT         NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'pending'
                             CHECK (status IN (
                                 'pending', 'assigned', 'running',
                                 'completed', 'failed', 'cancelled'
                             )),
    agent_id     UUID        REFERENCES agents (id) ON DELETE SET NULL,
    script_dir   TEXT        NOT NULL DEFAULT '',
    source_path  TEXT        NOT NULL DEFAULT '',
    output_path  TEXT        NOT NULL DEFAULT '',
    variables    JSONB       NOT NULL DEFAULT '{}',
    exit_code    INT,
    -- Encoding result metrics (populated on completion)
    frames_encoded  BIGINT,
    avg_fps         FLOAT,
    output_size     BIGINT,
    duration_sec    BIGINT,
    vmaf_score      FLOAT,
    psnr            FLOAT,
    ssim            FLOAT,
    error_msg    TEXT,
    started_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (job_id, chunk_index)
);

CREATE INDEX idx_tasks_job_id   ON tasks (job_id);
CREATE INDEX idx_tasks_status   ON tasks (status);
CREATE INDEX idx_tasks_agent_id ON tasks (agent_id) WHERE agent_id IS NOT NULL;
-- Fast scheduler query: pending tasks ordered by job priority
CREATE INDEX idx_tasks_dispatch ON tasks (status, job_id) WHERE status = 'pending';
