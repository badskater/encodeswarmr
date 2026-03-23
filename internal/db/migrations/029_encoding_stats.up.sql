-- 026_encoding_stats.up.sql
-- Aggregated encoding statistics used for cost estimation and confidence
-- interval computation.  Rows are upserted after each completed encode task.

CREATE TABLE IF NOT EXISTS encoding_stats (
    id              UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    codec           TEXT    NOT NULL,
    resolution      TEXT    NOT NULL DEFAULT '',
    preset          TEXT    NOT NULL DEFAULT '',
    avg_fps         DOUBLE PRECISION NOT NULL DEFAULT 0,
    avg_size_per_min DOUBLE PRECISION NOT NULL DEFAULT 0,
    sample_count    INT     NOT NULL DEFAULT 0,
    -- Confidence interval fields (95 % CI on avg_fps)
    fps_stddev      DOUBLE PRECISION NOT NULL DEFAULT 0,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_encoding_stats UNIQUE (codec, resolution, preset)
);
