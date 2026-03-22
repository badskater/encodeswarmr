-- 024_job_chaining.down.sql

DROP INDEX IF EXISTS idx_jobs_chain_group;
DROP INDEX IF EXISTS idx_jobs_depends_on;

ALTER TABLE jobs
    DROP COLUMN IF EXISTS audio_config,
    DROP COLUMN IF EXISTS chain_group,
    DROP COLUMN IF EXISTS depends_on;
