package transcoder

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"gofiber-template/domain/ports"
	"gofiber-template/pkg/logger"
)

type FFmpegConfig struct {
	FFmpegPath  string // path to ffmpeg binary
	FFprobePath string // path to ffprobe binary
}

type FFmpegTranscoder struct {
	ffmpegPath  string
	ffprobePath string
}

func NewFFmpegTranscoder(config FFmpegConfig) (ports.TranscoderPort, error) {
	ffmpegPath := config.FFmpegPath
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}

	ffprobePath := config.FFprobePath
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}

	transcoder := &FFmpegTranscoder{
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
	}

	// ตรวจสอบว่า ffmpeg ใช้งานได้
	if !transcoder.IsAvailable() {
		return nil, fmt.Errorf("ffmpeg not available at path: %s", ffmpegPath)
	}

	return transcoder, nil
}

// IsAvailable ตรวจสอบว่า ffmpeg พร้อมใช้งาน
func (t *FFmpegTranscoder) IsAvailable() bool {
	cmd := exec.Command(t.ffmpegPath, "-version")
	err := cmd.Run()
	return err == nil
}

// GetVideoInfo ดึงข้อมูลวิดีโอด้วย ffprobe
func (t *FFmpegTranscoder) GetVideoInfo(ctx context.Context, inputPath string) (*ports.VideoInfo, error) {
	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		inputPath,
	}

	cmd := exec.CommandContext(ctx, t.ffprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		logger.ErrorContext(ctx, "ffprobe failed", "error", err, "path", inputPath)
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probeData ffprobeOutput
	if err := json.Unmarshal(output, &probeData); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	info := &ports.VideoInfo{}

	// ดึง duration จาก format
	if probeData.Format.Duration != "" {
		if duration, err := strconv.ParseFloat(probeData.Format.Duration, 64); err == nil {
			info.Duration = int(duration)
		}
	}

	// ดึง bitrate
	if probeData.Format.BitRate != "" {
		if bitrate, err := strconv.ParseInt(probeData.Format.BitRate, 10, 64); err == nil {
			info.Bitrate = bitrate
		}
	}

	// ดึงข้อมูล video และ audio streams
	for _, stream := range probeData.Streams {
		switch stream.CodecType {
		case "video":
			info.Width = stream.Width
			info.Height = stream.Height
			info.Codec = stream.CodecName
			if stream.RFrameRate != "" {
				info.FrameRate = parseFrameRate(stream.RFrameRate)
			}
		case "audio":
			info.AudioCodec = stream.CodecName
		}
	}

	return info, nil
}

