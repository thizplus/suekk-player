package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/suekk/my_worker/go/reel/config"
	"github.com/suekk/my_worker/go/reel/domain"
	"github.com/suekk/my_worker/go/reel/domain/ports"
)

type Processor struct {
	config *config.Config
	logger *slog.Logger
	client *http.Client
}

func NewProcessor(cfg *config.Config) *Processor {
	return &Processor{
		config: cfg,
		logger: slog.Default().With("component", "ffmpeg-processor"),
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// ProcessSegments cuts video segments, applies style transformation, and concatenates
func (p *Processor) ProcessSegments(ctx context.Context, inputPath, outputPath string, segments []domain.Segment, style domain.ReelStyle, cropX, cropY int, onProgress ports.ProgressCallback) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided")
	}

	os.MkdirAll(filepath.Dir(outputPath), 0755)

	// Get output dimensions
	outWidth, outHeight := style.GetOutputDimensions()

	// Get source video info for scaling calculations
	info, err := p.GetVideoInfo(inputPath)
	if err != nil {
		p.logger.Warn("Failed to get video info, using defaults", "error", err)
		info = &ports.VideoInfo{Width: 1920, Height: 1080}
	}

	// Create temp directory for segment files
	tempDir := filepath.Dir(outputPath) + "/segments"
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	var segmentFiles []string
	totalSegments := len(segments)

	// Extract and transform each segment
	for i, seg := range segments {
		segFile := filepath.Join(tempDir, fmt.Sprintf("seg_%03d.mp4", i))

		duration := seg.EndTime - seg.StartTime

		// Build complex filter for style transformation
		filter := p.buildStyleFilter(style, info.Width, info.Height, outWidth, outHeight, cropX, cropY)

		// Build encoding args
		var videoArgs []string
		if p.config.GPUEnabled {
			videoArgs = []string{"-c:v", "h264_nvenc", "-preset", "p4", "-b:v", p.config.OutputBitrate}
		} else {
			videoArgs = []string{"-c:v", "libx264", "-preset", "fast", "-b:v", p.config.OutputBitrate}
		}

		args := []string{
			"-y",
			"-ss", fmt.Sprintf("%.3f", seg.StartTime),
			"-i", inputPath,
			"-t", fmt.Sprintf("%.3f", duration),
			"-vf", filter,
			"-r", strconv.Itoa(p.config.OutputFPS),
		}
		args = append(args, videoArgs...)
		args = append(args, "-c:a", "aac", "-b:a", "128k", segFile)

		if err := p.runFFmpeg(ctx, args); err != nil {
			return fmt.Errorf("failed to extract segment %d: %w", i, err)
		}

		segmentFiles = append(segmentFiles, segFile)

		if onProgress != nil {
			pct := float64(i+1) / float64(totalSegments) * 80 // 0-80%
			onProgress(pct, fmt.Sprintf("ตัด segment %d/%d", i+1, totalSegments))
		}
	}

	// If single segment, just move it
	if len(segmentFiles) == 1 {
		return os.Rename(segmentFiles[0], outputPath)
	}

	// Concat multiple segments
	if onProgress != nil {
		onProgress(85, "กำลังรวม segments...")
	}

	return p.concatSegments(ctx, segmentFiles, outputPath)
}

