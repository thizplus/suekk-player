package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gofiber-template/domain/ports"
)

// ErrNotSupported ใช้สำหรับ features ที่ไม่รองรับใน Local Storage
var ErrNotSupported = errors.New("this feature is not supported in local storage")

// LocalStorage implements StoragePort สำหรับเก็บไฟล์ใน local filesystem
type LocalStorage struct {
	basePath string // เส้นทางหลักที่เก็บไฟล์ (เช่น ./videos)
	baseURL  string // URL สำหรับเข้าถึงไฟล์ (เช่น http://localhost:8080/files)
}

type LocalStorageConfig struct {
	BasePath string // ./videos
	BaseURL  string // http://localhost:8080/files
}

// NewLocalStorage สร้าง LocalStorage instance
func NewLocalStorage(config LocalStorageConfig) (ports.StoragePort, error) {
	// สร้าง base directory ถ้ายังไม่มี
	if err := os.MkdirAll(config.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &LocalStorage{
		basePath: config.BasePath,
		baseURL:  strings.TrimSuffix(config.BaseURL, "/"),
	}, nil
}

// UploadFile อัปโหลดไฟล์ไปยัง local filesystem
func (l *LocalStorage) UploadFile(file io.Reader, path string, contentType string) (string, error) {
	// Normalize path separators
	path = strings.ReplaceAll(path, "\\", "/")

	// สร้าง full path
	fullPath := filepath.Join(l.basePath, path)

	// สร้าง directory ถ้ายังไม่มี
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// สร้างไฟล์
	dst, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Copy content
	if _, err := io.Copy(dst, file); err != nil {
		// ลบไฟล์ที่สร้างไม่สำเร็จ
		os.Remove(fullPath)
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Return URL
	return l.GetFileURL(path), nil
}

// DeleteFile ลบไฟล์จาก local filesystem
func (l *LocalStorage) DeleteFile(path string) error {
	path = strings.ReplaceAll(path, "\\", "/")
	fullPath := filepath.Join(l.basePath, path)

	// ตรวจสอบว่าไฟล์มีอยู่
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		// ไฟล์ไม่มีอยู่แล้ว ถือว่าสำเร็จ
		return nil
	}

	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// ลอง cleanup empty directories
	l.cleanupEmptyDirs(filepath.Dir(fullPath))

	return nil
}

// DeleteFolder ลบ folder ทั้งหมดจาก local filesystem
func (l *LocalStorage) DeleteFolder(prefix string) error {
	prefix = strings.ReplaceAll(prefix, "\\", "/")
	fullPath := filepath.Join(l.basePath, prefix)

	// ตรวจสอบว่า folder มีอยู่
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		// folder ไม่มีอยู่แล้ว ถือว่าสำเร็จ
		return nil
	}

	// ลบทั้ง folder
	if err := os.RemoveAll(fullPath); err != nil {
		return fmt.Errorf("failed to delete folder: %w", err)
	}

	// Cleanup empty parent directories
	l.cleanupEmptyDirs(filepath.Dir(fullPath))

	return nil
}

// GetFileURL สร้าง URL สำหรับเข้าถึงไฟล์
func (l *LocalStorage) GetFileURL(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return l.baseURL + path
}

// GetFileContent อ่านไฟล์จาก local filesystem
func (l *LocalStorage) GetFileContent(path string) (io.ReadCloser, string, error) {
	path = strings.ReplaceAll(path, "\\", "/")
	fullPath := filepath.Join(l.basePath, path)

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open file: %w", err)
	}

	// Detect content type from extension
	ext := strings.ToLower(filepath.Ext(path))
	contentType := "application/octet-stream"
	switch ext {
	case ".m3u8":
		contentType = "application/vnd.apple.mpegurl"
	case ".ts":
		contentType = "video/MP2T"
	case ".mp4":
		contentType = "video/mp4"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".webp":
		contentType = "image/webp"
	}

	return file, contentType, nil
}

