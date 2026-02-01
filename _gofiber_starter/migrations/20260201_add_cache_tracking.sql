-- Migration: Add cache tracking columns to videos table
-- Date: 2026-02-01
-- Purpose: Track CDN cache warming status for each video

-- Add cache tracking columns
ALTER TABLE videos ADD COLUMN IF NOT EXISTS cache_status VARCHAR(20) DEFAULT 'pending';
ALTER TABLE videos ADD COLUMN IF NOT EXISTS cache_percentage DECIMAL(5,2) DEFAULT 0;
ALTER TABLE videos ADD COLUMN IF NOT EXISTS cache_error TEXT;
ALTER TABLE videos ADD COLUMN IF NOT EXISTS last_warmed_at TIMESTAMPTZ;

-- Create index for filtering by cache_status
CREATE INDEX IF NOT EXISTS idx_videos_cache_status ON videos(cache_status);

-- Add comments
COMMENT ON COLUMN videos.cache_status IS 'CDN cache status: pending|warming|cached|failed';
COMMENT ON COLUMN videos.cache_percentage IS 'Percentage of segments cached (0-100)';
COMMENT ON COLUMN videos.cache_error IS 'Error message if cache warming failed';
COMMENT ON COLUMN videos.last_warmed_at IS 'Last time cache was warmed';

-- Update existing ready videos to have cache_status = 'pending' (need warming)
UPDATE videos SET cache_status = 'pending' WHERE status = 'ready' AND cache_status IS NULL;
