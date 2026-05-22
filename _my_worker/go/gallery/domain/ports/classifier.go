package ports

import (
	"context"

	"github.com/suekk/my_worker/go/gallery/domain"
)

// ClassifierPort defines interface for NSFW classification
type ClassifierPort interface {
	// ClassifyBatch classifies multiple images for NSFW content
	// Returns classification results for each image
	ClassifyBatch(ctx context.Context, imagePaths []string, onProgress ProgressCallback) ([]domain.ClassificationResult, error)

	// IsAvailable checks if classifier is ready (Python + models loaded)
	IsAvailable() bool
}
