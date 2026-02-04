-- Migration: Add cover_time column for custom thumbnail/cover frame selection
-- Date: 2026-02-04
-- Description: Allow users to select a specific frame for reel cover/thumbnail

-- Add cover_time column (-1 = auto middle of segment)
ALTER TABLE reels ADD COLUMN IF NOT EXISTS cover_time FLOAT DEFAULT -1;

-- Add comment explaining the column
COMMENT ON COLUMN reels.cover_time IS 'Cover/thumbnail frame time in seconds (-1 = auto middle of segment)';
