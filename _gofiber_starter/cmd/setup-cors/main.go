package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	// Load .env
	godotenv.Load()

	endpoint := os.Getenv("S3_ENDPOINT")
	accessKey := os.Getenv("S3_ACCESS_KEY")
	secretKey := os.Getenv("S3_SECRET_KEY")
	bucket := os.Getenv("S3_BUCKET")
	useSSL := os.Getenv("S3_USE_SSL") == "true"
	region := os.Getenv("S3_REGION")

	fmt.Printf("Connecting to: %s\n", endpoint)
	fmt.Printf("Bucket: %s\n", bucket)
	fmt.Printf("Region: %s\n", region)

	// Create MinIO client
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
		Region: region,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	// Set bucket policy for public read on hls/*
	policy := map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			{
				"Effect":    "Allow",
				"Principal": "*",
				"Action":    []string{"s3:GetObject"},
				"Resource":  []string{fmt.Sprintf("arn:aws:s3:::%s/hls/*", bucket)},
			},
		},
	}

	policyJSON, _ := json.Marshal(policy)

	fmt.Println("\nSetting bucket policy for public HLS access...")
	err = client.SetBucketPolicy(ctx, bucket, string(policyJSON))
	if err != nil {
		log.Printf("Warning: Failed to set policy: %v", err)
	} else {
		fmt.Println("✓ Bucket policy set successfully")
	}

	// Verify settings
	fmt.Println("\n--- Current Policy ---")
	currentPolicy, err := client.GetBucketPolicy(ctx, bucket)
	if err != nil {
		fmt.Printf("Policy: (not set or error: %v)\n", err)
	} else {
		fmt.Printf("Policy: %s\n", currentPolicy)
	}

	fmt.Println("\n✓ Policy setup complete!")
	fmt.Println("\n⚠️  IMPORTANT: You must set CORS manually in IDrive e2 Dashboard!")
	fmt.Println("   Go to: https://e2.idrive.com → Bucket Settings → CORS")
	fmt.Println(`
   Add this CORS rule:
   {
     "AllowedOrigins": ["*"],
     "AllowedMethods": ["GET", "HEAD"],
     "AllowedHeaders": ["*"],
     "ExposeHeaders": ["Content-Length", "Content-Range", "Accept-Ranges", "ETag"],
     "MaxAgeSeconds": 3600
   }
`)
	fmt.Println("\nTest URL:")
	fmt.Printf("  https://%s.s3.%s.idrivee2.com/hls/<video_code>/master.m3u8\n", bucket, region)
}
