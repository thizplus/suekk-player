package classifier

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"sort"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// NSFWClassifier - Go wrapper for Python NudeNet classifier
// Uses subprocess to call classify_batch.py for batch processing
// ═══════════════════════════════════════════════════════════════════════════════

// NSFWClassifier wraps Python NudeNet classifier
type NSFWClassifier struct {
	config ClassifierConfig
	logger *slog.Logger
}

// NewNSFWClassifier creates a new NSFW classifier instance
func NewNSFWClassifier(config ClassifierConfig, logger *slog.Logger) *NSFWClassifier {
	if logger == nil {
		logger = slog.Default()
	}

	return &NSFWClassifier{
		config: config,
		logger: logger.With("component", "nsfw-classifier"),
	}
}

// NewNSFWClassifierWithDefaults creates classifier with default config
func NewNSFWClassifierWithDefaults(logger *slog.Logger) *NSFWClassifier {
	return NewNSFWClassifier(DefaultConfig(), logger)
}

// ClassifyBatch classifies all images in a folder
// Returns BatchResult with classification results for each image
func (c *NSFWClassifier) ClassifyBatch(ctx context.Context, inputPath string) (*BatchResult, error) {
	c.logger.Info("starting batch classification",
		"input_path", inputPath,
		"timeout", c.config.Timeout,
		"verbose", c.config.Verbose,
		"super_safe_threshold", c.config.SuperSafeThreshold,
		"nsfw_threshold", c.config.NsfwThreshold,
		"min_face_score", c.config.MinFaceScore,
	)

	startTime := time.Now()

	// Create context with timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Duration(c.config.Timeout)*time.Second)
	defer cancel()

	// Build command with all thresholds
	args := []string{
		c.config.ScriptPath,
		"--input", inputPath,
		"--threshold", fmt.Sprintf("%.2f", c.config.NsfwThreshold),
		"--super-safe-threshold", fmt.Sprintf("%.2f", c.config.SuperSafeThreshold),
		"--min-face-score", fmt.Sprintf("%.2f", c.config.MinFaceScore),
	}

	// Add verbose flag for detailed per-image logging
	if c.config.Verbose {
		args = append(args, "--verbose")
	}

	// Add skip flags for performance
	if c.config.SkipMosaic {
		args = append(args, "--skip-mosaic")
	}
	if c.config.SkipPOV {
		args = append(args, "--skip-pov")
	}

	// Add dedup flags
	if c.config.SkipDedup {
		args = append(args, "--skip-dedup")
	}
	if c.config.DedupThreshold > 0 {
		args = append(args, "--dedup-threshold", fmt.Sprintf("%d", c.config.DedupThreshold))
	}

	cmd := exec.CommandContext(ctxWithTimeout, c.config.PythonPath, args...)

	// Run command and capture both stdout and stderr
	output, err := cmd.Output()
	if err != nil {
		// Check if it was a timeout
		if ctxWithTimeout.Err() == context.DeadlineExceeded {
			c.logger.Error("classification timeout",
				"input_path", inputPath,
				"timeout_sec", c.config.Timeout,
			)
			return nil, fmt.Errorf("classification timeout after %d seconds", c.config.Timeout)
		}

		// Get stderr for error details
		if exitErr, ok := err.(*exec.ExitError); ok {
			c.logger.Error("classification failed",
				"input_path", inputPath,
				"stderr", string(exitErr.Stderr),
				"error", err,
			)
			return nil, fmt.Errorf("classification failed: %s", string(exitErr.Stderr))
		}

		return nil, fmt.Errorf("classification error: %w", err)
	}

	// Parse JSON output
	var result BatchResult
	if err := json.Unmarshal(output, &result); err != nil {
		c.logger.Error("failed to parse classification result",
			"output", string(output),
			"error", err,
		)
		return nil, fmt.Errorf("failed to parse result: %w", err)
	}

	processingTime := time.Since(startTime).Seconds()

	// Log with Three-Tier stats
	c.logger.Info("batch classification complete",
		"input_path", inputPath,
		"original", result.Stats.OriginalImages,
		"duplicates_removed", result.Stats.DuplicatesRemoved,
		"unique", result.Stats.TotalImages,
		"super_safe", result.Stats.SuperSafeCount,
		"safe", result.Stats.SafeCount,
		"nsfw", result.Stats.NsfwCount,
		"mosaic", result.Stats.MosaicCount,
		"errors", result.Stats.ErrorCount,
		"avg_nsfw_score", result.Stats.AvgNsfwScore,
		"avg_face_score", result.Stats.AvgFaceScore,
		"time_sec", processingTime,
	)

	// Log detailed per-image results if verbose
	if c.config.Verbose {
		for filename, imgResult := range result.Results {
			c.logger.Info("image_classified",
				"filename", filename,
				"classification", imgResult.Classification,
				"falconsai", imgResult.FalconsaiScore,
				"nudenet", imgResult.NudenetScore,
				"combined", imgResult.NsfwScore,
				"face", imgResult.FaceScore,
				"mosaic", imgResult.MosaicDetected,
				"mosaic_score", imgResult.MosaicScore,
				"reason", imgResult.Reason,
			)
		}
	}

	return &result, nil
}

