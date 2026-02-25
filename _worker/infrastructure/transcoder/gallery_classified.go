package transcoder

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"suekk-worker/infrastructure/classifier"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Gallery Generation with NSFW Classification
// สร้าง gallery images พร้อม classify แยก safe/nsfw
// ═══════════════════════════════════════════════════════════════════════════════

const (
	// MinSuperSafeImages - จำนวนภาพ super_safe ขั้นต่ำที่ต้องการ (สำหรับ Public SEO)
	MinSuperSafeImages = 10

	// MinSafeImages - จำนวนภาพ safe ขั้นต่ำที่ต้องการ
	MinSafeImages = 12

	// MaxSafeImages - จำนวนภาพ safe สูงสุดที่เก็บ
	MaxSafeImages = 10

	// MaxNsfwImages - จำนวนภาพ nsfw สูงสุดที่เก็บ
	MaxNsfwImages = 20

	// FramesPerMinute - จำนวน frame ต่อนาที
	FramesPerMinute = 10
)

// ClassifiedGalleryConfig การตั้งค่าสำหรับ gallery พร้อม classification
type ClassifiedGalleryConfig struct {
	InputPath      string  // Path to video file
	OutputDir      string  // Local output directory
	VideoCode      string  // Video code for folder naming
	DurationSec    float64 // Video duration in seconds
	ClassifierPath string  // Path to classify_batch.py
}

// ClassifiedGalleryResult ผลลัพธ์ของ gallery พร้อม classification (Three-Tier)
type ClassifiedGalleryResult struct {
	SuperSafeImages []classifier.ClassificationResult // ภาพ super_safe (< 0.15 + face) สำหรับ Public SEO
	SafeImages      []classifier.ClassificationResult // ภาพ safe (0.15-0.3) Lazy load
	NsfwImages      []classifier.ClassificationResult // ภาพ nsfw (top 30)
	SuperSafeDir    string                            // Local path to super_safe images
	SafeDir         string                            // Local path to safe images
	NsfwDir         string                            // Local path to nsfw images
	Stats           classifier.ClassificationStats    // สถิติ
	RoundsUsed      int                               // จำนวน round ที่ใช้
	TotalFrames     int                               // จำนวน frame ทั้งหมดที่ดึง
}

// TimestampTracker ติดตาม timestamps ที่ใช้ไปแล้ว (ป้องกันภาพซ้ำ)
type TimestampTracker struct {
	used   map[int]bool
	minGap int // minimum gap in seconds
}

// NewTimestampTracker สร้าง tracker ใหม่
func NewTimestampTracker(minGap int) *TimestampTracker {
	return &TimestampTracker{
		used:   make(map[int]bool),
		minGap: minGap,
	}
}

// IsAvailable ตรวจสอบว่า timestamp นี้ยังไม่ถูกใช้
func (t *TimestampTracker) IsAvailable(timestamp float64) bool {
	sec := int(timestamp)
	for i := sec - t.minGap; i <= sec+t.minGap; i++ {
		if t.used[i] {
			return false
		}
	}
	return true
}

// Mark ทำเครื่องหมายว่า timestamp นี้ถูกใช้แล้ว
func (t *TimestampTracker) Mark(timestamp float64) {
	t.used[int(timestamp)] = true
}

