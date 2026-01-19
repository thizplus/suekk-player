//go:build windows

package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// GetDiskInfo ดึงข้อมูลพื้นที่ disk ของ path ที่ระบุ (Windows)
func GetDiskInfo(path string) (*DiskInfo, error) {
	// ตรวจสอบว่า path มีอยู่จริง
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// ถ้าไม่มี ให้ใช้ parent directory
		path = filepath.Dir(path)
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64

	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("failed to convert path: %w", err)
	}

	err = windows.GetDiskFreeSpaceEx(
		pathPtr,
		&freeBytesAvailable,
		&totalBytes,
		&totalFreeBytes,
	)
	if err != nil {
		return nil, fmt.Errorf("GetDiskFreeSpaceEx failed: %w", err)
	}

	used := totalBytes - totalFreeBytes
	usedPercent := float64(used) / float64(totalBytes) * 100

	return &DiskInfo{
		Total:       totalBytes,
		Free:        totalFreeBytes,
		Used:        used,
		UsedPercent: usedPercent,
	}, nil
}
