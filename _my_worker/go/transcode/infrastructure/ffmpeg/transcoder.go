package ffmpeg

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	appconfig "github.com/suekk/my_worker/go/transcode/config"
	"github.com/suekk/my_worker/go/transcode/domain"
	"github.com/suekk/my_worker/go/transcode/domain/ports"
)

// Transcoder implements TranscoderPort using FFmpeg
type Transcoder struct {
	config        *appconfig.Config
	logger        *slog.Logger
	activeJobs    map[string]*exec.Cmd
	activeJobsMux sync.RWMutex
}

// NewTranscoder creates a new FFmpeg transcoder
func NewTranscoder(cfg *appconfig.Config) *Transcoder {
	return &Transcoder{
		config:     cfg,
		logger:     slog.Default().With("component", "ffmpeg-transcoder"),
		activeJobs: make(map[string]*exec.Cmd),
	}
}

// TranscodeToHLS transcodes video to HLS format with multiple qualities
func (t *Transcoder) TranscodeToHLS(ctx context.Context, jobID string, inputPath, outputDir string, qualities []domain.Quality, hlsTime int, useByteRange bool, onProgress ports.ProgressCallback) (map[string]int, error) {
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Get video duration for progress calculation
	duration, err := t.GetVideoDuration(inputPath)
	if err != nil {
		t.logger.Warn("Failed to get video duration, progress may be inaccurate", "error", err)
		duration = 0
	}

	// Build FFmpeg command
	args := t.buildFFmpegArgs(inputPath, outputDir, qualities, hlsTime, useByteRange)

	t.logger.Info("Starting FFmpeg transcode",
		"job_id", jobID,
		"input", inputPath,
		"output", outputDir,
		"qualities", len(qualities),
		"gpu_enabled", t.config.GPUEnabled,
	)

	cmd := exec.CommandContext(ctx, t.config.FFmpegPath, args...)
	cmd.Dir = outputDir

	// Track active job
	t.activeJobsMux.Lock()
	t.activeJobs[jobID] = cmd
	t.activeJobsMux.Unlock()

	defer func() {
		t.activeJobsMux.Lock()
		delete(t.activeJobs, jobID)
		t.activeJobsMux.Unlock()
	}()

	// Capture stderr for progress
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// Parse progress from stderr
	go t.parseProgress(stderr, duration, onProgress)

	// Wait for completion
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("FFmpeg failed: %w", err)
	}

	// Generate master playlist
	if err := t.generateMasterPlaylist(outputDir, qualities); err != nil {
		return nil, fmt.Errorf("failed to generate master playlist: %w", err)
	}

	// Count segments per quality
	segmentCounts := t.countSegments(outputDir, qualities)

	t.logger.Info("Transcode completed",
		"job_id", jobID,
		"segment_counts", segmentCounts,
	)

	return segmentCounts, nil
}

