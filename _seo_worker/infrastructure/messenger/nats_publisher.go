package messenger

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"

	"seo-worker/domain/models"
	"seo-worker/domain/ports"
)

type NATSPublisher struct {
	nc     *nats.Conn
	logger *slog.Logger
}

func NewNATSPublisher(nc *nats.Conn) *NATSPublisher {
	return &NATSPublisher{
		nc:     nc,
		logger: slog.Default().With("component", "nats_publisher"),
	}
}

// SendProgress ส่ง progress update ไปที่ NATS
// Subject: seo.progress.{video_id}
func (p *NATSPublisher) SendProgress(ctx context.Context, update *models.ProgressUpdate) error {
	subject := fmt.Sprintf("seo.progress.%s", update.VideoID)

	data, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal progress update: %w", err)
	}

	if err := p.nc.Publish(subject, data); err != nil {
		return fmt.Errorf("failed to publish progress: %w", err)
	}

	p.logger.DebugContext(ctx, "Progress sent",
		"video_id", update.VideoID,
		"stage", update.Stage,
		"progress", update.Progress,
	)

	return nil
}

// SendCompleted ส่งแจ้งว่า job เสร็จแล้ว
func (p *NATSPublisher) SendCompleted(ctx context.Context, videoID string) error {
	update := &models.ProgressUpdate{
		VideoID:   videoID,
		Stage:     ports.StageCompleted,
		Progress:  100,
		Message:   "Article generated successfully",
		Timestamp: time.Now().Unix(),
	}
	return p.SendProgress(ctx, update)
}

// SendFailed ส่งแจ้งว่า job failed
func (p *NATSPublisher) SendFailed(ctx context.Context, videoID string, err error) error {
	update := &models.ProgressUpdate{
		VideoID:   videoID,
		Stage:     ports.StageFailed,
		Progress:  0,
		Error:     err.Error(),
		Timestamp: time.Now().Unix(),
	}
	return p.SendProgress(ctx, update)
}

// Verify interface implementation
var _ ports.MessengerPort = (*NATSPublisher)(nil)
