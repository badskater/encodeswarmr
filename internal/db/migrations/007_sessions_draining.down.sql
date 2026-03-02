-- Rollback: migration 007

-- Drop sessions table
DROP TABLE IF EXISTS sessions;

-- Restore original status check (without draining)
ALTER TABLE agents
    DROP CONSTRAINT IF EXISTS agents_status_check;

ALTER TABLE agents
    ADD CONSTRAINT agents_status_check
    CHECK (status IN (
        'pending_approval', 'idle', 'busy',
        'offline', 'disabled'
    ));
