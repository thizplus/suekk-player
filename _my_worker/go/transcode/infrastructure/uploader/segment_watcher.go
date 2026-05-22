package uploader

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/time/rate"

	"github.com/suekk/my_worker/go/transcode/domain/ports"
)

// SegmentWatcher watches for new HLS segments and uploads them in parallel
type SegmentWatcher struct {
	watcher     *fsnotify.Watcher
	storage     ports.StoragePort
	uploadQueue chan uploadTask
	workers     int
	wg          sync.WaitGroup
	logger      *slog.Logger

	// Error tracking
	errors   []error
	errorsMu sync.Mutex

	// Stats
	uploadedCount atomic.Int64
	uploadedBytes atomic.Int64
	queuedCount   atomic.Int64

	// Backpressure settings
	maxUncommitted int64

	// Rate limiter
	rateLimiter    *rate.Limiter
	baseRateLimit  float64
	renditionCount int

	// Window buffer
	windowBuffer  []uploadTask
	windowMu      sync.Mutex
	windowSize    int
	flushInterval time.Duration
	lastFlushTime time.Time

	// Per-quality stats
	qualityBytes        map[string]*atomic.Int64
	qualitySegmentCount map[string]*atomic.Int64
	qualityStatsMu      sync.RWMutex

	// In-flight tracking
	inFlightUploads atomic.Int32

	// Uploaded segments tracking
	uploadedSegments   map[string]bool
	uploadedSegmentsMu sync.RWMutex

	// Configuration
	useTempFile  bool
	pendingFiles map[string]time.Time
	pendingMu    sync.Mutex

	// Context
	ctx    context.Context
	cancel context.CancelFunc

	// Path tracking
	localDir     string
	remotePrefix string
}

type uploadTask struct {
	localPath  string
	remotePath string
}

// Config configuration for SegmentWatcher
type Config struct {
	Workers        int
	UseTempFile    bool
	MaxUncommitted int64
	BaseRateLimit  float64
	RenditionCount int
	WindowSize     int
	FlushInterval  time.Duration
}

// NewSegmentWatcher creates a new SegmentWatcher
func NewSegmentWatcher(storage ports.StoragePort, cfg Config) (*SegmentWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	// Defaults
	if cfg.Workers <= 0 {
		cfg.Workers = 5
	}
	if cfg.MaxUncommitted <= 0 {
		cfg.MaxUncommitted = 100
	}
	if cfg.BaseRateLimit <= 0 {
		cfg.BaseRateLimit = 6.0
	}
	if cfg.RenditionCount <= 0 {
		cfg.RenditionCount = 2
	}
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = 6
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}

	effectiveRate := cfg.BaseRateLimit / float64(cfg.RenditionCount)
	limiter := rate.NewLimiter(rate.Limit(effectiveRate), cfg.WindowSize)

	ctx, cancel := context.WithCancel(context.Background())

	return &SegmentWatcher{
		watcher:             watcher,
		storage:             storage,
		uploadQueue:         make(chan uploadTask, 100),
		workers:             cfg.Workers,
		logger:              slog.Default().With("component", "segment-watcher"),
		useTempFile:         cfg.UseTempFile,
		pendingFiles:        make(map[string]time.Time),
		qualityBytes:        make(map[string]*atomic.Int64),
		qualitySegmentCount: make(map[string]*atomic.Int64),
		maxUncommitted:      cfg.MaxUncommitted,
		rateLimiter:         limiter,
		baseRateLimit:       cfg.BaseRateLimit,
		renditionCount:      cfg.RenditionCount,
		windowBuffer:        make([]uploadTask, 0, cfg.WindowSize),
		windowSize:          cfg.WindowSize,
		flushInterval:       cfg.FlushInterval,
		lastFlushTime:       time.Now(),
		uploadedSegments:    make(map[string]bool),
		ctx:                 ctx,
		cancel:              cancel,
	}, nil
}

