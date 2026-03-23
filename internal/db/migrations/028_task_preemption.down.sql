-- 025_task_preemption.down.sql
ALTER TABLE tasks
    DROP COLUMN IF EXISTS preemptible,
    DROP COLUMN IF EXISTS preempted_at;
