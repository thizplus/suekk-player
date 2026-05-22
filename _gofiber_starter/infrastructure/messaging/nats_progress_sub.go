package messaging

import (
	"context"

	"gofiber-template/domain/ports"
	natspkg "gofiber-template/infrastructure/nats"
	"gofiber-template/pkg/logger"
)

// NATSProgressSubscriber implements ProgressSubscriberPort using NATS Pub/Sub
type NATSProgressSubscriber struct {
	subscriber *natspkg.Subscriber
	cancel     context.CancelFunc
}

// NewNATSProgressSubscriber สร้าง ProgressSubscriberPort adapter สำหรับ NATS
func NewNATSProgressSubscriber(subscriber *natspkg.Subscriber) ports.ProgressSubscriberPort {
	return &NATSProgressSubscriber{
		subscriber: subscriber,
	}
}

// Subscribe เริ่ม listen progress updates
func (s *NATSProgressSubscriber) Subscribe(ctx context.Context, handler ports.ProgressHandler) error {
	// Store cancel function for Unsubscribe
	ctx, s.cancel = context.WithCancel(ctx)

	// Wrap handler to convert NATS type to port type
	natsHandler := func(update *natspkg.ProgressUpdate) {
		// ดัก nil data
		if update == nil {
			logger.Warn("Received nil progress update from NATS")
			return
		}

		// ดัก invalid data - ต้องมี VideoID/EntityID หรือ ReelID
		// รองรับทั้ง legacy (video_id) และ standard (entity_id) format
		if update.VideoID == "" && update.EntityID == "" && update.ReelID == "" {
			logger.Warn("Received progress update with empty video_id/entity_id and reel_id")
			return
		}

		// Convert to port type and call handler
		data := &ports.ProgressData{
			VideoID:         update.VideoID,
			VideoCode:       update.VideoCode,
			Status:          update.Status,
			Stage:           update.Stage,
			Progress:        update.Progress,
			Quality:         update.Quality,
			Message:         update.Message,
			Error:           update.Error,
			OutputPath:      update.OutputPath,
			AudioPath:       update.AudioPath,
			WorkerID:        update.WorkerID,
			SubtitleID:      update.SubtitleID,
			CurrentLanguage: update.CurrentLanguage,
			// Reel-specific fields
			ReelID:   update.ReelID,
			FileSize: update.FileSize,
		}

		// Extract output data from new worker format
		if update.Output != nil {
			data.Duration = update.Output.Duration
			data.DiskUsage = update.Output.DiskUsage
			data.QualitySizes = update.Output.QualitySizes
		}

		handler(data)
	}

	// Register handler
	s.subscriber.OnProgress(natsHandler)

	// Start subscriber if not already running
	if !s.subscriber.IsRunning() {
		return s.subscriber.Start()
	}

	return nil
}

// Unsubscribe หยุด listen
func (s *NATSProgressSubscriber) Unsubscribe() error {
	if s.cancel != nil {
		s.cancel()
	}
	return s.subscriber.Stop()
}
