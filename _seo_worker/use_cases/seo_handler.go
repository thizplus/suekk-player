package use_cases

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"seo-worker/domain/models"
	"seo-worker/domain/ports"
)

type SEOHandler struct {
	srtFetcher        ports.SRTFetcherPort
	suekkVideoFetcher ports.SuekkVideoFetcherPort
	metadataFetcher   ports.MetadataFetcherPort
	imageSelector     ports.ImageSelectorPort
	aiService         ports.AIPort
	ttsService        ports.TTSPort
	embeddingService  ports.EmbeddingPort
	articlePublisher  ports.ArticlePublisherPort
	messenger         ports.MessengerPort
	storage           ports.StoragePort

	logger *slog.Logger
}

func NewSEOHandler(
	srtFetcher ports.SRTFetcherPort,
	suekkVideoFetcher ports.SuekkVideoFetcherPort,
	metadataFetcher ports.MetadataFetcherPort,
	imageSelector ports.ImageSelectorPort,
	aiService ports.AIPort,
	ttsService ports.TTSPort,
	embeddingService ports.EmbeddingPort,
	articlePublisher ports.ArticlePublisherPort,
	messenger ports.MessengerPort,
	storage ports.StoragePort,
) *SEOHandler {
	return &SEOHandler{
		srtFetcher:        srtFetcher,
		suekkVideoFetcher: suekkVideoFetcher,
		metadataFetcher:   metadataFetcher,
		imageSelector:     imageSelector,
		aiService:         aiService,
		ttsService:        ttsService,
		embeddingService:  embeddingService,
		articlePublisher:  articlePublisher,
		messenger:         messenger,
		storage:           storage,
		logger:            slog.Default().With("component", "seo_handler"),
	}
}