// buildStyleFilter builds FFmpeg filter for style transformation
func (p *Processor) buildStyleFilter(style domain.ReelStyle, srcW, srcH, outW, outH, cropX, cropY int) string {
	// Calculate scale based on style
	switch style {
	case domain.StyleSquare:
		// 1:1 - crop to square from source, then scale
		// Calculate crop dimensions to get 1:1 aspect
		cropSize := srcH
		if srcW < srcH {
			cropSize = srcW
		}
		// Apply cropX/cropY (0-100) to determine crop position
		maxOffsetX := srcW - cropSize
		maxOffsetY := srcH - cropSize
		offsetX := int(float64(maxOffsetX) * float64(cropX) / 100)
		offsetY := int(float64(maxOffsetY) * float64(cropY) / 100)

		return fmt.Sprintf("crop=%d:%d:%d:%d,scale=%d:%d",
			cropSize, cropSize, offsetX, offsetY, outW, outH)

	case domain.StyleFullCover:
		// 9:16 - crop to fill, then scale
		// Calculate crop to get 9:16 aspect from source
		targetAspect := float64(outW) / float64(outH) // 0.5625 for 9:16
		srcAspect := float64(srcW) / float64(srcH)

		var cropW, cropH int
		if srcAspect > targetAspect {
			// Source is wider, crop width
			cropH = srcH
			cropW = int(float64(srcH) * targetAspect)
		} else {
			// Source is taller, crop height
			cropW = srcW
			cropH = int(float64(srcW) / targetAspect)
		}

		// Apply cropX/cropY for position
		maxOffsetX := srcW - cropW
		maxOffsetY := srcH - cropH
		offsetX := int(float64(maxOffsetX) * float64(cropX) / 100)
		offsetY := int(float64(maxOffsetY) * float64(cropY) / 100)

		return fmt.Sprintf("crop=%d:%d:%d:%d,scale=%d:%d",
			cropW, cropH, offsetX, offsetY, outW, outH)

	case domain.StyleLetterbox:
		fallthrough
	default:
		// 9:16 with letterbox - scale to fit, add black bars
		return fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:black",
			outW, outH, outW, outH)
	}
}

// concatSegments concatenates multiple video segments
func (p *Processor) concatSegments(ctx context.Context, segmentFiles []string, outputPath string) error {
	// Create concat file
	concatFile := filepath.Dir(outputPath) + "/concat.txt"
	var lines []string
	for _, f := range segmentFiles {
		// Escape single quotes in path
		escapedPath := strings.ReplaceAll(f, "'", "'\\''")
		lines = append(lines, fmt.Sprintf("file '%s'", escapedPath))
	}
	os.WriteFile(concatFile, []byte(strings.Join(lines, "\n")), 0644)
	defer os.Remove(concatFile)

	args := []string{
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", concatFile,
		"-c", "copy",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, p.config.FFmpegPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to concat segments: %w", err)
	}

	return nil
}

// AddTextOverlay adds text overlays to video
func (p *Processor) AddTextOverlay(ctx context.Context, inputPath, outputPath string, title, line1, line2 string, onProgress ports.ProgressCallback) error {
	if title == "" && line1 == "" && line2 == "" {
		// No text to add, just copy
		return p.copyVideo(ctx, inputPath, outputPath)
	}

	os.MkdirAll(filepath.Dir(outputPath), 0755)

	// Build drawtext filters
	var filters []string

	// Escape text for FFmpeg
	escapeText := func(s string) string {
		s = strings.ReplaceAll(s, ":", "\\:")
		s = strings.ReplaceAll(s, "'", "\\'")
		return s
	}

	fontFile := p.config.FontPath
	fontColor := p.config.FontColor

	// Title at top center
	if title != "" {
		filters = append(filters, fmt.Sprintf(
			"drawtext=text='%s':fontfile='%s':fontsize=%d:fontcolor=%s:x=(w-text_w)/2:y=100:shadowcolor=black:shadowx=2:shadowy=2",
			escapeText(title), fontFile, p.config.TitleFontSize, fontColor))
	}

	// Line1 below title
	if line1 != "" {
		filters = append(filters, fmt.Sprintf(
			"drawtext=text='%s':fontfile='%s':fontsize=%d:fontcolor=%s:x=(w-text_w)/2:y=200:shadowcolor=black:shadowx=2:shadowy=2",
			escapeText(line1), fontFile, p.config.FontSize, fontColor))
	}

	// Line2 at bottom
	if line2 != "" {
		filters = append(filters, fmt.Sprintf(
			"drawtext=text='%s':fontfile='%s':fontsize=%d:fontcolor=%s:x=(w-text_w)/2:y=h-150:shadowcolor=black:shadowx=2:shadowy=2",
			escapeText(line2), fontFile, p.config.FontSize, fontColor))
	}

	filter := strings.Join(filters, ",")

	args := []string{
		"-y",
		"-i", inputPath,
		"-vf", filter,
		"-c:v", "libx264", "-preset", "fast",
		"-c:a", "copy",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, p.config.FFmpegPath, args...)
	if p.config.Verbose {
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add text overlay: %w", err)
	}

	return nil
}

