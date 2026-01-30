package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"gofiber-template/domain/ports"
	"gofiber-template/pkg/logger"
)

// S3Storage implements StoragePort สำหรับ S3-Compatible Storage (MinIO / Cloudflare R2)
type S3Storage struct {
	client    *minio.Client
	bucket    string
	publicURL string // URL สำหรับเข้าถึงไฟล์ public (ถ้ามี)
	endpoint  string
	useSSL    bool
}

type S3StorageConfig struct {
	Endpoint  string // minio:9000 หรือ xxx.r2.cloudflarestorage.com
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
	Region    string
	PublicURL string // URL สำหรับเข้าถึงไฟล์ public (optional)
}

// NewS3Storage สร้าง S3Storage instance
func NewS3Storage(config S3StorageConfig) (ports.StoragePort, error) {
	// สร้าง custom transport สำหรับ connection pool ที่ใหญ่ขึ้น
	// รองรับ cache warming หลาย concurrent requests
	transport := &http.Transport{
		MaxIdleConns:        100, // idle connections ทั้งหมด
		MaxIdleConnsPerHost: 50,  // idle connections ต่อ host (e2)
		MaxConnsPerHost:     100, // connections ทั้งหมดต่อ host
	}

	// สร้าง MinIO client
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure:    config.UseSSL,
		Region:    config.Region,
		Transport: transport,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	// ตรวจสอบ connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// ตรวจสอบว่า bucket มีอยู่หรือไม่
	exists, err := client.BucketExists(ctx, config.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket: %w", err)
	}

	// สร้าง bucket ถ้ายังไม่มี
	if !exists {
		err = client.MakeBucket(ctx, config.Bucket, minio.MakeBucketOptions{
			Region: config.Region,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
		logger.Info("S3 bucket created", "bucket", config.Bucket)
	}

	logger.Info("S3 storage initialized",
		"endpoint", config.Endpoint,
		"bucket", config.Bucket,
		"ssl", config.UseSSL,
	)

	return &S3Storage{
		client:    client,
		bucket:    config.Bucket,
		publicURL: strings.TrimSuffix(config.PublicURL, "/"),
		endpoint:  config.Endpoint,
		useSSL:    config.UseSSL,
	}, nil
}

// UploadFile อัปโหลดไฟล์ไปยัง S3
func (s *S3Storage) UploadFile(file io.Reader, path string, contentType string) (string, error) {
	ctx := context.Background()

	// Normalize path
	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, "\\", "/")

	// อัปโหลดไฟล์
	// ใช้ -1 สำหรับ size เพื่อให้ MinIO อ่านจนจบ (streaming)
	_, err := s.client.PutObject(ctx, s.bucket, path, file, -1, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	logger.Debug("File uploaded to S3", "path", path, "content_type", contentType)

	return s.GetFileURL(path), nil
}

// DeleteFile ลบไฟล์จาก S3
func (s *S3Storage) DeleteFile(path string) error {
	ctx := context.Background()

	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, "\\", "/")

	err := s.client.RemoveObject(ctx, s.bucket, path, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	logger.Debug("File deleted from S3", "path", path)
	return nil
}

// DeleteFolder ลบไฟล์ทั้งหมดใน folder (prefix)
// เช่น DeleteFolder("hls/abc123/") จะลบทุกไฟล์ที่ขึ้นต้นด้วย "hls/abc123/"
func (s *S3Storage) DeleteFolder(prefix string) error {
	ctx := context.Background()

	prefix = strings.TrimPrefix(prefix, "/")
	prefix = strings.ReplaceAll(prefix, "\\", "/")
	// ต้องลงท้ายด้วย / เพื่อให้เป็น folder prefix
	if !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	// List all objects with prefix
	objectsCh := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	// Collect objects to delete
	var objectsToDelete []minio.ObjectInfo
	for obj := range objectsCh {
		if obj.Err != nil {
			return fmt.Errorf("failed to list objects: %w", obj.Err)
		}
		objectsToDelete = append(objectsToDelete, obj)
	}

	if len(objectsToDelete) == 0 {
		logger.Debug("No objects found to delete", "prefix", prefix)
		return nil
	}

	// Delete objects
	deletedCount := 0
	for _, obj := range objectsToDelete {
		err := s.client.RemoveObject(ctx, s.bucket, obj.Key, minio.RemoveObjectOptions{})
		if err != nil {
			logger.Warn("Failed to delete object", "key", obj.Key, "error", err)
			// Continue deleting other objects
		} else {
			deletedCount++
		}
	}

	logger.Info("Folder deleted from S3",
		"prefix", prefix,
		"total_objects", len(objectsToDelete),
		"deleted", deletedCount,
	)

	return nil
}

// GetFileURL สร้าง URL สำหรับเข้าถึงไฟล์
func (s *S3Storage) GetFileURL(path string) string {
	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, "\\", "/")

	// ถ้ามี public URL ให้ใช้
	if s.publicURL != "" {
		return s.publicURL + "/" + path
	}

	// สร้าง URL จาก endpoint
	scheme := "http"
	if s.useSSL {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s/%s/%s", scheme, s.endpoint, s.bucket, path)
}

// GetFileContent อ่านไฟล์จาก S3 และ return io.ReadCloser
func (s *S3Storage) GetFileContent(path string) (io.ReadCloser, string, error) {
	ctx := context.Background()

	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, "\\", "/")

	obj, err := s.client.GetObject(ctx, s.bucket, path, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get object: %w", err)
	}

	// Get content type from object info
	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, "", fmt.Errorf("failed to stat object: %w", err)
	}

	return obj, info.ContentType, nil
}

