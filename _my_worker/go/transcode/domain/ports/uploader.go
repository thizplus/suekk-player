package ports

import "context"

// UploaderPort defines interface for segment upload operations
type UploaderPort interface {
	// Start begins watching the directory and uploading segments
	Start(localDir, remotePrefix string) error

	// Stop stops the watcher and waits for pending uploads
	Stop()

	// Wait waits for all pending uploads to complete
	Wait() error

	// FlushNow forces an immediate flush of the window buffer
	FlushNow()

	// UploadPlaylists uploads all .m3u8 files
	UploadPlaylists(ctx context.Context, localDir, remotePrefix string) error

	// GetStats returns upload statistics (count, bytes)
	GetStats() (count int64, bytes int64)

	// GetQualitySizes returns bytes per quality
	GetQualitySizes() map[string]int64

	// GetQualitySegmentCounts returns segment count per quality
	GetQualitySegmentCounts() map[string]int

	// CleanupUploadedSegments deletes all uploaded segments from local disk
	CleanupUploadedSegments() (deleted int, freedBytes int64)
}