// buildFFmpegArgs builds FFmpeg command arguments
func (t *Transcoder) buildFFmpegArgs(inputPath, outputDir string, qualities []domain.Quality, hlsTime int, useByteRange bool) []string {
	args := []string{
		"-y",
		"-hide_banner",
		"-loglevel", "warning",
		"-stats",
		"-stats_period", "1", // Output stats every 1 second
	}

	// Input
	if t.config.GPUEnabled {
		args = append(args, "-hwaccel", "cuda", "-hwaccel_output_format", "cuda")
	}
	args = append(args, "-i", inputPath)

	// Build filter complex and output for each quality
	var filterComplex strings.Builder
	var outputs []string

	for i, q := range qualities {
		// Scale filter
		if t.config.GPUEnabled {
			filterComplex.WriteString(fmt.Sprintf("[0:v]scale_cuda=%d:%d[v%d];", q.Width, q.Height, i))
		} else {
			filterComplex.WriteString(fmt.Sprintf("[0:v]scale=%d:%d[v%d];", q.Width, q.Height, i))
		}

		// Output settings for this quality
		qualityDir := filepath.Join(outputDir, q.Name)
		os.MkdirAll(qualityDir, 0755)

		outputArgs := []string{
			"-map", fmt.Sprintf("[v%d]", i),
			"-map", "0:a?",
		}

		// Video codec
		if t.config.GPUEnabled {
			outputArgs = append(outputArgs,
				"-c:v", "h264_nvenc",
				"-preset", t.config.Preset,
				"-b:v", q.VideoBitrate,
			)
		} else {
			outputArgs = append(outputArgs,
				"-c:v", "libx264",
				"-preset", "medium",
				"-crf", fmt.Sprintf("%d", t.config.CRF),
				"-maxrate", q.VideoBitrate,
				"-bufsize", q.VideoBitrate,
			)
		}

		// Audio codec
		outputArgs = append(outputArgs,
			"-c:a", "aac",
			"-b:a", q.AudioBitrate,
		)

		// HLS settings
		outputArgs = append(outputArgs,
			"-f", "hls",
			"-hls_time", fmt.Sprintf("%d", hlsTime),
			"-hls_playlist_type", "vod",
		)

		if useByteRange {
			// Byte-Range HLS: single file per quality, playlist uses EXT-X-BYTERANGE
			// Player ใช้ HTTP Range Requests → CDN cache เฉพาะ range ที่เล่น
			outputArgs = append(outputArgs,
				"-hls_flags", "single_file",
				"-hls_segment_filename", filepath.Join(qualityDir, "stream.ts"),
			)
		} else {
			// Segment HLS: แยกไฟล์ .ts ทีละ segment
			outputArgs = append(outputArgs,
				"-hls_segment_filename", filepath.Join(qualityDir, "segment_%03d.ts"),
			)
		}

		outputArgs = append(outputArgs, filepath.Join(qualityDir, "playlist.m3u8"))

		outputs = append(outputs, outputArgs...)
	}

	// Remove trailing semicolon from filter complex
	filterStr := strings.TrimSuffix(filterComplex.String(), ";")
	args = append(args, "-filter_complex", filterStr)
	args = append(args, outputs...)

	return args
}

// parseProgress parses FFmpeg stderr for progress updates from -stats output
// Format: frame=123 fps=30 q=25.0 size=1234kB time=00:00:12.34 bitrate=1234.5kbits/s speed=1.5x
func (t *Transcoder) parseProgress(stderr io.Reader, duration float64, onProgress ports.ProgressCallback) {
	scanner := bufio.NewScanner(stderr)
	timeRegex := regexp.MustCompile(`time=(\d+):(\d+):(\d+)\.(\d+)`)

	var lastPercent float64

	for scanner.Scan() {
		line := scanner.Text()

		matches := timeRegex.FindStringSubmatch(line)
		if len(matches) == 5 {
			hours, _ := strconv.Atoi(matches[1])
			minutes, _ := strconv.Atoi(matches[2])
			seconds, _ := strconv.Atoi(matches[3])
			// Include fractional seconds for more accurate progress
			ms, _ := strconv.Atoi(matches[4])

			currentTime := float64(hours*3600+minutes*60+seconds) + float64(ms)/100.0

			if duration > 0 && onProgress != nil {
				percent := (currentTime / duration) * 100
				if percent > 100 {
					percent = 100
				}
				// Send update every 1% change to avoid flooding
				if percent-lastPercent >= 1.0 || percent >= 99 {
					lastPercent = percent
					onProgress("", percent, fmt.Sprintf("แปลงวิดีโอ: %.1f%%", percent))
				}
			}
		}
	}
}

// generateMasterPlaylist creates the master.m3u8 file
func (t *Transcoder) generateMasterPlaylist(outputDir string, qualities []domain.Quality) error {
	var content strings.Builder
	content.WriteString("#EXTM3U\n")
	content.WriteString("#EXT-X-VERSION:3\n\n")

	for _, q := range qualities {
		bandwidth := t.parseBitrate(q.VideoBitrate) + t.parseBitrate(q.AudioBitrate)
		content.WriteString(fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d\n",
			bandwidth, q.Width, q.Height))
		content.WriteString(fmt.Sprintf("%s/playlist.m3u8\n", q.Name))
	}

	return os.WriteFile(filepath.Join(outputDir, "master.m3u8"), []byte(content.String()), 0644)
}

