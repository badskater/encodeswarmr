-- Migration 014 rollback: remove task_type column
ALTER TABLE tasks DROP COLUMN IF EXISTS task_type;
