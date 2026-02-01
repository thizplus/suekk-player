package di

import (
	"context"
	"fmt"
	"time"

	"gofiber-template/application/serviceimpl"
	"gofiber-template/domain/ports"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/infrastructure/messaging"
	natspkg "gofiber-template/infrastructure/nats"
	"gofiber-template/infrastructure/postgres"
	redispkg "gofiber-template/infrastructure/redis"
	"gofiber-template/infrastructure/storage"
	"gofiber-template/infrastructure/telegram"
	"gofiber-template/infrastructure/transcoder"
	"gofiber-template/infrastructure/websocket"
	"gofiber-template/interfaces/api/handlers"
	"gofiber-template/pkg/config"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/scheduler"
	"gofiber-template/pkg/settings"

	"gorm.io/gorm"
)

type Container struct {
	// Configuration
	Config *config.Config

	// Infrastructure
	DB             *gorm.DB
	RedisClient    *redispkg.Client        // Redis client สำหรับ cache (optional)
	NATSClient     *natspkg.Client         // NATS connection + JetStream
	NATSPublisher  *natspkg.Publisher      // Publish jobs to JetStream
	Storage        ports.StoragePort       // Port/Adapter pattern
	Transcoder     ports.TranscoderPort    // FFmpeg transcoder
	EventScheduler scheduler.EventScheduler

	// Services (Shared)
	StreamCookieService *serviceimpl.StreamCookieService // Signed cookie สำหรับ CDN access

	// Repositories
	UserRepository             repositories.UserRepository
	TaskRepository             repositories.TaskRepository
	FileRepository             repositories.FileRepository
	JobRepository              repositories.JobRepository
	VideoRepository            repositories.VideoRepository
	CategoryRepository         repositories.CategoryRepository
	AllowedDomainRepository    repositories.AllowedDomainRepository
	WhitelistRepository        repositories.WhitelistRepository
	AdStatsRepository          repositories.AdStatsRepository
	SettingRepository          repositories.SettingRepository
	SubtitleRepository         repositories.SubtitleRepository

	// Services
	UserService            services.UserService
	TaskService            services.TaskService
	FileService            services.FileService
	JobService             services.JobService
	VideoService           services.VideoService
	CategoryService        services.CategoryService
	TranscodingService     services.TranscodingService
	StorageService         services.StorageService
	WhitelistService       services.WhitelistService
	SettingService         services.SettingService
	SubtitleService        services.SubtitleService
	QueueService           services.QueueService

	// Settings Cache
	SettingsCache *settings.SettingsCache

	// WebSocket & Broadcasting
	NATSSubscriber       *natspkg.Subscriber            // NATS Pub/Sub subscriber
	ProgressBroadcaster  *websocket.ProgressBroadcaster // Progress → WebSocket

	// Messaging Ports (Clean Architecture interfaces)
	JobQueue           ports.JobQueuePort           // Job queue abstraction
	ProgressPublisher  ports.ProgressPublisherPort  // Progress publisher abstraction
	ProgressSubscriber ports.ProgressSubscriberPort // Progress subscriber abstraction

	// Notifications
	Notifier      ports.NotifierPort       // Telegram/Email notifications
	DLQSubscriber *natspkg.DLQSubscriber   // DLQ notification subscriber
}

func NewContainer() *Container {
	return &Container{}
}

func (c *Container) Initialize() error {
	if err := c.initConfig(); err != nil {
		return err
	}

	if err := c.initLogger(); err != nil {
		return err
	}

	if err := c.initInfrastructure(); err != nil {
		return err
	}

	if err := c.initRepositories(); err != nil {
		return err
	}

	if err := c.initServices(); err != nil {
		return err
	}

	if err := c.initScheduler(); err != nil {
		return err
	}

	if err := c.initTranscoding(); err != nil {
		return err
	}

	// Initialize QueueService after TranscodingService is available
	c.QueueService = serviceimpl.NewQueueService(
		c.VideoRepository,
		c.SubtitleRepository,
		c.TranscodingService,
		c.SubtitleService,
		c.NATSPublisher,
		c.NATSClient, // สำหรับ PurgeSubtitleStream
	)
	logger.Info("Queue service initialized")

	if err := c.initStorageCleanup(); err != nil {
		return err
	}

	if err := c.initStuckDetector(); err != nil {
		return err
	}

	if err := c.initProgressBroadcaster(); err != nil {
		return err
	}

	if err := c.initNotifications(); err != nil {
		return err
	}

	// Inject notifier หลังจาก initNotifications
	c.injectNotifierToProgressBroadcaster()

	return nil
}

