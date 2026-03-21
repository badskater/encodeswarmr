CREATE TABLE IF NOT EXISTS agent_metrics (
    id          BIGSERIAL   PRIMARY KEY,
    agent_id    UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    cpu_pct     REAL        NOT NULL DEFAULT 0,
    gpu_pct     REAL        NOT NULL DEFAULT 0,
    mem_pct     REAL        NOT NULL DEFAULT 0,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_metrics_agent_recorded
    ON agent_metrics (agent_id, recorded_at DESC);
