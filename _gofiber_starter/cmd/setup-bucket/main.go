package main

import (
	"bytes"
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

	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("  IDrive e2 Bucket Setup for Direct Upload")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("\nEndpoint: %s\n", endpoint)
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

	// Check bucket exists
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		log.Fatalf("Failed to check bucket: %v", err)
	}
	if !exists {
		log.Fatalf("Bucket '%s' does not exist!", bucket)
	}
	fmt.Printf("\n✓ Bucket '%s' exists\n", bucket)

	// Set bucket policy
	// - Public read for hls/* (streaming)
	// - Authenticated write for videos/* (uploads)
	policy := map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{
			{
				"Sid":       "PublicReadHLS",
				"Effect":    "Allow",
				"Principal": "*",
				"Action":    []string{"s3:GetObject"},
				"Resource":  []string{fmt.Sprintf("arn:aws:s3:::%s/hls/*", bucket)},
			},
		},
	}

	policyJSON, _ := json.MarshalIndent(policy, "", "  ")

	fmt.Println("\n--- Setting Bucket Policy ---")
	fmt.Println(string(policyJSON))

	err = client.SetBucketPolicy(ctx, bucket, string(policyJSON))
	if err != nil {
		log.Printf("⚠️  Warning: Failed to set policy: %v", err)
	} else {
		fmt.Println("\n✓ Bucket policy set successfully")
	}

	// Print CORS instructions
	fmt.Println("\n═══════════════════════════════════════════════════════════════")
	fmt.Println("  ⚠️  MANUAL STEP REQUIRED: Configure CORS in IDrive Dashboard")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println(`
1. Go to: https://e2.idrive.com
2. Select bucket: ` + bucket + `
3. Go to "Permissions" → "CORS Configuration"
4. Add this CORS rule (JSON format):

[
  {
    "AllowedOrigins": ["*"],
    "AllowedMethods": ["GET", "HEAD", "PUT"],
    "AllowedHeaders": ["*"],
    "ExposeHeaders": ["ETag", "Content-Length", "Content-Range", "Accept-Ranges"],
    "MaxAgeSeconds": 3600
  }
]

หมายเหตุ:
- PUT จำเป็นสำหรับ Direct Upload ผ่าน Presigned URL
- ETag ใน ExposeHeaders จำเป็นสำหรับ multipart upload
- ถ้าต้องการจำกัด origin ให้เปลี่ยน "*" เป็น URL ของ frontend เช่น:
  ["https://admin.suekk.com", "http://localhost:5173"]
`)

	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("  🔑 Check Access Key Permissions")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println(`
ถ้ายังขึ้น "Access Denied" หลังตั้ง CORS แล้ว:

1. ไปที่ IDrive e2 Dashboard → Access Keys
2. ตรวจสอบว่า Access Key ที่ใช้มีสิทธิ์:
   - s3:PutObject (สำหรับ upload)
   - s3:GetObject (สำหรับ download)
   - s3:DeleteObject (สำหรับลบ)
   - s3:ListBucket (สำหรับ list)
   - s3:ListBucketMultipartUploads
   - s3:ListMultipartUploadParts
   - s3:AbortMultipartUpload

3. ถ้าใช้ Access Key แบบ restricted:
   - ลอง regenerate Access Key แบบ "Full Access" ใหม่
   - หรือเพิ่ม permission สำหรับ multipart uploads

4. ทดสอบ connection:
   cd _gofiber_starter
   go run ./cmd/setup-bucket
`)

	// Test basic operations
	fmt.Println("\n--- Testing Basic Operations ---")

	// Test list (check read permission)
	fmt.Print("Testing ListObjects... ")
	objCh := client.ListObjects(ctx, bucket, minio.ListObjectsOptions{MaxKeys: 1})
	listOK := true
	for obj := range objCh {
		if obj.Err != nil {
			fmt.Printf("❌ Failed: %v\n", obj.Err)
			listOK = false
			break
		}
	}
	if listOK {
		fmt.Println("✓ OK")
	}

	// Test StatObject on a known path (if exists)
	fmt.Print("Testing StatObject on hls/... ")
	_, err = client.StatObject(ctx, bucket, "hls/test", minio.StatObjectOptions{})
	if err != nil {
		// This is expected if file doesn't exist
		if err.Error() == "The specified key does not exist." {
			fmt.Println("✓ OK (file not found, but permission OK)")
		} else {
			fmt.Printf("⚠️  %v\n", err)
		}
	} else {
		fmt.Println("✓ OK")
	}

	// Test regular PutObject (single upload)
	fmt.Print("Testing PutObject (single upload)... ")
	testContent := []byte("test content for upload permission check")
	_, err = client.PutObject(ctx, bucket, "test/upload-test.txt",
		bytes.NewReader(testContent), int64(len(testContent)),
		minio.PutObjectOptions{ContentType: "text/plain"})
	if err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		fmt.Println("✓ OK")
		// Cleanup test file
		client.RemoveObject(ctx, bucket, "test/upload-test.txt", minio.RemoveObjectOptions{})
	}

	// Test Multipart Upload (critical for Direct Upload)
	fmt.Print("Testing CreateMultipartUpload... ")
	core := minio.Core{Client: client}
	uploadID, err := core.NewMultipartUpload(ctx, bucket, "test/multipart-test.mp4", minio.PutObjectOptions{
		ContentType: "video/mp4",
	})
	if err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
		fmt.Println("\n⚠️  Multipart Upload ไม่ทำงาน!")
		fmt.Println("   นี่คือสาเหตุที่ Direct Upload ขึ้น 'Access Denied'")
		fmt.Println("\n   วิธีแก้ไข:")
		fmt.Println("   1. ไปที่ IDrive e2 Dashboard → Access Keys")
		fmt.Println("   2. สร้าง Access Key ใหม่แบบ 'Full Access' หรือ 'Administrator'")
		fmt.Println("   3. หรือติดต่อ IDrive support เพื่อเปิดใช้ multipart upload")
	} else {
		fmt.Println("✓ OK")
		// Abort the test upload
		core.AbortMultipartUpload(ctx, bucket, "test/multipart-test.mp4", uploadID)
		fmt.Println("   ✓ Multipart Upload พร้อมใช้งาน!")
	}

	fmt.Println("\n═══════════════════════════════════════════════════════════════")
	fmt.Println("  Setup Complete!")
	fmt.Println("═══════════════════════════════════════════════════════════════")
}
