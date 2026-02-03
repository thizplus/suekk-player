package dto

import (
	"time"

	"github.com/google/uuid"
)

// === Queue Stats ===

// QueueStatsResponse สถิติ queue ทั้งหมด
type QueueStatsResponse struct {
	Transcode TranscodeStats `json:"transcode"`
	Subtitle  SubtitleStats  `json:"subtitle"`
	WarmCache WarmCacheStats `json:"warmCache"`
	Reel      ReelStats      `json:"reel"`
}

// TranscodeStats สถิติ transcode queue
type TranscodeStats struct {
	Pending    int64 `json:"pending"`
	Queued     int64 `json:"queued"`
	Processing int64 `json:"processing"`
	Failed     int64 `json:"failed"`
	DeadLetter int64 `json:"deadLetter"`
}

// SubtitleStats สถิติ subtitle queue
type SubtitleStats struct {
	Queued     int64 `json:"queued"`     // stuck - NATS job หาย
	Processing int64 `json:"processing"` // processing + translating + detecting
	Failed     int64 `json:"failed"`
}

// WarmCacheStats สถิติ warm cache queue
type WarmCacheStats struct {
	NotCached int64 `json:"notCached"` // pending
	Warming   int64 `json:"warming"`
	Cached    int64 `json:"cached"`
	Failed    int64 `json:"failed"`
}

// ReelStats สถิติ reel export queue
type ReelStats struct {
	Draft     int64 `json:"draft"`     // กำลังแก้ไข
	Exporting int64 `json:"exporting"` // กำลัง export
	Ready     int64 `json:"ready"`     // export สำเร็จ
	Failed    int64 `json:"failed"`    // export ล้มเหลว
}

// === Transcode Queue Items ===

// TranscodeQueueItem รายการ video ใน transcode queue
type TranscodeQueueItem struct {
	ID         uuid.UUID `json:"id"`
	Code       string    `json:"code"`
	Title      string    `json:"title"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
	RetryCount int       `json:"retryCount"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// TranscodeQueueListResponse รายการ transcode failed
type TranscodeQueueListResponse struct {
	Items []TranscodeQueueItem `json:"items"`
}

// === Subtitle Queue Items ===

// SubtitleQueueItem รายการ subtitle ใน queue
type SubtitleQueueItem struct {
	ID         uuid.UUID `json:"id"`
	VideoID    uuid.UUID `json:"videoId"`
	VideoCode  string    `json:"videoCode"`
	VideoTitle string    `json:"videoTitle"`
	Language   string    `json:"language"`
	Type       string    `json:"type"` // original | translated
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// SubtitleQueueListResponse รายการ subtitle stuck/failed
type SubtitleQueueListResponse struct {
	Items []SubtitleQueueItem `json:"items"`
}

// === Warm Cache Queue Items ===

// WarmCacheQueueItem รายการ video ใน warm cache queue
type WarmCacheQueueItem struct {
	ID              uuid.UUID `json:"id"`
	Code            string    `json:"code"`
	Title           string    `json:"title"`
	CacheStatus     string    `json:"cacheStatus"`
	CachePercentage float64   `json:"cachePercentage"`
	Qualities       []string  `json:"qualities"`
	Error           string    `json:"error,omitempty"`
	LastWarmedAt    *string   `json:"lastWarmedAt,omitempty"`
}

// WarmCacheQueueListResponse รายการ warm cache pending/failed
type WarmCacheQueueListResponse struct {
	Items []WarmCacheQueueItem `json:"items"`
}

// === Retry Responses ===

// RetryResponse response หลัง retry
type RetryResponse struct {
	TotalFound   int      `json:"totalFound"`
	TotalRetried int      `json:"totalRetried"`
	Skipped      int      `json:"skipped"`
	Message      string   `json:"message"`
	Errors       []string `json:"errors,omitempty"`
}

// WarmCacheResponse response หลัง warm cache
type WarmCacheResponse struct {
	VideoID string `json:"videoId"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WarmAllResponse response หลัง warm all
type WarmAllResponse struct {
	TotalFound  int    `json:"totalFound"`
	TotalQueued int    `json:"totalQueued"`
	Message     string `json:"message"`
}

// ClearResponse response หลัง clear queue
type ClearResponse struct {
	TotalFound      int    `json:"totalFound"`
	TotalDeleted    int    `json:"totalDeleted"`
	Skipped         int    `json:"skipped"`
	NATSJobsPurged  int    `json:"natsJobsPurged"`  // จำนวน jobs ที่ถูก purge จาก NATS
	Message         string `json:"message"`
}

// QueueMissingResponse response หลัง queue missing subtitles
type QueueMissingResponse struct {
	TotalVideos    int    `json:"totalVideos"`    // จำนวน video ทั้งหมดที่ ready
	TotalMissing   int    `json:"totalMissing"`   // จำนวน video ที่ยังไม่มี subtitle
	TotalQueued    int    `json:"totalQueued"`    // จำนวน video ที่ queue สำเร็จ
	Skipped        int    `json:"skipped"`        // จำนวนที่ skip (ไม่มี audio, etc.)
	Message        string `json:"message"`
}
