-- 020_task_retry.down.sql
DROP INDEX IF EXISTS idx_tasks_retry_after;
ALTER TABLE jobs  DROP COLUMN IF EXISTS max_retries;
ALTER TABLE tasks DROP COLUMN IF EXISTS retry_after;
ALTER TABLE tasks DROP COLUMN IF EXISTS retry_count;
