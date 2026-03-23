-- 030_agent_pools: named tag groups for organising agents into pools.
-- Pools are a UI-layer abstraction; the actual dispatch mechanism (ClaimNextTask
-- tag matching) is unchanged.

CREATE TABLE agent_pools (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL UNIQUE,
    description TEXT        NOT NULL DEFAULT '',
    tags        TEXT[]      NOT NULL DEFAULT '{}',
    color       TEXT        NOT NULL DEFAULT '#6366f1',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
