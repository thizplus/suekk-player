-- Migration: Add style-based reel fields
-- Date: 2026-02-04
-- Description: Simplify reel system from complex layer-based to 3 fixed styles

-- Add new style-based columns
ALTER TABLE reels ADD COLUMN IF NOT EXISTS style VARCHAR(20) NOT NULL DEFAULT 'letterbox';
ALTER TABLE reels ADD COLUMN IF NOT EXISTS line1 VARCHAR(255);
ALTER TABLE reels ADD COLUMN IF NOT EXISTS line2 VARCHAR(255);
ALTER TABLE reels ADD COLUMN IF NOT EXISTS show_logo BOOLEAN DEFAULT true;

-- Add index for style column (for filtering)
CREATE INDEX IF NOT EXISTS idx_reels_style ON reels(style);

-- Migrate existing data: Map old video_fit to new style
UPDATE reels SET
    style = CASE
        WHEN video_fit = 'fill' THEN 'fullcover'
        WHEN video_fit = 'crop-1:1' THEN 'square'
        ELSE 'letterbox'
    END,
    line1 = COALESCE(description, ''),
    show_logo = true
WHERE style = 'letterbox' OR style IS NULL;

-- Add comment explaining the migration
COMMENT ON COLUMN reels.style IS 'Reel display style: letterbox (16:9 centered), square (1:1 centered), fullcover (fill 9:16)';
COMMENT ON COLUMN reels.line1 IS 'Secondary text line 1 (replaces description)';
COMMENT ON COLUMN reels.line2 IS 'Secondary text line 2';
COMMENT ON COLUMN reels.show_logo IS 'Whether to show logo overlay';

-- Note: Legacy columns (output_format, video_fit, crop_x, crop_y, layers, template_id)
-- are NOT dropped for backward compatibility. They can be removed in a future migration
-- after confirming all existing data has been migrated and no code depends on them.
