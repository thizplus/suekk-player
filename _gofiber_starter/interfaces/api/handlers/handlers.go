package handlers

import (
	"gofiber-template/application/serviceimpl"
	"gofiber-template/domain/ports"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	natspkg "gofiber-template/infrastructure/nats"
	"gofiber-template/pkg/config"
)

// Services contains all the services needed for handlers
type Services struct {
	UserService        services.UserService
	TaskService        services.TaskService
	FileService        services.FileService
	JobService         services.JobService
	VideoService       services.VideoService
	CategoryService    services.CategoryService
	TranscodingService services.TranscodingService
	StorageService     services.StorageService
	StoragePort        ports.StoragePort // Storage interface for HLS proxy
	WhitelistService   services.WhitelistService // Phase 6: Domain Whitelist & Ad Management
	SettingService     services.SettingService   // Admin Settings
	SubtitleService    services.SubtitleService  // Subtitle management
	QueueService       services.QueueService     // Queue management (transcode/subtitle/warmcache)
	ReelService        services.ReelService      // Reel Generator
	VideoRepository    repositories.VideoRepository // สำหรับ SubtitleHandler
	StreamCookieService     *serviceimpl.StreamCookieService         // Signed cookie สำหรับ CDN access
	NATSPublisher           *natspkg.Publisher                       // NATS JetStream publisher (แทน AsynqClient)
	GoogleConfig       config.GoogleOAuthConfig
	StorageBasePath    string // สำหรับ VideoHandler (legacy)
	StorageType        string // "local" หรือ "s3"
	BaseURL            string // Base URL สำหรับ embed URLs
	CDNBaseURL         string // Cloudflare Worker URL สำหรับ HLS streaming
	JWTSecret          string // JWT Secret สำหรับ stream access token
}

// Handlers contains all HTTP handlers
type Handlers struct {
	UserHandler          *UserHandler
	TaskHandler          *TaskHandler
	FileHandler          *FileHandler
	JobHandler           *JobHandler
	VideoHandler         *VideoHandler
	CategoryHandler      *CategoryHandler
	AuthHandler          *AuthHandler
	TranscodingHandler   *TranscodingHandler
	HLSHandler           *HLSHandler
	StorageHandler       *StorageHandler
	ProgressHandler      *ProgressHandler
	EmbedHandler         *EmbedHandler
	MonitoringHandler    *MonitoringHandler               // JetStream monitoring
	WhitelistHandler     *WhitelistHandler                // Phase 6: Domain Whitelist & Ad Management
	SettingHandler       *SettingHandler                  // Admin Settings
	SubtitleHandler      *SubtitleHandler                 // Subtitle management
	QueueHandler         *QueueHandler                    // Queue management (transcode/subtitle/warmcache)
	DirectUploadHandler  *DirectUploadHandler             // Direct Upload via Presigned URL
	ReelHandler          *ReelHandler                     // Reel Generator
	StreamCookieService  *serviceimpl.StreamCookieService // Signed cookie สำหรับ CDN access
}

// NewHandlers creates a new instance of Handlers with all dependencies
func NewHandlers(services *Services) *Handlers {
	return &Handlers{
		UserHandler:          NewUserHandler(services.UserService),
		TaskHandler:          NewTaskHandler(services.TaskService),
		FileHandler:          NewFileHandler(services.FileService),
		JobHandler:           NewJobHandler(services.JobService),
		VideoHandler:         NewVideoHandler(services.VideoService, services.TranscodingService, services.SettingService, services.NATSPublisher, services.StorageBasePath, services.StorageType),
		CategoryHandler:      NewCategoryHandler(services.CategoryService),
		AuthHandler:          NewAuthHandler(services.UserService, services.GoogleConfig),
		TranscodingHandler:   NewTranscodingHandler(services.VideoService, services.SettingService, services.NATSPublisher),
		HLSHandler:           NewHLSHandler(services.VideoService, services.StoragePort, services.CDNBaseURL, services.JWTSecret),
		StorageHandler:       NewStorageHandler(services.StorageService, services.VideoService),
		ProgressHandler:      NewProgressHandler(),
		EmbedHandler:         NewEmbedHandler(services.VideoService, services.BaseURL),
		MonitoringHandler:    NewMonitoringHandler(services.NATSPublisher),
		WhitelistHandler:     NewWhitelistHandler(services.WhitelistService, services.StreamCookieService, services.CDNBaseURL+"/hls"),
		SettingHandler:       NewSettingHandler(services.SettingService),
		SubtitleHandler:      NewSubtitleHandler(services.SubtitleService, services.VideoRepository),
		QueueHandler:         NewQueueHandler(services.QueueService),
		DirectUploadHandler:  NewDirectUploadHandler(services.StoragePort, services.VideoService, services.SettingService, services.NATSPublisher),
		ReelHandler:          NewReelHandler(services.ReelService),
		StreamCookieService:  services.StreamCookieService,
	}
}
