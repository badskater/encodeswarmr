-- Rollback migration 024: drop archive tables
DROP TABLE IF EXISTS task_log_archive;
DROP TABLE IF EXISTS task_archive;
DROP TABLE IF EXISTS job_archive;