func (c *Container) initConfig() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	c.Config = cfg
	logger.Info("Configuration loaded")
	return nil
}

func (c *Container) initLogger() error {
	logConfig := logger.Config{
		Level:      c.Config.Log.Level,
		Format:     c.Config.Log.Format,
		Output:     c.Config.Log.Output,
		FilePath:   c.Config.Log.FilePath,
		MaxSize:    c.Config.Log.MaxSize,
		MaxBackups: c.Config.Log.MaxBackups,
		MaxAge:     c.Config.Log.MaxAge,
		Compress:   c.Config.Log.Compress,
	}

	if err := logger.Init(logConfig); err != nil {
		return err
	}

	logger.Info("Logger initialized",
		"level", c.Config.Log.Level,
		"format", c.Config.Log.Format,
		"output", c.Config.Log.Output,
		"file", c.Config.Log.FilePath,
	)
	return nil
}

func (c *Container) initInfrastructure() error {
	// Initialize Database
	dbConfig := postgres.DatabaseConfig{
		Host:     c.Config.Database.Host,
		Port:     c.Config.Database.Port,
		User:     c.Config.Database.User,
		Password: c.Config.Database.Password,
		DBName:   c.Config.Database.DBName,
		SSLMode:  c.Config.Database.SSLMode,
	}

	db, err := postgres.NewDatabase(dbConfig)
	if err != nil {
		return err
	}
	c.DB = db
	logger.Info("Database connected", "host", c.Config.Database.Host, "db", c.Config.Database.DBName)

	// Run migrations
	if err := postgres.Migrate(db); err != nil {
		return err
	}
	logger.Info("Database migrated")

	// Initialize Redis Client (optional - graceful degradation)
	if c.Config.Redis.URL != "" {
		redisClient, err := redispkg.NewClient(&c.Config.Redis)
		if err != nil {
			logger.Warn("Redis client initialization failed (cache disabled)", "error", err)
		} else {
			c.RedisClient = redisClient
			logger.Info("Redis client initialized", "url", c.Config.Redis.URL)
		}
	}

	// Initialize Stream Cookie Service
	if c.Config.Stream.CookieKey != "" && c.Config.Stream.CookieKey != "change-this-to-a-secure-32-char-key" {
		c.StreamCookieService = serviceimpl.NewStreamCookieService(&c.Config.Stream)
		logger.Info("Stream cookie service initialized",
			"domain", c.Config.Stream.CookieDomain,
			"max_age", c.Config.Stream.CookieMaxAge,
		)
	} else {
		logger.Warn("Stream cookie service disabled (STREAM_COOKIE_KEY not configured)")
	}

	// Initialize NATS Client + JetStream
	natsConfig := natspkg.ClientConfig{
		URL: c.Config.NATS.URL,
	}
	natsClient, err := natspkg.NewClient(natsConfig)
	if err != nil {
		logger.Warn("NATS client initialization failed", "error", err)
	} else {
		c.NATSClient = natsClient
		c.NATSPublisher = natspkg.NewPublisher(natsClient)
		logger.Info("NATS client initialized", "url", c.Config.NATS.URL)

		// Initialize Messaging Ports (Clean Architecture)
		c.initMessagingPorts()
	}

	// Initialize Storage (Port/Adapter pattern)
	if err := c.initStorage(); err != nil {
		return err
	}

	return nil
}

