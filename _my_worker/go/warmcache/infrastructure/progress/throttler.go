package progress

import (
	"sync"
	"time"
)

// Throttler controls the rate of progress updates
// ส่ง update เมื่อ:
// 1. เวลาผ่านไป >= interval (default 2 วินาที)
// 2. Progress เปลี่ยนไป >= percentStep (default 5%)
// 3. Status เป็น completed หรือ failed (ส่งเสมอ)
type Throttler struct {
	interval    time.Duration
	percentStep int

	// Per-job tracking
	lastSent    map[string]time.Time
	lastPercent map[string]int
	mu          sync.Mutex
}

// ThrottlerConfig holds throttler configuration
type ThrottlerConfig struct {
	Interval    time.Duration // ส่งทุกกี่วินาที (default: 2s)
	PercentStep int           // ส่งทุกกี่ % (default: 5)
}

// DefaultConfig returns sensible defaults
func DefaultConfig() ThrottlerConfig {
	return ThrottlerConfig{
		Interval:    2 * time.Second,
		PercentStep: 5,
	}
}

// NewThrottler creates a new progress throttler
func NewThrottler(cfg ThrottlerConfig) *Throttler {
	if cfg.Interval <= 0 {
		cfg.Interval = 2 * time.Second
	}
	if cfg.PercentStep <= 0 {
		cfg.PercentStep = 5
	}

	return &Throttler{
		interval:    cfg.Interval,
		percentStep: cfg.PercentStep,
		lastSent:    make(map[string]time.Time),
		lastPercent: make(map[string]int),
	}
}

// ShouldSend returns true if this update should be sent
func (t *Throttler) ShouldSend(jobID string, progress float64, status string) bool {
	// Always send terminal states immediately
	if status == "completed" || status == "failed" {
		t.Cleanup(jobID)
		return true
	}

	// Always send first update
	t.mu.Lock()
	defer t.mu.Unlock()

	lastSent, exists := t.lastSent[jobID]
	if !exists || progress == 0 {
		t.lastSent[jobID] = time.Now()
		t.lastPercent[jobID] = int(progress)
		return true
	}

	currentPercent := int(progress)
	lastPercent := t.lastPercent[jobID]

	crossedThreshold := (currentPercent / t.percentStep) > (lastPercent / t.percentStep)
	timeElapsed := time.Since(lastSent) >= t.interval

	if crossedThreshold || timeElapsed {
		t.lastSent[jobID] = time.Now()
		t.lastPercent[jobID] = currentPercent
		return true
	}

	return false
}

// Cleanup removes tracking data for a completed job
func (t *Throttler) Cleanup(jobID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.lastSent, jobID)
	delete(t.lastPercent, jobID)
}
