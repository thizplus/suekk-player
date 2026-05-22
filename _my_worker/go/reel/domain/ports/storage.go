package ports

import (
	"context"
)

// DownloadCallback reports download progress
type DownloadCallback func(downloaded, total int64)

// StoragePort defines interface for file storage
type StoragePort interface {
	Download(ctx context.Context, remotePath, localPath string, onProgress DownloadCallback) error
	Upload(ctx context.Context, remotePath, localPath string) error
	Delete(ctx context.Context, remotePath string) error
}
