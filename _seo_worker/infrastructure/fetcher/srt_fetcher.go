package fetcher

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"seo-worker/domain/ports"
)

type SRTFetcher struct {
	storage ports.StoragePort
	logger  *slog.Logger
}

// NewSRTFetcher สร้าง SRT fetcher
// อ่าน SRT โดยตรงจาก storage (R2/S3) ที่ path: subtitles/{code}/th.srt
func NewSRTFetcher(storage ports.StoragePort) *SRTFetcher {
	return &SRTFetcher{
		storage: storage,
		logger:  slog.Default().With("component", "srt_fetcher"),
	}
}

func (f *SRTFetcher) FetchSRT(ctx context.Context, videoCode string) (string, error) {
	// Storage path: subtitles/{code}/th.srt
	storagePath := fmt.Sprintf("subtitles/%s/th.srt", videoCode)

	f.logger.InfoContext(ctx, "Fetching SRT from storage",
		"video_code", videoCode,
		"path", storagePath,
	)

	// ดึงไฟล์จาก storage
	reader, _, err := f.storage.GetFileContent(storagePath)
	if err != nil {
		return "", fmt.Errorf("failed to get SRT from storage: %w", err)
	}
	defer reader.Close()

	srtContent, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read SRT content: %w", err)
	}

	f.logger.InfoContext(ctx, "SRT fetched",
		"video_code", videoCode,
		"size", len(srtContent),
	)

	return string(srtContent), nil
}

// Verify interface implementation
var _ ports.SRTFetcherPort = (*SRTFetcher)(nil)
