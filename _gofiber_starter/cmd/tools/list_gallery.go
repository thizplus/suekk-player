package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	videoCode := "3993bp6j"
	if len(os.Args) > 1 {
		videoCode = os.Args[1]
	}

	fmt.Println("===========================================")
	fmt.Printf("  List Gallery Files: %s\n", videoCode)
	fmt.Println("===========================================")

	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Create S3 client
	s3Client, err := createS3Client()
	if err != nil {
		log.Fatalf("Failed to create S3 client: %v", err)
	}
	fmt.Println("✓ Connected to E2 storage")

	bucket := getEnv("S3_BUCKET", "suekk-01")
	prefix := fmt.Sprintf("gallery/%s/", videoCode)

	fmt.Printf("\nListing: %s\n\n", prefix)

	// List objects
	ctx := context.Background()
	objectCh := s3Client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	count := 0
	for obj := range objectCh {
		if obj.Err != nil {
			log.Printf("Error: %v", obj.Err)
			continue
		}
		fmt.Printf("  %s (%d bytes)\n", obj.Key, obj.Size)
		count++
	}

	fmt.Printf("\n===========================================\n")
	fmt.Printf("  Total: %d files\n", count)
	fmt.Println("===========================================")
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func createS3Client() (*minio.Client, error) {
	endpoint := getEnv("S3_ENDPOINT", "")
	accessKey := getEnv("S3_ACCESS_KEY", "")
	secretKey := getEnv("S3_SECRET_KEY", "")
	useSSL := getEnv("S3_USE_SSL", "true") == "true"

	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
}
