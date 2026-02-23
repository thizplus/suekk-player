package container

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"

	"seo-worker/config"
	"seo-worker/domain/ports"
	"seo-worker/infrastructure/ai"
	"seo-worker/infrastructure/auth"
	"seo-worker/infrastructure/consumer"
	"seo-worker/infrastructure/embedding"
	"seo-worker/infrastructure/fetcher"
	"seo-worker/infrastructure/imagecopier"
	"seo-worker/infrastructure/imageselector"
	"seo-worker/infrastructure/messenger"
	"seo-worker/infrastructure/publisher"
	"seo-worker/infrastructure/storage"
	"seo-worker/infrastructure/tts"
	"seo-worker/use_cases"
)

// Container - Dependency Injection Container
type Container struct {
	Config *config.Config

	// External connections
	NATSConn *nats.Conn
	DB       *sql.DB

	// Ports (Interfaces)
	SRTFetcher         ports.SRTFetcherPort
	SuekkVideoFetcher  ports.SuekkVideoFetcherPort
	MetadataFetcher    ports.MetadataFetcherPort
	ImageSelector      ports.ImageSelectorPort
	AIService          ports.AIPort
	TTSService         ports.TTSPort
	EmbeddingService   ports.EmbeddingPort
	ArticlePublisher   ports.ArticlePublisherPort
	ImageCopier        ports.ImageCopierPort
	Consumer           ports.ConsumerPort
	Messenger          ports.MessengerPort
	Storage            ports.StoragePort
	SuekkStorage       ports.StoragePort  // e2 source for image copy

	// Use Cases
	SEOHandler *use_cases.SEOHandler

	// Internal
	geminiClient *ai.GeminiClient
	logger       *slog.Logger
}

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
		nats.ReconnectWait(2*1000*1000*1000), // 2 seconds in nanoseconds
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}
	c.logger.Info("Connected to NATS", "url", cfg.NATS.URL)

	// Database Connection
	c.DB, err = sql.Open("postgres", cfg.Database.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	if err := c.DB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	c.logger.Info("Connected to database")

	// ─────────────────────────────────────────────────────────────────────────────
	// 2. Infrastructure Layer
	// ─────────────────────────────────────────────────────────────────────────────

	// Auth Clients (auto-login with email/password)
	suekkAuth := auth.NewAuthClient(cfg.SuekkAPI.URL, cfg.SuekkAPI.Email, cfg.SuekkAPI.Password)
	subthAuth := auth.NewAuthClient(cfg.SubthAPI.URL, cfg.SubthAPI.Email, cfg.SubthAPI.Password)
	c.logger.Info("Auth clients created")

	// Suekk Storage (IDrive e2) - source for SRT files and image copy
	if cfg.SuekkStorage.Endpoint != "" {
		suekkStorageClient, err := storage.NewR2Client(storage.R2Config{
			Endpoint:  cfg.SuekkStorage.Endpoint,
			AccessKey: cfg.SuekkStorage.AccessKey,
			SecretKey: cfg.SuekkStorage.SecretKey,
			Bucket:    cfg.SuekkStorage.Bucket,
			PublicURL: cfg.SuekkStorage.PublicURL,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create suekk storage client: %w", err)
		}
		c.SuekkStorage = suekkStorageClient
		c.logger.Info("Suekk storage (IDrive e2) created", "bucket", cfg.SuekkStorage.Bucket)
	}

	// SRT Fetcher - อ่านจาก IDrive storage
	c.SRTFetcher = fetcher.NewSRTFetcher(c.SuekkStorage)
	c.logger.Info("SRT fetcher created")

	// Suekk Video Fetcher (api.suekk.com) - ดึง duration, gallery
	c.SuekkVideoFetcher = fetcher.NewSuekkVideoFetcher(cfg.SuekkAPI.URL, suekkAuth, c.SuekkStorage)
	c.logger.Info("Suekk video fetcher created", "url", cfg.SuekkAPI.URL)

	// Metadata Fetcher (api.subth.com)
	c.MetadataFetcher = fetcher.NewMetadataFetcher(cfg.SubthAPI.URL, subthAuth)
	c.logger.Info("Metadata fetcher created", "url", cfg.SubthAPI.URL)

	// Image Selector (Python - NSFW filter, face detection, aesthetic scoring)
	c.ImageSelector = imageselector.NewPythonImageSelector(imageselector.PythonImageSelectorConfig{
		PythonPath: cfg.ImageSelector.PythonPath,
		ScriptPath: cfg.ImageSelector.ScriptPath,
		Device:     cfg.ImageSelector.Device,
	})
	c.logger.Info("Image selector created",
		"python_path", cfg.ImageSelector.PythonPath,
		"script_path", cfg.ImageSelector.ScriptPath,
		"device", cfg.ImageSelector.Device,
	)

	// Gemini AI Service
	c.geminiClient, err = ai.NewGeminiClient(cfg.Gemini.APIKey, cfg.Gemini.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}
	c.AIService = c.geminiClient
	c.logger.Info("Gemini client created", "model", cfg.Gemini.Model)

	// ElevenLabs TTS Service
	if cfg.ElevenLabs.APIKey != "" {
		c.TTSService = tts.NewElevenLabsClient(tts.ElevenLabsConfig{
			APIKey:  cfg.ElevenLabs.APIKey,
			VoiceID: cfg.ElevenLabs.VoiceID,
			Model:   cfg.ElevenLabs.Model,
		})
		c.logger.Info("ElevenLabs client created",
			"voice_id", cfg.ElevenLabs.VoiceID,
			"model", cfg.ElevenLabs.Model,
		)
	} else {
		c.logger.Warn("ElevenLabs API key not set, TTS disabled")
	}

	// pgvector Embedding Service
	c.EmbeddingService = embedding.NewPgVectorClient(c.DB)
	c.logger.Info("pgvector client created")

	// Article Publisher (api.subth.com)
	c.ArticlePublisher = publisher.NewArticlePublisher(cfg.SubthAPI.URL, subthAuth)
	c.logger.Info("Article publisher created")

	// NATS Consumer
	consumerImpl, err := consumer.NewNATSConsumer(consumer.NATSConsumerConfig{
		URL:             cfg.NATS.URL,
		Stream:          cfg.NATS.Stream,
		Subject:         cfg.NATS.Subject,
		ConsumerName:    cfg.NATS.Consumer,
		Concurrency:     cfg.Worker.Concurrency,
		ShutdownTimeout: cfg.NATS.ShutdownTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}
	c.Consumer = consumerImpl
	c.logger.Info("NATS consumer created", "stream", cfg.NATS.Stream)

	// NATS Messenger (Progress Publisher)
	c.Messenger = messenger.NewNATSPublisher(c.NATSConn)
	c.logger.Info("NATS messenger created")

	// Subth Storage (R2) - for uploading audio files and images
	if cfg.SubthStorage.Endpoint != "" {
		storageClient, err := storage.NewR2Client(storage.R2Config{
			Endpoint:  cfg.SubthStorage.Endpoint,
			AccessKey: cfg.SubthStorage.AccessKey,
			SecretKey: cfg.SubthStorage.SecretKey,
			Bucket:    cfg.SubthStorage.Bucket,
			PublicURL: cfg.SubthStorage.PublicURL,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create subth storage client: %w", err)
		}
		c.Storage = storageClient
		c.logger.Info("Subth storage (R2) created", "bucket", cfg.SubthStorage.Bucket)
	} else {
		c.logger.Warn("Subth storage endpoint not set, audio upload disabled")
	}

	// Image Copier (e2 → r2) - copy gallery images from suekk to subth
	if c.SuekkStorage != nil && c.Storage != nil {
		c.ImageCopier = imagecopier.NewImageCopier(c.SuekkStorage, c.Storage)
		c.logger.Info("Image copier created (e2 → r2)")
	} else {
		c.logger.Warn("Image copier not created (missing source or destination storage)")
	}

	// ─────────────────────────────────────────────────────────────────────────────
	// 3. Use Cases Layer
	// ─────────────────────────────────────────────────────────────────────────────

	c.SEOHandler = use_cases.NewSEOHandler(
		c.SRTFetcher,
		c.SuekkVideoFetcher,
		c.MetadataFetcher,
		c.ImageSelector,
		c.AIService,
		c.TTSService,
		c.EmbeddingService,
		c.ArticlePublisher,
		c.ImageCopier,
		c.Messenger,
		c.Storage,
	)
	c.logger.Info("SEO handler created")

	// Wire handler to consumer
	c.Consumer.SetHandler(c.SEOHandler.ProcessJob)

	c.logger.Info("Container initialized successfully")
	return c, nil
}

// Start เริ่ม services ทั้งหมด
func (c *Container) Start(ctx context.Context) error {
	c.logger.Info("Starting container services...")

	// Start consumer (blocking)
	if err := c.Consumer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start consumer: %w", err)
	}

	return nil
}

// Stop หยุด services ทั้งหมด (graceful shutdown)
func (c *Container) Stop() {
	c.logger.Info("Stopping container services...")

	// Stop consumer
	c.Consumer.Stop()
	c.logger.Info("Consumer stopped")

	// Close Gemini client
	if c.geminiClient != nil {
		c.geminiClient.Close()
		c.logger.Info("Gemini client closed")
	}

	// Close database
	if c.DB != nil {
		c.DB.Close()
		c.logger.Info("Database connection closed")
	}

	// Close NATS
	if c.NATSConn != nil {
		c.NATSConn.Close()
		c.logger.Info("NATS connection closed")
	}

	c.logger.Info("Container stopped")
}