// AddLogoOverlay adds logo overlay to video
func (p *Processor) AddLogoOverlay(ctx context.Context, inputPath, outputPath, logoPath string, onProgress ports.ProgressCallback) error {
	if logoPath == "" {
		return p.copyVideo(ctx, inputPath, outputPath)
	}

	// Check if logo file exists
	if _, err := os.Stat(logoPath); os.IsNotExist(err) {
		p.logger.Warn("Logo file not found, skipping", "path", logoPath)
		return p.copyVideo(ctx, inputPath, outputPath)
	}

	os.MkdirAll(filepath.Dir(outputPath), 0755)

	// Logo at bottom-right with configured width and opacity
	filter := fmt.Sprintf(
		"[1:v]scale=%d:-1,format=rgba,colorchannelmixer=aa=%.2f[logo];[0:v][logo]overlay=W-w-%d:H-h-%d",
		p.config.LogoWidth, p.config.LogoOpacity, p.config.LogoX, p.config.LogoY)

	args := []string{
		"-y",
		"-i", inputPath,
		"-i", logoPath,
		"-filter_complex", filter,
		"-c:v", "libx264", "-preset", "fast",
		"-c:a", "copy",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, p.config.FFmpegPath, args...)
	if p.config.Verbose {
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add logo overlay: %w", err)
	}

	return nil
}

// GenerateTTS generates TTS audio using ElevenLabs API
func (p *Processor) GenerateTTS(ctx context.Context, text, voice, outputPath string) error {
	if p.config.TTSAPIKey == "" {
		return fmt.Errorf("TTS API key not configured")
	}

	if text == "" {
		return fmt.Errorf("no text provided for TTS")
	}

	// Use default voice if not specified
	if voice == "" {
		voice = p.config.TTSDefaultVoice
	}

	// ElevenLabs text-to-speech endpoint
	url := fmt.Sprintf("%s/text-to-speech/%s", p.config.TTSAPIUrl, voice)

	// Request body
	body := map[string]interface{}{
		"text":     text,
		"model_id": "eleven_multilingual_v2",
		"voice_settings": map[string]float64{
			"stability":        0.5,
			"similarity_boost": 0.75,
		},
	}

	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create TTS request: %w", err)
	}

	req.Header.Set("Accept", "audio/mpeg")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", p.config.TTSAPIKey)

	p.logger.Info("Calling ElevenLabs TTS", "voice", voice, "text_length", len(text))

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("TTS API call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("TTS API error: %d - %s", resp.StatusCode, string(body))
	}

	// Save to file
	os.MkdirAll(filepath.Dir(outputPath), 0755)
	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write TTS audio: %w", err)
	}

	p.logger.Info("TTS audio generated", "path", outputPath, "size", written)

	return nil
}

// MergeAudioVideo merges TTS audio with video
func (p *Processor) MergeAudioVideo(ctx context.Context, videoPath, audioPath, outputPath string) error {
	os.MkdirAll(filepath.Dir(outputPath), 0755)

	// Get video duration to decide how to handle audio
	videoDuration, _ := p.GetVideoDuration(videoPath)
	audioDuration, _ := p.GetVideoDuration(audioPath)

	p.logger.Info("Merging audio with video",
		"video_duration", videoDuration,
		"audio_duration", audioDuration,
	)

	// Build filter: mix original audio with TTS, handle duration differences
	// TTS audio is secondary, original audio (if any) is primary
	args := []string{
		"-y",
		"-i", videoPath,
		"-i", audioPath,
	}

	// If TTS is longer than video, we trim it
	// If video is longer, TTS just ends when it ends
	if audioDuration > videoDuration && videoDuration > 0 {
		// Trim audio to match video
		args = append(args,
			"-filter_complex", "[1:a]atrim=0:"+fmt.Sprintf("%.3f", videoDuration)+"[tts];[0:a][tts]amix=inputs=2:duration=first:dropout_transition=2[aout]",
			"-map", "0:v",
			"-map", "[aout]",
		)
	} else {
		// Mix audio, using video duration as reference
		args = append(args,
			"-filter_complex", "[0:a][1:a]amix=inputs=2:duration=first:dropout_transition=2[aout]",
			"-map", "0:v",
			"-map", "[aout]",
		)
	}

	args = append(args,
		"-c:v", "copy",
		"-c:a", "aac",
		"-b:a", "192k",
		"-shortest",
		outputPath,
	)

	cmd := exec.CommandContext(ctx, p.config.FFmpegPath, args...)
	if p.config.Verbose {
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		// Fallback: try simpler approach without mixing
		p.logger.Warn("Audio mix failed, trying simple replace", "error", err)
		return p.replaceAudio(ctx, videoPath, audioPath, outputPath)
	}

	return nil
}

