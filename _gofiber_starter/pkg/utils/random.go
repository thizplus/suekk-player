package utils

import (
	"crypto/rand"
	"math/big"
)

const (
	// ตัวอักษรที่ใช้สำหรับ video code (ไม่มีตัวที่สับสน เช่น 0, O, l, 1)
	alphanumeric = "abcdefghjkmnpqrstuvwxyz23456789"
)

// GenerateRandomString สร้าง random string ความยาว n ตัวอักษร
func GenerateRandomString(n int) string {
	result := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphanumeric))))
		if err != nil {
			// fallback ถ้า crypto/rand ใช้ไม่ได้
			result[i] = alphanumeric[i%len(alphanumeric)]
			continue
		}
		result[i] = alphanumeric[num.Int64()]
	}
	return string(result)
}

// GenerateVideoCode สร้าง unique video code (8 ตัวอักษร)
func GenerateVideoCode() string {
	return GenerateRandomString(8)
}
