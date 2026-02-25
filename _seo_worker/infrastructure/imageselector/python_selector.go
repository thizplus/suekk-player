package imageselector

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"seo-worker/domain/models"
)

// PythonImageSelector - เรียก Python script สำหรับคัดเลือกภาพ
type PythonImageSelector struct {
	pythonPath string // path to python executable
	scriptPath string // path to image_selector.py
	device     string // cuda or cpu
	logger     *slog.Logger
}

// PythonImageSelectorConfig - configuration for PythonImageSelector
type PythonImageSelectorConfig struct {
	PythonPath string // e.g., "python" or "/usr/bin/python3"
	ScriptPath string // e.g., "python/image_selector.py"
	Device     string // "cuda" or "cpu"
}

func NewPythonImageSelector(cfg PythonImageSelectorConfig) *PythonImageSelector {
	pythonPath := cfg.PythonPath
	if pythonPath == "" {
		pythonPath = "python"
	}

	scriptPath := cfg.ScriptPath
	if scriptPath == "" {
		scriptPath = "python/image_selector.py"
	}

	device := cfg.Device
	if device == "" {
		device = "cuda"
	}

	return &PythonImageSelector{
		pythonPath: pythonPath,
		scriptPath: scriptPath,
		device:     device,
		logger:     slog.Default().With("component", "image_selector"),
	}
}

// SelectImages - คัดเลือกภาพ cover และ gallery ที่เหมาะสม
// กรอง NSFW ออก, เลือก cover ที่เห็นหน้าชัด, เลือก gallery 12 ภาพที่หลากหลาย
func (s *PythonImageSelector) SelectImages(ctx context.Context, imageURLs []string) (*models.ImageSelectionResult, error) {
	if len(imageURLs) == 0 {
		return &models.ImageSelectionResult{
			Cover:          nil,
			Gallery:        []models.ImageScore{},
			TotalImages:    0,
			SafeImages:     0,
			ProcessingTime: 0,
		}, nil
	}

	startTime := time.Now()
	s.logger.InfoContext(ctx, "Starting image selection",
		"total_images", len(imageURLs),
	)

	// สร้าง temp file สำหรับ input URLs
	inputFile, err := os.CreateTemp("", "image_urls_*.json")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp input file: %w", err)
	}
	defer os.Remove(inputFile.Name())

	// เขียน URLs ลงไฟล์
	urlsJSON, err := json.Marshal(imageURLs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal URLs: %w", err)
	}
	if _, err := inputFile.Write(urlsJSON); err != nil {
		return nil, fmt.Errorf("failed to write URLs to file: %w", err)
	}
	inputFile.Close()

	// สร้าง temp file สำหรับ output
	outputFile, err := os.CreateTemp("", "image_selected_*.json")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp output file: %w", err)
	}
	outputPath := outputFile.Name()
	outputFile.Close()
	defer os.Remove(outputPath)

	// สร้าง absolute path สำหรับ script
	scriptAbsPath, err := filepath.Abs(s.scriptPath)
	if err != nil {
		scriptAbsPath = s.scriptPath
	}

	// เรียก Python script
	cmd := exec.CommandContext(ctx, s.pythonPath, scriptAbsPath,
		"--input", inputFile.Name(),
		"--output", outputPath,
		"--device", s.device,
	)

	// Capture stderr for debugging
	s.logger.InfoContext(ctx, "[DEBUG] Running Python script",
		"script", scriptAbsPath,
		"input", inputFile.Name(),
		"output", outputPath,
		"device", s.device,
	)

	output, err := cmd.CombinedOutput()

	// Log output regardless of error
	s.logger.InfoContext(ctx, "[DEBUG] Python script output",
		"output_length", len(output),
		"output", string(output),
	)

	if err != nil {
		s.logger.ErrorContext(ctx, "Python script failed",
			"error", err,
			"output", string(output),
		)
		return nil, fmt.Errorf("python script failed: %w\nOutput: %s", err, string(output))
	}

	// อ่านผลลัพธ์
	resultJSON, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read output file: %w", err)
	}

	s.logger.InfoContext(ctx, "[DEBUG] Result JSON",
		"json_length", len(resultJSON),
		"json_preview", string(resultJSON[:min(500, len(resultJSON))]),
	)

	// Parse JSON result
	var pythonResult struct {
		Cover   *pythonImageScore   `json:"cover"`
		Gallery []pythonImageScore  `json:"gallery"`
		Stats   pythonStats         `json:"stats"`
	}

	if err := json.Unmarshal(resultJSON, &pythonResult); err != nil {
		return nil, fmt.Errorf("failed to parse result JSON: %w", err)
	}

	// Convert to models
	result := &models.ImageSelectionResult{
		TotalImages:    pythonResult.Stats.TotalImages,
		SafeImages:     pythonResult.Stats.SafeImages,
		BlurredImages:  pythonResult.Stats.BlurredImages,
		ProcessingTime: pythonResult.Stats.ProcessingTime,
	}

	if pythonResult.Cover != nil {
		result.Cover = &models.ImageScore{
			URL:            pythonResult.Cover.URL,
			Filename:       pythonResult.Cover.Filename,
			NSFWScore:      pythonResult.Cover.NSFWScore,
			FaceScore:      pythonResult.Cover.FaceScore,
			AestheticScore: pythonResult.Cover.AestheticScore,
			CombinedScore:  pythonResult.Cover.CombinedScore,
			IsSafe:         pythonResult.Cover.IsSafe,
			IsBlurred:      pythonResult.Cover.IsBlurred,
			BlurredPath:    pythonResult.Cover.BlurredPath,
		}
	}

	for _, g := range pythonResult.Gallery {
		result.Gallery = append(result.Gallery, models.ImageScore{
			URL:            g.URL,
			Filename:       g.Filename,
			NSFWScore:      g.NSFWScore,
			FaceScore:      g.FaceScore,
			AestheticScore: g.AestheticScore,
			CombinedScore:  g.CombinedScore,
			IsSafe:         g.IsSafe,
			IsBlurred:      g.IsBlurred,
			BlurredPath:    g.BlurredPath,
		})
	}

	s.logger.InfoContext(ctx, "Image selection completed",
		"total", result.TotalImages,
		"safe", result.SafeImages,
		"blurred", result.BlurredImages,
		"gallery_selected", len(result.Gallery),
		"has_cover", result.Cover != nil,
		"processing_time", time.Since(startTime),
	)

	return result, nil
}

// pythonImageScore - Python output structure
type pythonImageScore struct {
	URL            string  `json:"url"`
	Filename       string  `json:"filename"`
	NSFWScore      float64 `json:"nsfw_score"`
	FaceScore      float64 `json:"face_score"`
	AestheticScore float64 `json:"aesthetic_score"`
	CombinedScore  float64 `json:"combined_score"`
	IsSafe         bool    `json:"is_safe"`
	IsBlurred      bool    `json:"is_blurred"`
	BlurredPath    string  `json:"blurred_path,omitempty"`
}

type pythonStats struct {
	TotalImages    int     `json:"total_images"`
	SafeImages     int     `json:"safe_images"`
	BlurredImages  int     `json:"blurred_images"`
	ProcessingTime float64 `json:"processing_time"`
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
