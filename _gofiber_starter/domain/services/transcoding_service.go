package services

import (
	"context"

	"github.com/google/uuid"
)

// TranscodingService interface สำหรับจัดการ video transcoding
type TranscodingService interface {
	// QueueTranscoding ส่งวิดีโอเข้า transcoding queue
	QueueTranscoding(ctx context.Context, videoID uuid.UUID) error

	// ProcessTranscoding ทำ transcoding (เรียกจาก worker)
	ProcessTranscoding(ctx context.Context, videoID uuid.UUID) error

	// GetQueueStatus ดึงสถานะของ queue
	GetQueueStatus() *TranscodingQueueStatus

	// GetStats ดึงสถิติจำนวนวิดีโอตาม status
	GetStats(ctx context.Context) (*TranscodingStats, error)

	// RecoverStuckJobs กู้คืน jobs ที่ค้างอยู่ (status=processing) ตอน server restart
	RecoverStuckJobs(ctx context.Context) (int, error)
}

// TranscodingQueueStatus สถานะของ transcoding queue
type TranscodingQueueStatus struct {
	QueueSize      int  `json:"queueSize"`
	WorkersRunning bool `json:"workersRunning"`
}

// TranscodingStats สถิติจำนวนวิดีโอแต่ละสถานะ
type TranscodingStats struct {
	Pending    int64 `json:"pending"`
	Queued     int64 `json:"queued"`     // รอคิว - job อยู่ใน NATS queue
	Processing int64 `json:"processing"`
	Completed  int64 `json:"completed"`
	Failed     int64 `json:"failed"`
}