// SeparateResults separates classification results into three tiers + error
// Three-Tier: SuperSafe (< 0.15 + face) | Safe (0.15-0.3) | NSFW (>= 0.3)
func (c *NSFWClassifier) SeparateResults(results map[string]ClassificationResult) *SeparatedImages {
	separated := &SeparatedImages{
		SuperSafe: make([]ClassificationResult, 0),
		Safe:      make([]ClassificationResult, 0),
		Nsfw:      make([]ClassificationResult, 0),
		Error:     make([]ClassificationResult, 0),
	}

	for _, result := range results {
		if result.Error != "" {
			// Error → treat as NSFW (safety first)
			separated.Error = append(separated.Error, result)
		} else if result.IsSuperSafe {
			// Super Safe: < 0.15 + has face (Public SEO)
			separated.SuperSafe = append(separated.SuperSafe, result)
		} else if result.IsSafe {
			// Safe: 0.15-0.3 or no face (Lazy load)
			separated.Safe = append(separated.Safe, result)
		} else {
			// NSFW: >= 0.3 (Member only)
			separated.Nsfw = append(separated.Nsfw, result)
		}
	}

	return separated
}

// SortByQuality sorts images by quality score (face_score * 2 + aesthetic_score)
// Prioritizes images with visible faces
func (c *NSFWClassifier) SortByQuality(images []ClassificationResult) {
	sort.Slice(images, func(i, j int) bool {
		scoreI := images[i].FaceScore*2 + images[i].AestheticScore
		scoreJ := images[j].FaceScore*2 + images[j].AestheticScore
		return scoreI > scoreJ
	})
}

// SelectTopNsfw selects top N NSFW images by quality (for storage limit)
func (c *NSFWClassifier) SelectTopNsfw(nsfwImages []ClassificationResult, maxCount int) []ClassificationResult {
	if len(nsfwImages) <= maxCount {
		return nsfwImages
	}

	// Sort by quality
	c.SortByQuality(nsfwImages)

	// Return top N
	return nsfwImages[:maxCount]
}

// GetImagePaths extracts file paths from classification results
func (c *NSFWClassifier) GetImagePaths(results []ClassificationResult, baseDir string) []string {
	paths := make([]string, len(results))
	for i, result := range results {
		paths[i] = filepath.Join(baseDir, result.Filename)
	}
	return paths
}

// HasEnoughSafeImages checks if we have minimum required safe images
func (c *NSFWClassifier) HasEnoughSafeImages(safeCount int) bool {
	return safeCount >= c.config.MinSafeImages
}

// GetConfig returns current classifier configuration
func (c *NSFWClassifier) GetConfig() ClassifierConfig {
	return c.config
}