// Start begins watching the directory and uploading segments
func (sw *SegmentWatcher) Start(localDir, remotePrefix string) error {
	sw.localDir = localDir
	sw.remotePrefix = remotePrefix

	sw.logger.Info("starting segment watcher",
		"local_dir", localDir,
		"remote_prefix", remotePrefix,
		"workers", sw.workers,
	)

	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for i := 0; i < sw.workers; i++ {
		sw.wg.Add(1)
		go sw.uploadWorker()
	}

	if !sw.useTempFile {
		go sw.stabilityChecker()
	}

	go sw.windowFlusher()

	if err := sw.watcher.Add(localDir); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	go sw.processEvents()

	return nil
}

// Stop stops the watcher and waits for pending uploads
func (sw *SegmentWatcher) Stop() {
	sw.cancel()
	sw.watcher.Close()
	close(sw.uploadQueue)
	sw.wg.Wait()
}

// Wait waits for all pending uploads to complete
func (sw *SegmentWatcher) Wait() error {
	for len(sw.uploadQueue) > 0 {
		time.Sleep(100 * time.Millisecond)
	}

	waitStart := time.Now()
	for sw.inFlightUploads.Load() > 0 {
		if time.Since(waitStart) > 10*time.Minute {
			sw.logger.Warn("timeout waiting for in-flight uploads")
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	sw.Stop()

	sw.errorsMu.Lock()
	defer sw.errorsMu.Unlock()

	if len(sw.errors) > 0 {
		return fmt.Errorf("upload errors: %d failures", len(sw.errors))
	}
	return nil
}

// FlushNow forces an immediate flush of the window buffer
func (sw *SegmentWatcher) FlushNow() {
	sw.flushWindowBuffer()
}

// UploadPlaylists uploads all .m3u8 files
func (sw *SegmentWatcher) UploadPlaylists(ctx context.Context, localDir, remotePrefix string) error {
	sw.logger.Info("uploading playlists", "local_dir", localDir)

	var playlists []string
	err := filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".m3u8") {
			playlists = append(playlists, path)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to find playlists: %w", err)
	}

	for _, playlist := range playlists {
		relPath, _ := filepath.Rel(localDir, playlist)
		remotePath := filepath.Join(remotePrefix, relPath)
		remotePath = strings.ReplaceAll(remotePath, "\\", "/")

		if err := sw.storage.Upload(ctx, remotePath, playlist); err != nil {
			return fmt.Errorf("failed to upload playlist %s: %w", relPath, err)
		}
	}

	sw.logger.Info("playlists uploaded", "count", len(playlists))
	return nil
}

// GetStats returns upload statistics
func (sw *SegmentWatcher) GetStats() (count int64, bytes int64) {
	return sw.uploadedCount.Load(), sw.uploadedBytes.Load()
}

// GetQualitySizes returns bytes per quality
func (sw *SegmentWatcher) GetQualitySizes() map[string]int64 {
	sw.qualityStatsMu.RLock()
	defer sw.qualityStatsMu.RUnlock()

	result := make(map[string]int64)
	for quality, counter := range sw.qualityBytes {
		result[quality] = counter.Load()
	}
	return result
}

// GetQualitySegmentCounts returns segment count per quality
func (sw *SegmentWatcher) GetQualitySegmentCounts() map[string]int {
	sw.qualityStatsMu.RLock()
	defer sw.qualityStatsMu.RUnlock()

	result := make(map[string]int)
	for quality, counter := range sw.qualitySegmentCount {
		result[quality] = int(counter.Load())
	}
	return result
}

// CleanupUploadedSegments deletes all uploaded segments from local disk
func (sw *SegmentWatcher) CleanupUploadedSegments() (deleted int, freedBytes int64) {
	sw.uploadedSegmentsMu.Lock()
	defer sw.uploadedSegmentsMu.Unlock()

	for path := range sw.uploadedSegments {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		if err := os.Remove(path); err != nil {
			sw.logger.Warn("failed to delete segment", "path", path, "error", err)
			continue
		}

		deleted++
		freedBytes += info.Size()
	}

	sw.uploadedSegments = make(map[string]bool)

	sw.logger.Info("cleaned up segments",
		"deleted", deleted,
		"freed_mb", float64(freedBytes)/(1024*1024),
	)

	return deleted, freedBytes
}

// ─────────────────────────────────────────────────────────────────────────────
// Private Methods
// ─────────────────────────────────────────────────────────────────────────────

func (sw *SegmentWatcher) processEvents() {
	for {
		select {
		case <-sw.ctx.Done():
			return
		case event, ok := <-sw.watcher.Events:
			if !ok {
				return
			}
			sw.handleEvent(event)
		case err, ok := <-sw.watcher.Errors:
			if !ok {
				return
			}
			sw.logger.Warn("watcher error (non-critical)", "error", err)
		}
	}
}

func (sw *SegmentWatcher) handleEvent(event fsnotify.Event) {
	if strings.HasSuffix(event.Name, ".tmp") {
		return
	}

	if event.Op&fsnotify.Create == fsnotify.Create {
		info, err := os.Stat(event.Name)
		if err == nil && info.IsDir() {
			// Skip gallery directories to avoid buffer overflow
			dirName := filepath.Base(event.Name)
			if dirName == "gallery" || dirName == "super_safe" || dirName == "safe" || dirName == "nsfw" || dirName == "all" {
				return
			}
			sw.watcher.Add(event.Name)
			return
		}
	}

	ext := filepath.Ext(event.Name)
	isSegment := ext == ".ts"
	isThumbnail := ext == ".jpg" || ext == ".jpeg"

	if !isSegment && !isThumbnail {
		return
	}

	if sw.useTempFile {
		if event.Op&fsnotify.Rename == fsnotify.Rename || event.Op&fsnotify.Create == fsnotify.Create {
			if _, err := os.Stat(event.Name); err == nil {
				sw.queueUpload(event.Name)
			}
		}
	} else {
		if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
			sw.pendingMu.Lock()
			sw.pendingFiles[event.Name] = time.Now()
			sw.pendingMu.Unlock()
		}
	}
}

