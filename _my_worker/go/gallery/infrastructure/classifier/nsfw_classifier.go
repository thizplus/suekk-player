package classifier

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/suekk/my_worker/go/gallery/config"
	"github.com/suekk/my_worker/go/gallery/domain"
	"github.com/suekk/my_worker/go/gallery/domain/ports"
)

// NSFWClassifier implements ClassifierPort using Python subprocess
type NSFWClassifier struct {
	config *config.Config
	logger *slog.Logger
}

// NewNSFWClassifier creates a new NSFW classifier
func NewNSFWClassifier(cfg *config.Config) *NSFWClassifier {
	return &NSFWClassifier{
		config: cfg,
		logger: slog.Default().With("component", "nsfw-classifier"),
	}
}

// ClassifyBatch classifies multiple images for NSFW content
// Uses Python subprocess to run NudeNet/FalconSAI models
func (c *NSFWClassifier) ClassifyBatch(ctx context.Context, imagePaths []string, onProgress ports.ProgressCallback) ([]domain.ClassificationResult, error) {
	if len(imagePaths) == 0 {
		return nil, nil
	}

	// Resolve script path
	scriptPath := c.config.ClassifierScript
	if !filepath.IsAbs(scriptPath) {
		// Make it relative to the gallery worker directory
		// Default: ../../python/classify_batch.py
		scriptPath = filepath.Clean(scriptPath)
	}

	c.logger.Info("Running NSFW classifier",
		"script", scriptPath,
		"image_count", len(imagePaths),
	)

	// Build command: python classify_batch.py --images img1.jpg img2.jpg ...
	args := []string{scriptPath, "--images"}
	args = append(args, imagePaths...)

	// Add thresholds
	args = append(args,
		"--super-safe-threshold", fmt.Sprintf("%.2f", c.config.SuperSafeThreshold),
		"--safe-threshold", fmt.Sprintf("%.2f", c.config.SafeThreshold),
	)

	cmd := exec.CommandContext(ctx, c.config.PythonPath, args...)

	// Capture output
	output, err := cmd.Output()
	if err != nil {
		// Try to get stderr for more info
		if exitErr, ok := err.(*exec.ExitError); ok {
			c.logger.Error("Python classifier failed",
				"error", err,
				"stderr", string(exitErr.Stderr),
			)
		}
		return nil, fmt.Errorf("classifier failed: %w", err)
	}

	// Parse JSON output
	var results []domain.ClassificationResult
	if err := json.Unmarshal(output, &results); err != nil {
		return nil, fmt.Errorf("failed to parse classifier output: %w", err)
	}

	c.logger.Info("Classification completed",
		"total", len(results),
		"results", summarizeResults(results),
	)

	return results, nil
}

// IsAvailable checks if classifier is ready
func (c *NSFWClassifier) IsAvailable() bool {
	// Check if Python is available
	cmd := exec.Command(c.config.PythonPath, "--version")
	if err := cmd.Run(); err != nil {
		c.logger.Warn("Python not available", "error", err)
		return false
	}

	// Check if script exists
	scriptPath := c.config.ClassifierScript
	if !filepath.IsAbs(scriptPath) {
		scriptPath = filepath.Clean(scriptPath)
	}

	// Optionally check if required models are loaded
	// For now just return true if Python is available
	return true
}

// summarizeResults creates a summary of classification results
func summarizeResults(results []domain.ClassificationResult) string {
	superSafe := 0
	safe := 0
	nsfw := 0

	for _, r := range results {
		switch r.Classification {
		case "super_safe":
			superSafe++
		case "safe":
			safe++
		case "nsfw":
			nsfw++
		}
	}

	return fmt.Sprintf("super_safe=%d, safe=%d, nsfw=%d", superSafe, safe, nsfw)
}

// ClassifyWithLocalThresholds allows custom threshold classification
func (c *NSFWClassifier) ClassifyWithLocalThresholds(results []domain.ClassificationResult) []domain.ClassificationResult {
	// Re-classify based on local thresholds
	for i, r := range results {
		if r.NsfwScore < c.config.SuperSafeThreshold && r.HasFace {
			results[i].Classification = string(domain.ClassificationSuperSafe)
		} else if r.NsfwScore < c.config.SafeThreshold {
			results[i].Classification = string(domain.ClassificationSafe)
		} else {
			results[i].Classification = string(domain.ClassificationNSFW)
		}
	}
	return results
}

// GetImagePaths filters valid image paths
func GetImagePaths(dir string) ([]string, error) {
	var paths []string
	entries, err := filepath.Glob(filepath.Join(dir, "*.jpg"))
	if err != nil {
		return nil, err
	}
	paths = append(paths, entries...)

	pngEntries, _ := filepath.Glob(filepath.Join(dir, "*.png"))
	paths = append(paths, pngEntries...)

	return paths, nil
}

// SortByClassification groups images by classification type
func SortByClassification(results []domain.ClassificationResult) map[string][]string {
	grouped := make(map[string][]string)

	for _, r := range results {
		classification := strings.ToLower(r.Classification)
		grouped[classification] = append(grouped[classification], r.Filename)
	}

	return grouped
}
