-- Migration 030 rollback: remove thumbnails column from sources table.
ALTER TABLE sources DROP COLUMN IF EXISTS thumbnails;
