-- Migration 011 rollback: revert jobs job_type constraint to original set

ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_job_type_check;

ALTER TABLE jobs
    ADD CONSTRAINT jobs_job_type_check
    CHECK (job_type IN ('encode', 'analysis', 'audio'));
