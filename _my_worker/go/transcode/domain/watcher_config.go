package domain

import "time"

// AdaptiveWatcherConfig config ที่ปรับตาม video duration
type AdaptiveWatcherConfig struct {
	Mode           string        // "turbo", "balanced", "stable"
	Workers        int
	MaxUncommitted int64
	BaseRateLimit  float64
	WindowSize     int
	FlushInterval  time.Duration
}

// GetAdaptiveWatcherConfig คืนค่า config ที่เหมาะสมตาม video duration
// - Turbo Mode: วิดีโอสั้น < 30 นาที (aggressive upload)
// - Balanced Mode: วิดีโอกลาง 30-90 นาที
// - Stable Mode: วิดีโอยาว > 90 นาที (conservative to prevent crashes)
func GetAdaptiveWatcherConfig(durationSec float64, renditionCount int) AdaptiveWatcherConfig {
	durationMin := durationSec / 60

	switch {
	case durationMin < 30:
		// Turbo Mode: วิดีโอสั้น - ลด rate limit เพื่อหลีกเลี่ยง S3 throttling
		return AdaptiveWatcherConfig{
			Mode:           "turbo",
			Workers:        30,
			MaxUncommitted: 300,
			BaseRateLimit:  30.0, // ลดจาก 50
			WindowSize:     2,
			FlushInterval:  1 * time.Second,
		}

	case durationMin < 90:
		// Balanced Mode: วิดีโอกลาง - balance speed/stability
		return AdaptiveWatcherConfig{
			Mode:           "balanced",
			Workers:        20,
			MaxUncommitted: 200,
			BaseRateLimit:  25.0, // ลดจาก 40
			WindowSize:     3,
			FlushInterval:  2 * time.Second,
		}

	default:
		// Stable Mode: วิดีโอยาว > 90 นาที - เน้นความเสถียร
		return AdaptiveWatcherConfig{
			Mode:           "stable",
			Workers:        15,
			MaxUncommitted: 150,
			BaseRateLimit:  20.0, // ลดจาก 30
			WindowSize:     4,
			FlushInterval:  2 * time.Second,
		}
	}
}