// GetFileRange อ่านไฟล์บางส่วนจาก local filesystem (byte range request)
func (l *LocalStorage) GetFileRange(path string, start, end int64) (io.ReadCloser, int64, error) {
	path = strings.ReplaceAll(path, "\\", "/")
	fullPath := filepath.Join(l.basePath, path)

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open file: %w", err)
	}

	// Get file size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, fmt.Errorf("failed to stat file: %w", err)
	}
	totalSize := info.Size()

	// Seek to start position
	_, err = file.Seek(start, io.SeekStart)
	if err != nil {
		file.Close()
		return nil, 0, fmt.Errorf("failed to seek: %w", err)
	}

	// Calculate actual end and create limited reader
	actualEnd := end
	if end < 0 || end >= totalSize {
		actualEnd = totalSize - 1
	}
	length := actualEnd - start + 1

	// Wrap file with LimitedReader to only read the requested range
	limitedReader := &limitedReadCloser{
		reader: io.LimitReader(file, length),
		closer: file,
	}

	return limitedReader, totalSize, nil
}

// limitedReadCloser wraps a LimitReader with a closer
type limitedReadCloser struct {
	reader io.Reader
	closer io.Closer
}

func (l *limitedReadCloser) Read(p []byte) (n int, err error) {
	return l.reader.Read(p)
}

func (l *limitedReadCloser) Close() error {
	return l.closer.Close()
}

// GetProviderName return ชื่อ provider
func (l *LocalStorage) GetProviderName() string {
	return "local"
}

// cleanupEmptyDirs ลบ directory ว่างๆ ขึ้นไปจนถึง basePath
func (l *LocalStorage) cleanupEmptyDirs(dir string) {
	// ไม่ลบ basePath
	absBase, _ := filepath.Abs(l.basePath)
	absDir, _ := filepath.Abs(dir)

	for absDir != absBase && strings.HasPrefix(absDir, absBase) {
		entries, err := os.ReadDir(absDir)
		if err != nil || len(entries) > 0 {
			break
		}
		os.Remove(absDir)
		absDir = filepath.Dir(absDir)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Multipart Upload - ไม่รองรับใน Local Storage
// ═══════════════════════════════════════════════════════════════════════════════

// CreateMultipartUpload ไม่รองรับใน Local Storage
func (l *LocalStorage) CreateMultipartUpload(path string, contentType string) (string, error) {
	return "", ErrNotSupported
}

// GetPresignedPartURL ไม่รองรับใน Local Storage
func (l *LocalStorage) GetPresignedPartURL(path string, uploadID string, partNumber int, expiry time.Duration) (string, error) {
	return "", ErrNotSupported
}

// CompleteMultipartUpload ไม่รองรับใน Local Storage
func (l *LocalStorage) CompleteMultipartUpload(path string, uploadID string, parts []ports.CompletedPart) error {
	return ErrNotSupported
}

// AbortMultipartUpload ไม่รองรับใน Local Storage
func (l *LocalStorage) AbortMultipartUpload(path string, uploadID string) error {
	return ErrNotSupported
}

// GetPresignedDownloadURL ไม่รองรับใน Local Storage
func (l *LocalStorage) GetPresignedDownloadURL(path string, expiry time.Duration) (string, error) {
	return "", ErrNotSupported
}

// ListFiles list ไฟล์ทั้งหมดใน prefix (folder)
func (l *LocalStorage) ListFiles(prefix string) ([]string, error) {
	prefix = strings.ReplaceAll(prefix, "\\", "/")
	prefix = strings.TrimPrefix(prefix, "/")
	fullPath := filepath.Join(l.basePath, prefix)

	// ตรวจสอบว่า folder มีอยู่
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return []string{}, nil
	}

	var files []string
	err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip directories
		if info.IsDir() {
			return nil
		}
		// Get relative path from basePath
		relPath, err := filepath.Rel(l.basePath, path)
		if err != nil {
			return err
		}
		// Normalize to forward slashes
		relPath = strings.ReplaceAll(relPath, "\\", "/")
		files = append(files, relPath)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return files, nil
}
