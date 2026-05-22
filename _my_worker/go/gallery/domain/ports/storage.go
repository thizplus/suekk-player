package ports

import (
	"context"
)

// DownloadCallback reports download progress
type DownloadCallback func(downloaded, total int64)

// StoragePort defines interface for file storage operations
type StoragePort interface {
	// Download downloads a file from remote storage to local path
	Download(ctx context.Context, remotePath, localPath string, onProgress DownloadCallback) error

	// Upload uploads a local file to remote storage
	Upload(ctx context.Context, remotePath, localPath string) error

	// UploadBytes uploads bytes directly to remote storage
	UploadBytes(ctx context.Context, remotePath string, data []byte, contentType string) error

	// Delete removes a file from remote storage
	Delete(ctx context.Context, remotePath string) error

	// GetPresignedURL returns a pre-signed URL for reading the file
	GetPresignedURL(ctx context.Context, remotePath string) (string, error)

	// ListFiles lists files with a given prefix
	ListFiles(ctx context.Context, prefix string) ([]string, error)
}