func (h *SEOHandler) ProcessJob(ctx context.Context, job *models.SEOArticleJob) error {
	startTime := time.Now()

	h.logger.InfoContext(ctx, "Processing SEO job",
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
		"generate_tts", job.GenerateTTS,
	)

	// === Stage 1: Fetch Raw Materials ===
	h.sendProgress(ctx, job.VideoID, ports.StageFetching, 10)

	// 1.1 Fetch SRT content (pre-validated at Admin UI)
	srtContent, err := h.srtFetcher.FetchSRT(ctx, job.VideoCode)
	if err != nil {
		h.messenger.SendFailed(ctx, job.VideoID, err)
		return fmt.Errorf("failed to fetch SRT: %w", err)
	}

	// 1.2 Fetch video info from api.suekk.com (duration, gallery)
	h.logger.InfoContext(ctx, "[DEBUG] Fetching Suekk video info...", "video_code", job.VideoCode)
	suekkVideoInfo, err := h.suekkVideoFetcher.FetchVideoInfo(ctx, job.VideoCode)
	if err != nil {
		h.logger.WarnContext(ctx, "[DEBUG] Failed to fetch Suekk video info (non-critical)",
			"video_code", job.VideoCode,
			"error", err,
		)
		// ใช้ค่า default ถ้าดึงไม่ได้
		suekkVideoInfo = &models.SuekkVideoInfo{
			Code:     job.VideoCode,
			Duration: 0,
		}
	} else {
		h.logger.InfoContext(ctx, "[DEBUG] Suekk video info received",
			"code", suekkVideoInfo.Code,
			"gallery_path", suekkVideoInfo.GalleryPath,
			"gallery_count", suekkVideoInfo.GalleryCount,
		)
	}

	// 1.3 Fetch metadata by video code from api.subth.com
	metadata, err := h.metadataFetcher.FetchVideoMetadataByCode(ctx, job.VideoCode)
	if err != nil {
		h.messenger.SendFailed(ctx, job.VideoID, err)
		return fmt.Errorf("failed to fetch metadata: %w", err)
	}

	// ใช้ duration จาก suekk ถ้ามี (แม่นยำกว่า)
	if suekkVideoInfo.Duration > 0 {
		metadata.Duration = suekkVideoInfo.Duration
	}

	// 1.4 Use cast/maker/tags from metadata (already fetched from /videos/:id)
	casts := metadata.Casts
	makerInfo := metadata.Maker
	tags := metadata.Tags

	// 1.5 Fetch previous works for each cast
	var previousWorks []models.PreviousWork
	for _, cast := range casts {
		works, _ := h.metadataFetcher.FetchPreviousWorks(ctx, cast.ID, 5)
		previousWorks = append(previousWorks, works...)
	}

	h.logger.InfoContext(ctx, "Metadata loaded from video response",
		"casts_count", len(casts),
		"tags_count", len(tags),
		"has_maker", makerInfo != nil,
	)

	// 1.7 Fetch gallery images from Suekk storage
	var galleryImages []models.GalleryImage
	var coverImage *models.ImageScore

	h.logger.InfoContext(ctx, "[DEBUG] Gallery fetch start",
		"gallery_path", suekkVideoInfo.GalleryPath,
		"gallery_count", suekkVideoInfo.GalleryCount,
		"gallery_safe_count", suekkVideoInfo.GallerySafeCount,
	)

	if suekkVideoInfo.GalleryPath != "" {
		// ดึงเฉพาะภาพ safe (pre-classified by _worker)
		imageURLs, err := h.suekkVideoFetcher.ListGalleryImages(ctx, suekkVideoInfo.GalleryPath)
		if err != nil {
			h.logger.WarnContext(ctx, "Failed to list gallery images",
				"gallery_path", suekkVideoInfo.GalleryPath,
				"error", err,
			)
		} else {
			h.logger.InfoContext(ctx, "Gallery images fetched (safe only)",
				"count", len(imageURLs),
				"expected", suekkVideoInfo.GallerySafeCount,
			)
		}

		// ภาพจาก /safe/ folder เป็น SFW ทั้งหมดแล้ว - ใช้ได้โดยตรง
		if len(imageURLs) > 0 {
			// ใช้ภาพแรกเป็น cover (TODO: อาจเพิ่ม face detection เลือก cover ที่ดีกว่า)
			coverImage = &models.ImageScore{
				URL:    imageURLs[0],
				IsSafe: true,
			}

			// เพิ่มทุกภาพเข้า gallery
			for _, url := range imageURLs {
				galleryImages = append(galleryImages, models.GalleryImage{
					URL: url,
				})
			}

			h.logger.InfoContext(ctx, "Gallery images ready (pre-classified safe)",
				"gallery_count", len(galleryImages),
				"has_cover", coverImage != nil,
			)
		}
	} else {
		h.logger.WarnContext(ctx, "[DEBUG] No gallery path available")
	}

	h.logger.InfoContext(ctx, "[DEBUG] Gallery images final",
		"gallery_count", len(galleryImages),
		"has_cover", coverImage != nil,
	)

	h.sendProgress(ctx, job.VideoID, ports.StageDataFetched, 25)

	// === Stage 2: AI Processing (Gemini with JSON Mode) ===
	h.sendProgress(ctx, job.VideoID, ports.StageAI, 30)

	aiInput := &ports.AIInput{
		SRTContent:    srtContent,
		VideoMetadata: metadata,
		Casts:         casts,
		PreviousWorks: previousWorks,
		GalleryCount:  len(galleryImages),
	}

	aiOutput, err := h.aiService.GenerateArticleContent(ctx, aiInput)
	if err != nil {
		h.messenger.SendFailed(ctx, job.VideoID, err)
		return fmt.Errorf("AI generation failed: %w", err)
	}

	h.sendProgress(ctx, job.VideoID, ports.StageAIComplete, 60)

	// === Stage 3: TTS & Embedding (Parallel) ===
	h.sendProgress(ctx, job.VideoID, ports.StageTTSEmbed, 65)

	var wg sync.WaitGroup
	var embedErr error
	var audioURL string
	var audioDuration int

	// 3.1 TTS Generation (Optional)
	if job.GenerateTTS && h.ttsService != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// สกัดใจความสำคัญ ~500 ตัวอักษร
			ttsScript := ports.ExtractTTSScript(aiOutput.Summary, aiOutput.Highlights)

			// Use empty string to use default voice from config
			ttsResult, err := h.ttsService.GenerateAudio(ctx, ttsScript, "")
			if err != nil {
				h.logger.WarnContext(ctx, "TTS failed (non-critical)",
					"video_id", job.VideoID,
					"error", err,
				)
				return
			}

			// Upload to storage
			audioPath := fmt.Sprintf("audio/articles/%s/summary.mp3", job.VideoCode)
			if err := h.storage.Upload(ctx, audioPath, ttsResult.AudioData, "audio/mpeg"); err != nil {
				h.logger.WarnContext(ctx, "TTS upload failed",
					"video_id", job.VideoID,
					"error", err,
				)
				return
			}

			audioURL = h.storage.GetPublicURL(audioPath)
			audioDuration = ttsResult.Duration
		}()
	}

	// 3.2 Embedding Generation
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Combine summary + highlights for embedding
		embeddingText := aiOutput.Summary
		for _, h := range aiOutput.Highlights {
			embeddingText += " " + h
		}

		vector, err := h.embeddingService.GenerateEmbedding(ctx, embeddingText)
		if err != nil {
			embedErr = err
			return
		}

		// Store in pgvector พร้อม metadata สำหรับ filtered search
		embeddingData := &models.EmbeddingData{
			VideoID:   job.VideoID,
			Vector:    vector,
			CastIDs:   metadata.CastIDs,
			MakerID:   metadata.MakerID,
			TagIDs:    metadata.TagIDs,
			CreatedAt: time.Now(),
		}
		if err := h.embeddingService.StoreEmbedding(ctx, embeddingData); err != nil {
			h.logger.WarnContext(ctx, "pgvector store failed (non-critical)",
				"video_id", job.VideoID,
				"error", err,
			)
		}
	}()

	wg.Wait()

	// Embedding error is non-critical (can retry later)
	if embedErr != nil {
		h.logger.WarnContext(ctx, "Embedding failed (non-critical)",
			"video_id", job.VideoID,
			"error", embedErr,
		)
	}

	h.sendProgress(ctx, job.VideoID, ports.StageTTSEmbedComplete, 90)

	// === Stage 4: Build Article ===
	h.sendProgress(ctx, job.VideoID, ports.StagePublishing, 95)

	article := h.buildArticle(job, metadata, aiOutput, casts, makerInfo, tags, previousWorks, galleryImages, coverImage, audioURL, audioDuration)

	// === DEBUG MODE: Save JSON for review instead of publishing ===
	outputPath := fmt.Sprintf("output/%s_article.json", job.VideoCode)
	if err := h.saveArticleJSON(article, outputPath); err != nil {
		h.logger.ErrorContext(ctx, "Failed to save article JSON", "error", err)
	} else {
		h.logger.InfoContext(ctx, "Article saved to JSON for review",
			"path", outputPath,
			"video_code", job.VideoCode,
		)
	}

	// TODO: Uncomment when ready to publish
	// if err := h.articlePublisher.PublishArticle(ctx, article); err != nil {
	// 	h.messenger.SendFailed(ctx, job.VideoID, err)
	// 	return fmt.Errorf("publish failed: %w", err)
	// }

	// === Done ===
	h.messenger.SendCompleted(ctx, job.VideoID)

	h.logger.InfoContext(ctx, "SEO job completed",
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
		"output_file", outputPath,
		"duration", time.Since(startTime),
	)

	return nil
}

