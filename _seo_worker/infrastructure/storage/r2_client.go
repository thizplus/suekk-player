package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"seo-worker/domain/ports"
)

type R2Client struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	publicURL     string
	logger        *slog.Logger
}

type R2Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	PublicURL string
}

func NewR2Client(cfg R2Config) (*R2Client, error) {
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: cfg.Endpoint,
		}, nil
	})

	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithEndpointResolverWithOptions(resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	presignClient := s3.NewPresignClient(client)

	return &R2Client{
		client:        client,
		presignClient: presignClient,
		bucket:        cfg.Bucket,
		publicURL:     cfg.PublicURL,
		logger:        slog.Default().With("component", "r2_storage"),
	}, nil
}

func (c *R2Client) Upload(ctx context.Context, path string, data []byte, contentType string) error {
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(path),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("failed to upload to R2: %w", err)
	}

	c.logger.InfoContext(ctx, "File uploaded",
		"path", path,
		"size", len(data),
	)

	return nil
}

func (c *R2Client) UploadReader(ctx context.Context, path string, reader io.Reader, contentType string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}
	return c.Upload(ctx, path, data, contentType)
}

func (c *R2Client) GetFileContent(path string) (io.ReadCloser, int64, error) {
	result, err := c.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get file from R2: %w", err)
	}

	return result.Body, *result.ContentLength, nil
}

func (c *R2Client) GetPublicURL(path string) string {
	return fmt.Sprintf("%s/%s", c.publicURL, path)
}

func (c *R2Client) Delete(ctx context.Context, path string) error {
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from R2: %w", err)
	}

	c.logger.InfoContext(ctx, "File deleted", "path", path)
	return nil
}

func (c *R2Client) Exists(ctx context.Context, path string) (bool, error) {
	_, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		// Check if it's a "not found" error
		return false, nil
	}
	return true, nil
}

func (c *R2Client) ListFiles(prefix string) ([]string, error) {
	var files []string

	paginator := s3.NewListObjectsV2Paginator(c.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range page.Contents {
			files = append(files, *obj.Key)
		}
	}

	return files, nil
}

func (c *R2Client) GetPresignedDownloadURL(path string, expiry time.Duration) (string, error) {
	presignedReq, err := c.presignClient.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(path),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("failed to presign URL: %w", err)
	}

	return presignedReq.URL, nil
}

// Verify interface implementation
var _ ports.StoragePort = (*R2Client)(nil)
