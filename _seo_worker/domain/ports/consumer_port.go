package ports

import (
	"context"

	"seo-worker/domain/models"
)

// JobHandler - Function signature สำหรับ handle job
type JobHandler func(ctx context.Context, job *models.SEOArticleJob) error

// ConsumerPort - Interface สำหรับ NATS Consumer
type ConsumerPort interface {
	// Start เริ่ม consume messages (blocking)
	Start(ctx context.Context) error

	// Stop หยุด consumer (graceful)
	Stop()

	// SetHandler กำหนด handler function
	SetHandler(handler JobHandler)

	// IsRunning ตรวจสอบว่ากำลังทำงานอยู่หรือไม่
	IsRunning() bool

	// IsPaused ตรวจสอบว่า paused อยู่หรือไม่
	IsPaused() bool

	// Pause หยุดรับ job ชั่วคราว
	Pause()

	// Resume กลับมารับ job ต่อ
	Resume()
}
