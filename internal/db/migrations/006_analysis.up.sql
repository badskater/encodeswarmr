-- Migration 006: analysis results (histogram, VMAF, scene detection)

CREATE TABLE analysis_results (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id  UUID        NOT NULL REFERENCES sources (id) ON DELETE CASCADE,
    -- "histogram", "vmaf", "scene_detect"
    type       TEXT        NOT NULL CHECK (type IN ('histogram', 'vmaf', 'scene_detect')),
    -- Per-frame data as JSONB array (can be large for histogram/vmaf).
    -- scene_detect stores: [{"frame":0,"pts":0.0,"score":0.82}, ...]
    frame_data JSONB,
    -- Aggregated summary (e.g. {"mean":94.2,"min":89.1,"psnr":41.3,"ssim":0.987})
    summary    JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Only one result of each type per source (latest wins).
    UNIQUE (source_id, type)
);

CREATE INDEX idx_analysis_results_source_id ON analysis_results (source_id);
