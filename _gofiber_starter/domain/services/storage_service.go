package services

import (
	"context"
)

// StorageService interface สำหรับจัดการ storage และ cleanup
type StorageService interface {
	// RunCleanup รัน cleanup tasks ทั้งหมด
	RunCleanup(ctx context.Context)

	// GetStorageStats ดึงสถิติ storage สำหรับ dashboard
	GetStorageStats(ctx context.Context) (*StorageStats, error)

	// RegisterCleanupJob ลงทะเบียน cleanup job กับ scheduler
	RegisterCleanupJob() error
}

// StorageStats สถิติ storage สำหรับ dashboard
type StorageStats struct {
	DiskTotal        uint64  `json:"diskTotal"`        // bytes
	DiskFree         uint64  `json:"diskFree"`         // bytes
	DiskUsed         uint64  `json:"diskUsed"`         // bytes
	DiskUsedPercent  float64 `json:"diskUsedPercent"`  // percentage
	VideosSize       int64   `json:"videosSize"`       // bytes
	TempSize         int64   `json:"tempSize"`         // bytes
	VideoFolderCount int     `json:"videoFolderCount"` // number of video folders
	PendingVideos    int64   `json:"pendingVideos"`
	ProcessingVideos int64   `json:"processingVideos"`
	ReadyVideos      int64   `json:"readyVideos"`
	FailedVideos     int64   `json:"failedVideos"`
}

// StorageStatsFormatted formatted version for display
type StorageStatsFormatted struct {
	DiskTotal       string `json:"diskTotal"`
	DiskFree        string `json:"diskFree"`
	DiskUsed        string `json:"diskUsed"`
	DiskUsedPercent string `json:"diskUsedPercent"`
	VideosSize      string `json:"videosSize"`
	TempSize        string `json:"tempSize"`
}
