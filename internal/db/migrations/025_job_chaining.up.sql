-- 024_job_chaining.up.sql
-- Adds job dependency / chaining support and audio encoding config.
--
-- depends_on   - UUID of a predecessor job that must reach "completed" before
--                this job transitions from "waiting" to "queued".
-- chain_group  - UUID that groups all jobs in one logical A/V pipeline chain.
-- audio_config - JSONB codec parameters for audio encoding jobs.

ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS depends_on   UUID REFERENCES jobs(id),
    ADD COLUMN IF NOT EXISTS chain_group  UUID,
    ADD COLUMN IF NOT EXISTS audio_config JSONB;

CREATE INDEX IF NOT EXISTS idx_jobs_depends_on   ON jobs (depends_on)   WHERE depends_on IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_chain_group  ON jobs (chain_group)  WHERE chain_group IS NOT NULL;
