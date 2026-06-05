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
			JobType:         update.JobType,
			SubtitleID:      update.SubtitleID,
			CurrentLanguage: update.CurrentLanguage,
			// Reel-specific fields
			ReelID:   update.ReelID,
			FileSize: update.FileSize,
		}

		// For subtitle entity_type: entity_id was mapped to SubtitleID in subscriber
		// VideoID may still be empty — try to get from EntityCode or RawOutput
		if update.EntityType == "subtitle" && data.VideoID == "" {
			// VideoCode is usable for WebSocket broadcast even without VideoID
			if update.EntityCode != "" {
				data.VideoCode = update.EntityCode
			}
		}

		// Extract output data from new worker format
		if update.Output != nil {
			data.Duration = update.Output.Duration
			data.DiskUsage = update.Output.DiskUsage
			data.QualitySizes = update.Output.QualitySizes
		}

		// Parse subtitle-specific output fields from RawOutput
		// Python workers send: {"output": {"language": "ja", "confidence": 0.95, "srt_path": "...", ...}}
		if update.RawOutput != nil {
			if lang, ok := update.RawOutput["language"].(string); ok {
				data.DetectedLanguage = lang
			}
			if conf, ok := update.RawOutput["confidence"].(float64); ok {
				data.Confidence = conf
			}
			if srtPath, ok := update.RawOutput["srt_path"].(string); ok {
				data.SRTPath = srtPath
			}
			if segments, ok := update.RawOutput["segments"].(float64); ok {
				data.Segments = int(segments)
			}

			// Translate worker sends {"translations": {"th": "subtitles/code/th.srt"}}
			// Extract first translation path as SRTPath
			if data.SRTPath == "" {
				if translations, ok := update.RawOutput["translations"].(map[string]interface{}); ok {
					for lang, path := range translations {
						if pathStr, ok := path.(string); ok {
							data.SRTPath = pathStr
							if data.CurrentLanguage == "" {
								data.CurrentLanguage = lang
							}
							break
						}
					}
				}
			}

			// For subtitle entity: try to get video_id from output
			if data.VideoID == "" {
				if vid, ok := update.RawOutput["video_id"].(string); ok {
					data.VideoID = vid
				}
			}
		}

		// For transcribe jobs: video_id is in the input, worker sends it back in progress
		// Ensure VideoID is populated for WebSocket broadcast routing
		if data.VideoID == "" && update.VideoID != "" {
			data.VideoID = update.VideoID
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
