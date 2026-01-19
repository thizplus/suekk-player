package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ErrorRecord บันทึกข้อผิดพลาดแต่ละครั้ง
type ErrorRecord struct {
	Attempt   int    `json:"attempt"`
	Error     string `json:"error"`
	WorkerID  string `json:"worker_id"`
	Stage     string `json:"stage"` // download, transcode, upload
	Timestamp string `json:"timestamp"`
}

// ErrorHistory เก็บประวัติ errors ทั้งหมด
type ErrorHistory []ErrorRecord

// Scan implements sql.Scanner for ErrorHistory
func (e *ErrorHistory) Scan(value interface{}) error {
	if value == nil {
		*e = ErrorHistory{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}

	return json.Unmarshal(bytes, e)
}

// Value implements driver.Valuer for ErrorHistory
func (e ErrorHistory) Value() (driver.Value, error) {
	if e == nil {
		return "[]", nil
	}
	return json.Marshal(e)
}

// QualitySizes เก็บขนาดไฟล์แยกตาม quality (bytes)
// Example: {"1080p": 2684354560, "720p": 1342177280, "480p": 671088640}
type QualitySizes map[string]int64

// Scan implements sql.Scanner for QualitySizes
func (q *QualitySizes) Scan(value interface{}) error {
	if value == nil {
		*q = QualitySizes{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}

	return json.Unmarshal(bytes, q)
}

// Value implements driver.Valuer for QualitySizes
func (q QualitySizes) Value() (driver.Value, error) {
	if q == nil {
		return "{}", nil
	}
	return json.Marshal(q)
}

// VideoStatus สถานะของ video
type VideoStatus string

const (
	VideoStatusPending    VideoStatus = "pending"
	VideoStatusQueued     VideoStatus = "queued"      // รอคิว - job อยู่ใน NATS queue
	VideoStatusProcessing VideoStatus = "processing"
	VideoStatusReady      VideoStatus = "ready"
	VideoStatusFailed     VideoStatus = "failed"
	VideoStatusDeadLetter VideoStatus = "dead_letter" // Poison pill - ต้องตรวจสอบ manual
)


type Video struct {
	ID           uuid.UUID   `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	UserID       uuid.UUID   `gorm:"type:uuid;not null"`
	CategoryID   *uuid.UUID  `gorm:"type:uuid"` // nullable
	Code         string      `gorm:"size:50;uniqueIndex;not null"`
	Title        string      `gorm:"size:255;not null"`
	Description  string      `gorm:"type:text"`
	Duration     int         `gorm:"default:0"` // วินาที
	Quality      string      `gorm:"size:20"`   // 720p, 1080p, 4K
	OriginalPath string      `gorm:"type:text"`
	HLSPath      string      `gorm:"type:text;column:hls_path"` // path to .m3u8
	HLSPathH264  string      `gorm:"type:text;column:hls_path_h264"` // H.264 fallback path
	ThumbnailURL string      `gorm:"type:text"`
	Status       VideoStatus `gorm:"size:20;default:'pending'"`
	Views        int64       `gorm:"default:0"`

	// Storage Tracking (bytes)
	OriginalSize int64        `gorm:"default:0"`               // ขนาดไฟล์ต้นฉบับ (ถูกลบหลัง transcode)
	HLSSize      int64        `gorm:"default:0"`               // ขนาด HLS output ทั้งหมด
	DiskUsage    int64        `gorm:"default:0"`               // = HLSSize (เพราะ original ถูกลบ)
	QualitySizes QualitySizes `gorm:"type:jsonb;default:'{}'"` // ขนาดแยกตาม quality {"1080p": bytes, "720p": bytes}

	// Retry tracking for failure handling
	RetryCount          int          `gorm:"default:0"`                      // จำนวนครั้งที่ retry
	LastError           string       `gorm:"type:text"`                      // error message ล่าสุด
	ErrorHistory        ErrorHistory `gorm:"type:jsonb;default:'[]'"`        // ประวัติ errors ทั้งหมด
	ProcessingStartedAt *time.Time   `gorm:"type:timestamptz"`               // เวลาเริ่ม processing (สำหรับ stuck detection)

	// Audio fields (สำหรับ subtitle worker - extracted during transcode)
	AudioPath        string `gorm:"type:text"` // S3 path to extracted audio (WAV)
	DetectedLanguage string `gorm:"size:10"`   // Detected language code (ja, en, etc.) - nullable

	CreatedAt time.Time
	UpdatedAt time.Time

	// Relations
	User      *User       `gorm:"foreignKey:UserID"`
	Category  *Category   `gorm:"foreignKey:CategoryID"`
	Subtitles []*Subtitle `gorm:"foreignKey:VideoID"`
}

func (Video) TableName() string {
	return "videos"
}

// IsReady ตรวจสอบว่า video พร้อม stream หรือยัง
func (v *Video) IsReady() bool {
	return v.Status == VideoStatusReady
}

// IsPending ตรวจสอบว่า video รอ transcode อยู่
func (v *Video) IsPending() bool {
	return v.Status == VideoStatusPending
}

// IsQueued ตรวจสอบว่า video อยู่ในคิวรอ worker
func (v *Video) IsQueued() bool {
	return v.Status == VideoStatusQueued
}

// IsProcessing ตรวจสอบว่า video กำลัง transcode
func (v *Video) IsProcessing() bool {
	return v.Status == VideoStatusProcessing
}

// IsFailed ตรวจสอบว่า video transcode ไม่สำเร็จ
func (v *Video) IsFailed() bool {
	return v.Status == VideoStatusFailed
}

// IsDeadLetter ตรวจสอบว่า video อยู่ใน DLQ (ต้องตรวจสอบ manual)
func (v *Video) IsDeadLetter() bool {
	return v.Status == VideoStatusDeadLetter
}

// CanRetry ตรวจสอบว่า video สามารถ retry ได้หรือไม่ (retry < 3)
func (v *Video) CanRetry() bool {
	return v.RetryCount < 3
}

// IncrementRetry เพิ่ม retry count และบันทึก error
func (v *Video) IncrementRetry(errMsg string) {
	v.RetryCount++
	v.LastError = errMsg
}

// AppendErrorHistory เพิ่ม error record ลงในประวัติ
func (v *Video) AppendErrorHistory(record ErrorRecord) {
	if v.ErrorHistory == nil {
		v.ErrorHistory = ErrorHistory{}
	}
	v.ErrorHistory = append(v.ErrorHistory, record)
	v.LastError = record.Error
}

// GetDiskUsageMB แปลง disk usage เป็น MB
func (v *Video) GetDiskUsageMB() float64 {
	return float64(v.DiskUsage) / 1024 / 1024
}

// GetDiskUsageGB แปลง disk usage เป็น GB
func (v *Video) GetDiskUsageGB() float64 {
	return float64(v.DiskUsage) / 1024 / 1024 / 1024
}

// GetOriginalSizeGB แปลง original size เป็น GB
func (v *Video) GetOriginalSizeGB() float64 {
	return float64(v.OriginalSize) / 1024 / 1024 / 1024
}

// GetHLSSizeGB แปลง HLS size เป็น GB
func (v *Video) GetHLSSizeGB() float64 {
	return float64(v.HLSSize) / 1024 / 1024 / 1024
}

// HasH264Fallback ตรวจสอบว่ามี H.264 fallback หรือไม่
func (v *Video) HasH264Fallback() bool {
	return v.HLSPathH264 != ""
}

// GetQualityCount จำนวน quality ที่มี
func (v *Video) GetQualityCount() int {
	if v.QualitySizes == nil {
		return 0
	}
	return len(v.QualitySizes)
}

// GetQualities รายชื่อ qualities ทั้งหมด
func (v *Video) GetQualities() []string {
	if v.QualitySizes == nil {
		return []string{}
	}
	qualities := make([]string, 0, len(v.QualitySizes))
	for q := range v.QualitySizes {
		qualities = append(qualities, q)
	}
	return qualities
}

// HasAudioExtracted ตรวจสอบว่ามี audio ที่ตัดไว้หรือไม่
func (v *Video) HasAudioExtracted() bool {
	return v.AudioPath != ""
}