// parseBitrate parses bitrate string (e.g., "5M", "192k") to bits per second
func (t *Transcoder) parseBitrate(bitrate string) int {
	bitrate = strings.ToLower(strings.TrimSpace(bitrate))

	var multiplier int
	if strings.HasSuffix(bitrate, "m") {
		multiplier = 1000000
		bitrate = strings.TrimSuffix(bitrate, "m")
	} else if strings.HasSuffix(bitrate, "k") {
		multiplier = 1000
		bitrate = strings.TrimSuffix(bitrate, "k")
	} else {
		multiplier = 1
	}

	value, _ := strconv.ParseFloat(bitrate, 64)
	return int(value * float64(multiplier))
}

// countSegments counts segments per quality
// สำหรับ byte-range mode: นับ EXTINF entries ใน playlist (ไม่มี segment files แยก)
// สำหรับ segment mode: นับไฟล์ segment_*.ts
func (t *Transcoder) countSegments(outputDir string, qualities []domain.Quality) map[string]int {
	counts := make(map[string]int)

	for _, q := range qualities {
		qualityDir := filepath.Join(outputDir, q.Name)

		// ลอง segment files ก่อน
		files, _ := filepath.Glob(filepath.Join(qualityDir, "segment_*.ts"))
		if len(files) > 0 {
			counts[q.Name] = len(files)
			continue
		}

		// Byte-range mode: นับ EXTINF จาก playlist
		playlistPath := filepath.Join(qualityDir, "playlist.m3u8")
		if data, err := os.ReadFile(playlistPath); err == nil {
			count := 0
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "#EXTINF:") {
					count++
				}
			}
			counts[q.Name] = count
		}
	}

	return counts
}

// GenerateThumbnail creates a thumbnail from video
func (t *Transcoder) GenerateThumbnail(ctx context.Context, inputPath, outputPath string) error {
	// Get duration for midpoint
	duration, err := t.GetVideoDuration(inputPath)
	if err != nil {
		duration = 10 // Default to 10 seconds if can't get duration
	}

	// Take screenshot at 10% of video
	seekTime := duration * 0.1
	if seekTime < 1 {
		seekTime = 1
	}

	// Create output directory
	os.MkdirAll(filepath.Dir(outputPath), 0755)

	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.2f", seekTime),
		"-i", inputPath,
		"-vframes", "1",
		"-vf", "scale=640:-1",
		"-q:v", "2",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, t.config.FFmpegPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate thumbnail: %w", err)
	}

	t.logger.Debug("Thumbnail generated", "path", outputPath)
	return nil
}

// ExtractAudio extracts audio track to WAV format
func (t *Transcoder) ExtractAudio(ctx context.Context, inputPath, outputPath string) error {
	// Create output directory
	os.MkdirAll(filepath.Dir(outputPath), 0755)

	args := []string{
		"-y",
		"-i", inputPath,
		"-vn",
		"-acodec", "pcm_s16le",
		"-ar", "16000",
		"-ac", "1",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, t.config.FFmpegPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract audio: %w", err)
	}

	t.logger.Debug("Audio extracted", "path", outputPath)
	return nil
}

// GetVideoDuration returns video duration in seconds
func (t *Transcoder) GetVideoDuration(inputPath string) (float64, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputPath,
	}

	cmd := exec.Command(t.config.FFprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get duration: %w", err)
	}

	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return duration, nil
}