// Transcode แปลงวิดีโอเป็น HLS format
// ใช้ VideoCodecConfig สำหรับ extensibility - รองรับ H264, H265, AV1
func (t *FFmpegTranscoder) Transcode(ctx context.Context, opts *ports.TranscodeOptions) (*ports.TranscodeResult, error) {
	// ใช้ H264Config เป็น default เพราะรองรับทุก browser
	codecConfig := opts.CodecConfig
	if codecConfig == nil {
		defaultConfig := ports.GetDefaultCodecConfig()
		codecConfig = &defaultConfig
	}

	logger.InfoContext(ctx, "Starting transcoding",
		"input", opts.InputPath,
		"output_dir", opts.OutputDir,
		"codec", string(codecConfig.Type),
		"encoder", codecConfig.Encoder,
	)

	// สร้าง output directory
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// ดึงข้อมูลวิดีโอ
	videoInfo, err := t.GetVideoInfo(ctx, opts.InputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	audioCodec := opts.AudioCodec
	if audioCodec == "" {
		audioCodec = "aac"
	}

	preset := opts.Preset
	if preset == "" {
		preset = "medium"
	}

	crf := opts.CRF
	if crf == 0 {
		crf = 23 // default CRF for H264 (18-28 is good range)
	}

	segmentTime := opts.SegmentTime
	if segmentTime == 0 {
		segmentTime = 10
	}

	// Output files
	masterPlaylist := filepath.Join(opts.OutputDir, "master.m3u8")
	segmentPattern := filepath.Join(opts.OutputDir, "segment_%03d.ts")

	// FFmpeg arguments for HLS - ใช้ค่าจาก VideoCodecConfig
	pixelFormat := codecConfig.PixelFormat
	if pixelFormat == "" {
		pixelFormat = "yuv420p"
	}

	args := []string{
		"-i", opts.InputPath,
		"-c:v", codecConfig.Encoder,
		"-pix_fmt", pixelFormat,
		"-preset", preset,
		"-crf", strconv.Itoa(crf),
	}

	// เพิ่ม profile และ level ถ้ามีกำหนด (H.264 ใช้ high, H.265 ใช้ main)
	if codecConfig.Profile != "" {
		args = append(args, "-profile:v", codecConfig.Profile)
	}
	if codecConfig.Level != "" {
		args = append(args, "-level", codecConfig.Level)
	}

	// คำนวณและเพิ่ม GOP settings สำหรับ HLS optimization และ P2P streaming
	if codecConfig.UseGOPAlignment && videoInfo.FrameRate > 0 {
		gopMultiplier := codecConfig.GOPMultiplier
		if gopMultiplier == 0 {
			gopMultiplier = 1
		}
		// GOP size = SegmentTime × FrameRate × Multiplier
		// เพื่อให้ keyframe ตรงกับจุดเริ่ม segment ทุก segment
		gopSize := int(float64(segmentTime) * videoInfo.FrameRate * float64(gopMultiplier))
		args = append(args, "-g", strconv.Itoa(gopSize))

		// KeyintMin สำหรับ P2P streaming (ไม่ให้ keyframe ถี่เกินไป)
		if codecConfig.KeyintMin > 0 {
			args = append(args, "-keyint_min", strconv.Itoa(codecConfig.KeyintMin))
		}

		logger.InfoContext(ctx, "GOP settings calculated",
			"frame_rate", videoInfo.FrameRate,
			"segment_time", segmentTime,
			"gop_size", gopSize,
			"keyint_min", codecConfig.KeyintMin,
		)
	}

	// เพิ่ม extra args จาก codec config (เช่น -tag:v hvc1 สำหรับ H.265)
	args = append(args, codecConfig.ExtraArgs...)

	args = append(args,
		"-c:a", audioCodec,
		"-b:a", "128k",
		"-ac", "2", // stereo
		"-f", "hls",
		"-hls_time", strconv.Itoa(segmentTime),
		"-hls_list_size", "0", // keep all segments in playlist
		"-hls_segment_filename", segmentPattern,
		"-hls_playlist_type", "vod",
	)

	// Add progress output if callback provided
	if opts.OnProgress != nil {
		args = append(args, "-progress", "pipe:1")
	}

	args = append(args, masterPlaylist)

	logger.InfoContext(ctx, "Executing ffmpeg", "args", strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, t.ffmpegPath, args...)
	cmd.Stderr = os.Stderr

	// Run with progress parsing if callback provided
	if opts.OnProgress != nil {
		if err := t.runWithProgress(ctx, cmd, videoInfo.Duration, opts.OnProgress); err != nil {
			logger.ErrorContext(ctx, "FFmpeg transcoding failed", "error", err)
			return nil, fmt.Errorf("ffmpeg failed: %w", err)
		}
	} else {
		cmd.Stdout = os.Stdout
		if err := cmd.Run(); err != nil {
			logger.ErrorContext(ctx, "FFmpeg transcoding failed", "error", err)
			return nil, fmt.Errorf("ffmpeg failed: %w", err)
		}
	}

	// สร้าง thumbnail
	thumbnailPath := filepath.Join(opts.OutputDir, "thumbnail.jpg")
	thumbnailAt := videoInfo.Duration / 10 // สร้าง thumbnail ที่ 10% ของความยาว
	if thumbnailAt < 1 {
		thumbnailAt = 1
	}

	if err := t.GenerateThumbnail(ctx, opts.InputPath, thumbnailPath, thumbnailAt); err != nil {
		logger.WarnContext(ctx, "Failed to generate thumbnail", "error", err)
		// ไม่ return error เพราะไม่ critical
	}

	logger.InfoContext(ctx, "Transcoding completed",
		"output", masterPlaylist,
		"duration", videoInfo.Duration,
		"quality", videoInfo.GetQualityLabel(),
		"codec_used", codecConfig.Encoder,
	)

	// Normalize paths to use forward slashes
	normalizedHLSPath := strings.ReplaceAll(masterPlaylist, "\\", "/")
	normalizedThumbnailPath := strings.ReplaceAll(thumbnailPath, "\\", "/")

	return &ports.TranscodeResult{
		HLSPath:      normalizedHLSPath,
		ThumbnailURL: normalizedThumbnailPath,
		Duration:     videoInfo.Duration,
		Quality:      videoInfo.GetQualityLabel(),
		CodecUsed:    codecConfig.Encoder,
	}, nil
}

// GenerateThumbnail สร้าง thumbnail จากวิดีโอ
func (t *FFmpegTranscoder) GenerateThumbnail(ctx context.Context, inputPath, outputPath string, atSecond int) error {
	args := []string{
		"-ss", strconv.Itoa(atSecond),
		"-i", inputPath,
		"-vframes", "1",
		"-vf", "scale=320:-1", // width 320px, maintain aspect ratio
		"-q:v", "2",
		"-y",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, t.ffmpegPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate thumbnail: %w", err)
	}

	return nil
}

// ffprobe JSON output structures
type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	CodecName  string `json:"codec_name"`
	CodecType  string `json:"codec_type"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	RFrameRate string `json:"r_frame_rate"`
}

type ffprobeFormat struct {
	Duration string `json:"duration"`
	BitRate  string `json:"bit_rate"`
}

// parseFrameRate แปลง frame rate จาก string (e.g., "30000/1001") เป็น float
func parseFrameRate(rate string) float64 {
	parts := strings.Split(rate, "/")
	if len(parts) != 2 {
		return 0
	}

	num, err1 := strconv.ParseFloat(parts[0], 64)
	den, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || den == 0 {
		return 0
	}

	return num / den
}

// TranscodeAdaptive แปลงวิดีโอเป็น HLS Adaptive Bitrate (multi-quality)
// ใช้ VideoCodecConfig สำหรับ extensibility - default: H264 (รองรับทุก browser)
func (t *FFmpegTranscoder) TranscodeAdaptive(ctx context.Context, opts *ports.AdaptiveTranscodeOptions) (*ports.TranscodeResult, error) {
	// ใช้ H264Config เป็น default เพราะรองรับทุก browser
	codecConfig := opts.CodecConfig
	if codecConfig == nil {
		defaultConfig := ports.GetDefaultCodecConfig()
		codecConfig = &defaultConfig
	}

	logger.InfoContext(ctx, "Starting adaptive transcoding",
		"input", opts.InputPath,
		"output_dir", opts.OutputDir,
		"qualities", len(opts.Qualities),
		"codec", string(codecConfig.Type),
		"encoder", codecConfig.Encoder,
	)

	// สร้าง output directories ตาม codec type
	codecDirName := string(codecConfig.Type) // h264, h265, av1
	primaryDir := filepath.Join(opts.OutputDir, codecDirName)
	h264Dir := filepath.Join(opts.OutputDir, "h264")

	if err := os.MkdirAll(primaryDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create %s directory: %w", codecDirName, err)
	}

	// ดึงข้อมูลวิดีโอต้นฉบับ
	videoInfo, err := t.GetVideoInfo(ctx, opts.InputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	// กำหนด qualities ที่จะ encode (filter ตาม source resolution)
	qualities := opts.Qualities
	if len(qualities) == 0 {
		qualities = filterQualitiesBySource(ports.DefaultQualityProfiles, videoInfo.Height)
	} else {
		qualities = filterQualitiesBySource(qualities, videoInfo.Height)
	}

	preset := opts.Preset
	if preset == "" {
		preset = "medium"
	}

	segmentTime := opts.SegmentTime
	if segmentTime == 0 {
		segmentTime = 10
	}

	// Transcode primary codec - multi-quality
	primaryMasterPlaylist, err := t.transcodeMultiQuality(ctx, opts.InputPath, primaryDir, qualities, codecConfig, videoInfo, preset, segmentTime, opts.OnProgress)
	if err != nil {
		return nil, fmt.Errorf("%s transcoding failed: %w", codecConfig.Encoder, err)
	}

	// Transcode H.264 fallback (ถ้าเปิด และ primary ไม่ใช่ H.264)
	var h264MasterPlaylist string
	if opts.GenerateH264 && codecConfig.Type != ports.CodecH264 {
		if err := os.MkdirAll(h264Dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create h264 directory: %w", err)
		}
		h264Config := ports.H264Config
		// H.264 fallback uses nil progress since it's optional and progress already reported by primary
		h264MasterPlaylist, err = t.transcodeMultiQuality(ctx, opts.InputPath, h264Dir, qualities, &h264Config, videoInfo, preset, segmentTime, nil)
		if err != nil {
			logger.WarnContext(ctx, "H.264 fallback transcoding failed", "error", err)
			// ไม่ return error - H.264 เป็น optional
		}
	}

	// สร้าง thumbnail
	thumbnailPath := filepath.Join(opts.OutputDir, "thumbnail.jpg")
	thumbnailAt := videoInfo.Duration / 10
	if thumbnailAt < 1 {
		thumbnailAt = 1
	}
	if err := t.GenerateThumbnail(ctx, opts.InputPath, thumbnailPath, thumbnailAt); err != nil {
		logger.WarnContext(ctx, "Failed to generate thumbnail", "error", err)
	}

	// คำนวณ disk usage
	diskUsage, _ := t.GetDiskUsage(opts.OutputDir)

	logger.InfoContext(ctx, "Adaptive transcoding completed",
		"primary_playlist", primaryMasterPlaylist,
		"h264_playlist", h264MasterPlaylist,
		"duration", videoInfo.Duration,
		"disk_usage_mb", diskUsage/1024/1024,
		"codec_used", codecConfig.Encoder,
	)

	return &ports.TranscodeResult{
		HLSPath:      strings.ReplaceAll(primaryMasterPlaylist, "\\", "/"),
		HLSPathH264:  strings.ReplaceAll(h264MasterPlaylist, "\\", "/"),
		ThumbnailURL: strings.ReplaceAll(thumbnailPath, "\\", "/"),
		Duration:     videoInfo.Duration,
		Quality:      videoInfo.GetQualityLabel(),
		DiskUsage:    diskUsage,
		CodecUsed:    codecConfig.Encoder,
	}, nil
}

// transcodeMultiQuality ทำ multi-quality HLS transcoding
// ใช้ VideoCodecConfig สำหรับ extensibility และ HLS optimization
func (t *FFmpegTranscoder) transcodeMultiQuality(ctx context.Context, inputPath, outputDir string, qualities []ports.QualityProfile, codecConfig *ports.VideoCodecConfig, videoInfo *ports.VideoInfo, preset string, segmentTime int, onProgress ports.ProgressCallback) (string, error) {
	// สร้าง master playlist content
	var masterPlaylistContent strings.Builder
	masterPlaylistContent.WriteString("#EXTM3U\n")
	masterPlaylistContent.WriteString("#EXT-X-VERSION:3\n")

	// คำนวณ GOP size สำหรับ HLS optimization
	var gopSize int
	if codecConfig.UseGOPAlignment && videoInfo.FrameRate > 0 {
		gopMultiplier := codecConfig.GOPMultiplier
		if gopMultiplier == 0 {
			gopMultiplier = 1
		}
		// GOP size = SegmentTime × FrameRate × Multiplier
		gopSize = int(float64(segmentTime) * videoInfo.FrameRate * float64(gopMultiplier))

		logger.InfoContext(ctx, "GOP settings for multi-quality",
			"frame_rate", videoInfo.FrameRate,
			"segment_time", segmentTime,
			"gop_size", gopSize,
			"keyint_min", codecConfig.KeyintMin,
		)
	}

	// Pixel format (default: yuv420p)
	pixelFormat := codecConfig.PixelFormat
	if pixelFormat == "" {
		pixelFormat = "yuv420p"
	}

	totalQualities := len(qualities)
	for qIndex, q := range qualities {
		// สร้าง quality directory
		qualityDir := filepath.Join(outputDir, q.Name)
		if err := os.MkdirAll(qualityDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create quality directory %s: %w", q.Name, err)
		}

		// Output files
		playlistFile := filepath.Join(qualityDir, "playlist.m3u8")
		segmentPattern := filepath.Join(qualityDir, "segment_%03d.ts")

		// Scale filter
		scaleFilter := fmt.Sprintf("scale=%d:%d", q.Width, q.Height)

		// FFmpeg arguments - ใช้ค่าจาก VideoCodecConfig
		args := []string{
			"-i", inputPath,
			"-vf", scaleFilter,
			"-c:v", codecConfig.Encoder,
			"-pix_fmt", pixelFormat,
			"-preset", preset,
			"-crf", strconv.Itoa(q.CRF),
			"-b:v", strconv.Itoa(q.VideoBPS),
			"-maxrate", strconv.Itoa(int(float64(q.VideoBPS) * 1.5)),
			"-bufsize", strconv.Itoa(q.VideoBPS * 2),
		}

		// เพิ่ม profile และ level ถ้ามีกำหนด
		if codecConfig.Profile != "" {
			args = append(args, "-profile:v", codecConfig.Profile)
		}
		if codecConfig.Level != "" {
			args = append(args, "-level", codecConfig.Level)
		}

		// เพิ่ม GOP settings สำหรับ HLS และ P2P streaming
		if gopSize > 0 {
			args = append(args, "-g", strconv.Itoa(gopSize))
			if codecConfig.KeyintMin > 0 {
				args = append(args, "-keyint_min", strconv.Itoa(codecConfig.KeyintMin))
			}
		}

		// เพิ่ม extra args จาก codec config
		args = append(args, codecConfig.ExtraArgs...)

		args = append(args,
			"-c:a", "aac",
			"-b:a", strconv.Itoa(q.AudioBPS),
			"-ac", "2",
			"-f", "hls",
			"-hls_time", strconv.Itoa(segmentTime),
			"-hls_list_size", "0",
			"-hls_segment_filename", segmentPattern,
			"-hls_playlist_type", "vod",
		)

		// Add progress output if callback provided
		if onProgress != nil {
			args = append(args, "-progress", "pipe:1")
		}

		args = append(args, playlistFile)

		logger.InfoContext(ctx, "Transcoding quality", "quality", q.Name, "encoder", codecConfig.Encoder, "quality_index", qIndex+1, "total", totalQualities)

		cmd := exec.CommandContext(ctx, t.ffmpegPath, args...)
		cmd.Stderr = os.Stderr

		// Run with progress or without
		if onProgress != nil {
			// Create quality-specific progress callback
			qualityProgress := func(ffmpegPercent int) {
				// Calculate overall progress across all qualities
				// Each quality gets an equal share of 0-100
				basePercent := (qIndex * 100) / totalQualities
				qualityShare := 100 / totalQualities
				overallPercent := basePercent + (ffmpegPercent * qualityShare / 100)
				onProgress(overallPercent)
			}
			if err := t.runWithProgress(ctx, cmd, videoInfo.Duration, qualityProgress); err != nil {
				return "", fmt.Errorf("ffmpeg failed for quality %s: %w", q.Name, err)
			}
		} else {
			cmd.Stdout = os.Stdout
			if err := cmd.Run(); err != nil {
				return "", fmt.Errorf("ffmpeg failed for quality %s: %w", q.Name, err)
			}
		}

		// เพิ่มลงใน master playlist
		bandwidth := q.VideoBPS + q.AudioBPS
		resolution := fmt.Sprintf("%dx%d", calculateWidth(q.Height), q.Height)
		masterPlaylistContent.WriteString(fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%s,NAME=\"%s\"\n", bandwidth, resolution, q.Name))
		masterPlaylistContent.WriteString(fmt.Sprintf("%s/playlist.m3u8\n", q.Name))
	}

	// เขียน master playlist
	masterPlaylistPath := filepath.Join(outputDir, "master.m3u8")
	if err := os.WriteFile(masterPlaylistPath, []byte(masterPlaylistContent.String()), 0644); err != nil {
		return "", fmt.Errorf("failed to write master playlist: %w", err)
	}

	return masterPlaylistPath, nil
}

// filterQualitiesBySource กรอง qualities ที่สูงกว่า source resolution ออก
func filterQualitiesBySource(profiles []ports.QualityProfile, sourceHeight int) []ports.QualityProfile {
	var filtered []ports.QualityProfile
	for _, p := range profiles {
		if p.Height <= sourceHeight {
			filtered = append(filtered, p)
		}
	}
	// ถ้าไม่มี quality ที่ต่ำกว่า source ให้ใช้ต่ำสุด
	if len(filtered) == 0 && len(profiles) > 0 {
		filtered = append(filtered, profiles[len(profiles)-1])
	}
	return filtered
}

// calculateWidth คำนวณ width จาก height (16:9 aspect ratio)
func calculateWidth(height int) int {
	return (height * 16) / 9
}

// GetDiskUsage คำนวณขนาดไฟล์ทั้งหมดในโฟลเดอร์
func (t *FFmpegTranscoder) GetDiskUsage(path string) (int64, error) {
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

// runWithProgress รัน FFmpeg command และ parse progress output
// FFmpeg -progress pipe:1 outputs: out_time_us=<microseconds>
func (t *FFmpegTranscoder) runWithProgress(ctx context.Context, cmd *exec.Cmd, totalDuration int, onProgress ports.ProgressCallback) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Parse progress in a goroutine
	go t.parseProgress(ctx, stdout, totalDuration, onProgress)

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}

// parseProgress อ่าน FFmpeg progress output และเรียก callback
func (t *FFmpegTranscoder) parseProgress(ctx context.Context, reader io.Reader, totalDuration int, onProgress ports.ProgressCallback) {
	scanner := bufio.NewScanner(reader)
	lastPercent := -1

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()

		// Parse out_time_us=<microseconds> or out_time_ms=<milliseconds>
		if strings.HasPrefix(line, "out_time_us=") {
			timeUs, err := strconv.ParseInt(strings.TrimPrefix(line, "out_time_us="), 10, 64)
			if err != nil {
				continue
			}

			// Calculate percentage
			currentSeconds := int(timeUs / 1000000)
			if totalDuration > 0 {
				percent := (currentSeconds * 100) / totalDuration
				if percent > 100 {
					percent = 100
				}
				// Only call callback if percent changed (avoid spam)
				if percent != lastPercent && percent >= 0 {
					lastPercent = percent
					onProgress(percent)
				}
			}
		}
	}
}
