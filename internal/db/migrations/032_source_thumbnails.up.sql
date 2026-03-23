-- Migration 030: Add thumbnails column to sources table.
-- Stores an array of relative paths to generated preview thumbnail images.
ALTER TABLE sources
    ADD COLUMN IF NOT EXISTS thumbnails JSONB NOT NULL DEFAULT '[]';
