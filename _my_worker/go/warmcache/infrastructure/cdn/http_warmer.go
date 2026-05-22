package cdn

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/suekk/my_worker/go/warmcache/config"
	"github.com/suekk/my_worker/go/warmcache/domain/ports"
)

// HLSAccessClaims JWT claims สำหรับ HLS access
type HLSAccessClaims struct {
	VideoCode string `json:"video_code"`
	VideoID   string `json:"video_id"`
	jwt.RegisteredClaims
}

// HTTPWarmer implements CDNPort using HTTP requests
type HTTPWarmer struct {
	config *config.Config
	client *http.Client
	logger *slog.Logger
}

// NewHTTPWarmer creates a new HTTP warmer
func NewHTTPWarmer(cfg *config.Config) *HTTPWarmer {
	return &HTTPWarmer{
		config: cfg,
		client: &http.Client{
			Timeout: time.Duration(cfg.TimeoutSec) * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        cfg.SegmentConcurrency * 2,
				MaxIdleConnsPerHost: cfg.SegmentConcurrency * 2,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger: slog.Default().With("component", "http-warmer"),
	}
}

// generateStreamToken creates a JWT token for accessing HLS streams
func (w *HTTPWarmer) generateStreamToken(videoCode string) (string, error) {
	if w.config.StreamJWTSecret == "" {
		return "", fmt.Errorf("STREAM_JWT_SECRET not configured")
	}

	claims := HLSAccessClaims{
		VideoCode: videoCode,
		VideoID:   "",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "warmcache-worker",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(w.config.StreamJWTSecret))
}

// WarmCache warms all HLS segments for a video
func (w *HTTPWarmer) WarmCache(ctx context.Context, entityCode string, hlsPath string, segmentCounts map[string]int, onProgress func(percent float64, completed, total int)) (*ports.WarmResult, error) {
	streamToken, err := w.generateStreamToken(entityCode)
	if err != nil {
		return nil, fmt.Errorf("failed to generate stream token: %w", err)
	}

	result := &ports.WarmResult{}

	// Build list of URLs to warm
	urls := w.buildURLList(hlsPath, segmentCounts)
	result.TotalSegments = len(urls)

	if len(urls) == 0 {
		w.logger.Warn("No URLs to warm", "entity_code", entityCode)
		return result, nil
	}

	w.logger.Info("Warming segments",
		"entity_code", entityCode,
		"total_urls", len(urls),
		"concurrency", w.config.SegmentConcurrency,
	)

	// Warm cache with concurrency limit
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, w.config.SegmentConcurrency)

	var cachedHit, cachedMiss, failed int64
	var completed int64
	var errorsMu sync.Mutex
	var errors []string

	totalURLs := int64(len(urls))
	lastProgressPercent := int64(0)

	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			status, _, err := w.warmURL(ctx, u, streamToken)
			if err != nil {
				atomic.AddInt64(&failed, 1)
				errorsMu.Lock()
				if len(errors) < 20 {
					errors = append(errors, fmt.Sprintf("%s: %v", filepath.Base(u), err))
				}
				errorsMu.Unlock()
			} else {
				switch status {
				case "HIT":
					atomic.AddInt64(&cachedHit, 1)
				case "MISS", "EXPIRED", "STALE", "REVALIDATED":
					atomic.AddInt64(&cachedMiss, 1)
				default:
					atomic.AddInt64(&cachedMiss, 1)
				}
			}

			// Report progress every 10%
			done := atomic.AddInt64(&completed, 1)
			currentPercent := (done * 100) / totalURLs
			if currentPercent >= lastProgressPercent+10 {
				atomic.StoreInt64(&lastProgressPercent, currentPercent)
				if onProgress != nil {
					onProgress(float64(currentPercent), int(done), int(totalURLs))
				}
			}
		}(url)
	}

	wg.Wait()

	result.CachedHit = int(cachedHit)
	result.CachedMiss = int(cachedMiss)
	result.Failed = int(failed)
	result.Errors = errors

	return result, nil
}

