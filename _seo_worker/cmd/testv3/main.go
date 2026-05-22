package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"seo-worker/config"
	"seo-worker/domain/models"
	"seo-worker/domain/ports"
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

// MockPublisher - Publisher ที่ไม่ส่งจริง แค่ save JSON
type MockPublisher struct {
	logger *slog.Logger
}

func NewMockPublisher() *MockPublisher {
	return &MockPublisher{
		logger: slog.Default().With("component", "mock_publisher"),
	}
}

func (p *MockPublisher) PublishArticle(ctx context.Context, article *models.ArticleContent) error {
	p.logger.Info("[MOCK] PublishArticle called - NOT publishing to real API",
		"video_id", article.VideoID,
	)
	return nil
}

func (p *MockPublisher) PublishArticleV3(ctx context.Context, article *models.ArticleContentV3) error {
	p.logger.Info("[MOCK] PublishArticleV3 called - NOT publishing to real API",
		"video_id", article.VideoID,
		"title", article.TitleBalanced,
		"rating", article.Rating,
	)

	// Save to file for inspection
	outputPath := fmt.Sprintf("output/%s_v3_test.json", article.Slug)
	jsonData, err := json.MarshalIndent(article, "", "  ")
	if err != nil {
		p.logger.Error("Failed to marshal article", "error", err)
		return nil
	}

	if err := os.MkdirAll("output", 0755); err != nil {
		p.logger.Error("Failed to create output dir", "error", err)
		return nil
	}

	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		p.logger.Error("Failed to write file", "error", err)
		return nil
	}

	p.logger.Info("[MOCK] Article V3 saved to file", "path", outputPath)
	return nil
}

func (p *MockPublisher) UpdateArticleStatus(ctx context.Context, videoID string, status string) error {
	p.logger.Info("[MOCK] UpdateArticleStatus called - NOT updating real API",
		"video_id", videoID,
		"status", status,
	)
	return nil
}

// Verify interface
var _ ports.ArticlePublisherPort = (*MockPublisher)(nil)

func main() {
	// Parse flags
	videoCode := flag.String("code", "", "Video code to process (required)")
	generateTTS := flag.Bool("tts", false, "Generate TTS (default: false for faster testing)")
	language := flag.String("lang", "th", "Language: th (default) or en")
	publishToAPI := flag.Bool("publish", false, "Publish to real API (api.subth.com) instead of saving locally")
	flag.Parse()

	if *videoCode == "" {
		fmt.Println("Usage: testv3 -code=<video_code> [-tts] [-lang=th|en] [-publish]")
		fmt.Println("")
		fmt.Println("Example:")
		fmt.Println("  testv3 -code=dldss-471")
		fmt.Println("  testv3 -code=dass-541 -lang=en")
		fmt.Println("  testv3 -code=dass-541 -lang=en -publish  # ส่งขึ้น API จริง!")
		os.Exit(1)
	}

	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("=== V3 Intent-Driven Pipeline Test ===",
		"video_code", *videoCode,
		"language", *language,
		"tts", *generateTTS,
		"publish", *publishToAPI,
	)

	// Load config
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	startTime := time.Now()

	// === Create dependencies ===

	// Auth clients
	suekkAuth := auth.NewAuthClient(cfg.SuekkAPI.URL, cfg.SuekkAPI.Email, cfg.SuekkAPI.Password)
	subthAuth := auth.NewAuthClient(cfg.SubthAPI.URL, cfg.SubthAPI.Email, cfg.SubthAPI.Password)

	// Storage (IDrive for SRT)
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

	// SRT Fetcher
	srtFetcher := fetcher.NewSRTFetcher(suekkStorage)

	// Suekk Video Fetcher
	suekkVideoFetcher := fetcher.NewSuekkVideoFetcher(cfg.SuekkAPI.URL, suekkAuth, suekkStorage)

	// Metadata Fetcher
	metadataFetcher := fetcher.NewMetadataFetcher(cfg.SubthAPI.URL, subthAuth)

	// Image Selector
	imageSelector := imageselector.NewPythonImageSelector(imageselector.PythonImageSelectorConfig{
		PythonPath: cfg.ImageSelector.PythonPath,
		ScriptPath: cfg.ImageSelector.ScriptPath,
		Device:     cfg.ImageSelector.Device,
	})

	// Gemini AI
	geminiClient, err := ai.NewGeminiClient(cfg.Gemini.APIKey, cfg.Gemini.Model)
	if err != nil {
		logger.Error("Failed to create gemini client", "error", err)
		os.Exit(1)
	}
	defer geminiClient.Close()
	logger.Info("Gemini client created", "model", cfg.Gemini.Model)

	// TTS (optional)
	var ttsClient *tts.ElevenLabsClient
	if *generateTTS {
		ttsClient = tts.NewElevenLabsClient(tts.ElevenLabsConfig{
			APIKey:  cfg.ElevenLabs.APIKey,
			VoiceID: cfg.ElevenLabs.VoiceID,
			Model:   cfg.ElevenLabs.Model,
		})
		logger.Info("TTS enabled")
	}

	// Embedding (stub)
	embeddingClient := embedding.NewPgVectorClient(nil)

	// Publisher - เลือกตาม flag
	var articlePublisher ports.ArticlePublisherPort
	if *publishToAPI {
		// Real Publisher - ส่งขึ้น api.subth.com จริง!
		articlePublisher = publisher.NewArticlePublisher(cfg.SubthAPI.URL, subthAuth)
		logger.Info("Using REAL publisher - will upload to api.subth.com!")
	} else {
		// Mock Publisher - ไม่ส่งจริง แค่ save JSON
		articlePublisher = NewMockPublisher()
		logger.Info("Using MOCK publisher - will NOT upload to real API")
	}

	// Messenger (no-op)
	noopMessenger := messenger.NewNoopMessenger()

	// Storage for audio (R2)
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

	// Image Copier
	imageCopier := imagecopier.NewImageCopier(suekkStorage, subthStorage)

	// === Create SEO Handler ===
	handler := use_cases.NewSEOHandler(
		srtFetcher,
		suekkVideoFetcher,
		metadataFetcher,
		imageSelector,
		geminiClient,
		ttsClient,
		embeddingClient,
		articlePublisher, // ใช้ mock หรือ real publisher ตาม flag
		imageCopier,
		noopMessenger,
		subthStorage,
	)

	// === Create V3 job ===
	job := &models.SEOArticleJob{
		VideoID:     "test-v3",
		VideoCode:   *videoCode,
		Priority:    2,
		GenerateTTS: *generateTTS,
		Language:    *language,
		UseV3:       true, // บังคับใช้ V3
	}

	logger.Info("Starting V3 pipeline...", "video_code", job.VideoCode)

	// === Process with V3! ===
	if err := handler.ProcessJobV3(ctx, job); err != nil {
		logger.Error("V3 Job failed", "error", err)
		os.Exit(1)
	}

	elapsed := time.Since(startTime)
	logger.Info("=== V3 Test Completed! ===",
		"elapsed", elapsed.String(),
		"output", fmt.Sprintf("output/%s_v3_test.json", *videoCode),
	)

	fmt.Println("")
	fmt.Printf("V3 test completed in %s\n", elapsed)
	fmt.Printf("Check output folder: output/%s_article_v3.json\n", *videoCode)
}
