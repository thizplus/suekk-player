-- Migration: Add gallery_super_safe_count column for Three-Tier Gallery System
-- Date: 2026-02-24
-- Purpose: Track super_safe images (< 0.15 + face) for Public SEO

-- Add gallery_super_safe_count column
ALTER TABLE videos ADD COLUMN IF NOT EXISTS gallery_super_safe_count INT DEFAULT 0;

-- Add comment
COMMENT ON COLUMN videos.gallery_super_safe_count IS 'Number of super_safe gallery images (nsfw_score < 0.15 AND has face) for Public SEO';

-- Update existing videos: set super_safe_count = safe_count for backward compatibility
-- (New worker will recalculate when gallery is regenerated)
UPDATE videos
SET gallery_super_safe_count = gallery_safe_count
WHERE gallery_count > 0 AND gallery_super_safe_count = 0;
