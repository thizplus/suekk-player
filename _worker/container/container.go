package container

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"

	"suekk-worker/config"
	"suekk-worker/domain/models"
	"suekk-worker/infrastructure/alert"
	"suekk-worker/infrastructure/auth"
	"suekk-worker/infrastructure/cleanup"
	"suekk-worker/infrastructure/consumer"
	"suekk-worker/infrastructure/gallery"
	"suekk-worker/infrastructure/messenger"
	"suekk-worker/infrastructure/monitor"
	"suekk-worker/infrastructure/repository"
	"suekk-worker/infrastructure/storage"
	"suekk-worker/infrastructure/transcoder"
	"suekk-worker/ports"
	"suekk-worker/services"
	"suekk-worker/use_cases"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Container - Dependency Injection Container
// รวมทุก dependencies และจัดการ lifecycle
// ═══════════════════════════════════════════════════════════════════════════════

// Container holds all application dependencies
type Container struct {
	// Configuration
	Config *config.Config

	// External connections
	NATSConn *nats.Conn
	DB       *sql.DB

	// Ports (Interfaces)
	Storage            ports.StoragePort
	Transcoder         ports.TranscoderPort
	Messenger          ports.MessengerPort
	Consumer           ports.ConsumerPort
	Repository         ports.VideoRepository
	DiskMonitor        ports.DiskMonitorPort
	Heartbeat          ports.HeartbeatPort
	Alert              ports.AlertPort
	WarmCachePublisher ports.WarmCachePublisherPort
	SubtitlePublisher  ports.SubtitlePublisherPort

	// Infrastructure (concrete types for internal use)
	diskMonitorImpl *monitor.DiskMonitor
	TempManager     *cleanup.TempManager
	alertImpl       *alert.AlertService
	consumerImpl    *consumer.NATSConsumer
	AuthClient      *auth.AuthClient

	// Services
	TranscodeService *services.TranscodeService
	AudioService     *services.AudioService

	// Gallery Service (shared between handlers)
	GalleryService  *gallery.Service
	GalleryUploader *gallery.Uploader

	// Use Cases
	TranscodeHandler *use_cases.TranscodeHandler
	GalleryHandler   *use_cases.GalleryHandler

	// Gallery Consumer (separate from transcode)
	galleryConsumer *consumer.GalleryConsumer

	// Logger
	logger *slog.Logger
}

