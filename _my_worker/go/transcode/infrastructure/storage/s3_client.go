package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	appconfig "github.com/suekk/my_worker/go/transcode/config"
)

// S3Client implements StoragePort for S3-compatible storage
type S3Client struct {
	client *s3.Client
	bucket string
	logger *slog.Logger
}

// NewS3Client creates a new S3 client
func NewS3Client(cfg *appconfig.Config) (*S3Client, error) {
	logger := slog.Default().With("component", "s3-client")

	// Build endpoint URL
	endpoint := cfg.S3Endpoint
	if !strings.HasPrefix(endpoint, "http") {
		if cfg.S3UseSSL {
			endpoint = "https://" + endpoint
		} else {
			endpoint = "http://" + endpoint
		}
	}

	// Create custom resolver
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               endpoint,
			HostnameImmutable: true,
		}, nil
	})

	// Create AWS config
	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.S3Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.S3AccessKey,
			cfg.S3SecretKey,
			"",
		)),
		config.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true // Required for MinIO and most S3-compatible storage
	})

	logger.Info("S3 client initialized",
		"endpoint", endpoint,
		"bucket", cfg.S3Bucket,
	)

	return &S3Client{
		client: client,
		bucket: cfg.S3Bucket,
		logger: logger,
	}, nil
}

// Download downloads a file from S3 to local path
func (s *S3Client) Download(ctx context.Context, remotePath, localPath string, onProgress func(downloaded, total int64)) error {
	// Get object size first
	headOutput, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(remotePath),
	})
	if err != nil {
		return fmt.Errorf("failed to get object info: %w", err)
	}

	totalSize := *headOutput.ContentLength

	// Create local directory
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create local file
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Download object
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(remotePath),
	})
	if err != nil {
		return fmt.Errorf("failed to download object: %w", err)
	}
	defer output.Body.Close()

	// Copy with progress
	var downloaded int64
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		n, readErr := output.Body.Read(buf)
		if n > 0 {
			_, writeErr := file.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("failed to write file: %w", writeErr)
			}
			downloaded += int64(n)
			if onProgress != nil {
				onProgress(downloaded, totalSize)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("failed to read from S3: %w", readErr)
		}
	}

	s.logger.Debug("Downloaded file",
		"remote", remotePath,
		"local", localPath,
		"size", totalSize,
	)

	return nil
}

// Upload uploads a local file to S3
func (s *S3Client) Upload(ctx context.Context, remotePath, localPath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get content type based on extension
	contentType := getContentType(localPath)

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(remotePath),
		Body:        file,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	s.logger.Debug("Uploaded file",
		"local", localPath,
		"remote", remotePath,
	)

	return nil
}

// UploadReader uploads from reader to S3
func (s *S3Client) UploadReader(ctx context.Context, remotePath string, reader io.Reader, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(remotePath),
		Body:        reader,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	return nil
}

// Delete deletes a file from S3
func (s *S3Client) Delete(ctx context.Context, remotePath string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(remotePath),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	s.logger.Debug("Deleted file", "remote", remotePath)
	return nil
}

// DeletePrefix deletes all files with given prefix
func (s *S3Client) DeletePrefix(ctx context.Context, prefix string) error {
	// List objects with prefix
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	var deleteCount int
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range page.Contents {
			_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(s.bucket),
				Key:    obj.Key,
			})
			if err != nil {
				s.logger.Warn("Failed to delete object", "key", *obj.Key, "error", err)
			} else {
				deleteCount++
			}
		}
	}

	s.logger.Debug("Deleted prefix", "prefix", prefix, "count", deleteCount)
	return nil
}

// Exists checks if a file exists
func (s *S3Client) Exists(ctx context.Context, remotePath string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(remotePath),
	})
	if err != nil {
		// Check if it's a "not found" error
		return false, nil
	}
	return true, nil
}

// GetSize returns file size in bytes
func (s *S3Client) GetSize(ctx context.Context, remotePath string) (int64, error) {
	output, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(remotePath),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get object info: %w", err)
	}
	return *output.ContentLength, nil
}

// getContentType returns content type based on file extension
func getContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".ts":
		return "video/MP2T"
	case ".mp4":
		return "video/mp4"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".wav":
		return "audio/wav"
	case ".srt":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}
