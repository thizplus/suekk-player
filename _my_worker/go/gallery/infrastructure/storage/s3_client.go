package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/suekk/my_worker/go/gallery/config"
	"github.com/suekk/my_worker/go/gallery/domain/ports"
)

// S3Client implements StoragePort for S3-compatible storage
type S3Client struct {
	client *s3.Client
	bucket string
	logger *slog.Logger
}

// NewS3Client creates a new S3 client
func NewS3Client(cfg *config.Config) (*S3Client, error) {
	// Create custom resolver for S3-compatible endpoints
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               cfg.S3Endpoint,
			SigningRegion:     cfg.S3Region,
			HostnameImmutable: true,
		}, nil
	})

	// Load AWS config
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.S3Region),
		awsconfig.WithEndpointResolverWithOptions(customResolver),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.S3AccessKey,
			cfg.S3SecretKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return &S3Client{
		client: client,
		bucket: cfg.S3Bucket,
		logger: slog.Default().With("component", "s3-client"),
	}, nil
}

// Download downloads a file from S3 to local path
func (s *S3Client) Download(ctx context.Context, remotePath, localPath string, onProgress ports.DownloadCallback) error {
	// Create output directory
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Get object
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(remotePath),
	})
	if err != nil {
		return fmt.Errorf("failed to get object: %w", err)
	}
	defer result.Body.Close()

	// Create local file
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy with progress
	var downloaded int64
	total := result.ContentLength
	if total == nil {
		var zero int64
		total = &zero
	}

	buf := make([]byte, 32*1024)
	for {
		n, err := result.Body.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("failed to write file: %w", writeErr)
			}
			downloaded += int64(n)
			if onProgress != nil {
				onProgress(downloaded, *total)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read object: %w", err)
		}
	}

	s.logger.Debug("Downloaded file", "remote", remotePath, "local", localPath, "size", downloaded)
	return nil
}

// Upload uploads a local file to S3
func (s *S3Client) Upload(ctx context.Context, remotePath, localPath string) error {
	// Open local file
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get content type
	contentType := getContentType(localPath)

	// Upload
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(remotePath),
		Body:        file,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	s.logger.Debug("Uploaded file", "local", localPath, "remote", remotePath)
	return nil
}

// UploadBytes uploads bytes directly to S3
func (s *S3Client) UploadBytes(ctx context.Context, remotePath string, data []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(remotePath),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("failed to upload bytes: %w", err)
	}

	s.logger.Debug("Uploaded bytes", "remote", remotePath, "size", len(data))
	return nil
}

// Delete removes a file from S3
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

// GetPresignedURL returns a pre-signed URL for reading
func (s *S3Client) GetPresignedURL(ctx context.Context, remotePath string) (string, error) {
	presignClient := s3.NewPresignClient(s.client)

	req, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(remotePath),
	}, s3.WithPresignExpires(1*time.Hour))
	if err != nil {
		return "", fmt.Errorf("failed to create presigned URL: %w", err)
	}

	return req.URL, nil
}

// ListFiles lists files with a given prefix
func (s *S3Client) ListFiles(ctx context.Context, prefix string) ([]string, error) {
	var files []string

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range page.Contents {
			if obj.Key != nil {
				files = append(files, *obj.Key)
			}
		}
	}

	return files, nil
}

// getContentType returns content type based on file extension
func getContentType(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}
