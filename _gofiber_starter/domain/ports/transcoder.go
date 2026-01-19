package ports

import "context"

// =====================================================
// Video Codec Configuration - Extensible Architecture
// =====================================================

// VideoCodecType ประเภท codec ที่รองรับ
type VideoCodecType string

const (
	CodecH264 VideoCodecType = "h264"
	CodecH265 VideoCodecType = "h265"
	CodecAV1  VideoCodecType = "av1"
)

// VideoCodecConfig การตั้งค่า codec สำหรับ transcoding
// ออกแบบให้ extensible สำหรับเพิ่ม codec ใหม่ในอนาคต
type VideoCodecConfig struct {
	Type        VideoCodecType // ประเภท codec
	Encoder     string         // ffmpeg encoder name (libx264, libx265, libsvtav1)
	Profile     string         // encoding profile (high, main, etc.)
	Level       string         // encoding level (4.1, 5.1, etc.)
	PixelFormat string         // pixel format (yuv420p, yuv420p10le)
	ExtraArgs   []string       // additional ffmpeg arguments

	// HLS Optimization
	UseGOPAlignment bool // ตั้งค่า GOP size ให้ตรงกับ segment time
	GOPMultiplier   int  // GOP = SegmentTime * FrameRate * GOPMultiplier (default: 1)

	// P2P Streaming Optimization
	KeyintMin int // minimum keyframe interval
}

// DefaultCodecConfigs - Predefined codec configurations
var (
	// H264Config - Default สำหรับ compatibility กับทุก browser
	// เหมาะสำหรับ P2P streaming เพราะรองรับทุก device
	H264Config = VideoCodecConfig{
		Type:            CodecH264,
		Encoder:         "libx264",
		Profile:         "high",
		Level:           "4.1",
		PixelFormat:     "yuv420p",
		UseGOPAlignment: true,
		GOPMultiplier:   1,
		KeyintMin:       25,
		ExtraArgs:       []string{"-movflags", "+faststart"},
	}

	// H265Config - สำหรับ devices ที่รองรับ HEVC
	// ไฟล์เล็กกว่า H264 ~40% ที่คุณภาพเดียวกัน
	H265Config = VideoCodecConfig{
		Type:            CodecH265,
		Encoder:         "libx265",
		Profile:         "main",
		Level:           "",
		PixelFormat:     "yuv420p",
		UseGOPAlignment: true,
		GOPMultiplier:   1,
		KeyintMin:       25,
		ExtraArgs:       []string{"-tag:v", "hvc1"}, // Safari/iOS compatibility
	}

	// AV1Config - Next-gen codec (สำหรับอนาคต)
	// ไฟล์เล็กที่สุด แต่ encoding ช้า และยังไม่รองรับทุก browser
	AV1Config = VideoCodecConfig{
		Type:            CodecAV1,
		Encoder:         "libsvtav1",
		Profile:         "",
		Level:           "",
		PixelFormat:     "yuv420p",
		UseGOPAlignment: true,
		GOPMultiplier:   1,
		KeyintMin:       25,
		ExtraArgs:       []string{},
	}
)

// GetDefaultCodecConfig คืนค่า default config (H264 เพื่อ compatibility)
func GetDefaultCodecConfig() VideoCodecConfig {
	return H264Config
}

// GetCodecConfig คืนค่า config ตาม codec type
func GetCodecConfig(codecType VideoCodecType) VideoCodecConfig {
	switch codecType {
	case CodecH265:
		return H265Config
	case CodecAV1:
		return AV1Config
	default:
		return H264Config
	}
}

// =====================================================
// Transcode Results & Options
// =====================================================

// TranscodeResult ผลลัพธ์จากการ transcode
type TranscodeResult struct {
	HLSPath      string // path to .m3u8 master playlist
	HLSPathH264  string // path to H.264 fallback playlist (ถ้ามี)
	ThumbnailURL string // path to thumbnail image
	Duration     int    // duration in seconds
	Quality      string // detected quality (720p, 1080p, etc.)
	DiskUsage    int64  // total disk usage in bytes
	CodecUsed    string // codec ที่ใช้ในการ encode
}

