package dto

import (
	"github.com/google/uuid"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Direct Upload DTOs - สำหรับ Presigned URL Upload ตรงจาก Frontend ไป S3
// ═══════════════════════════════════════════════════════════════════════════════

// === Requests ===

// InitDirectUploadRequest ข้อมูลสำหรับเริ่ม direct upload
type InitDirectUploadRequest struct {
	Filename    string `json:"filename" validate:"required,min=1,max=255"`
	Size        int64  `json:"size" validate:"required,min=1"`
	ContentType string `json:"contentType" validate:"required"`
	Title       string `json:"title" validate:"omitempty,max=255"`
}

// CompleteDirectUploadRequest ข้อมูลสำหรับ complete upload
type CompleteDirectUploadRequest struct {
	UploadID    string          `json:"uploadId" validate:"required"`
	VideoCode   string          `json:"videoCode" validate:"required"`
	Path        string          `json:"path" validate:"required"`
	Filename    string          `json:"filename" validate:"required"`
	Title       string          `json:"title" validate:"omitempty,max=255"`
	Description string          `json:"description" validate:"omitempty,max=1000"`
	Parts       []CompletedPart `json:"parts" validate:"required,min=1"`
}

// CompletedPart ข้อมูล part ที่ upload สำเร็จ
type CompletedPart struct {
	PartNumber int    `json:"partNumber" validate:"required,min=1"`
	ETag       string `json:"etag" validate:"required"`
}

// AbortDirectUploadRequest ข้อมูลสำหรับยกเลิก upload
type AbortDirectUploadRequest struct {
	UploadID string `json:"uploadId" validate:"required"`
	Path     string `json:"path" validate:"required"`
}

// === Responses ===

// InitDirectUploadResponse ผลลัพธ์จากการ init upload
// Note: ไม่มี VideoID เพราะ video จะถูกสร้างตอน CompleteUpload เท่านั้น
type InitDirectUploadResponse struct {
	UploadID      string        `json:"uploadId"`
	VideoCode     string        `json:"videoCode"`
	Path          string        `json:"path"`
	PartSize      int64         `json:"partSize"`      // ขนาดแต่ละ part (bytes)
	TotalParts    int           `json:"totalParts"`    // จำนวน parts ทั้งหมด
	PresignedURLs []PartURLInfo `json:"presignedUrls"` // URLs สำหรับแต่ละ part
	ExpiresIn     int           `json:"expiresIn"`     // ระยะเวลาที่ URLs ใช้ได้ (วินาที)
}

// PartURLInfo ข้อมูล URL สำหรับแต่ละ part
type PartURLInfo struct {
	PartNumber int    `json:"partNumber"`
	URL        string `json:"url"`
}

// CompleteDirectUploadResponse ผลลัพธ์จากการ complete upload
type CompleteDirectUploadResponse struct {
	VideoID      uuid.UUID `json:"videoId"`
	VideoCode    string    `json:"videoCode"`
	Title        string    `json:"title"`
	Status       string    `json:"status"`
	AutoEnqueued bool      `json:"autoEnqueued"`
}