// GenerateGalleryWithClassification สร้าง gallery พร้อม NSFW classification
// ใช้ Two-Phase extraction:
// - Phase 1 (นาทีที่ 1-10): หา super_safe + safe
// - Phase 2 (นาทีที่ 11-30): หา nsfw
func GenerateGalleryWithClassification(
	ctx context.Context,
	cfg ClassifiedGalleryConfig,
	logger *slog.Logger,
) (*ClassifiedGalleryResult, error) {

	// ตรวจสอบว่า video ยาวพอไหม
	if cfg.DurationSec < MinDurationForGallery {
		logger.Info("video too short for gallery",
			"video_code", cfg.VideoCode,
			"duration_sec", cfg.DurationSec,
			"min_duration", MinDurationForGallery,
		)
		return nil, nil
	}

	// สร้าง directories (Three-Tier)
	baseDir := filepath.Join(cfg.OutputDir, cfg.VideoCode)
	allFramesDir := filepath.Join(baseDir, "all")
	superSafeDir := filepath.Join(baseDir, "super_safe")
	safeDir := filepath.Join(baseDir, "safe")
	nsfwDir := filepath.Join(baseDir, "nsfw")

	for _, dir := range []string{allFramesDir, superSafeDir, safeDir, nsfwDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	// Initialize classifier (Three-Tier config)
	classifierConfig := classifier.ClassifierConfig{
		PythonPath:         "python",
		ScriptPath:         cfg.ClassifierPath,
		NsfwThreshold:      0.3,
		SuperSafeThreshold: 0.15,
		MinFaceScore:       0.1,
		Timeout:            300,
		MaxNsfwImages:      MaxNsfwImages,
		MaxSafeImages:      MaxSafeImages,
		MinSafeImages:      MinSafeImages,
		MinSuperSafeImages: MinSuperSafeImages,
		SkipMosaic:         true,
		SkipPOV:            true,
	}
	nsfwClassifier := classifier.NewNSFWClassifier(classifierConfig, logger)

	// Timestamp tracker (minimum 3 second gap between frames)
	tracker := NewTimestampTracker(3)

	// Results (Three-Tier)
	var allSuperSafeResults []classifier.ClassificationResult
	var allSafeResults []classifier.ClassificationResult
	var allNsfwResults []classifier.ClassificationResult
	totalFrames := 0

	videoDurationMin := int(cfg.DurationSec / 60)

	logger.Info("starting two-phase gallery generation",
		"video_code", cfg.VideoCode,
		"duration_sec", cfg.DurationSec,
		"duration_min", videoDurationMin,
	)

	// ═══════════════════════════════════════════════════════════════
	// Phase 1: นาทีที่ 1-10 → หา super_safe + safe
	// ═══════════════════════════════════════════════════════════════
	phase1Start := 0
	phase1End := 10
	if phase1End > videoDurationMin {
		phase1End = videoDurationMin
	}

	logger.Info("phase 1: extracting super_safe candidates",
		"start_minute", phase1Start+1,
		"end_minute", phase1End,
	)

	frameCount1 := extractTimeBasedFrames(
		ctx, cfg.InputPath, allFramesDir, cfg.DurationSec,
		phase1Start, phase1End, FramesPerMinute,
		tracker, totalFrames, logger,
	)

	if frameCount1 > 0 {
		totalFrames += frameCount1

		result1, err := nsfwClassifier.ClassifyBatch(ctx, allFramesDir)
		if err != nil {
			logger.Warn("phase 1 classification failed", "error", err)
		} else {
			logger.Info("phase 1 classification complete",
				"total_images", result1.Stats.TotalImages,
				"super_safe", result1.Stats.SuperSafeCount,
				"safe", result1.Stats.SafeCount,
				"nsfw", result1.Stats.NsfwCount,
			)

			separated1 := nsfwClassifier.SeparateResults(result1.Results)
			moveClassifiedFilesThreeTier(allFramesDir, superSafeDir, safeDir, nsfwDir, separated1, logger)

			// Phase 1: เก็บ super_safe และ safe เท่านั้น
			allSuperSafeResults = append(allSuperSafeResults, separated1.SuperSafe...)
			allSafeResults = append(allSafeResults, separated1.Safe...)
			// ลบ nsfw จาก phase 1 ออก
			for _, r := range separated1.Nsfw {
				os.Remove(filepath.Join(nsfwDir, r.Filename))
			}

			logger.Info("phase 1 complete",
				"super_safe_found", len(separated1.SuperSafe),
				"safe_found", len(separated1.Safe),
				"nsfw_discarded", len(separated1.Nsfw),
			)
		}
	}

	// ═══════════════════════════════════════════════════════════════
	// Phase 2: นาทีที่ 11-30 → หา nsfw
	// ═══════════════════════════════════════════════════════════════
	phase2Start := 10
	phase2End := 30
	if phase2Start >= videoDurationMin {
		logger.Warn("video too short for phase 2", "video_duration_min", videoDurationMin)
	} else {
		if phase2End > videoDurationMin {
			phase2End = videoDurationMin
		}

		logger.Info("phase 2: extracting nsfw candidates",
			"start_minute", phase2Start+1,
			"end_minute", phase2End,
		)

		frameCount2 := extractTimeBasedFrames(
			ctx, cfg.InputPath, allFramesDir, cfg.DurationSec,
			phase2Start, phase2End, FramesPerMinute,
			tracker, totalFrames, logger,
		)

		if frameCount2 > 0 {
			totalFrames += frameCount2

			result2, err := nsfwClassifier.ClassifyBatch(ctx, allFramesDir)
			if err != nil {
				logger.Warn("phase 2 classification failed", "error", err)
			} else {
				logger.Info("phase 2 classification complete",
					"total_images", result2.Stats.TotalImages,
					"super_safe", result2.Stats.SuperSafeCount,
					"safe", result2.Stats.SafeCount,
					"nsfw", result2.Stats.NsfwCount,
				)

				separated2 := nsfwClassifier.SeparateResults(result2.Results)
				moveClassifiedFilesThreeTier(allFramesDir, superSafeDir, safeDir, nsfwDir, separated2, logger)

				// Phase 2: เก็บ nsfw เท่านั้น
				allNsfwResults = append(allNsfwResults, separated2.Nsfw...)
				// ลบ super_safe และ safe จาก phase 2 ออก
				for _, r := range separated2.SuperSafe {
					os.Remove(filepath.Join(superSafeDir, r.Filename))
				}
				for _, r := range separated2.Safe {
					os.Remove(filepath.Join(safeDir, r.Filename))
				}

				logger.Info("phase 2 complete",
					"nsfw_found", len(separated2.Nsfw),
					"super_safe_discarded", len(separated2.SuperSafe),
					"safe_discarded", len(separated2.Safe),
				)
			}
		}
	}

	// Sort and limit NSFW images to top 20
	nsfwClassifier.SortByQuality(allNsfwResults)
	if len(allNsfwResults) > MaxNsfwImages {
		for i := MaxNsfwImages; i < len(allNsfwResults); i++ {
			os.Remove(filepath.Join(nsfwDir, allNsfwResults[i].Filename))
		}
		allNsfwResults = allNsfwResults[:MaxNsfwImages]
	}

	// Sort and limit Safe images to top 10
	nsfwClassifier.SortByQuality(allSafeResults)
	if len(allSafeResults) > MaxSafeImages {
		for i := MaxSafeImages; i < len(allSafeResults); i++ {
			os.Remove(filepath.Join(safeDir, allSafeResults[i].Filename))
		}
		allSafeResults = allSafeResults[:MaxSafeImages]
	}

	// Sort super_safe images by quality
	nsfwClassifier.SortByQuality(allSuperSafeResults)

	// Calculate stats
	avgNsfwScore := 0.0
	allResults := append(append(allSuperSafeResults, allSafeResults...), allNsfwResults...)
	for _, r := range allResults {
		avgNsfwScore += r.NsfwScore
	}
	totalClassified := len(allResults)
	if totalClassified > 0 {
		avgNsfwScore /= float64(totalClassified)
	}

	// Cleanup all frames dir (temporary)
	os.RemoveAll(allFramesDir)

	result := &ClassifiedGalleryResult{
		SuperSafeImages: allSuperSafeResults,
		SafeImages:      allSafeResults,
		NsfwImages:      allNsfwResults,
		SuperSafeDir:    superSafeDir,
		SafeDir:         safeDir,
		NsfwDir:         nsfwDir,
		Stats: classifier.ClassificationStats{
			TotalImages:    totalClassified,
			SuperSafeCount: len(allSuperSafeResults),
			SafeCount:      len(allSafeResults),
			NsfwCount:      len(allNsfwResults),
			AvgNsfwScore:   avgNsfwScore,
		},
		RoundsUsed:  2,
		TotalFrames: totalFrames,
	}

	logger.Info("two-phase gallery complete",
		"video_code", cfg.VideoCode,
		"total_frames", totalFrames,
		"super_safe_count", len(allSuperSafeResults),
		"safe_count", len(allSafeResults),
		"nsfw_count", len(allNsfwResults),
		"avg_nsfw_score", avgNsfwScore,
	)

	return result, nil
}

// extractTimeBasedFrames ดึงภาพตามช่วงเวลา (นาที)
func extractTimeBasedFrames(
	ctx context.Context,
	inputPath string,
	outputDir string,
	durationSec float64,
	startMinute int,
	endMinute int,
	framesPerMinute int,
	tracker *TimestampTracker,
	filenameOffset int,
	logger *slog.Logger,
) int {
	extracted := 0

	for minute := startMinute; minute < endMinute; minute++ {
		minuteStartSec := float64(minute * 60)
		intervalSec := 60.0 / float64(framesPerMinute)

		for i := 0; i < framesPerMinute; i++ {
			select {
			case <-ctx.Done():
				return extracted
			default:
			}

			timestamp := minuteStartSec + float64(i)*intervalSec

			// Skip if beyond video duration
			if timestamp >= durationSec {
				continue
			}

			// Skip if timestamp already used
			if !tracker.IsAvailable(timestamp) {
				continue
			}

			// Generate filename
			frameNum := filenameOffset + extracted + 1
			outputPath := filepath.Join(outputDir, fmt.Sprintf("%03d.jpg", frameNum))

			// Capture frame
			if err := captureFrame(ctx, inputPath, outputPath, timestamp); err != nil {
				logger.Debug("failed to capture frame",
					"minute", minute,
					"timestamp", timestamp,
					"error", err,
				)
				continue
			}

			// Verify file exists
			if _, err := os.Stat(outputPath); err == nil {
				tracker.Mark(timestamp)
				extracted++
			}
		}
	}

	return extracted
}

// moveClassifiedFilesThreeTier ย้ายไฟล์ไปยัง directory ที่ถูกต้อง (Three-Tier)
func moveClassifiedFilesThreeTier(
	srcDir, superSafeDir, safeDir, nsfwDir string,
	separated *classifier.SeparatedImages,
	logger *slog.Logger,
) {
	// Move super_safe files (< 0.15 + face) - สำหรับ Public SEO
	for _, img := range separated.SuperSafe {
		src := filepath.Join(srcDir, img.Filename)
		dst := filepath.Join(superSafeDir, img.Filename)
		if err := os.Rename(src, dst); err != nil {
			logger.Warn("failed to move super_safe image", "file", img.Filename, "error", err)
		}
	}

	// Move safe files (0.15-0.3) - Lazy load
	for _, img := range separated.Safe {
		src := filepath.Join(srcDir, img.Filename)
		dst := filepath.Join(safeDir, img.Filename)
		if err := os.Rename(src, dst); err != nil {
			logger.Warn("failed to move safe image", "file", img.Filename, "error", err)
		}
	}

	// Move nsfw files (>= 0.3) - Member only
	for _, img := range separated.Nsfw {
		src := filepath.Join(srcDir, img.Filename)
		dst := filepath.Join(nsfwDir, img.Filename)
		if err := os.Rename(src, dst); err != nil {
			logger.Warn("failed to move nsfw image", "file", img.Filename, "error", err)
		}
	}

	// Move error files to nsfw (safety first)
	for _, img := range separated.Error {
		src := filepath.Join(srcDir, img.Filename)
		dst := filepath.Join(nsfwDir, img.Filename)
		if err := os.Rename(src, dst); err != nil {
			logger.Warn("failed to move error image", "file", img.Filename, "error", err)
		}
	}
}

// UploadClassifiedGallery uploads super_safe, safe, and nsfw folders to S3 (Three-Tier)
func UploadClassifiedGallery(
	ctx context.Context,
	result *ClassifiedGalleryResult,
	remotePrefix string,
	uploader GalleryUploader,
	logger *slog.Logger,
) (superSafeUploaded, safeUploaded, nsfwUploaded int, err error) {

	// Upload super_safe images (for Public SEO)
	superSafeRemote := filepath.ToSlash(filepath.Join(remotePrefix, "super_safe"))
	superSafeCount, _, err := UploadGallery(ctx, result.SuperSafeDir, superSafeRemote, uploader, logger)
	if err != nil {
		logger.Warn("failed to upload super_safe gallery", "error", err)
	}

	// Upload safe images (borderline, lazy load)
	safeRemote := filepath.ToSlash(filepath.Join(remotePrefix, "safe"))
	safeCount, _, err := UploadGallery(ctx, result.SafeDir, safeRemote, uploader, logger)
	if err != nil {
		logger.Warn("failed to upload safe gallery", "error", err)
	}

	// Upload nsfw images (member only)
	nsfwRemote := filepath.ToSlash(filepath.Join(remotePrefix, "nsfw"))
	nsfwCount, _, err := UploadGallery(ctx, result.NsfwDir, nsfwRemote, uploader, logger)
	if err != nil {
		logger.Warn("failed to upload nsfw gallery", "error", err)
	}

	logger.Info("three-tier gallery uploaded",
		"remote_prefix", remotePrefix,
		"super_safe_uploaded", superSafeCount,
		"safe_uploaded", safeCount,
		"nsfw_uploaded", nsfwCount,
	)

	return superSafeCount, safeCount, nsfwCount, nil
}
