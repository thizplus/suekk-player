package ports

import (
	"context"
	"io"
)

// StoragePort defines interface for S3-compatible storage operations
type StoragePort interface {
	// Download downloads a file from storage to local path
	// onProgress callback receives (downloaded bytes, total bytes)
	Download(ctx context.Context, remotePath, localPath string, onProgress func(downloaded, total int64)) error

	// Upload uploads a local file to storage
	Upload(ctx context.Context, remotePath, localPath string) error

	// UploadReader uploads from reader to storage
	UploadReader(ctx context.Context, remotePath string, reader io.Reader, contentType string) error

	// Delete deletes a file from storage
	Delete(ctx context.Context, remotePath string) error

	// DeletePrefix deletes all files with given prefix
	DeletePrefix(ctx context.Context, prefix string) error

	// Exists checks if a file exists
	Exists(ctx context.Context, remotePath string) (bool, error)

	// GetSize returns file size in bytes
	GetSize(ctx context.Context, remotePath string) (int64, error)
}