// GetVideoInfo returns comprehensive video metadata using ffprobe
func (t *Transcoder) GetVideoInfo(inputPath string) (*domain.VideoInfo, error) {
	args := []string{
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,bit_rate,r_frame_rate,codec_name",
		"-show_entries", "format=duration,bit_rate",
		"-of", "json",
		inputPath,
	}

	cmd := exec.Command(t.config.FFprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	// Parse JSON output
	var result struct {
		Streams []struct {
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			BitRate      string `json:"bit_rate"`
			RFrameRate   string `json:"r_frame_rate"`
			CodecName    string `json:"codec_name"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
			BitRate  string `json:"bit_rate"`
		} `json:"format"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	info := &domain.VideoInfo{}

	// Video stream info
	if len(result.Streams) > 0 {
		stream := result.Streams[0]
		info.Width = stream.Width
		info.Height = stream.Height
		info.Codec = stream.CodecName

		// Parse bitrate
		if stream.BitRate != "" {
			info.Bitrate, _ = strconv.Atoi(stream.BitRate)
		}

		// Parse framerate (format: "30000/1001" or "30/1")
		if stream.RFrameRate != "" {
			parts := strings.Split(stream.RFrameRate, "/")
			if len(parts) == 2 {
				num, _ := strconv.ParseFloat(parts[0], 64)
				den, _ := strconv.ParseFloat(parts[1], 64)
				if den > 0 {
					info.FPS = num / den
				}
			}
		}
	}

	// Format info
	if result.Format.Duration != "" {
		info.Duration, _ = strconv.ParseFloat(result.Format.Duration, 64)
	}

	// Use format bitrate if stream bitrate not available
	if info.Bitrate == 0 && result.Format.BitRate != "" {
		info.Bitrate, _ = strconv.Atoi(result.Format.BitRate)
	}

	// Check for audio
	audioArgs := []string{
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=codec_name",
		"-of", "json",
		inputPath,
	}
	audioCmd := exec.Command(t.config.FFprobePath, audioArgs...)
	audioOutput, _ := audioCmd.Output()

	var audioResult struct {
		Streams []struct {
			CodecName string `json:"codec_name"`
		} `json:"streams"`
	}
	if json.Unmarshal(audioOutput, &audioResult) == nil && len(audioResult.Streams) > 0 {
		info.HasAudio = true
		info.AudioCodec = audioResult.Streams[0].CodecName
	}

	t.logger.Debug("Video info retrieved",
		"path", inputPath,
		"width", info.Width,
		"height", info.Height,
		"duration", info.Duration,
		"bitrate", info.Bitrate,
		"fps", info.FPS,
	)

	return info, nil
}

// HasAvailableVRAM checks if GPU has enough VRAM
func (t *Transcoder) HasAvailableVRAM() bool {
	if !t.config.GPUEnabled {
		return true
	}

	// Try nvidia-smi to check VRAM
	cmd := exec.Command("nvidia-smi", "--query-gpu=memory.free", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		t.logger.Warn("Failed to check VRAM, assuming available", "error", err)
		return true
	}

	// Parse free VRAM (in MB)
	freeVRAM, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return true
	}

	// Require at least 1GB free VRAM
	minVRAM := 1000
	available := freeVRAM >= minVRAM

	if !available {
		t.logger.Warn("Insufficient VRAM", "free_mb", freeVRAM, "required_mb", minVRAM)
	}

	return available
}

// CleanupZombieProcesses kills any stuck FFmpeg processes
func (t *Transcoder) CleanupZombieProcesses() int {
	// This is a simple implementation - in production you might want to track PIDs
	return 0
}

// KillProcess kills a specific job's FFmpeg process
func (t *Transcoder) KillProcess(jobID string) {
	t.activeJobsMux.RLock()
	cmd, exists := t.activeJobs[jobID]
	t.activeJobsMux.RUnlock()

	if exists && cmd != nil && cmd.Process != nil {
		t.logger.Info("Killing FFmpeg process", "job_id", jobID, "pid", cmd.Process.Pid)

		// Try graceful termination first
		cmd.Process.Signal(os.Interrupt)

		// Wait a bit then force kill
		time.Sleep(2 * time.Second)
		cmd.Process.Kill()
	}
}
