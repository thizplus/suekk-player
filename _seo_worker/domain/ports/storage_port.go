package ports

import (
	"context"
	"io"
	"time"
)

// StoragePort - Interface สำหรับ Object Storage (R2/S3)
type StoragePort interface {
	// Upload อัพโหลดไฟล์ไปยัง storage
	Upload(ctx context.Context, path string, data []byte, contentType string) error

	// UploadReader อัพโหลดจาก reader
	UploadReader(ctx context.Context, path string, reader io.Reader, contentType string) error

	// GetFileContent ดึงเนื้อหาไฟล์
	GetFileContent(path string) (io.ReadCloser, int64, error)

	// GetPublicURL สร้าง public URL สำหรับไฟล์
	GetPublicURL(path string) string

	// Delete ลบไฟล์
	Delete(ctx context.Context, path string) error

	// Exists ตรวจสอบว่าไฟล์มีอยู่หรือไม่
	Exists(ctx context.Context, path string) (bool, error)

	// ListFiles ดึงรายการไฟล์ใน path (prefix)
	ListFiles(prefix string) ([]string, error)

	// GetPresignedDownloadURL สร้าง presigned URL สำหรับดาวน์โหลดไฟล์ (สำหรับ private bucket)
	GetPresignedDownloadURL(path string, expiry time.Duration) (string, error)
}
