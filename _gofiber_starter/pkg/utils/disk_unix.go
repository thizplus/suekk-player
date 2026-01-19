//go:build !windows

package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// GetDiskInfo ดึงข้อมูลพื้นที่ disk ของ path ที่ระบุ (Unix/Linux)
func GetDiskInfo(path string) (*DiskInfo, error) {
	// ตรวจสอบว่า path มีอยู่จริง
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// ถ้าไม่มี ให้ใช้ parent directory
		path = filepath.Dir(path)
	}

	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return nil, fmt.Errorf("statfs failed: %w", err)
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bfree * uint64(stat.Bsize)
	used := totalBytes - freeBytes
	usedPercent := float64(used) / float64(totalBytes) * 100

	return &DiskInfo{
		Total:       totalBytes,
		Free:        freeBytes,
		Used:        used,
		UsedPercent: usedPercent,
	}, nil
}