// replaceAudio replaces video audio with TTS (fallback)
func (p *Processor) replaceAudio(ctx context.Context, videoPath, audioPath, outputPath string) error {
	args := []string{
		"-y",
		"-i", videoPath,
		"-i", audioPath,
		"-c:v", "copy",
		"-c:a", "aac",
		"-map", "0:v:0",
		"-map", "1:a:0",
		"-shortest",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, p.config.FFmpegPath, args...)
	return cmd.Run()
}

// GenerateThumbnail creates a thumbnail at specific time
func (p *Processor) GenerateThumbnail(ctx context.Context, videoPath, outputPath string, coverTime float64) error {
	os.MkdirAll(filepath.Dir(outputPath), 0755)

	// If coverTime is -1, use middle of video
	if coverTime < 0 {
		duration, _ := p.GetVideoDuration(videoPath)
		coverTime = duration / 2
	}

	if coverTime < 0 {
		coverTime = 1
	}

	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.2f", coverTime),
		"-i", videoPath,
		"-vframes", "1",
		"-q:v", strconv.Itoa(p.config.JpegQuality),
		outputPath,
	}

	cmd := exec.CommandContext(ctx, p.config.FFmpegPath, args...)
	return cmd.Run()
}

// GetVideoDuration returns video duration in seconds
func (p *Processor) GetVideoDuration(videoPath string) (float64, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	}

	cmd := exec.Command(p.config.FFprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, err
	}

	return duration, nil
}

// GetVideoInfo returns video metadata
func (p *Processor) GetVideoInfo(videoPath string) (*ports.VideoInfo, error) {
	args := []string{
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,r_frame_rate,bit_rate",
		"-show_entries", "format=duration",
		"-of", "json",
		videoPath,
	}

	cmd := exec.Command(p.config.FFprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probe struct {
		Streams []struct {
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			FrameRate string `json:"r_frame_rate"`
			BitRate   string `json:"bit_rate"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}

	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	info := &ports.VideoInfo{}

	if len(probe.Streams) > 0 {
		info.Width = probe.Streams[0].Width
		info.Height = probe.Streams[0].Height

		// Parse frame rate (e.g., "30/1" or "30000/1001")
		if parts := strings.Split(probe.Streams[0].FrameRate, "/"); len(parts) == 2 {
			num, _ := strconv.ParseFloat(parts[0], 64)
			den, _ := strconv.ParseFloat(parts[1], 64)
			if den > 0 {
				info.FPS = num / den
			}
		}

		if probe.Streams[0].BitRate != "" {
			info.Bitrate, _ = strconv.Atoi(probe.Streams[0].BitRate)
		}
	}

	if probe.Format.Duration != "" {
		info.Duration, _ = strconv.ParseFloat(probe.Format.Duration, 64)
	}

	return info, nil
}

// copyVideo copies video without modification
func (p *Processor) copyVideo(ctx context.Context, inputPath, outputPath string) error {
	os.MkdirAll(filepath.Dir(outputPath), 0755)
	args := []string{"-y", "-i", inputPath, "-c", "copy", outputPath}
	return p.runFFmpeg(ctx, args)
}

// runFFmpeg executes FFmpeg command and captures stderr for better error messages
func (p *Processor) runFFmpeg(ctx context.Context, args []string) error {
	cmd := exec.CommandContext(ctx, p.config.FFmpegPath, args...)

	var stderr bytes.Buffer
	if p.config.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	} else {
		cmd.Stderr = &stderr
	}

	if err := cmd.Run(); err != nil {
		// Include stderr in error for debugging
		stderrStr := stderr.String()
		if len(stderrStr) > 500 {
			stderrStr = stderrStr[len(stderrStr)-500:] // Last 500 chars
		}
		return fmt.Errorf("%w\nFFmpeg stderr: %s", err, stderrStr)
	}
	return nil
}