// NewContainer สร้าง Container ใหม่และ wire dependencies
func NewContainer(cfg *config.Config) (*Container, error) {
	c := &Container{
		Config: cfg,
		logger: slog.Default().With("component", "container"),
	}

	var err error

	// ─────────────────────────────────────────────────────────────────────────────
	// 1. External Connections
	// ─────────────────────────────────────────────────────────────────────────────

	// NATS Connection
	c.NATSConn, err = nats.Connect(cfg.NATS.URL,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}
	c.logger.Info("connected to NATS", "url", cfg.NATS.URL)

	// Database Connection
	c.DB, err = sql.Open("postgres", cfg.Database.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	if err := c.DB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	c.logger.Info("connected to database")

	// ─────────────────────────────────────────────────────────────────────────────
	// 2. Infrastructure Layer
	// ─────────────────────────────────────────────────────────────────────────────

	// Storage (S3/MinIO/R2)
	storageClient, err := storage.NewS3Client(storage.S3Config{
		Endpoint:  cfg.Storage.Endpoint,
		AccessKey: cfg.Storage.AccessKeyID,
		SecretKey: cfg.Storage.SecretAccessKey,
		Region:    cfg.Storage.Region,
		Bucket:    cfg.Storage.Bucket,
		UseSSL:    cfg.Storage.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}
	c.Storage = storageClient
	c.logger.Info("storage client created", "endpoint", cfg.Storage.Endpoint)

	// Transcoder (FFmpeg)
	c.Transcoder = transcoder.NewFFmpegClient(transcoder.FFmpegConfig{
		UseGPU:  cfg.Transcoder.GPUEnabled,
		Preset:  cfg.Transcoder.Preset,
		HLSTime: cfg.Transcoder.HLSTime,
	})
	c.logger.Info("transcoder created", "gpu_enabled", cfg.Transcoder.GPUEnabled)

	// Messenger (NATS Publisher)
	c.Messenger = messenger.NewNATSPublisher(c.NATSConn, cfg.Worker.ID)
	c.logger.Info("messenger created")

	// Warm Cache Publisher
	warmCachePub, err := messenger.NewWarmCachePublisher(c.NATSConn)
	if err != nil {
		return nil, fmt.Errorf("failed to create warm cache publisher: %w", err)
	}
	c.WarmCachePublisher = warmCachePub
	c.logger.Info("warm cache publisher created")

	// Subtitle Publisher
	subtitlePub, err := messenger.NewSubtitlePublisher(c.NATSConn)
	if err != nil {
		return nil, fmt.Errorf("failed to create subtitle publisher: %w", err)
	}
	c.SubtitlePublisher = subtitlePub
	c.logger.Info("subtitle publisher created")

	// Repository (PostgreSQL)
	c.Repository = repository.NewPostgresClientFromDB(c.DB)
	c.logger.Info("repository created")

	// Temp Manager
	ramCfg := cleanup.RAMDiskConfig{
		Enabled:   cfg.RAMDisk.Enabled,
		Path:      cfg.RAMDisk.Path,
		MinFreeMB: cfg.RAMDisk.MinFreeMB,
	}
	c.TempManager = cleanup.NewTempManagerWithRAM(cfg.TempPath, ramCfg)
	c.logger.Info("temp manager created", "path", cfg.TempPath)

	// Alert Service
	c.alertImpl = alert.NewAlertService(alert.AlertConfig{
		Enabled:        cfg.Alert.Enabled,
		DiscordWebhook: cfg.Alert.DiscordWebhook,
		LineToken:      cfg.Alert.LineToken,
		WorkerID:       cfg.Worker.ID,
	})
	c.Alert = c.alertImpl
	c.logger.Info("alert service created")

	// Disk Monitor
	c.diskMonitorImpl = monitor.NewDiskMonitor(monitor.DiskMonitorConfig{
		TempPath:           cfg.TempPath,
		CheckIntervalSec:   int(cfg.DiskMonitor.CheckInterval.Seconds()),
		WarningThreshold:   cfg.DiskMonitor.WarningPercent,
		CautionThreshold:   cfg.DiskMonitor.CriticalPercent,
		CriticalThreshold:  cfg.DiskMonitor.EmergencyPercent,
	})
	c.diskMonitorImpl.SetCleanupService(c.TempManager)
	c.diskMonitorImpl.SetAlertSender(c.alertImpl)
	c.DiskMonitor = c.diskMonitorImpl
	c.logger.Info("disk monitor created")

	// Heartbeat Publisher
	heartbeatPub, err := monitor.NewHeartbeatPublisher(c.NATSConn, monitor.HeartbeatConfig{
		WorkerID:    cfg.Worker.ID,
		WorkerType:  cfg.Worker.Type,
		Concurrency: cfg.Worker.Concurrency,
		GPUEnabled:  cfg.Transcoder.GPUEnabled,
		Preset:      cfg.Transcoder.Preset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create heartbeat publisher: %w", err)
	}
	heartbeatPub.SetDiskMonitor(c.diskMonitorImpl)
	c.Heartbeat = heartbeatPub
	c.logger.Info("heartbeat publisher created")

	// Consumer (NATS JetStream)
	c.consumerImpl, err = consumer.NewNATSConsumer(consumer.NATSConsumerConfig{
		URL:             cfg.NATS.URL,
		Concurrency:     cfg.Worker.Concurrency,
		WorkerID:        cfg.Worker.ID,
		ShutdownTimeout: 60 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}
	c.Consumer = c.consumerImpl
	// Set disk monitor as job pauser for disk-full scenarios
	c.diskMonitorImpl.SetJobPauser(c.consumerImpl)
	c.logger.Info("consumer created")

	// ─────────────────────────────────────────────────────────────────────────────
	// 3. Services Layer
	// ─────────────────────────────────────────────────────────────────────────────

	// Transcode Service
	c.TranscodeService = services.NewTranscodeService(
		c.Storage,
		c.Transcoder,
		c.Messenger,
		services.TranscodeConfig{
			DefaultQualities: models.DefaultQualities,
			GPUEnabled:       cfg.Transcoder.GPUEnabled,
		},
	)
	c.logger.Info("transcode service created")

	// Audio Service
	c.AudioService = services.NewAudioService(
		c.Storage,
		c.Transcoder,
		services.DefaultAudioConfig(),
	)
	c.logger.Info("audio service created")

	// Gallery Service (shared between TranscodeHandler and GalleryHandler)
	c.GalleryService = gallery.NewService(gallery.DefaultConfig(), c.logger)
	c.GalleryUploader = gallery.NewUploader(c.Storage, c.logger)
	c.logger.Info("gallery service created (shared)")

	// ─────────────────────────────────────────────────────────────────────────────
	// 4. Use Cases Layer
	// ─────────────────────────────────────────────────────────────────────────────

	// Create AuthClient for API authentication
	c.logger.Info("DEBUG: auth config loaded",
		"api_url", cfg.AutoSubtitle.APIURL,
		"email", cfg.AutoSubtitle.Email,
		"password_len", len(cfg.AutoSubtitle.Password),
	)
	c.AuthClient = auth.NewAuthClient(auth.AuthClientConfig{
		APIURL:   cfg.AutoSubtitle.APIURL,
		Email:    cfg.AutoSubtitle.Email,
		Password: cfg.AutoSubtitle.Password,
	}, c.logger)
	if c.AuthClient.IsConfigured() {
		c.logger.Info("auth client created", "api_url", cfg.AutoSubtitle.APIURL, "email", cfg.AutoSubtitle.Email)
		// ทดสอบ login ตอน startup เลย
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		token, err := c.AuthClient.Login(ctx)
		cancel()
		if err != nil {
			c.logger.Error("auth login failed at startup", "error", err)
			return nil, fmt.Errorf("auth login failed: %w", err)
		}
		c.logger.Info("auth login successful at startup", "token_len", len(token))
	} else {
		c.logger.Warn("auth client not fully configured (missing email/password)")
	}

	c.TranscodeHandler = use_cases.NewTranscodeHandler(
		c.TranscodeService,
		c.AudioService,
		c.Storage,
		c.Repository,
		c.Heartbeat,
		c.WarmCachePublisher,
		c.AuthClient,
		c.TempManager,
		c.GalleryService,
		c.GalleryUploader,
		use_cases.TranscodeHandlerConfig{
			AutoSubtitle: use_cases.AutoSubtitleConfig{
				Enabled: cfg.AutoSubtitle.Enabled,
				APIURL:  cfg.AutoSubtitle.APIURL,
			},
		},
	)
	c.logger.Info("transcode handler created")

	// Set handler for consumer
	c.Consumer.SetHandler(c.TranscodeHandler.ProcessJob)

	// Gallery Handler (uses S3 presigned URLs for HLS access)
	// TEST_MODE: Set GALLERY_TEST_MODE=true to skip upload & DB update
	testMode := os.Getenv("GALLERY_TEST_MODE") == "true"
	if testMode {
		c.logger.Warn("========================================")
		c.logger.Warn("GALLERY TEST MODE ENABLED")
		c.logger.Warn("Upload and DB update will be SKIPPED")
		c.logger.Warn("Files will be kept locally for inspection")
		c.logger.Warn("========================================")
	}

	c.GalleryHandler = use_cases.NewGalleryHandler(
		c.Storage,
		c.Messenger,
		c.Repository,
		c.AuthClient,
		c.GalleryService,
		c.GalleryUploader,
		use_cases.GalleryHandlerConfig{
			TempDir:  cfg.TempPath,
			APIURL:   cfg.AutoSubtitle.APIURL, // Reuse API URL from auto subtitle config
			TestMode: testMode,
		},
	)
	c.logger.Info("gallery handler created", "test_mode", testMode)

	// Gallery Consumer
	c.galleryConsumer, err = consumer.NewGalleryConsumer(consumer.GalleryConsumerConfig{
		URL: cfg.NATS.URL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gallery consumer: %w", err)
	}
	// Use ProcessJobWithClassification for NSFW classification
	c.galleryConsumer.SetHandler(c.GalleryHandler.ProcessJobWithClassification)
	c.logger.Info("gallery consumer created (with NSFW classification)")

	c.logger.Info("container initialized successfully")
	return c, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Lifecycle Management
// ─────────────────────────────────────────────────────────────────────────────

// Start เริ่ม services ทั้งหมด
func (c *Container) Start(ctx context.Context) error {
	c.logger.Info("starting container services...")

	// Start disk monitor in goroutine
	go c.DiskMonitor.Start(ctx)
	c.logger.Info("disk monitor started")

	// Start heartbeat
	if err := c.Heartbeat.Start(ctx); err != nil {
		return fmt.Errorf("failed to start heartbeat: %w", err)
	}
	c.logger.Info("heartbeat started")

	// Start background cleanup
	c.TempManager.StartBackgroundCleanup(10 * time.Minute)
	c.logger.Info("background cleanup started")

	// Start gallery consumer in goroutine
	go func() {
		if err := c.galleryConsumer.Start(ctx); err != nil {
			c.logger.Error("gallery consumer error", "error", err)
		}
	}()
	c.logger.Info("gallery consumer started")

	// Start consumer (blocking)
	if err := c.Consumer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start consumer: %w", err)
	}

	return nil
}

// Stop หยุด services ทั้งหมด (graceful shutdown)
func (c *Container) Stop() {
	c.logger.Info("stopping container services...")

	// Set stopping status
	c.Heartbeat.SetStopping()

	// Stop consumers
	c.Consumer.Stop()
	c.logger.Info("transcode consumer stopped")

	if c.galleryConsumer != nil {
		c.galleryConsumer.Stop()
		c.logger.Info("gallery consumer stopped")
	}

	// Stop heartbeat
	c.Heartbeat.Stop()
	c.logger.Info("heartbeat stopped")

	// Stop disk monitor
	c.DiskMonitor.Stop()
	c.logger.Info("disk monitor stopped")

	// Stop background cleanup
	c.TempManager.StopBackgroundCleanup()
	c.logger.Info("background cleanup stopped")

	// Close database
	if c.DB != nil {
		c.DB.Close()
		c.logger.Info("database connection closed")
	}

	// Close NATS
	if c.NATSConn != nil {
		c.NATSConn.Close()
		c.logger.Info("NATS connection closed")
	}

	c.logger.Info("container stopped")
}

// ─────────────────────────────────────────────────────────────────────────────
// Health Checks
// ─────────────────────────────────────────────────────────────────────────────

// HealthCheck ตรวจสอบ health ของ services
func (c *Container) HealthCheck(ctx context.Context) error {
	// Check NATS
	if !c.NATSConn.IsConnected() {
		return fmt.Errorf("NATS not connected")
	}

	// Check Database
	if err := c.DB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Check consumer
	if !c.Consumer.IsRunning() {
		return fmt.Errorf("consumer not running")
	}

	return nil
}

// GetStatus returns current status of the container
func (c *Container) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"nats_connected":   c.NATSConn.IsConnected(),
		"consumer_running": c.Consumer.IsRunning(),
		"consumer_paused":  c.Consumer.IsPaused(),
		"disk_usage":       c.DiskMonitor.GetUsagePercent(),
	}
}
