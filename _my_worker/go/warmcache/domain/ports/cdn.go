package ports

import "context"

// WarmResult contains the result of cache warming
type WarmResult struct {
	TotalSegments int
	CachedHit     int // Already cached (cf-cache-status: HIT)
	CachedMiss    int // Newly cached (cf-cache-status: MISS)
	Failed        int // Failed to fetch
	Errors        []string
}

// VerifyResult contains the result of cache verification
type VerifyResult struct {
	TotalSampled int
	HitCount     int
	HitPercent   float64
	Passed       bool
}

// CDNPort defines interface for CDN cache operations
type CDNPort interface {
	// WarmCache warms all HLS segments for a video
	// Returns result with hit/miss/failed counts
	WarmCache(ctx context.Context, entityCode string, hlsPath string, segmentCounts map[string]int, onProgress func(percent float64, completed, total int)) (*WarmResult, error)

	// VerifyCache samples random segments to verify cache status
	// Returns verification result with hit percentage
	VerifyCache(ctx context.Context, entityCode string, hlsPath string, segmentCounts map[string]int) (*VerifyResult, error)
}
