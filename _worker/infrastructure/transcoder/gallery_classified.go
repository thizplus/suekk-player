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

	// MinSafeImages - จำนวนภาพ safe ขั้นต่ำที่ต้องการ (รวม super_safe + borderline)
	MinSafeImages = 12

	// MaxNsfwImages - จำนวนภาพ nsfw สูงสุดที่เก็บ
	MaxNsfwImages = 30

	// MaxExtractionRounds - จำนวน round สูงสุดในการดึงภาพเพิ่ม
	MaxExtractionRounds = 5
)

// ExtractionRound กำหนดช่วงเวลาในการดึงภาพ
type ExtractionRound struct {
	Name       string  // ชื่อ round
	StartPct   float64 // เริ่มที่ % ของ video
	EndPct     float64 // จบที่ % ของ video
	FrameCount int     // จำนวนภาพที่จะดึง
	Offset     float64 // offset จาก interval ปกติ (0-1)
}

// DefaultExtractionRounds ลำดับการดึงภาพ
var DefaultExtractionRounds = []ExtractionRound{
	// Round 1: Standard (กระจายทั้ง video)
	{Name: "standard", StartPct: 0.05, EndPct: 0.95, FrameCount: 100, Offset: 0},
	// Round 2: Intro focus (0-15%) - มักเป็นการพูดคุย
	{Name: "intro", StartPct: 0.00, EndPct: 0.15, FrameCount: 20, Offset: 0},
	// Round 3: Outro focus (90-100%) - ending
	{Name: "outro", StartPct: 0.90, EndPct: 1.00, FrameCount: 15, Offset: 0},
	// Round 4: Gap fill (ระหว่าง Round 1)
	{Name: "gap_fill", StartPct: 0.05, EndPct: 0.95, FrameCount: 30, Offset: 0.5},
	// Round 5: Dense intro (ถ้ายังไม่พอ)
	{Name: "dense_intro", StartPct: 0.00, EndPct: 0.10, FrameCount: 30, Offset: 0.25},
}

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
// ใช้ Multi-Round extraction เพื่อให้ได้ภาพ safe อย่างน้อย MinSafeImages
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
	allFramesDir := filepath.Join(baseDir, "all")           // เก็บ frame ทั้งหมดชั่วคราว
	superSafeDir := filepath.Join(baseDir, "super_safe")    // ภาพ super_safe (< 0.15 + face) สำหรับ Public SEO
	safeDir := filepath.Join(baseDir, "safe")               // ภาพ safe (0.15-0.3) Lazy load
	nsfwDir := filepath.Join(baseDir, "nsfw")               // ภาพ nsfw (>= 0.3) Member only

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
		Timeout:            90,
		MaxNsfwImages:      MaxNsfwImages,
		MinSafeImages:      MinSafeImages,
		MinSuperSafeImages: MinSuperSafeImages,
	}
	nsfwClassifier := classifier.NewNSFWClassifier(classifierConfig, logger)

	// Timestamp tracker (minimum 3 second gap between frames)
	tracker := NewTimestampTracker(3)

	// Results (Three-Tier)
	var allSuperSafeResults []classifier.ClassificationResult
	var allSafeResults []classifier.ClassificationResult
	var allNsfwResults []classifier.ClassificationResult
	totalFrames := 0
	roundsUsed := 0

	logger.Info("starting classified gallery generation (three-tier)",
		"video_code", cfg.VideoCode,
		"duration_sec", cfg.DurationSec,
		"min_super_safe_images", MinSuperSafeImages,
		"min_safe_images", MinSafeImages,
	)

	// Multi-Round extraction (Three-Tier)
	for _, round := range DefaultExtractionRounds {
		// หยุดถ้าได้ภาพ super_safe และ safe พอแล้ว
		hasEnoughSuperSafe := len(allSuperSafeResults) >= MinSuperSafeImages
		totalSafeCount := len(allSuperSafeResults) + len(allSafeResults)
		hasEnoughSafe := totalSafeCount >= MinSafeImages

		if hasEnoughSuperSafe && hasEnoughSafe {
			break
		}

		roundsUsed++

		logger.Info("extraction round",
			"round", round.Name,
			"current_super_safe", len(allSuperSafeResults),
			"current_safe", len(allSafeResults),
			"target_super_safe", MinSuperSafeImages,
			"target_safe", MinSafeImages,
		)

		// Extract frames for this round
		frameCount := extractRoundFrames(
			ctx,
			cfg.InputPath,
			allFramesDir,
			cfg.DurationSec,
			round,
			tracker,
			totalFrames, // offset for filename
			logger,
		)

		if frameCount == 0 {
			continue
		}

		totalFrames += frameCount

		// Classify all frames in the directory
		result, err := nsfwClassifier.ClassifyBatch(ctx, allFramesDir)
		if err != nil {
			logger.Warn("classification failed",
				"round", round.Name,
				"error", err,
			)
			continue
		}

		// Separate results (Three-Tier)
		separated := nsfwClassifier.SeparateResults(result.Results)

		// Add to cumulative results
		allSuperSafeResults = append(allSuperSafeResults, separated.SuperSafe...)
		allSafeResults = append(allSafeResults, separated.Safe...)
		allNsfwResults = append(allNsfwResults, separated.Nsfw...)

		// Move classified files to appropriate directories (Three-Tier)
		moveClassifiedFilesThreeTier(allFramesDir, superSafeDir, safeDir, nsfwDir, separated, logger)

		logger.Info("round complete",
			"round", round.Name,
			"frames_extracted", frameCount,
			"super_safe_found", len(separated.SuperSafe),
			"safe_found", len(separated.Safe),
			"nsfw_found", len(separated.Nsfw),
			"total_super_safe", len(allSuperSafeResults),
			"total_safe", len(allSafeResults),
		)
	}

	// Sort and limit NSFW images
	nsfwClassifier.SortByQuality(allNsfwResults)
	if len(allNsfwResults) > MaxNsfwImages {
		// Delete excess NSFW images
		for i := MaxNsfwImages; i < len(allNsfwResults); i++ {
			excessPath := filepath.Join(nsfwDir, allNsfwResults[i].Filename)
			os.Remove(excessPath)
		}
		allNsfwResults = allNsfwResults[:MaxNsfwImages]
	}

	// Sort super_safe and safe images by quality
	nsfwClassifier.SortByQuality(allSuperSafeResults)
	nsfwClassifier.SortByQuality(allSafeResults)

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
		RoundsUsed:  roundsUsed,
		TotalFrames: totalFrames,
	}

	logger.Info("classified gallery complete (three-tier)",
		"video_code", cfg.VideoCode,
		"total_frames", totalFrames,
		"super_safe_count", len(allSuperSafeResults),
		"safe_count", len(allSafeResults),
		"nsfw_count", len(allNsfwResults),
		"rounds_used", roundsUsed,
		"avg_nsfw_score", avgNsfwScore,
		"super_safe_target_met", len(allSuperSafeResults) >= MinSuperSafeImages,
	)

	return result, nil
}

// extractRoundFrames ดึงภาพจาก video ตาม round ที่กำหนด
func extractRoundFrames(
	ctx context.Context,
	inputPath string,
	outputDir string,
	durationSec float64,
	round ExtractionRound,
	tracker *TimestampTracker,
	filenameOffset int,
	logger *slog.Logger,
) int {
	startTime := durationSec * round.StartPct
	endTime := durationSec * round.EndPct
	usableDuration := endTime - startTime

	if usableDuration <= 0 || round.FrameCount <= 0 {
		return 0
	}

	interval := usableDuration / float64(round.FrameCount)
	extracted := 0

	for i := 0; i < round.FrameCount; i++ {
		select {
		case <-ctx.Done():
			return extracted
		default:
		}

		// Calculate timestamp with offset
		timestamp := startTime + (float64(i)+round.Offset)*interval

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
				"round", round.Name,
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
