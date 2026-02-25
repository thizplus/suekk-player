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
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	fmt.Println("===========================================")
	fmt.Println("  Clear Gallery Tool")
	fmt.Println("  ลบ gallery จาก E2 และ reset DB")
	fmt.Println("===========================================")

	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Connect to database
	db, err := connectDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	fmt.Println("✓ Connected to database")

	// Create S3 client
	s3Client, err := createS3Client()
	if err != nil {
		log.Fatalf("Failed to create S3 client: %v", err)
	}
	fmt.Println("✓ Connected to E2 storage")

	// Get videos with gallery
	videos, err := getVideosWithGallery(db)
	if err != nil {
		log.Fatalf("Failed to get videos: %v", err)
	}

	if len(videos) == 0 {
		fmt.Println("\nไม่มี video ที่มี gallery")
		return
	}

	fmt.Printf("\nพบ %d videos ที่มี gallery:\n", len(videos))
	for _, v := range videos {
		fmt.Printf("  - %s (path: %s)\n", v.Code, v.GalleryPath)
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
	fmt.Println("\n[1/2] กำลังลบจาก E2...")
	for _, v := range videos {
		if v.GalleryPath == "" {
			continue
		}

		// Normalize path
		path := strings.TrimPrefix(v.GalleryPath, "/")
		if !strings.HasSuffix(path, "/") {
			path += "/"
		}

		fmt.Printf("  ลบ: %s ... ", path)
		deleted, err := deleteS3Folder(s3Client, bucket, path)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
		} else {
			fmt.Printf("OK (%d files)\n", deleted)
		}
	}

	// Reset database
	fmt.Println("\n[2/2] กำลัง reset database...")
	count, err := resetGalleryInDB(db)
	if err != nil {
		log.Fatalf("Failed to reset database: %v", err)
	}
	fmt.Printf("  Reset %d videos\n", count)

	fmt.Println("\n===========================================")
	fmt.Println("  เสร็จสิ้น!")
	fmt.Println("===========================================")
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func connectDB() (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", ""),
		getEnv("DB_NAME", "suekk_stream"),
		getEnv("DB_SSL_MODE", "disable"),
	)

	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
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

type VideoGallery struct {
	ID          string `gorm:"column:id"`
	Code        string `gorm:"column:code"`
	GalleryPath string `gorm:"column:gallery_path"`
}

func (VideoGallery) TableName() string {
	return "videos"
}

func getVideosWithGallery(db *gorm.DB) ([]VideoGallery, error) {
	var videos []VideoGallery
	err := db.Select("id, code, COALESCE(gallery_path, '') as gallery_path").
		Where("gallery_count > 0 OR gallery_super_safe_count > 0 OR gallery_source_count > 0").
		Find(&videos).Error
	return videos, err
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

func resetGalleryInDB(db *gorm.DB) (int64, error) {
	result := db.Exec(`
		UPDATE videos SET
			gallery_path = '',
			gallery_status = 'none',
			gallery_count = 0,
			gallery_source_count = 0,
			gallery_safe_count = 0,
			gallery_nsfw_count = 0,
			gallery_super_safe_count = 0
		WHERE gallery_count > 0 OR gallery_super_safe_count > 0 OR gallery_source_count > 0
	`)
	return result.RowsAffected, result.Error
}
