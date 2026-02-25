package messenger

import (
	"context"
	"log/slog"

	"seo-worker/domain/models"
	"seo-worker/domain/ports"
)

// NoopMessenger - Messenger ที่ไม่ส่งอะไรเลย ใช้สำหรับ testing
type NoopMessenger struct {
	logger *slog.Logger
}

func NewNoopMessenger() *NoopMessenger {
	return &NoopMessenger{
		logger: slog.Default().With("component", "noop_messenger"),
	}
}

func (m *NoopMessenger) SendProgress(ctx context.Context, update *models.ProgressUpdate) error {
	m.logger.InfoContext(ctx, "Progress (noop)",
		"video_id", update.VideoID,
		"stage", update.Stage,
		"progress", update.Progress,
	)
	return nil
}

func (m *NoopMessenger) SendCompleted(ctx context.Context, videoID string) error {
	m.logger.InfoContext(ctx, "Completed (noop)", "video_id", videoID)
	return nil
}

func (m *NoopMessenger) SendFailed(ctx context.Context, videoID string, err error) error {
	m.logger.WarnContext(ctx, "Failed (noop)", "video_id", videoID, "error", err)
	return nil
}

// Verify interface implementation
var _ ports.MessengerPort = (*NoopMessenger)(nil)
