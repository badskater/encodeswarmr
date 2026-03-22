-- Migration 024: job_archive table for archived completed/failed jobs

-- job_archive mirrors the jobs table structure exactly (including all columns
-- and constraints) but holds jobs that have been moved out of the active jobs
-- table after the retention period expires.
CREATE TABLE job_archive (LIKE jobs INCLUDING ALL);

-- Remove the foreign-key constraint on source_id since sources may have been
-- deleted by the time a job is archived.
ALTER TABLE job_archive DROP CONSTRAINT IF EXISTS job_archive_source_id_fkey;

-- Add archived_at timestamp so we know when the record was moved.
ALTER TABLE job_archive ADD COLUMN archived_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- task_archive mirrors the tasks table for archived tasks.
CREATE TABLE task_archive (LIKE tasks INCLUDING ALL);
ALTER TABLE task_archive DROP CONSTRAINT IF EXISTS task_archive_job_id_fkey;
ALTER TABLE task_archive ADD COLUMN archived_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- task_log_archive mirrors the task_logs table for archived logs.
CREATE TABLE task_log_archive (LIKE task_logs INCLUDING ALL);
ALTER TABLE task_log_archive DROP CONSTRAINT IF EXISTS task_log_archive_task_id_fkey;
ALTER TABLE task_log_archive ADD COLUMN archived_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Index on archived_at for efficient range scans during export.
CREATE INDEX idx_job_archive_archived_at  ON job_archive  (archived_at);
CREATE INDEX idx_job_archive_status       ON job_archive  (status);
CREATE INDEX idx_job_archive_created_at   ON job_archive  (created_at);
CREATE INDEX idx_task_archive_job_id      ON task_archive (job_id);
