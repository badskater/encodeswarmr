-- 025_task_preemption.up.sql
-- Adds preemption support to tasks: a task can be interrupted mid-run when a
-- higher-priority task needs the agent.

ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS preemptible   BOOL        NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS preempted_at  TIMESTAMPTZ;
