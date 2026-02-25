package models

import "time"

// SEOArticleJob - Job สำหรับสร้าง SEO Article
// ส่งมาจาก api.subth.com ผ่าน NATS JetStream
type SEOArticleJob struct {
	VideoID     string `json:"video_id"`
	VideoCode   string `json:"video_code"`
	Priority    int    `json:"priority"`     // 1=urgent, 2=normal, 3=backfill
	GenerateTTS bool   `json:"generate_tts"` // ต้องการ TTS หรือไม่
	CreatedAt   int64  `json:"created_at"`
}

// NewSEOArticleJob สร้าง job ใหม่
func NewSEOArticleJob(videoID, videoCode string, generateTTS bool) *SEOArticleJob {
	return &SEOArticleJob{
		VideoID:     videoID,
		VideoCode:   videoCode,
		Priority:    2, // normal
		GenerateTTS: generateTTS,
		CreatedAt:   time.Now().Unix(),
	}
}

// DLQJob - Job ที่ถูกย้ายไป Dead Letter Queue
type DLQJob struct {
	OriginalJob SEOArticleJob `json:"original_job"`
	Error       string        `json:"error"`
	Attempts    int           `json:"attempts"`
	WorkerID    string        `json:"worker_id"`
	FailedAt    int64         `json:"failed_at"`
	Stage       string        `json:"stage"` // fetch, ai, tts, embedding, publish
}

// ProgressUpdate - ส่ง progress กลับไปที่ Admin UI
type ProgressUpdate struct {
	VideoID   string `json:"video_id"`
	Stage     string `json:"stage"`
	Progress  int    `json:"progress"` // 0-100
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

func NewProgressUpdate(videoID, stage string, progress int) *ProgressUpdate {
	return &ProgressUpdate{
		VideoID:   videoID,
		Stage:     stage,
		Progress:  progress,
		Timestamp: time.Now().Unix(),
	}
}
