-- Migration 004: script templates and global variables

-- ---------------------------------------------------------------------------
-- templates
-- ---------------------------------------------------------------------------
CREATE TABLE templates (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL UNIQUE,
    description TEXT        NOT NULL DEFAULT '',
    -- "run_script" (.bat) or "frameserver" (.avs/.vpy)
    type        TEXT        NOT NULL DEFAULT 'run_script'
                            CHECK (type IN ('run_script', 'frameserver')),
    -- file extension without leading dot: "bat", "avs", "vpy"
    extension   TEXT        NOT NULL DEFAULT 'bat',
    content     TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- variables
-- ---------------------------------------------------------------------------
CREATE TABLE variables (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL UNIQUE,
    value       TEXT        NOT NULL DEFAULT '',
    description TEXT        NOT NULL DEFAULT '',
    category    TEXT        NOT NULL DEFAULT 'general',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_variables_category ON variables (category);
