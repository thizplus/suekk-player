package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"

	"gofiber-template/domain/ports"
	natspkg "gofiber-template/infrastructure/nats"
)

// NATSProgressPublisher implements ProgressPublisherPort using NATS Pub/Sub
type NATSProgressPublisher struct {
	conn *nats.Conn
}

// NewNATSProgressPublisher สร้าง ProgressPublisherPort adapter สำหรับ NATS
func NewNATSProgressPublisher(conn *nats.Conn) ports.ProgressPublisherPort {
	return &NATSProgressPublisher{
		conn: conn,
	}
}

// PublishProgress ส่ง progress update ผ่าน NATS Pub/Sub
func (p *NATSProgressPublisher) PublishProgress(ctx context.Context, progress *ports.ProgressData) error {
	if progress == nil {
		return fmt.Errorf("progress cannot be nil")
	}
	if progress.VideoID == "" {
		return fmt.Errorf("video_id is required")
	}

	// Convert to NATS type
	natsProgress := &natspkg.ProgressUpdate{
		VideoID:    progress.VideoID,
		VideoCode:  progress.VideoCode,
		Status:     progress.Status,
		Progress:   progress.Progress,
		Quality:    progress.Quality,
		Message:    progress.Message,
		Error:      progress.Error,
		OutputPath: progress.OutputPath,
	}

	// Serialize
	data, err := json.Marshal(natsProgress)
	if err != nil {
		return fmt.Errorf("failed to marshal progress: %w", err)
	}

	// Publish to subject: progress.{videoID}
	subject := fmt.Sprintf("progress.%s", progress.VideoID)
	return p.conn.Publish(subject, data)
}
