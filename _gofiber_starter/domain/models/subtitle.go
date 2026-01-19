package models

import (
	"time"

	"github.com/google/uuid"
)

// SubtitleStatus สถานะของ subtitle
type SubtitleStatus string

const (
	SubtitleStatusPending     SubtitleStatus = "pending"     // รอ process
	SubtitleStatusQueued      SubtitleStatus = "queued"      // อยู่ใน queue รอ worker
	SubtitleStatusDetecting   SubtitleStatus = "detecting"   // กำลัง detect language
	SubtitleStatusDetected    SubtitleStatus = "detected"    // detect เสร็จ รอสร้าง SRT
	SubtitleStatusProcessing  SubtitleStatus = "processing"  // กำลังสร้าง SRT
	SubtitleStatusReady       SubtitleStatus = "ready"       // พร้อมใช้งาน
	SubtitleStatusTranslating SubtitleStatus = "translating" // กำลังแปล
	SubtitleStatusFailed      SubtitleStatus = "failed"      // ล้มเหลว
)

// SubtitleType ประเภทของ subtitle
type SubtitleType string

const (
	SubtitleTypeOriginal   SubtitleType = "original"   // ภาษาต้นฉบับ (from Whisper)
	SubtitleTypeTranslated SubtitleType = "translated" // แปลจากภาษาอื่น
)

// Subtitle แต่ละ record = 1 ภาษา ของ 1 video
// 1 Video สามารถมีหลาย Subtitles (หลายภาษา)
type Subtitle struct {
	ID      uuid.UUID `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	VideoID uuid.UUID `gorm:"type:uuid;not null;index"`

	// Language info
	Language   string       `gorm:"size:10;not null"` // ja, en, th, zh, ko, ru
	Type       SubtitleType `gorm:"size:20;not null"` // original or translated
	Confidence float64      `gorm:"default:0"`        // 0-1 (สำหรับ original จาก detection)

	// Source info (สำหรับ translated)
	SourceLanguage string `gorm:"size:10"` // ภาษาต้นฉบับที่แปลมา (nullable)

	// SRT Path
	SRTPath string `gorm:"type:text"` // S3 path: subtitles/{video_code}/{language}.srt

	// Status
	Status SubtitleStatus `gorm:"size:20;default:'pending'"`
	Error  string         `gorm:"type:text"`

	// Stuck Detection: บันทึกเวลาที่ worker เริ่มทำจริง
	ProcessingStartedAt *time.Time `gorm:"index"`

	CreatedAt time.Time
	UpdatedAt time.Time

	// Relations
	Video *Video `gorm:"foreignKey:VideoID"`
}

func (Subtitle) TableName() string {
	return "subtitles"
}

// IsReady ตรวจสอบว่า subtitle พร้อมใช้งานหรือไม่
func (s *Subtitle) IsReady() bool {
	return s.Status == SubtitleStatusReady && s.SRTPath != ""
}

// IsOriginal ตรวจสอบว่าเป็น original subtitle หรือไม่
func (s *Subtitle) IsOriginal() bool {
	return s.Type == SubtitleTypeOriginal
}

// IsFailed ตรวจสอบว่า subtitle ล้มเหลวหรือไม่
func (s *Subtitle) IsFailed() bool {
	return s.Status == SubtitleStatusFailed
}

// IsProcessing ตรวจสอบว่ากำลัง process อยู่หรือไม่
func (s *Subtitle) IsProcessing() bool {
	return s.Status == SubtitleStatusDetecting ||
		s.Status == SubtitleStatusProcessing ||
		s.Status == SubtitleStatusTranslating
}

// IsQueued ตรวจสอบว่าอยู่ใน queue รอ worker หรือไม่
func (s *Subtitle) IsQueued() bool {
	return s.Status == SubtitleStatusQueued
}

// IsInProgress ตรวจสอบว่ากำลังทำงานอยู่ (queued หรือ processing)
func (s *Subtitle) IsInProgress() bool {
	return s.IsQueued() || s.IsProcessing()
}