// initStorage สร้าง storage adapter ตาม config
func (c *Container) initStorage() error {
	switch c.Config.Storage.Type {
	case "s3":
		// S3-Compatible Storage (MinIO / Cloudflare R2)
		s3Config := storage.S3StorageConfig{
			Endpoint:  c.Config.Storage.S3.Endpoint,
			AccessKey: c.Config.Storage.S3.AccessKey,
			SecretKey: c.Config.Storage.S3.SecretKey,
			Bucket:    c.Config.Storage.S3.Bucket,
			UseSSL:    c.Config.Storage.S3.UseSSL,
			Region:    c.Config.Storage.S3.Region,
			PublicURL: c.Config.Storage.S3.PublicURL,
		}
		s3Storage, err := storage.NewS3Storage(s3Config)
		if err != nil {
			return fmt.Errorf("failed to initialize S3 storage: %w", err)
		}
		c.Storage = s3Storage
		logger.Info("S3 Storage initialized",
			"endpoint", c.Config.Storage.S3.Endpoint,
			"bucket", c.Config.Storage.S3.Bucket,
		)

	case "local":
		localConfig := storage.LocalStorageConfig{
			BasePath: c.Config.Storage.BasePath,
			BaseURL:  c.Config.Storage.BaseURL,
		}
		localStorage, err := storage.NewLocalStorage(localConfig)
		if err != nil {
			return fmt.Errorf("failed to initialize local storage: %w", err)
		}
		c.Storage = localStorage
		logger.Info("Local Storage initialized", "path", c.Config.Storage.BasePath)

	default:
		// Default to local storage
		localConfig := storage.LocalStorageConfig{
			BasePath: c.Config.Storage.BasePath,
			BaseURL:  c.Config.Storage.BaseURL,
		}
		localStorage, err := storage.NewLocalStorage(localConfig)
		if err != nil {
			return fmt.Errorf("failed to initialize local storage: %w", err)
		}
		c.Storage = localStorage
		logger.Info("Local Storage initialized (default)", "path", c.Config.Storage.BasePath)
	}

	return nil
}

// initMessagingPorts สร้าง messaging adapters (Clean Architecture)
func (c *Container) initMessagingPorts() {
	if c.NATSClient == nil {
		logger.Warn("Skipping messaging ports initialization (NATS not available)")
		return
	}

	// Job Queue Port
	c.JobQueue = messaging.NewNATSJobQueue(c.NATSClient, c.NATSPublisher)

	// Progress Publisher Port
	c.ProgressPublisher = messaging.NewNATSProgressPublisher(c.NATSClient.Conn())

	// Progress Subscriber Port
	natsSubscriber := natspkg.NewSubscriber(c.NATSClient.Conn())
	c.NATSSubscriber = natsSubscriber // เก็บ concrete type สำหรับ cleanup
	c.ProgressSubscriber = messaging.NewNATSProgressSubscriber(natsSubscriber)

	logger.Info("Messaging ports initialized (Clean Architecture)")
}

func (c *Container) initRepositories() error {
	c.UserRepository = postgres.NewUserRepository(c.DB)
	c.TaskRepository = postgres.NewTaskRepository(c.DB)
	c.FileRepository = postgres.NewFileRepository(c.DB)
	c.JobRepository = postgres.NewJobRepository(c.DB)
	c.VideoRepository = postgres.NewVideoRepository(c.DB)
	c.CategoryRepository = postgres.NewCategoryRepository(c.DB)
	c.AllowedDomainRepository = postgres.NewAllowedDomainRepository(c.DB)
	// Phase 6: Whitelist & Ad Stats
	c.WhitelistRepository = postgres.NewWhitelistRepository(c.DB)
	c.AdStatsRepository = postgres.NewAdStatsRepository(c.DB)
	// Admin Settings
	c.SettingRepository = postgres.NewSettingRepository(c.DB)
	// Subtitle
	c.SubtitleRepository = postgres.NewSubtitleRepository(c.DB)
	logger.Info("Repositories initialized")
	return nil
}

func (c *Container) initServices() error {
	c.UserService = serviceimpl.NewUserService(
		c.UserRepository,
		c.Config.JWT.Secret,
		c.Config.Google.ClientID,
		c.Config.Google.ClientSecret,
		c.Config.Google.RedirectURL,
	)
	c.TaskService = serviceimpl.NewTaskService(c.TaskRepository, c.UserRepository)
	c.FileService = serviceimpl.NewFileService(c.FileRepository, c.UserRepository, c.Storage)
	c.CategoryService = serviceimpl.NewCategoryService(c.CategoryRepository)

	// Video Service (with optional Redis cache + Singleflight locking)
	if c.RedisClient != nil {
		c.VideoService = serviceimpl.NewVideoServiceWithCache(
			c.VideoRepository,
			c.CategoryRepository,
			c.UserRepository,
			c.SubtitleRepository,
			c.Storage,
			c.RedisClient,
			c.Config,
		)
		logger.Info("Video service initialized with Redis cache")
	} else {
		c.VideoService = serviceimpl.NewVideoService(c.VideoRepository, c.CategoryRepository, c.UserRepository, c.SubtitleRepository, c.Storage, c.Config)
		logger.Info("Video service initialized without cache")
	}

	// Phase 6: Whitelist Service (with optional Redis cache)
	if c.RedisClient != nil {
		c.WhitelistService = serviceimpl.NewWhitelistServiceWithCache(
			c.WhitelistRepository,
			c.AdStatsRepository,
			c.RedisClient,
		)
		logger.Info("Whitelist service initialized with Redis cache")
	} else {
		c.WhitelistService = serviceimpl.NewWhitelistService(
			c.WhitelistRepository,
			c.AdStatsRepository,
		)
		logger.Info("Whitelist service initialized without cache")
	}

	// Admin Settings Service with cache
	c.SettingsCache = settings.InitCache(c.SettingRepository)
	c.SettingService = serviceimpl.NewSettingService(c.SettingRepository, c.SettingsCache)

	// Initialize default settings in database
	ctx := context.Background()
	if err := c.SettingService.InitializeDefaults(ctx); err != nil {
		logger.Warn("Failed to initialize default settings", "error", err)
	}

	// Subtitle Service with NATS job publisher
	c.SubtitleService = serviceimpl.NewSubtitleService(c.VideoRepository, c.SubtitleRepository, c.NATSPublisher)
	logger.Info("Subtitle service initialized", "has_publisher", c.NATSPublisher != nil)

	// Queue Service (unified queue management)
	// Note: TranscodingService ต้องถูก init ก่อนใน initTranscoding()
	// จึงย้ายไป init หลังจาก initTranscoding()
	logger.Info("Queue service will be initialized after transcoding")

	logger.Info("Services initialized")
	return nil
}

