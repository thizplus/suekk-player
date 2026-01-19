package messaging

import (
	"context"
	"fmt"
	"time"

	"gofiber-template/domain/ports"
	natspkg "gofiber-template/infrastructure/nats"
)

// NATSJobQueue implements JobQueuePort using NATS JetStream
type NATSJobQueue struct {
	publisher *natspkg.Publisher
	client    *natspkg.Client
}

// NewNATSJobQueue สร้าง JobQueuePort adapter สำหรับ NATS
func NewNATSJobQueue(client *natspkg.Client, publisher *natspkg.Publisher) ports.JobQueuePort {
	return &NATSJobQueue{
		publisher: publisher,
		client:    client,
	}
}

// PublishJob ส่ง transcode job เข้า queue
func (q *NATSJobQueue) PublishJob(ctx context.Context, job *ports.TranscodeJobData) error {
	// Validate input
	if job == nil {
		return fmt.Errorf("job cannot be nil")
	}
	if job.VideoID == "" {
		return fmt.Errorf("video_id is required")
	}

	// Convert to NATS-specific type
	natsJob := &natspkg.TranscodeJob{
		VideoID:      job.VideoID,
		VideoCode:    job.VideoCode,
		InputPath:    job.InputPath,
		OutputPath:   job.OutputPath,
		Codec:        job.Codec,
		Qualities:    job.Qualities,
		UseByteRange: job.UseByteRange,
		CreatedAt:    time.Now().Unix(),
	}

	return q.publisher.PublishTranscodeJob(ctx, natsJob)
}

// GetQueueStatus ดึงสถานะ queue
func (q *NATSJobQueue) GetQueueStatus(ctx context.Context) (*ports.QueueStatus, error) {
	status, err := q.client.GetStatus(ctx)
	if err != nil {
		return nil, err
	}

	return &ports.QueueStatus{
		StreamName:  status.Stream.Name,
		PendingJobs: status.Consumer.NumPending,
		AckPending:  uint64(status.Consumer.NumAckPending),
		TotalBytes:  status.Stream.Bytes,
	}, nil
}
