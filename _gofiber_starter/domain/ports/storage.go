package ports

import (
	"io"
	"time"
)

// StoragePort คือ interface หลักสำหรับ storage
// ทำให้เปลี่ยน storage provider ได้ง่าย (Local, Bunny, S3, etc.)
type StoragePort interface {
	// UploadFile อัปโหลดไฟล์ไปยัง storage
	// path: เส้นทางที่จะเก็บไฟล์ (เช่น "videos/uuid/file.mp4")
	// contentType: MIME type ของไฟล์
	// return: URL ที่เข้าถึงไฟล์ได้
	UploadFile(file io.Reader, path string, contentType string) (string, error)

	// DeleteFile ลบไฟล์จาก storage
	DeleteFile(path string) error

	// DeleteFolder ลบไฟล์ทั้งหมดใน folder (prefix)
	// สำหรับลบ HLS folder ที่มีหลายไฟล์
	DeleteFolder(prefix string) error

	// GetFileURL รับ URL สำหรับเข้าถึงไฟล์
	GetFileURL(path string) string

	// GetFileContent อ่านไฟล์จาก storage
	// return: io.ReadCloser, contentType, error
	GetFileContent(path string) (io.ReadCloser, string, error)

	// GetFileRange อ่านไฟล์บางส่วนจาก storage (สำหรับ byte range requests)
	// start: byte position เริ่มต้น
	// end: byte position สิ้นสุด (-1 = ถึงท้ายไฟล์)
	// return: io.ReadCloser, totalFileSize, error
	GetFileRange(path string, start, end int64) (io.ReadCloser, int64, error)

	// GetProviderName ชื่อ provider (local, bunny, s3)
	GetProviderName() string

	// ═══════════════════════════════════════════════════════════════════════════════
	// Multipart Upload (Direct Upload) - สำหรับไฟล์ใหญ่ที่อัปโหลดตรงจาก Frontend
	// ═══════════════════════════════════════════════════════════════════════════════

	// CreateMultipartUpload สร้าง multipart upload session
	// return: uploadId สำหรับใช้อ้างอิง
	CreateMultipartUpload(path string, contentType string) (uploadId string, err error)

	// GetPresignedPartURL สร้าง presigned URL สำหรับ upload แต่ละ part
	// partNumber: เริ่มจาก 1
	// expiry: ระยะเวลาที่ URL ใช้ได้
	GetPresignedPartURL(path string, uploadId string, partNumber int, expiry time.Duration) (url string, err error)

	// CompleteMultipartUpload รวม parts ทั้งหมดเป็นไฟล์เดียว
	CompleteMultipartUpload(path string, uploadId string, parts []CompletedPart) error

	// AbortMultipartUpload ยกเลิก multipart upload ที่ค้าง
	AbortMultipartUpload(path string, uploadId string) error
}

// CompletedPart ข้อมูล part ที่ upload สำเร็จ
type CompletedPart struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"etag"`
}