func (sw *SegmentWatcher) queueUpload(localPath string) {
	relPath, err := filepath.Rel(sw.localDir, localPath)
	if err != nil {
		sw.logger.Error("failed to get relative path", "error", err)
		return
	}

	remotePath := filepath.Join(sw.remotePrefix, relPath)
	remotePath = strings.ReplaceAll(remotePath, "\\", "/")

	// Backpressure
	waitStart := time.Now()
	for sw.getUncommittedCount() >= sw.maxUncommitted {
		if time.Since(waitStart) > 5*time.Minute {
			return
		}
		select {
		case <-sw.ctx.Done():
			return
		case <-time.After(200 * time.Millisecond):
		}
	}

	task := uploadTask{localPath: localPath, remotePath: remotePath}

	sw.windowMu.Lock()
	sw.windowBuffer = append(sw.windowBuffer, task)
	shouldFlush := len(sw.windowBuffer) >= sw.windowSize || time.Since(sw.lastFlushTime) >= sw.flushInterval
	sw.windowMu.Unlock()

	if shouldFlush {
		sw.flushWindowBuffer()
	}
}

func (sw *SegmentWatcher) stabilityChecker() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-sw.ctx.Done():
			return
		case <-ticker.C:
			sw.checkPendingFiles()
		}
	}
}

func (sw *SegmentWatcher) checkPendingFiles() {
	sw.pendingMu.Lock()
	defer sw.pendingMu.Unlock()

	now := time.Now()
	stableThreshold := 1 * time.Second

	for path, lastWrite := range sw.pendingFiles {
		if now.Sub(lastWrite) > stableThreshold {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				sw.queueUpload(path)
			}
			delete(sw.pendingFiles, path)
		}
	}
}

func (sw *SegmentWatcher) windowFlusher() {
	ticker := time.NewTicker(sw.flushInterval / 2)
	defer ticker.Stop()

	for {
		select {
		case <-sw.ctx.Done():
			sw.flushWindowBuffer()
			return
		case <-ticker.C:
			sw.windowMu.Lock()
			shouldFlush := len(sw.windowBuffer) > 0 && time.Since(sw.lastFlushTime) >= sw.flushInterval
			sw.windowMu.Unlock()

			if shouldFlush {
				sw.flushWindowBuffer()
			}
		}
	}
}

