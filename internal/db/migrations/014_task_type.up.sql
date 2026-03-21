-- Migration 014: add task_type column to tasks table
-- task_type distinguishes regular encode tasks from post-processing tasks
-- such as "concat" (ffmpeg segment merge).  The default empty string preserves
-- backward compatibility with existing rows.

ALTER TABLE tasks ADD COLUMN task_type TEXT NOT NULL DEFAULT '';

-- Allow the job type enum on jobs to include 'concat' as well so the
-- scheduler can filter correctly.
-- Note: task_type is intentionally unconstrained by CHECK so new types can
-- be added without a migration.
