package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Video codes ที่มี gallery (จาก query ก่อนหน้า)
var videoCodes = []string{
	"utywgage",
	"jpsnjy6b",
	"8ubvpb7k",
	"psfyxgqh",
	"93dp7swp",
	"jqgchcnn",
	"6gnbzx67",
	"8635tjug",
	"2fxguryn",
	"mv5ku3uv",
	"qevzp7a8",
	"86rg5suf",
	"t8jamqgs",
	"n64rcdhd",
	"3993bp6j",
}

func main() {
	fmt.Println("===========================================")
	fmt.Println("  Clear E2 Gallery Tool")
	fmt.Println("  ลบ gallery จาก E2 เท่านั้น")
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

	fmt.Printf("\nจะลบ gallery ของ %d videos:\n", len(videoCodes))
	for _, code := range videoCodes {
		fmt.Printf("  - gallery/%s/\n", code)
	}

	// Confirm
	fmt.Print("\nต้องการลบทั้งหมด? (y/N): ")
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Println("ยกเลิก")
		return
	}

	bucket := getEnv("S3_BUCKET", "suekk-01")

	// Delete from E2
	fmt.Println("\nกำลังลบจาก E2...")
	totalDeleted := 0
	for _, code := range videoCodes {
		path := fmt.Sprintf("gallery/%s/", code)
		fmt.Printf("  ลบ: %s ... ", path)
		deleted, err := deleteS3Folder(s3Client, bucket, path)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
		} else {
			fmt.Printf("OK (%d files)\n", deleted)
			totalDeleted += deleted
		}
	}

	fmt.Println("\n===========================================")
	fmt.Printf("  เสร็จสิ้น! ลบทั้งหมด %d files\n", totalDeleted)
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

func deleteS3Folder(client *minio.Client, bucket, prefix string) (int, error) {
	ctx := context.Background()
	deleted := 0

	// List and delete objects
	objectCh := client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	for obj := range objectCh {
		if obj.Err != nil {
			return deleted, obj.Err
		}

		err := client.RemoveObject(ctx, bucket, obj.Key, minio.RemoveObjectOptions{})
		if err != nil {
			log.Printf("Failed to delete %s: %v", obj.Key, err)
		} else {
			deleted++
		}
	}

	return deleted, nil
}
