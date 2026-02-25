package ports

import (
	"context"

	"seo-worker/domain/models"
)

// MessengerPort - Interface สำหรับส่ง Progress Updates
type MessengerPort interface {
	// SendProgress ส่ง progress update ไปที่ NATS
	// Admin UI จะ subscribe เพื่อแสดง progress bar
	SendProgress(ctx context.Context, update *models.ProgressUpdate) error

	// SendCompleted ส่งแจ้งว่า job เสร็จแล้ว
	SendCompleted(ctx context.Context, videoID string) error

	// SendFailed ส่งแจ้งว่า job failed
	SendFailed(ctx context.Context, videoID string, err error) error
}

// Progress stages
const (
	StageFetching   = "fetching_data"
	StageDataFetched = "data_fetched"
	StageAI         = "ai_processing"
	StageAIComplete = "ai_completed"
	StageTTSEmbed   = "tts_embedding"
	StageTTSEmbedComplete = "tts_embedding_completed"
	StagePublishing = "publishing"
	StageCompleted  = "completed"
	StageFailed     = "failed"
)