func (sw *SegmentWatcher) flushWindowBuffer() {
	sw.windowMu.Lock()
	if len(sw.windowBuffer) == 0 {
		sw.windowMu.Unlock()
		return
	}

	tasks := make([]uploadTask, len(sw.windowBuffer))
	copy(tasks, sw.windowBuffer)
	sw.windowBuffer = sw.windowBuffer[:0]
	sw.lastFlushTime = time.Now()
	sw.windowMu.Unlock()

	for _, task := range tasks {
		sw.queuedCount.Add(1)
		select {
		case sw.uploadQueue <- task:
		case <-sw.ctx.Done():
			return
		}
	}
}

func (sw *SegmentWatcher) uploadWorker() {
	defer sw.wg.Done()

	for {
		select {
		case <-sw.ctx.Done():
			return
		case task, ok := <-sw.uploadQueue:
			if !ok {
				return
			}
			if err := sw.uploadWithRetry(task.localPath, task.remotePath); err != nil {
				sw.errorsMu.Lock()
				sw.errors = append(sw.errors, err)
				sw.errorsMu.Unlock()
			}
		}
	}
}

func (sw *SegmentWatcher) uploadWithRetry(localPath, remotePath string) error {
	sw.inFlightUploads.Add(1)
	defer sw.inFlightUploads.Add(-1)

	if err := sw.rateLimiter.Wait(sw.ctx); err != nil {
		return fmt.Errorf("rate limiter cancelled: %w", err)
	}

	maxRetries := 5
	backoff := time.Second

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		info, err := os.Stat(localPath)
		if err != nil {
			return fmt.Errorf("file not found: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		err = sw.storage.Upload(ctx, remotePath, localPath)
		cancel()

		if err == nil {
			sw.uploadedCount.Add(1)
			sw.uploadedBytes.Add(info.Size())

			quality := sw.extractQuality(remotePath)
			if quality != "" {
				sw.addQualityStats(quality, info.Size())
			}

			sw.markSegmentUploaded(localPath)
			return nil
		}

		lastErr = err

		if isRateLimitError(err) {
			waitTime := 30*time.Second + time.Duration(rand.Intn(30))*time.Second
			sw.logger.Warn("rate limited, backing off", "wait_seconds", waitTime.Seconds())

			select {
			case <-sw.ctx.Done():
				return sw.ctx.Err()
			case <-time.After(waitTime):
				continue
			}
		}

		select {
		case <-sw.ctx.Done():
			return sw.ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}

	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

func (sw *SegmentWatcher) getUncommittedCount() int64 {
	uncommitted := sw.queuedCount.Load() - sw.uploadedCount.Load()
	if uncommitted < 0 {
		return 0
	}
	return uncommitted
}

func (sw *SegmentWatcher) extractQuality(remotePath string) string {
	parts := strings.Split(remotePath, "/")
	knownQualities := map[string]bool{
		"1080p": true, "720p": true, "480p": true, "360p": true, "240p": true, "4k": true, "2160p": true,
	}
	for _, part := range parts {
		if knownQualities[strings.ToLower(part)] {
			return part
		}
	}
	return ""
}

func (sw *SegmentWatcher) addQualityStats(quality string, bytes int64) {
	sw.qualityStatsMu.Lock()
	defer sw.qualityStatsMu.Unlock()

	if _, exists := sw.qualityBytes[quality]; !exists {
		sw.qualityBytes[quality] = &atomic.Int64{}
	}
	sw.qualityBytes[quality].Add(bytes)

	if _, exists := sw.qualitySegmentCount[quality]; !exists {
		sw.qualitySegmentCount[quality] = &atomic.Int64{}
	}
	sw.qualitySegmentCount[quality].Add(1)
}

func (sw *SegmentWatcher) markSegmentUploaded(localPath string) {
	sw.uploadedSegmentsMu.Lock()
	defer sw.uploadedSegmentsMu.Unlock()
	sw.uploadedSegments[localPath] = true
}

func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "rate limit")
}
