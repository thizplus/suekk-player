-- Migration: Add retry tracking columns to videos table
-- Date: 2026-01-17
-- Purpose: Support job failure handling with retry count and stuck detection

-- Add retry_count column (default 0)
ALTER TABLE videos ADD COLUMN IF NOT EXISTS retry_count INT DEFAULT 0;

-- Add last_error column (stores last error message)
ALTER TABLE videos ADD COLUMN IF NOT EXISTS last_error TEXT;

-- Add processing_started_at column (for stuck job detection)
ALTER TABLE videos ADD COLUMN IF NOT EXISTS processing_started_at TIMESTAMPTZ;

-- Create index for stuck job detection query
CREATE INDEX IF NOT EXISTS idx_videos_processing_stuck
ON videos (status, processing_started_at)
WHERE status = 'processing';

-- Create index for retry count queries
CREATE INDEX IF NOT EXISTS idx_videos_retry_count
ON videos (retry_count)
WHERE retry_count > 0;

-- Verify columns were added
-- SELECT column_name, data_type, column_default
-- FROM information_schema.columns
-- WHERE table_name = 'videos'
-- AND column_name IN ('retry_count', 'last_error', 'processing_started_at');
