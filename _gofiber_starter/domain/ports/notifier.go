package ports

import "context"

// ═══════════════════════════════════════════════════════════════════════════════
// Notifier Port - สำหรับส่งการแจ้งเตือน (Telegram, Email, etc.)
// ═══════════════════════════════════════════════════════════════════════════════

// DLQNotification - ข้อมูลสำหรับแจ้งเตือน DLQ
type DLQNotification struct {
	VideoID    string
	VideoCode  string
	Title      string
	Error      string
	Attempts   int
	WorkerID   string
	Stage      string // downloading, transcoding, uploading
	FailedAt   string
}

// NotifierPort - Interface สำหรับส่งการแจ้งเตือน
type NotifierPort interface {
	// SendDLQAlert ส่งแจ้งเตือนเมื่อวิดีโอเข้า DLQ
	SendDLQAlert(ctx context.Context, notification *DLQNotification) error

	// SendTranscodeCompleteAlert ส่งแจ้งเตือนเมื่อ transcode สำเร็จ
	SendTranscodeCompleteAlert(ctx context.Context, videoCode, title string) error

	// SendTranscodeFailAlert ส่งแจ้งเตือนเมื่อ transcode ล้มเหลว
	SendTranscodeFailAlert(ctx context.Context, videoCode, title, errorMsg string) error

	// SendWorkerOfflineAlert ส่งแจ้งเตือนเมื่อ worker offline
	SendWorkerOfflineAlert(ctx context.Context, workerID, hostname string, lastSeen string) error

	// IsEnabled ตรวจสอบว่าเปิดใช้งานการแจ้งเตือนหรือไม่
	IsEnabled() bool
}
