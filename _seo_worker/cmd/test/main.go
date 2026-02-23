package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"seo-worker/config"
	"seo-worker/domain/models"
	"seo-worker/infrastructure/ai"
	"seo-worker/infrastructure/auth"
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

func main() {
	// Parse flags
	videoCode := flag.String("code", "utywgage", "Video code to process")
	generateTTS := flag.Bool("tts", false, "Generate TTS (default: false for faster testing)")
	flag.Parse()

	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("Direct test mode", "video_code", *videoCode, "tts", *generateTTS)

	// Load config
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// === Create dependencies manually (no NATS needed) ===

	// 1. Auth clients
	suekkAuth := auth.NewAuthClient(cfg.SuekkAPI.URL, cfg.SuekkAPI.Email, cfg.SuekkAPI.Password)
	subthAuth := auth.NewAuthClient(cfg.SubthAPI.URL, cfg.SubthAPI.Email, cfg.SubthAPI.Password)

	// 2. Storage (IDrive for SRT)
	suekkStorage, err := storage.NewR2Client(storage.R2Config{
		Endpoint:  cfg.SuekkStorage.Endpoint,
		AccessKey: cfg.SuekkStorage.AccessKey,
		SecretKey: cfg.SuekkStorage.SecretKey,
		Bucket:    cfg.SuekkStorage.Bucket,
		PublicURL: cfg.SuekkStorage.PublicURL,
	})
	if err != nil {
		logger.Error("Failed to create suekk storage", "error", err)
		os.Exit(1)
	}

	// 3. SRT Fetcher (from IDrive storage)
	srtFetcher := fetcher.NewSRTFetcher(suekkStorage)

	// 4. Suekk Video Fetcher (from api.suekk.com)
	suekkVideoFetcher := fetcher.NewSuekkVideoFetcher(cfg.SuekkAPI.URL, suekkAuth, suekkStorage)

	// 5. Metadata Fetcher (from api.subth.com)
	metadataFetcher := fetcher.NewMetadataFetcher(cfg.SubthAPI.URL, subthAuth)

	// 6. Image Selector (Python - NSFW filter, face detection, aesthetic scoring)
	imageSelector := imageselector.NewPythonImageSelector(imageselector.PythonImageSelectorConfig{
		PythonPath: cfg.ImageSelector.PythonPath,
		ScriptPath: cfg.ImageSelector.ScriptPath,
		Device:     cfg.ImageSelector.Device,
	})
	logger.Info("Image selector created",
		"python_path", cfg.ImageSelector.PythonPath,
		"script_path", cfg.ImageSelector.ScriptPath,
		"device", cfg.ImageSelector.Device,
	)

	// 7. Gemini AI
	geminiClient, err := ai.NewGeminiClient(cfg.Gemini.APIKey, cfg.Gemini.Model)
	if err != nil {
		logger.Error("Failed to create gemini client", "error", err)
		os.Exit(1)
	}
	defer geminiClient.Close()

	// 8. TTS (ElevenLabs) - nil ถ้าไม่ต้องการ TTS
	var ttsClient *tts.ElevenLabsClient
	if *generateTTS {
		ttsClient = tts.NewElevenLabsClient(tts.ElevenLabsConfig{
			APIKey:  cfg.ElevenLabs.APIKey,
			VoiceID: cfg.ElevenLabs.VoiceID,
			Model:   cfg.ElevenLabs.Model,
		})
		logger.Info("TTS enabled")
	} else {
		logger.Info("TTS disabled (use -tts flag to enable)")
	}

	// 9. Embedding (pgvector) - create stub for testing
	embeddingClient := embedding.NewPgVectorClient(nil) // nil DB = will log warning but not fail

	// 10. Article Publisher - stub
	articlePublisher := publisher.NewArticlePublisher(cfg.SubthAPI.URL, subthAuth)

	// 11. Messenger - no-op for testing
	noopMessenger := messenger.NewNoopMessenger()

	// 12. Storage for audio upload (R2)
	subthStorage, err := storage.NewR2Client(storage.R2Config{
		Endpoint:  cfg.SubthStorage.Endpoint,
		AccessKey: cfg.SubthStorage.AccessKey,
		SecretKey: cfg.SubthStorage.SecretKey,
		Bucket:    cfg.SubthStorage.Bucket,
		PublicURL: cfg.SubthStorage.PublicURL,
	})
	if err != nil {
		logger.Error("Failed to create subth storage", "error", err)
		os.Exit(1)
	}

	// 13. Image Copier (e2 → r2)
	imageCopier := imagecopier.NewImageCopier(suekkStorage, subthStorage)
	logger.Info("Image copier created (e2 → r2)")

	// === Create SEO Handler ===
	handler := use_cases.NewSEOHandler(
		srtFetcher,
		suekkVideoFetcher,
		metadataFetcher,
		imageSelector,
		geminiClient,
		ttsClient,
		embeddingClient,
		articlePublisher,
		imageCopier,
		noopMessenger,
		subthStorage,
	)

	// === Create test job ===
	job := &models.SEOArticleJob{
		VideoID:     "test-video-id", // Will be replaced with actual ID from metadata
		VideoCode:   *videoCode,
		Priority:    2,
		GenerateTTS: *generateTTS,
	}

	logger.Info("Processing job directly", "video_code", job.VideoCode)

	// === Process! ===
	if err := handler.ProcessJob(ctx, job); err != nil {
		logger.Error("Job failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Job completed! Check output folder for JSON file")
}