// QualityProfile การตั้งค่า quality สำหรับ Adaptive Bitrate
type QualityProfile struct {
	Name       string // 1080p, 720p, 480p
	Width      int    // output width (-1 = auto)
	Height     int    // output height
	VideoBPS   int    // video bitrate (bps)
	AudioBPS   int    // audio bitrate (bps)
	CRF        int    // constant rate factor
}

// DefaultQualityProfiles profiles มาตรฐาน
var DefaultQualityProfiles = []QualityProfile{
	{Name: "1080p", Width: -1, Height: 1080, VideoBPS: 5000000, AudioBPS: 192000, CRF: 23},
	{Name: "720p", Width: -1, Height: 720, VideoBPS: 2500000, AudioBPS: 128000, CRF: 25},
	{Name: "480p", Width: -1, Height: 480, VideoBPS: 1000000, AudioBPS: 96000, CRF: 28},
}

// AdaptiveTranscodeOptions ตัวเลือกสำหรับ Adaptive Bitrate
type AdaptiveTranscodeOptions struct {
	InputPath        string           // path to original video
	OutputDir        string           // directory for HLS output
	Qualities        []QualityProfile // quality profiles to generate
	GenerateH264     bool             // also generate H.264 fallback
	CodecConfig      *VideoCodecConfig // codec configuration (default: H264Config)
	Preset           string           // encoding preset
	SegmentTime      int              // HLS segment duration
	OnProgress       ProgressCallback // optional callback for progress updates (0-100)
}

// ProgressCallback callback function for progress updates
type ProgressCallback func(percent int)

// TranscodeOptions ตัวเลือกในการ transcode
type TranscodeOptions struct {
	InputPath   string            // path to original video
	OutputDir   string            // directory for HLS output
	CodecConfig *VideoCodecConfig // codec configuration (default: H264Config)
	AudioCodec  string            // default: aac
	Preset      string            // ultrafast, fast, medium, slow
	CRF         int               // quality (0-51, lower = better, default: 23 for H264)
	SegmentTime int               // HLS segment duration (default: 10 seconds)
	OnProgress  ProgressCallback  // optional callback for progress updates (0-100)
}

// TranscoderPort interface สำหรับ transcoding (Port/Adapter pattern)
type TranscoderPort interface {
	// Transcode แปลงวิดีโอเป็น HLS format (single quality)
	Transcode(ctx context.Context, opts *TranscodeOptions) (*TranscodeResult, error)

	// TranscodeAdaptive แปลงวิดีโอเป็น HLS Adaptive Bitrate (multi-quality)
	TranscodeAdaptive(ctx context.Context, opts *AdaptiveTranscodeOptions) (*TranscodeResult, error)

	// GetVideoInfo ดึงข้อมูลวิดีโอ (duration, resolution, etc.)
	GetVideoInfo(ctx context.Context, inputPath string) (*VideoInfo, error)

	// GenerateThumbnail สร้าง thumbnail จากวิดีโอ
	GenerateThumbnail(ctx context.Context, inputPath, outputPath string, atSecond int) error

	// IsAvailable ตรวจสอบว่า transcoder พร้อมใช้งาน
	IsAvailable() bool

	// GetDiskUsage คำนวณขนาดไฟล์ในโฟลเดอร์
	GetDiskUsage(path string) (int64, error)
}

// VideoInfo ข้อมูลของวิดีโอ
type VideoInfo struct {
	Duration   int    // duration in seconds
	Width      int    // video width
	Height     int    // video height
	Bitrate    int64  // bitrate in bps
	Codec      string // video codec name
	FrameRate  float64
	AudioCodec string
}

// GetQualityLabel แปลง resolution เป็น quality label
func (v *VideoInfo) GetQualityLabel() string {
	switch {
	case v.Height >= 2160:
		return "4K"
	case v.Height >= 1440:
		return "1440p"
	case v.Height >= 1080:
		return "1080p"
	case v.Height >= 720:
		return "720p"
	case v.Height >= 480:
		return "480p"
	default:
		return "SD"
	}
}