// saveArticleJSON saves article content to JSON file for review
func (h *SEOHandler) saveArticleJSON(article *models.ArticleContent, path string) error {
	// Create output directory if not exists
	if err := os.MkdirAll("output", 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	jsonData, err := json.MarshalIndent(article, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal article: %w", err)
	}

	if err := os.WriteFile(path, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (h *SEOHandler) sendProgress(ctx context.Context, videoID, stage string, progress int) {
	update := models.NewProgressUpdate(videoID, stage, progress)
	if err := h.messenger.SendProgress(ctx, update); err != nil {
		h.logger.WarnContext(ctx, "Failed to send progress", "error", err)
	}
}

func (h *SEOHandler) buildArticle(
	job *models.SEOArticleJob,
	metadata *models.VideoMetadata,
	aiOutput *ports.AIOutput,
	casts []models.CastMetadata,
	maker *models.MakerMetadata,
	tags []models.TagMetadata,
	previousWorks []models.PreviousWork,
	galleryImages []models.GalleryImage,
	coverImage *models.ImageScore,
	audioURL string,
	audioDuration int,
) *models.ArticleContent {
	now := time.Now()

	// Build cast profiles with AI-generated bios
	castProfiles := make([]models.CastProfile, len(casts))
	for i, cast := range casts {
		bio := ""
		for _, cb := range aiOutput.CastBios {
			if cb.CastID == cast.ID {
				bio = cb.Bio
				break
			}
		}
		castProfiles[i] = models.CastProfile{
			ID:         cast.ID,
			Name:       cast.Name,
			NameTH:     cast.NameTH,
			Bio:        bio,
			ImageURL:   cast.ImageURL,
			ProfileURL: fmt.Sprintf("/casts/%s", cast.Slug),
		}
	}

	// Add alt texts to gallery images
	for i := range galleryImages {
		if i < len(aiOutput.GalleryAlts) {
			galleryImages[i].Alt = aiOutput.GalleryAlts[i]
		}
	}

	// Add URLs to key moments และแปลงหน่วยจาก ms เป็น seconds ถ้าจำเป็น
	for i := range aiOutput.KeyMoments {
		// ถ้า startOffset > 10000 แสดงว่าเป็น milliseconds ให้แปลงเป็น seconds
		if aiOutput.KeyMoments[i].StartOffset > 10000 {
			aiOutput.KeyMoments[i].StartOffset = aiOutput.KeyMoments[i].StartOffset / 1000
		}
		if aiOutput.KeyMoments[i].EndOffset > 10000 {
			aiOutput.KeyMoments[i].EndOffset = aiOutput.KeyMoments[i].EndOffset / 1000
		}
		aiOutput.KeyMoments[i].URL = fmt.Sprintf("/videos/%s?t=%d", job.VideoCode, aiOutput.KeyMoments[i].StartOffset)
	}

	// Build MakerInfo
	var makerInfo *models.MakerInfo
	if maker != nil {
		makerInfo = &models.MakerInfo{
			ID:         maker.ID,
			Name:       maker.Name,
			ProfileURL: fmt.Sprintf("/makers/%s", maker.Slug),
		}
	}

	// Build TagDescriptions โดย merge กับ AI descriptions
	tagDescs := make([]models.TagDesc, 0, len(tags))
	for _, tag := range tags {
		desc := ""
		// หา description จาก AI output
		for _, td := range aiOutput.TagDescriptions {
			if td.ID == tag.ID || td.Name == tag.Name {
				desc = td.Description
				break
			}
		}
		tagDescs = append(tagDescs, models.TagDesc{
			ID:          tag.ID,
			Name:        tag.Name,
			Description: desc,
			URL:         fmt.Sprintf("/tags/%s", tag.Slug),
		})
	}

	// Convert TopQuotes from ports to models
	topQuotes := make([]models.TopQuote, len(aiOutput.TopQuotes))
	for i, tq := range aiOutput.TopQuotes {
		topQuotes[i] = models.TopQuote{
			Text:      tq.Text,
			Timestamp: tq.Timestamp,
			Emotion:   tq.Emotion,
			Context:   tq.Context,
		}
	}

	// Calculate reading time (200 words per minute)
	wordCount := len(aiOutput.Summary) + len(aiOutput.DetailedReview)
	readingTime := wordCount / 200
	if readingTime < 1 {
		readingTime = 1
	}

	// ใช้ cover image ที่คัดเลือกแล้ว (ถ้ามี) หรือ fallback เป็น thumbnail เดิม
	thumbnailURL := metadata.Thumbnail
	if coverImage != nil && coverImage.URL != "" {
		thumbnailURL = coverImage.URL
	}

	return &models.ArticleContent{
		// === Core ===
		VideoID:          metadata.ID,
		Title:            aiOutput.Title,
		MetaTitle:        aiOutput.MetaTitle,
		MetaDescription:  aiOutput.MetaDescription,
		Slug:             job.VideoCode,
		VideoName:        aiOutput.Title,
		VideoDescription: aiOutput.MetaDescription,
		ThumbnailURL:     thumbnailURL,
		ThumbnailAlt:     aiOutput.ThumbnailAlt,
		UploadDate:       metadata.ReleaseDate,
		Duration:         formatDuration(metadata.Duration),
		ContentURL:       fmt.Sprintf("https://subth.com/videos/%s", job.VideoCode),
		EmbedURL:         fmt.Sprintf("https://subth.com/embed/%s", job.VideoCode),
		KeyMoments:       aiOutput.KeyMoments,
		Summary:          aiOutput.Summary,
		Highlights:       aiOutput.Highlights,
		DetailedReview:   aiOutput.DetailedReview,

		// === Cast & Relations ===
		CastProfiles:    castProfiles,
		MakerInfo:       makerInfo,
		PreviousWorks:   previousWorks,
		TagDescriptions: tagDescs,

		// === [E] Experience ===
		SceneLocations: aiOutput.SceneLocations,

		// === [E] Expertise ===
		DialogueAnalysis:      aiOutput.DialogueAnalysis,
		CharacterInsight:      aiOutput.CharacterInsight,
		TopQuotes:             topQuotes,
		LanguageNotes:         aiOutput.LanguageNotes,
		ActorPerformanceTrend: aiOutput.ActorPerformanceTrend,
		ComparisonNote:        aiOutput.ComparisonNote,

		// === [A] Authoritativeness ===
		SummaryShort:       aiOutput.SummaryShort,
		CharacterDynamic:   aiOutput.CharacterDynamic,
		PlotAnalysis:       aiOutput.PlotAnalysis,
		Recommendation:     aiOutput.Recommendation,
		RecommendedFor:     aiOutput.RecommendedFor,
		ThematicKeywords:   aiOutput.ThematicKeywords,
		SettingDescription: aiOutput.SettingDescription,
		MoodTone:           aiOutput.MoodTone,

		// === [T] Trustworthiness ===
		TranslationMethod: aiOutput.TranslationMethod,
		TranslationNote:   aiOutput.TranslationNote,
		SubtitleQuality:   aiOutput.SubtitleQuality,
		TechnicalFAQ:      aiOutput.TechnicalFAQ,

		// === SEO Enhancement ===
		ExpertAnalysis:   aiOutput.ExpertAnalysis,
		QualityScore:     aiOutput.QualityScore,
		Keywords:         aiOutput.Keywords,
		LongTailKeywords: aiOutput.LongTailKeywords,
		ReadingTime:      readingTime,

		// === TTS ===
		AudioSummaryURL: audioURL,
		AudioDuration:   audioDuration,

		// === Gallery & FAQ ===
		GalleryImages: galleryImages,
		FAQItems:      aiOutput.FAQItems,

		// === Timestamps ===
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// formatDuration converts seconds to ISO 8601 duration (PT1H30M)
func formatDuration(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	result := "PT"
	if hours > 0 {
		result += fmt.Sprintf("%dH", hours)
	}
	if minutes > 0 {
		result += fmt.Sprintf("%dM", minutes)
	}
	if secs > 0 || (hours == 0 && minutes == 0) {
		result += fmt.Sprintf("%dS", secs)
	}
	return result
}