// VerifyCache samples random segments to verify cache status
func (w *HTTPWarmer) VerifyCache(ctx context.Context, entityCode string, hlsPath string, segmentCounts map[string]int) (*ports.VerifyResult, error) {
	streamToken, err := w.generateStreamToken(entityCode)
	if err != nil {
		return nil, fmt.Errorf("failed to generate stream token: %w", err)
	}

	// Build list of segment URLs only (excluding playlists)
	allURLs := w.buildSegmentURLsOnly(hlsPath, segmentCounts)
	if len(allURLs) == 0 {
		return &ports.VerifyResult{Passed: true, TotalSampled: 0, HitPercent: 100}, nil
	}

	// Sample X% of segments
	sampleSize := len(allURLs) * w.config.VerifySamplePercent / 100
	if sampleSize < 10 {
		sampleSize = 10
	}
	if sampleSize > len(allURLs) {
		sampleSize = len(allURLs)
	}

	// Random sample
	rand.Shuffle(len(allURLs), func(i, j int) {
		allURLs[i], allURLs[j] = allURLs[j], allURLs[i]
	})
	sampledURLs := allURLs[:sampleSize]

	w.logger.Debug("Verifying cache",
		"entity_code", entityCode,
		"total_segments", len(allURLs),
		"sample_size", sampleSize,
	)

	// Check cache status of sampled URLs
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, w.config.SegmentConcurrency)
	var hitCount int64

	for _, url := range sampledURLs {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			status, err := w.checkCacheStatus(ctx, u, streamToken)
			if err == nil && status == "HIT" {
				atomic.AddInt64(&hitCount, 1)
			}
		}(url)
	}

	wg.Wait()

	hitPercent := float64(hitCount) / float64(sampleSize) * 100
	passed := hitPercent >= float64(w.config.VerifyThreshold)

	return &ports.VerifyResult{
		TotalSampled: sampleSize,
		HitCount:     int(hitCount),
		HitPercent:   hitPercent,
		Passed:       passed,
	}, nil
}

// buildURLList builds list of URLs to warm (master.m3u8, playlists, segments)
func (w *HTTPWarmer) buildURLList(hlsPath string, segmentCounts map[string]int) []string {
	var urls []string
	baseURL := strings.TrimSuffix(w.config.CDNURL, "/")
	hlsPath = strings.TrimPrefix(hlsPath, "/")

	// Master playlist
	urls = append(urls, fmt.Sprintf("%s/%s/master.m3u8", baseURL, hlsPath))

	// Quality playlists and segments
	for quality, count := range segmentCounts {
		urls = append(urls, fmt.Sprintf("%s/%s/%s/playlist.m3u8", baseURL, hlsPath, quality))
		for i := 0; i < count; i++ {
			urls = append(urls, fmt.Sprintf("%s/%s/%s/segment_%03d.ts", baseURL, hlsPath, quality, i))
		}
	}

	return urls
}

// buildSegmentURLsOnly builds list of segment URLs only (for verification)
func (w *HTTPWarmer) buildSegmentURLsOnly(hlsPath string, segmentCounts map[string]int) []string {
	var urls []string
	baseURL := strings.TrimSuffix(w.config.CDNURL, "/")
	hlsPath = strings.TrimPrefix(hlsPath, "/")

	for quality, count := range segmentCounts {
		for i := 0; i < count; i++ {
			urls = append(urls, fmt.Sprintf("%s/%s/%s/segment_%03d.ts", baseURL, hlsPath, quality, i))
		}
	}

	return urls
}

// warmURL fetches a single URL to warm the cache
func (w *HTTPWarmer) warmURL(ctx context.Context, url string, streamToken string) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", 0, err
	}

	req.Header.Set("X-Stream-Token", streamToken)
	req.Header.Set("Origin", "https://player.suekk.com")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")

	resp, err := w.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	// Drain body to reuse connection
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == 404 {
		return "SKIP", 404, nil
	}

	if resp.StatusCode != 200 {
		return "", resp.StatusCode, fmt.Errorf("status %d", resp.StatusCode)
	}

	cacheStatus := resp.Header.Get("cf-cache-status")
	return cacheStatus, 200, nil
}

// checkCacheStatus checks cache status using HEAD request (faster)
func (w *HTTPWarmer) checkCacheStatus(ctx context.Context, url string, streamToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("X-Stream-Token", streamToken)
	req.Header.Set("Origin", "https://player.suekk.com")

	resp, err := w.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	return resp.Header.Get("cf-cache-status"), nil
}