// GetFileRange อ่านไฟล์บางส่วนจาก S3 (byte range request)
// สำหรับ HLS byte-range segments
func (s *S3Storage) GetFileRange(path string, start, end int64) (io.ReadCloser, int64, error) {
	ctx := context.Background()

	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, "\\", "/")

	// Get file info first to get total size
	info, err := s.client.StatObject(ctx, s.bucket, path, minio.StatObjectOptions{})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to stat object: %w", err)
	}

	totalSize := info.Size

	// Calculate actual end position
	actualEnd := end
	if end < 0 || end >= totalSize {
		actualEnd = totalSize - 1
	}

	// Create GetObjectOptions with Range
	opts := minio.GetObjectOptions{}
	err = opts.SetRange(start, actualEnd)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to set range: %w", err)
	}

	obj, err := s.client.GetObject(ctx, s.bucket, path, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get object range: %w", err)
	}

	return obj, totalSize, nil
}

// GetProviderName return ชื่อ provider
func (s *S3Storage) GetProviderName() string {
	return "s3"
}

// ═══════════════════════════════════════════════════════════════════════════════
// Multipart Upload (Direct Upload) - สำหรับไฟล์ใหญ่ที่อัปโหลดตรงจาก Frontend
// ═══════════════════════════════════════════════════════════════════════════════

// CreateMultipartUpload สร้าง multipart upload session
func (s *S3Storage) CreateMultipartUpload(path string, contentType string) (string, error) {
	ctx := context.Background()

	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, "\\", "/")

	// ใช้ Core client สำหรับ low-level multipart operations
	core := minio.Core{Client: s.client}
	uploadID, err := core.NewMultipartUpload(ctx, s.bucket, path, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create multipart upload: %w", err)
	}

	logger.Debug("Multipart upload created", "path", path, "upload_id", uploadID)
	return uploadID, nil
}

// GetPresignedPartURL สร้าง presigned URL สำหรับ upload แต่ละ part
func (s *S3Storage) GetPresignedPartURL(path string, uploadID string, partNumber int, expiry time.Duration) (string, error) {
	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, "\\", "/")

	// สร้าง presigned URL สำหรับ PUT part
	// ใช้ Presign method พร้อม query params สำหรับ multipart
	reqParams := make(url.Values)
	reqParams.Set("partNumber", fmt.Sprintf("%d", partNumber))
	reqParams.Set("uploadId", uploadID)

	presignedURL, err := s.client.Presign(context.Background(), "PUT", s.bucket, path, expiry, reqParams)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	logger.Debug("Presigned part URL generated",
		"path", path,
		"part_number", partNumber,
		"expiry", expiry,
	)

	return presignedURL.String(), nil
}

// CompleteMultipartUpload รวม parts ทั้งหมดเป็นไฟล์เดียว
func (s *S3Storage) CompleteMultipartUpload(path string, uploadID string, parts []ports.CompletedPart) error {
	ctx := context.Background()

	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, "\\", "/")

	// แปลง ports.CompletedPart เป็น minio.CompletePart
	completeParts := make([]minio.CompletePart, len(parts))
	for i, p := range parts {
		completeParts[i] = minio.CompletePart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
		}
	}

	// ใช้ Core client สำหรับ low-level multipart operations
	core := minio.Core{Client: s.client}
	_, err := core.CompleteMultipartUpload(ctx, s.bucket, path, uploadID, completeParts, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	logger.Info("Multipart upload completed",
		"path", path,
		"upload_id", uploadID,
		"total_parts", len(parts),
	)

	return nil
}

// AbortMultipartUpload ยกเลิก multipart upload ที่ค้าง
func (s *S3Storage) AbortMultipartUpload(path string, uploadID string) error {
	ctx := context.Background()

	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, "\\", "/")

	// ใช้ Core client สำหรับ low-level multipart operations
	core := minio.Core{Client: s.client}
	err := core.AbortMultipartUpload(ctx, s.bucket, path, uploadID)
	if err != nil {
		return fmt.Errorf("failed to abort multipart upload: %w", err)
	}

	logger.Info("Multipart upload aborted", "path", path, "upload_id", uploadID)
	return nil
}