func (c *Container) initScheduler() error {
	c.EventScheduler = scheduler.NewEventScheduler()
	c.JobService = serviceimpl.NewJobService(c.JobRepository, c.EventScheduler)

	// Start the scheduler
	c.EventScheduler.Start()
	logger.Info("Event scheduler started")

	// Load and schedule existing active jobs
	ctx := context.Background()
	jobs, _, err := c.JobService.ListJobs(ctx, 0, 1000)
	if err != nil {
		logger.Warn("Failed to load existing jobs", "error", err)
		return nil
	}

	activeJobCount := 0
	for _, job := range jobs {
		if job.IsActive {
			err := c.EventScheduler.AddJob(job.ID.String(), job.CronExpr, func() {
				c.JobService.ExecuteJob(ctx, job)
			})
			if err != nil {
				logger.Warn("Failed to schedule job", "job", job.Name, "error", err)
			} else {
				activeJobCount++
			}
		}
	}

	if activeJobCount > 0 {
		logger.Info("Scheduled active jobs", "count", activeJobCount)
	}

	return nil
}

func (c *Container) initTranscoding() error {
	// Initialize FFmpeg Transcoder
	ffmpegConfig := transcoder.FFmpegConfig{
		FFmpegPath:  c.Config.Storage.FFmpegPath,
		FFprobePath: "ffprobe", // ใช้ ffprobe จาก PATH
	}

	trans, err := transcoder.NewFFmpegTranscoder(ffmpegConfig)
	if err != nil {
		logger.Warn("FFmpeg not available, transcoding will be disabled", "error", err)
		// ไม่ return error เพราะไม่ critical - สามารถรัน app ได้โดยไม่มี transcoding
	} else {
		c.Transcoder = trans
		logger.Info("FFmpeg Transcoder initialized", "path", c.Config.Storage.FFmpegPath)
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// Transcoding Service (NATS Distributed Workers Only)
	// Jobs จะถูกส่งไป NATS JetStream ให้ standalone worker service ประมวลผล
	// ═══════════════════════════════════════════════════════════════════════════════
	if c.Transcoder != nil && c.JobQueue != nil {
		transConfig := serviceimpl.TranscodingConfig{
			VideoBasePath:    c.Config.Storage.BasePath,
			TempPath:         c.Config.Storage.TempPath,
			CleanupOriginal:  c.Config.Storage.CleanupOriginal,
			FFmpegPreset:     "medium",
			CRF:              28,
			DefaultQualities: c.Config.Storage.TranscodeQualities,
		}

		c.TranscodingService = serviceimpl.NewTranscodingService(
			c.VideoRepository,
			c.Transcoder,
			c.Storage,
			c.JobQueue,
			c.SettingService,
			transConfig,
		)
		logger.Info("Transcoding Service initialized",
			"mode", "nats",
			"description", "Jobs sent to NATS for distributed workers",
		)
	}

	return nil
}

func (c *Container) initStorageCleanup() error {
	// Initialize Storage Cleanup Service
	cleanupConfig := serviceimpl.StorageCleanupConfig{
		VideoBasePath:     c.Config.Storage.BasePath,
		TempPath:          c.Config.Storage.TempPath,
		CleanupCron:       "0 3 * * *",        // 3 AM daily
		TempFileMaxAge:    24 * time.Hour,     // 24 hours
		FailedVideoMaxAge: 7 * 24 * time.Hour, // 7 days
		MinFreeSpaceGB:    10,
	}

	c.StorageService = serviceimpl.NewStorageCleanupService(
		cleanupConfig,
		c.VideoRepository,
		c.EventScheduler,
	)

	// Register cleanup job with scheduler
	if err := c.StorageService.RegisterCleanupJob(); err != nil {
		logger.Warn("Failed to register storage cleanup job", "error", err)
	} else {
		logger.Info("Storage cleanup job registered", "cron", cleanupConfig.CleanupCron)
	}

	logger.Info("Storage Cleanup Service initialized")
	return nil
}

func (c *Container) initStuckDetector() error {
	// Initialize Stuck Detector Service
	// ตรวจจับ jobs ที่ค้างและ mark เป็น failed อัตโนมัติ
	detectorConfig := serviceimpl.StuckDetectorConfig{
		CheckInterval:     30 * time.Second, // ตรวจสอบทุก 30 วินาที
		ProcessingTimeout: 10 * time.Minute, // ถ้า processing > 10 นาที = stuck (worker crash)
		PendingTimeout:    5 * time.Minute,  // ถ้า pending > 5 นาที = stuck (publish failed)
		// ไม่มี QueuedTimeout - jobs รอใน queue ได้นานเท่าที่ต้องการ
	}

	stuckDetector := serviceimpl.NewStuckDetectorService(
		detectorConfig,
		c.VideoRepository,
		c.EventScheduler,
	)

	// Register detector job with scheduler
	if err := stuckDetector.RegisterDetectorJob(); err != nil {
		logger.Warn("Failed to register stuck detector job", "error", err)
	} else {
		logger.Info("Video stuck detector job registered",
			"check_interval", "30s",
			"processing_timeout", "10m",
			"pending_timeout", "5m",
		)
	}

	// === Subtitle Stuck Detector ===
	subtitleDetectorConfig := serviceimpl.SubtitleStuckDetectorConfig{
		CheckInterval:     30 * time.Second, // ตรวจสอบทุก 30 วินาที
		ProcessingTimeout: 10 * time.Minute, // ถ้า processing > 10 นาที = stuck (worker crash)
		// ไม่มี QueuedTimeout - jobs รอใน queue ได้นานเท่าที่ต้องการ
	}

	subtitleStuckDetector := serviceimpl.NewSubtitleStuckDetectorService(
		subtitleDetectorConfig,
		c.SubtitleRepository,
		c.EventScheduler,
	)

	// Register subtitle detector job
	if err := subtitleStuckDetector.RegisterDetectorJob(); err != nil {
		logger.Warn("Failed to register subtitle stuck detector job", "error", err)
	} else {
		logger.Info("Subtitle stuck detector job registered",
			"check_interval", "30s",
			"processing_timeout", "10m",
		)
	}

	logger.Info("Stuck Detector Services initialized (Video + Subtitle)")
	return nil
}

func (c *Container) initProgressBroadcaster() error {
	// ตรวจสอบว่ามี ProgressSubscriber (interface) หรือไม่
	if c.ProgressSubscriber == nil {
		logger.Warn("ProgressSubscriber not available, progress broadcasting disabled")
		return nil
	}

	// สร้าง Progress Broadcaster ใช้ interface (Clean Architecture)
	c.ProgressBroadcaster = websocket.NewProgressBroadcaster(c.ProgressSubscriber, c.VideoRepository)

	// เริ่ม broadcaster
	if err := c.ProgressBroadcaster.Start(); err != nil {
		logger.Warn("Failed to start progress broadcaster", "error", err)
		return nil
	}

	logger.Info("Progress broadcaster started (Messaging → WebSocket)")
	return nil
}

// injectNotifierToProgressBroadcaster inject notifier หลังจาก initNotifications
func (c *Container) injectNotifierToProgressBroadcaster() {
	if c.ProgressBroadcaster != nil && c.Notifier != nil {
		c.ProgressBroadcaster.SetNotifier(c.Notifier)
		logger.Info("Notifier injected into progress broadcaster (transcode complete/fail notifications enabled)")
	}
}

func (c *Container) initNotifications() error {
	// Initialize Telegram Notifier
	c.Notifier = telegram.NewTelegramNotifier(c.SettingService)
	logger.Info("Telegram notifier initialized")

	// Initialize DLQ Subscriber (sends notifications when jobs enter DLQ)
	if c.NATSClient != nil {
		dlqSubscriber, err := natspkg.NewDLQSubscriber(c.NATSClient.Conn(), c.Notifier)
		if err != nil {
			logger.Warn("Failed to create DLQ subscriber", "error", err)
			return nil
		}
		c.DLQSubscriber = dlqSubscriber

		// Start DLQ subscriber
		ctx := context.Background()
		if err := c.DLQSubscriber.Start(ctx); err != nil {
			logger.Warn("Failed to start DLQ subscriber", "error", err)
		} else {
			logger.Info("DLQ subscriber started (sends Telegram alerts)")
		}
	} else {
		logger.Warn("DLQ subscriber disabled (NATS not available)")
	}

	return nil
}

func (c *Container) Cleanup() error {
	logger.Info("Starting cleanup...")

	// Stop DLQ subscriber
	if c.DLQSubscriber != nil {
		c.DLQSubscriber.Stop()
		logger.Info("DLQ subscriber stopped")
	}

	// Stop progress broadcaster
	if c.ProgressBroadcaster != nil {
		c.ProgressBroadcaster.Stop()
		logger.Info("Progress broadcaster stopped")
	}

	// Stop NATS subscriber
	if c.NATSSubscriber != nil {
		c.NATSSubscriber.Stop()
		logger.Info("NATS subscriber stopped")
	}

	// Stop scheduler
	if c.EventScheduler != nil {
		if c.EventScheduler.IsRunning() {
			c.EventScheduler.Stop()
			logger.Info("Event scheduler stopped")
		}
	}

	// Close NATS connection
	if c.NATSClient != nil {
		if err := c.NATSClient.Close(); err != nil {
			logger.Warn("Failed to close NATS connection", "error", err)
		} else {
			logger.Info("NATS connection closed")
		}
	}

	// Close Redis connection
	if c.RedisClient != nil {
		if err := c.RedisClient.Close(); err != nil {
			logger.Warn("Failed to close Redis connection", "error", err)
		} else {
			logger.Info("Redis connection closed")
		}
	}

	// Close database connection
	if c.DB != nil {
		sqlDB, err := c.DB.DB()
		if err == nil {
			if err := sqlDB.Close(); err != nil {
				logger.Warn("Failed to close database connection", "error", err)
			} else {
				logger.Info("Database connection closed")
			}
		}
	}

	logger.Info("Cleanup completed")
	return nil
}

func (c *Container) GetServices() (services.UserService, services.TaskService, services.FileService, services.JobService) {
	return c.UserService, c.TaskService, c.FileService, c.JobService
}

func (c *Container) GetConfig() *config.Config {
	return c.Config
}

func (c *Container) GetHandlerServices() *handlers.Services {
	// สร้าง base URL สำหรับ embed
	baseURL := c.Config.Storage.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%s", c.Config.App.Port)
	}

	// CDN Base URL (Cloudflare Worker)
	cdnBaseURL := c.Config.Storage.CDNBaseURL
	if cdnBaseURL == "" {
		// Fallback to local API if no CDN configured
		cdnBaseURL = fmt.Sprintf("http://localhost:%s", c.Config.App.Port)
	}

	return &handlers.Services{
		UserService:         c.UserService,
		TaskService:         c.TaskService,
		FileService:         c.FileService,
		JobService:          c.JobService,
		VideoService:        c.VideoService,
		CategoryService:     c.CategoryService,
		TranscodingService:  c.TranscodingService,
		StorageService:      c.StorageService,
		StoragePort:         c.Storage,
		WhitelistService:    c.WhitelistService,
		SettingService:      c.SettingService,
		SubtitleService:     c.SubtitleService,
		QueueService:        c.QueueService,
		VideoRepository:     c.VideoRepository, // สำหรับ SubtitleHandler
		StreamCookieService: c.StreamCookieService, // Signed cookie สำหรับ CDN access
		NATSPublisher:       c.NATSPublisher,
		GoogleConfig:        c.Config.Google,
		StorageBasePath:     c.Config.Storage.BasePath,
		StorageType:         c.Config.Storage.Type,
		BaseURL:             baseURL,
		CDNBaseURL:          cdnBaseURL,
		JWTSecret:           c.Config.JWT.Secret,
	}
}
