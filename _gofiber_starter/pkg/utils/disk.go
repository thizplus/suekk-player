package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// DiskInfo ข้อมูลพื้นที่ disk
type DiskInfo struct {
	Total       uint64  // พื้นที่ทั้งหมด (bytes)
	Free        uint64  // พื้นที่ว่าง (bytes)
	Used        uint64  // พื้นที่ที่ใช้ (bytes)
	UsedPercent float64 // % ที่ใช้
}

// CheckDiskSpace ตรวจสอบว่ามีพื้นที่ว่างเพียงพอหรือไม่
// requiredBytes: พื้นที่ที่ต้องการ (bytes)
// minFreePercent: % พื้นที่ว่างขั้นต่ำที่ต้องเหลือ (default: 10%)
func CheckDiskSpace(path string, requiredBytes int64, minFreePercent float64) (bool, *DiskInfo, error) {
	if minFreePercent == 0 {
		minFreePercent = 10.0 // default 10%
	}

	info, err := GetDiskInfo(path)
	if err != nil {
		return false, nil, err
	}

	// ตรวจสอบว่ามีพื้นที่เพียงพอ
	if int64(info.Free) < requiredBytes {
		return false, info, nil
	}

	// ตรวจสอบว่าหลังจากใช้แล้วยังเหลือพื้นที่ตาม minFreePercent
	remainingFree := int64(info.Free) - requiredBytes
	remainingPercent := float64(remainingFree) / float64(info.Total) * 100
	if remainingPercent < minFreePercent {
		return false, info, nil
	}

	return true, info, nil
}

// FormatBytes แปลง bytes เป็น human-readable format
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// GetDirectorySize คำนวณขนาดไฟล์ทั้งหมดใน directory
func GetDirectorySize(path string) (int64, error) {
	var totalSize int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	return totalSize, err
}

// DiskSpaceError error สำหรับ disk space ไม่พอ
type DiskSpaceError struct {
	Required  int64
	Available uint64
	Message   string
}

func (e *DiskSpaceError) Error() string {
	return fmt.Sprintf("%s: required %s, available %s",
		e.Message,
		FormatBytes(uint64(e.Required)),
		FormatBytes(e.Available),
	)
}

// NewDiskSpaceError สร้าง DiskSpaceError
func NewDiskSpaceError(required int64, available uint64) *DiskSpaceError {
	return &DiskSpaceError{
		Required:  required,
		Available: available,
		Message:   "insufficient disk space",
	}
}
