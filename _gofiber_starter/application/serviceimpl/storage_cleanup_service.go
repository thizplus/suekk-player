package serviceimpl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/scheduler"
	"gofiber-template/pkg/utils"
)

// StorageCleanupConfig การตั้งค่าสำหรับ cleanup
type StorageCleanupConfig struct {
	VideoBasePath     string        // base path for video storage
	TempPath          string        // temp directory path
	CleanupCron       string        // cron expression for cleanup (default: "0 3 * * *" = 3 AM daily)
	TempFileMaxAge    time.Duration // max age for temp files (default: 24 hours)
	FailedVideoMaxAge time.Duration // max age for failed video files (default: 7 days)
	MinFreeSpaceGB    int           // minimum free space in GB before alert
}

// StorageCleanupService handles storage cleanup operations
type StorageCleanupService struct {
	config    StorageCleanupConfig
	videoRepo repositories.VideoRepository
	scheduler scheduler.EventScheduler
}

// Ensure StorageCleanupService implements StorageService interface
var _ services.StorageService = (*StorageCleanupService)(nil)

// NewStorageCleanupService creates a new cleanup service
func NewStorageCleanupService(
	config StorageCleanupConfig,
	videoRepo repositories.VideoRepository,
	eventScheduler scheduler.EventScheduler,
) services.StorageService {
	service := &StorageCleanupService{
		config:    config,
		videoRepo: videoRepo,
		scheduler: eventScheduler,
	}

	// Set defaults
	if service.config.CleanupCron == "" {
		service.config.CleanupCron = "0 3 * * *" // 3 AM daily
	}
	if service.config.TempFileMaxAge == 0 {
		service.config.TempFileMaxAge = 24 * time.Hour
	}
	if service.config.FailedVideoMaxAge == 0 {
		service.config.FailedVideoMaxAge = 7 * 24 * time.Hour // 7 days
	}
	if service.config.MinFreeSpaceGB == 0 {
		service.config.MinFreeSpaceGB = 10
	}

	return service
}

// RegisterCleanupJob registers the cleanup job with scheduler
func (s *StorageCleanupService) RegisterCleanupJob() error {
	return s.scheduler.AddJob("storage_cleanup", s.config.CleanupCron, func() {
		ctx := context.Background()
		s.RunCleanup(ctx)
	})
}

// RunCleanup runs all cleanup tasks
func (s *StorageCleanupService) RunCleanup(ctx context.Context) {
	logger.InfoContext(ctx, "Starting storage cleanup")

	// 1. Cleanup temp files
	tempCleaned, tempSize := s.cleanupTempFiles(ctx)
	logger.InfoContext(ctx, "Temp files cleaned", "count", tempCleaned, "size_mb", tempSize/1024/1024)

	// 2. Cleanup orphaned video files (no DB record)
	orphanCleaned, orphanSize := s.cleanupOrphanedFiles(ctx)
	logger.InfoContext(ctx, "Orphaned files cleaned", "count", orphanCleaned, "size_mb", orphanSize/1024/1024)

	// 3. Cleanup old failed video files
	failedCleaned, failedSize := s.cleanupFailedVideos(ctx)
	logger.InfoContext(ctx, "Failed video files cleaned", "count", failedCleaned, "size_mb", failedSize/1024/1024)

	// 4. Check disk space
	s.checkDiskSpace(ctx)

	totalSize := tempSize + orphanSize + failedSize
	logger.InfoContext(ctx, "Storage cleanup completed",
		"total_files_cleaned", tempCleaned+orphanCleaned+failedCleaned,
		"total_space_freed_mb", totalSize/1024/1024,
	)
}

// cleanupTempFiles removes old temp files
func (s *StorageCleanupService) cleanupTempFiles(ctx context.Context) (int, int64) {
	if s.config.TempPath == "" {
		return 0, 0
	}

	count := 0
	var totalSize int64
	cutoff := time.Now().Add(-s.config.TempFileMaxAge)

	err := filepath.Walk(s.config.TempPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			return nil
		}

		if info.ModTime().Before(cutoff) {
			size := info.Size()
			if err := os.Remove(path); err == nil {
				count++
				totalSize += size
				logger.DebugContext(ctx, "Deleted temp file", "path", path)
			}
		}
		return nil
	})

	if err != nil {
		logger.WarnContext(ctx, "Error walking temp directory", "error", err)
	}

	return count, totalSize
}

// cleanupOrphanedFiles removes video files that don't have a database record
func (s *StorageCleanupService) cleanupOrphanedFiles(ctx context.Context) (int, int64) {
	videosDir := filepath.Join(s.config.VideoBasePath, "videos")
	if _, err := os.Stat(videosDir); os.IsNotExist(err) {
		return 0, 0
	}

	count := 0
	var totalSize int64

	// List all video code directories
	entries, err := os.ReadDir(videosDir)
	if err != nil {
		logger.WarnContext(ctx, "Error reading videos directory", "error", err)
		return 0, 0
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		videoCode := entry.Name()

		// Check if video exists in database
		_, err := s.videoRepo.GetByCode(ctx, videoCode)
		if err != nil {
			// Video not found in DB - this is orphaned
			videoDir := filepath.Join(videosDir, videoCode)
			dirSize, _ := utils.GetDirectorySize(videoDir)

			if err := os.RemoveAll(videoDir); err == nil {
				count++
				totalSize += dirSize
				logger.InfoContext(ctx, "Deleted orphaned video directory", "code", videoCode, "size_mb", dirSize/1024/1024)
			} else {
				logger.WarnContext(ctx, "Failed to delete orphaned directory", "code", videoCode, "error", err)
			}
		}
	}

	return count, totalSize
}

