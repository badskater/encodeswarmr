-- Migration 034: add update_channel column to agents table
-- Valid values: "stable" (default), "beta", "nightly"

ALTER TABLE agents
    ADD COLUMN IF NOT EXISTS update_channel TEXT NOT NULL DEFAULT 'stable';
