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

	// ค่าจาก worker เก่าที่ผ่าน IDrive E2 ได้: 5 workers, 6 req/s, window 6
	switch {
	case durationMin < 30:
		return AdaptiveWatcherConfig{
			Mode:           "turbo",
			Workers:        8,
			MaxUncommitted: 100,
			BaseRateLimit:  8.0,
			WindowSize:     6,
			FlushInterval:  3 * time.Second,
		}

	case durationMin < 90:
		return AdaptiveWatcherConfig{
			Mode:           "balanced",
			Workers:        6,
			MaxUncommitted: 100,
			BaseRateLimit:  6.0,
			WindowSize:     6,
			FlushInterval:  4 * time.Second,
		}

	default:
		// Stable Mode: ตรงกับ worker เก่า
		return AdaptiveWatcherConfig{
			Mode:           "stable",
			Workers:        5,
			MaxUncommitted: 100,
			BaseRateLimit:  6.0,
			WindowSize:     6,
			FlushInterval:  5 * time.Second,
		}
	}
}