// cleanupFailedVideos removes files for videos that have been failed for too long
func (s *StorageCleanupService) cleanupFailedVideos(ctx context.Context) (int, int64) {
	count := 0
	var totalSize int64
	cutoff := time.Now().Add(-s.config.FailedVideoMaxAge)

	// Get failed videos
	failedVideos, err := s.videoRepo.GetByStatus(ctx, models.VideoStatusFailed, 0, 1000)
	if err != nil {
		logger.WarnContext(ctx, "Error getting failed videos", "error", err)
		return 0, 0
	}

	for _, video := range failedVideos {
		if video.UpdatedAt.Before(cutoff) {
			videoDir := filepath.Join(s.config.VideoBasePath, "videos", video.Code)
			dirSize, _ := utils.GetDirectorySize(videoDir)

			// Delete video files
			if err := os.RemoveAll(videoDir); err == nil {
				count++
				totalSize += dirSize

				// Delete video record from database
				if err := s.videoRepo.Delete(ctx, video.ID); err != nil {
					logger.WarnContext(ctx, "Failed to delete video record", "video_id", video.ID, "error", err)
				} else {
					logger.InfoContext(ctx, "Deleted old failed video", "video_id", video.ID, "code", video.Code)
				}
			}
		}
	}

	return count, totalSize
}

// checkDiskSpace checks available disk space and logs warning if low
func (s *StorageCleanupService) checkDiskSpace(ctx context.Context) {
	info, err := utils.GetDiskInfo(s.config.VideoBasePath)
	if err != nil {
		logger.WarnContext(ctx, "Failed to get disk info", "error", err)
		return
	}

	freeGB := info.Free / 1024 / 1024 / 1024

	if freeGB < uint64(s.config.MinFreeSpaceGB) {
		logger.WarnContext(ctx, "Low disk space warning",
			"free_gb", freeGB,
			"min_required_gb", s.config.MinFreeSpaceGB,
			"used_percent", info.UsedPercent,
		)
	} else {
		logger.InfoContext(ctx, "Disk space check",
			"free_gb", freeGB,
			"used_percent", info.UsedPercent,
		)
	}
}

// GetStorageStats returns current storage statistics
func (s *StorageCleanupService) GetStorageStats(ctx context.Context) (*services.StorageStats, error) {
	// Disk info
	diskInfo, err := utils.GetDiskInfo(s.config.VideoBasePath)
	if err != nil {
		return nil, err
	}

	// Calculate videos storage usage
	videosDir := filepath.Join(s.config.VideoBasePath, "videos")
	videosSize, _ := utils.GetDirectorySize(videosDir)

	// Calculate temp storage usage
	var tempSize int64
	if s.config.TempPath != "" {
		tempSize, _ = utils.GetDirectorySize(s.config.TempPath)
	}

	// Count video folders
	videoFolders := 0
	if entries, err := os.ReadDir(videosDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				videoFolders++
			}
		}
	}

	// Count by status
	pendingCount, _ := s.videoRepo.CountByStatus(ctx, models.VideoStatusPending)
	processingCount, _ := s.videoRepo.CountByStatus(ctx, models.VideoStatusProcessing)
	readyCount, _ := s.videoRepo.CountByStatus(ctx, models.VideoStatusReady)
	failedCount, _ := s.videoRepo.CountByStatus(ctx, models.VideoStatusFailed)

	return &services.StorageStats{
		DiskTotal:        diskInfo.Total,
		DiskFree:         diskInfo.Free,
		DiskUsed:         diskInfo.Used,
		DiskUsedPercent:  diskInfo.UsedPercent,
		VideosSize:       videosSize,
		TempSize:         tempSize,
		VideoFolderCount: videoFolders,
		PendingVideos:    pendingCount,
		ProcessingVideos: processingCount,
		ReadyVideos:      readyCount,
		FailedVideos:     failedCount,
	}, nil
}

// FormatStorageStats formats storage stats for display
func FormatStorageStats(s *services.StorageStats) *services.StorageStatsFormatted {
	return &services.StorageStatsFormatted{
		DiskTotal:       utils.FormatBytes(s.DiskTotal),
		DiskFree:        utils.FormatBytes(s.DiskFree),
		DiskUsed:        utils.FormatBytes(s.DiskUsed),
		DiskUsedPercent: strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", s.DiskUsedPercent), "0"), ".") + "%",
		VideosSize:      utils.FormatBytes(uint64(s.VideosSize)),
		TempSize:        utils.FormatBytes(uint64(s.TempSize)),
	}
}

