package use_cases

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

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
	imageCopier       ports.ImageCopierPort
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
	imageCopier ports.ImageCopierPort,
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
		imageCopier:       imageCopier,
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

	// 1.5 Fetch previous works for each cast (จาก articles ที่ publish แล้ว)
	var previousWorks []models.PreviousWork
	for _, cast := range casts {
		works, _ := h.metadataFetcher.FetchPreviousWorks(ctx, cast.Slug, 5)
		previousWorks = append(previousWorks, works...)
	}

	h.logger.InfoContext(ctx, "Metadata loaded from video response",
		"casts_count", len(casts),
		"tags_count", len(tags),
		"has_maker", makerInfo != nil,
	)

	// 1.7 Fetch ALL gallery images from Suekk storage (Three-Tier)
	var galleryImages []models.GalleryImage
	var memberGalleryImages []models.GalleryImage
	var coverURL string

	h.logger.InfoContext(ctx, "[DEBUG] Gallery fetch start (Two-Tier)",
		"gallery_path", suekkVideoInfo.GalleryPath,
		"gallery_count", suekkVideoInfo.GalleryCount,
		"gallery_safe_count", suekkVideoInfo.GallerySafeCount,
		"gallery_nsfw_count", suekkVideoInfo.GalleryNsfwCount,
	)

	if suekkVideoInfo.GalleryPath != "" {
		// ดึงภาพจากทุก tier (safe, nsfw) - Two-Tier System
		tieredImages, err := h.suekkVideoFetcher.ListAllGalleryImages(ctx, suekkVideoInfo.GalleryPath)
		if err != nil {
			h.logger.WarnContext(ctx, "Failed to list tiered gallery images",
				"gallery_path", suekkVideoInfo.GalleryPath,
				"error", err,
			)
		} else if tieredImages != nil {
			h.logger.InfoContext(ctx, "Tiered gallery images fetched",
				"safe", len(tieredImages.Safe),
				"nsfw", len(tieredImages.NSFW),
			)

			// Copy ทุก tier ไป R2 แยก path (public/ และ member/)
			if h.imageCopier != nil {
				copyResult, err := h.imageCopier.CopyTieredGallery(ctx, job.VideoCode, tieredImages)
				if err != nil {
					h.logger.WarnContext(ctx, "Tiered gallery copy failed",
						"error", err,
					)
				} else if copyResult != nil {
					galleryImages = copyResult.PublicImages
					memberGalleryImages = copyResult.MemberImages
					coverURL = copyResult.CoverURL

					h.logger.InfoContext(ctx, "Gallery copied to R2",
						"public_count", len(galleryImages),
						"member_count", len(memberGalleryImages),
						"cover_url", coverURL,
					)
				}
			} else {
				// Fallback: ใช้ safe/nsfw URLs ตรงๆ (ไม่ copy)
				for _, url := range tieredImages.Safe {
					galleryImages = append(galleryImages, models.GalleryImage{URL: url, Width: 1280, Height: 720})
				}
				for _, url := range tieredImages.NSFW {
					memberGalleryImages = append(memberGalleryImages, models.GalleryImage{URL: url, Width: 1280, Height: 720})
				}
			}
		}
	} else {
		h.logger.WarnContext(ctx, "[DEBUG] No gallery path available")
	}

	h.logger.InfoContext(ctx, "[DEBUG] Gallery images final",
		"public_count", len(galleryImages),
		"member_count", len(memberGalleryImages),
		"has_cover", coverURL != "",
	)

	h.sendProgress(ctx, job.VideoID, ports.StageDataFetched, 25)

	// === Stage 2: AI Processing (Gemini with JSON Mode) ===
	h.sendProgress(ctx, job.VideoID, ports.StageAI, 30)

	// Build related articles for contextual linking (from previous works)
	relatedArticles := h.buildRelatedArticlesForAI(previousWorks, casts, tags)

	aiInput := &ports.AIInput{
		SRTContent:      srtContent,
		VideoMetadata:   metadata,
		Casts:           casts,
		Tags:            tags,
		PreviousWorks:   previousWorks,
		GalleryCount:    len(galleryImages),
		RelatedArticles: relatedArticles,
	}

	// ใช้ V2: 7-chunk pipeline (Atomic Chunking + Context Feeding)
	aiOutput, err := h.aiService.GenerateArticleContentV2(ctx, aiInput)
	if err != nil {
		h.messenger.SendFailed(ctx, job.VideoID, err)
		return fmt.Errorf("AI generation failed: %w", err)
	}

	// Sanitize AI output: แก้ไขชื่อนักแสดงที่ผสมภาษา
	h.sanitizeAIOutput(aiOutput, casts)

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

			// ใช้ SummaryShort ที่ AI สร้างมาเป็น TTS script โดยตรง
			ttsScript := aiOutput.SummaryShort
			if ttsScript == "" {
				h.logger.WarnContext(ctx, "SummaryShort is empty, skipping TTS")
				return
			}

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
	// (Images already copied to R2 in Stage 1.7)
	h.sendProgress(ctx, job.VideoID, ports.StagePublishing, 95)

	article := h.buildArticle(job, metadata, aiOutput, casts, makerInfo, tags, previousWorks, galleryImages, memberGalleryImages, coverURL, audioURL, audioDuration, relatedArticles)

	// Save JSON for debug/review (always)
	outputPath := fmt.Sprintf("output/%s_article.json", job.VideoCode)
	if err := h.saveArticleJSON(article, outputPath); err != nil {
		h.logger.WarnContext(ctx, "Failed to save article JSON", "error", err)
	} else {
		h.logger.InfoContext(ctx, "Article saved to JSON for review",
			"path", outputPath,
			"video_code", job.VideoCode,
		)
	}

	// Publish article to api.subth.com
	if err := h.articlePublisher.PublishArticle(ctx, article); err != nil {
		h.messenger.SendFailed(ctx, job.VideoID, err)
		return fmt.Errorf("publish failed: %w", err)
	}

	h.logger.InfoContext(ctx, "Article published successfully",
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
	)

	// === Done ===
	h.messenger.SendCompleted(ctx, job.VideoID)

	h.logger.InfoContext(ctx, "SEO job completed",
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
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
	memberGalleryImages []models.GalleryImage,
	coverURL string,
	audioURL string,
	audioDuration int,
	relatedArticles []ports.RelatedArticleForAI,
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
	// ใช้ AI-generated alt ที่อธิบายฉากจาก script (ดูดีกว่า format แห้งๆ)
	for i := range galleryImages {
		if i < len(aiOutput.GalleryAlts) {
			galleryImages[i].Alt = aiOutput.GalleryAlts[i]
		} else {
			// Fallback สำหรับภาพที่เกินจำนวน alt ที่ AI generate
			galleryImages[i].Alt = fmt.Sprintf("ฉากจาก %s", metadata.RealCode)
		}
	}

	// Filter & validate key moments
	// Option B: เก็บเฉพาะ moments ในช่วง 10 นาทีแรก (600 วินาที) เพื่อหลีกเลี่ยง explicit content
	const safeThresholdSeconds = 600 // 10 นาที - ช่วง intro/story setup

	originalCount := len(aiOutput.KeyMoments)
	var safeKeyMoments []models.KeyMoment
	for _, km := range aiOutput.KeyMoments {
		// แปลง milliseconds เป็น seconds ถ้าจำเป็น
		if km.StartOffset > 10000 {
			km.StartOffset = km.StartOffset / 1000
		}
		if km.EndOffset > 10000 {
			km.EndOffset = km.EndOffset / 1000
		}

		// กรองเฉพาะ moments ที่อยู่ในช่วง safe (10 นาทีแรก)
		if km.StartOffset > safeThresholdSeconds {
			continue // ข้าม moments หลังช่วง safe
		}

		// Validate minimum duration: ต้องยาวอย่างน้อย 30 วินาที
		duration := km.EndOffset - km.StartOffset
		if duration < 30 {
			km.EndOffset = km.StartOffset + 30
			if metadata.Duration > 0 && km.EndOffset > metadata.Duration {
				km.EndOffset = metadata.Duration
			}
		}

		km.URL = fmt.Sprintf("/videos/%s?t=%d", job.VideoCode, km.StartOffset)
		safeKeyMoments = append(safeKeyMoments, km)
	}

	// ใช้เฉพาะ safe moments
	aiOutput.KeyMoments = safeKeyMoments

	h.logger.Info("Key moments filtered for safety",
		"original_count", originalCount,
		"safe_count", len(safeKeyMoments),
		"threshold_seconds", safeThresholdSeconds,
	)

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

	// ใช้ cover image ที่ copy ไป R2 แล้ว (ถ้ามี) หรือ fallback เป็น thumbnail เดิม
	thumbnailURL := metadata.Thumbnail
	if coverURL != "" {
		thumbnailURL = coverURL
	}

	// ใช้ RealCode (movie code เช่น DLDSS-471) เป็น slug สำหรับ SEO
	// Fallback เป็น internal code ถ้าไม่มี RealCode
	slug := strings.ToLower(metadata.RealCode)
	if slug == "" {
		slug = job.VideoCode
	}

	return &models.ArticleContent{
		// === Core ===
		VideoID:          metadata.ID,
		Title:            aiOutput.Title,
		MetaTitle:        aiOutput.MetaTitle,
		MetaDescription:  aiOutput.MetaDescription,
		Slug:             slug,
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
		ContextualLinks: h.filterValidContextualLinks(aiOutput.ContextualLinks, relatedArticles, slug),

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

		// === Technical Specs ===
		VideoQuality: aiOutput.VideoQuality,
		AudioQuality: aiOutput.AudioQuality,

		// === SEO Enhancement ===
		ExpertAnalysis:   aiOutput.ExpertAnalysis,
		QualityScore:     aiOutput.QualityScore,
		Keywords:         aiOutput.Keywords,
		LongTailKeywords: aiOutput.LongTailKeywords,
		ReadingTime:      readingTime,

		// === Chunk 4: Deep Analysis (SEO Text boost) ===
		CinematographyAnalysis: aiOutput.CinematographyAnalysis,
		VisualStyle:            aiOutput.VisualStyle,
		AtmosphereNotes:        aiOutput.AtmosphereNotes,
		CharacterJourney:       aiOutput.CharacterJourney,
		EmotionalArc:           convertEmotionalArcToModels(aiOutput.EmotionalArc),
		ThematicExplanation:    aiOutput.ThematicExplanation,
		CulturalContext:        aiOutput.CulturalContext,
		GenreInsights:          aiOutput.GenreInsights,
		StudioComparison:       aiOutput.StudioComparison,
		ActorEvolution:         aiOutput.ActorEvolution,
		GenreRanking:           aiOutput.GenreRanking,
		ViewingTips:            aiOutput.ViewingTips,
		BestMoments:            aiOutput.BestMoments,
		AudienceMatch:          aiOutput.AudienceMatch,
		ReplayValue:            aiOutput.ReplayValue,

		// === TTS ===
		AudioSummaryURL: audioURL,
		AudioDuration:   audioDuration,

		// === Gallery & FAQ ===
		GalleryImages:       galleryImages,       // Public (safe - admin approved) - R2
		MemberGalleryImages: memberGalleryImages, // Member only (nsfw - admin approved) - R2
		MemberGalleryCount:  len(memberGalleryImages),
		FAQItems:            aiOutput.FAQItems,

		// === Timestamps ===
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// convertEmotionalArcToModels แปลง ports.EmotionalArcPoint เป็น models.EmotionalArcPoint
func convertEmotionalArcToModels(arc []ports.EmotionalArcPoint) []models.EmotionalArcPoint {
	if len(arc) == 0 {
		return nil
	}
	result := make([]models.EmotionalArcPoint, len(arc))
	for i, p := range arc {
		result[i] = models.EmotionalArcPoint{
			Phase:       p.Phase,
			Emotion:     p.Emotion,
			Description: p.Description,
		}
	}
	return result
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

// filterValidContextualLinks กรอง contextual links ที่ valid
// - slug ต้องมีอยู่จริง (ป้องกัน AI แต่ง slug ขึ้นมาเอง)
// - ห้าม link ไปหาตัวเอง (self-reference)
// - เพิ่ม ThumbnailUrl จาก validArticles
func (h *SEOHandler) filterValidContextualLinks(
	links []models.ContextualLink,
	validArticles []ports.RelatedArticleForAI,
	currentSlug string,
) []models.ContextualLink {
	if len(links) == 0 || len(validArticles) == 0 {
		return nil
	}

	// สร้าง map ของ valid slugs -> article data (รวม ThumbnailUrl)
	validArticleMap := make(map[string]ports.RelatedArticleForAI)
	for _, article := range validArticles {
		validArticleMap[article.Slug] = article
	}

	// กรองเฉพาะ links ที่:
	// 1. slug อยู่ใน valid slugs
	// 2. ไม่ใช่ตัวเอง (self-reference)
	filtered := make([]models.ContextualLink, 0, len(links))
	for _, link := range links {
		// ห้าม link ไปหาตัวเอง
		if link.LinkedSlug == currentSlug {
			h.logger.Warn("Filtered out self-referencing contextual link",
				"slug", link.LinkedSlug,
				"reason", "self-reference",
			)
			continue
		}

		if article, ok := validArticleMap[link.LinkedSlug]; ok {
			// เพิ่ม ThumbnailUrl และ QualityScore จาก validArticles
			link.ThumbnailUrl = article.ThumbnailUrl
			link.QualityScore = article.QualityScore
			filtered = append(filtered, link)
		} else {
			h.logger.Warn("Filtered out invalid contextual link",
				"slug", link.LinkedSlug,
				"reason", "slug not in valid articles",
			)
		}
	}

	h.logger.Info("Filtered contextual links",
		"original", len(links),
		"valid", len(filtered),
	)

	return filtered
}

// buildRelatedArticlesForAI สร้าง RelatedArticles สำหรับ AI ใช้สร้าง contextual links
// ใช้ข้อมูลจาก previousWorks (ผลงานก่อนหน้าของ cast เดียวกัน)
func (h *SEOHandler) buildRelatedArticlesForAI(
	previousWorks []models.PreviousWork,
	casts []models.CastMetadata,
	tags []models.TagMetadata,
) []ports.RelatedArticleForAI {
	if len(previousWorks) == 0 {
		return nil
	}

	// Extract cast names
	castNames := make([]string, len(casts))
	for i, cast := range casts {
		castNames[i] = cast.Name
	}

	// Extract tag names
	tagNames := make([]string, len(tags))
	for i, tag := range tags {
		tagNames[i] = tag.Name
	}

	// Convert previousWorks to RelatedArticleForAI (max 5)
	maxRelated := 5
	if len(previousWorks) < maxRelated {
		maxRelated = len(previousWorks)
	}

	related := make([]ports.RelatedArticleForAI, maxRelated)
	for i := 0; i < maxRelated; i++ {
		work := previousWorks[i]

		// ใช้ Slug จาก API โดยตรง (e.g., "dass-541")
		// ไม่ใช้ VideoCode เพราะเป็น internal code (e.g., "3993bp6j")
		slug := work.Slug
		if slug == "" {
			// Fallback: ถ้าไม่มี slug ให้ใช้ lowercase VideoCode (legacy)
			slug = strings.ToLower(work.VideoCode)
		}

		related[i] = ports.RelatedArticleForAI{
			Slug:         slug,
			Title:        work.Title,
			RealCode:     work.VideoCode,
			CastNames:    castNames,         // Same cast as current video
			Tags:         tagNames,          // Use current video's tags (approximation)
			ThumbnailUrl: work.ThumbnailUrl, // Thumbnail for display
			QualityScore: work.QualityScore, // คะแนนคุณภาพ 1-10
		}
	}

	h.logger.Info("Built related articles for contextual links",
		"count", len(related),
	)

	return related
}

// ============================================================================
// Cast Name Sanitization - ป้องกัน AI ผสมภาษาและชื่อซ้ำๆ
// ============================================================================

// removeRepeatedNames ลบชื่อที่ซ้ำติดกัน เช่น "Megami Megami Jun" → "Megami Jun"
// Go regex ไม่รองรับ backreference จึงใช้วิธี split และ compare
func removeRepeatedNames(text string) string {
	// Step 1: ลบคำซ้ำติดกัน (single word)
	result := removeConsecutiveDuplicateWords(text)

	// Step 2: ลบ phrase ซ้ำติดกัน (2-3 words)
	result = removeRepeatedPhrases(result)

	return result
}

// removeConsecutiveDuplicateWords ลบคำที่ซ้ำติดกัน เช่น "Megami Megami" → "Megami"
func removeConsecutiveDuplicateWords(text string) string {
	// Split text into tokens (preserve punctuation)
	words := strings.Fields(text)
	if len(words) < 2 {
		return text
	}

	var result []string
	result = append(result, words[0])

	for i := 1; i < len(words); i++ {
		// Compare lowercase to handle case variations
		current := strings.ToLower(words[i])
		previous := strings.ToLower(words[i-1])

		// Skip if same as previous word (but not short words like "the", "a")
		if current == previous && len(current) > 2 {
			continue
		}
		result = append(result, words[i])
	}

	return strings.Join(result, " ")
}

// removeRepeatedPhrases ลบวลีที่ซ้ำติดกัน เช่น "Megami Jun Megami Jun" → "Megami Jun"
func removeRepeatedPhrases(text string) string {
	words := strings.Fields(text)
	if len(words) < 4 {
		return text
	}

	// ลอง 2-word phrase ก่อน
	result := removeDuplicatePhrase(words, 2)

	// แล้วลอง 3-word phrase
	result = removeDuplicatePhrase(result, 3)

	return strings.Join(result, " ")
}

// removeDuplicatePhrase ลบ phrase ที่ซ้ำติดกันตาม phraseLen
func removeDuplicatePhrase(words []string, phraseLen int) []string {
	if len(words) < phraseLen*2 {
		return words
	}

	var result []string
	i := 0

	for i < len(words) {
		// Check if next phrase is a duplicate
		if i+phraseLen*2 <= len(words) {
			phrase1 := strings.ToLower(strings.Join(words[i:i+phraseLen], " "))
			phrase2 := strings.ToLower(strings.Join(words[i+phraseLen:i+phraseLen*2], " "))

			if phrase1 == phrase2 {
				// Add phrase once and skip duplicates
				result = append(result, words[i:i+phraseLen]...)

				// Skip all consecutive duplicates
				j := i + phraseLen*2
				for j+phraseLen <= len(words) {
					nextPhrase := strings.ToLower(strings.Join(words[j:j+phraseLen], " "))
					if nextPhrase != phrase1 {
						break
					}
					j += phraseLen
				}
				i = j
				continue
			}
		}

		result = append(result, words[i])
		i++
	}

	return result
}

// replaceExcessiveNamesWithPronouns แทนชื่อที่ใช้บ่อยเกินไปด้วยสรรพนาม
// Pattern: FullName → FirstName → เธอ → FirstName → FirstName → เธอ (วนซ้ำ)
func replaceExcessiveNamesWithPronouns(text string, casts []models.CastMetadata) string {
	if len(casts) == 0 {
		return text
	}

	result := text

	// สำหรับแต่ละ cast
	for _, cast := range casts {
		fullName := cast.Name
		nameParts := strings.Fields(fullName)
		if len(nameParts) == 0 {
			continue
		}

		firstName := nameParts[0]

		// นับจำนวนครั้งที่ชื่อเต็มปรากฏ
		fullNameCount := strings.Count(result, fullName)

		// ถ้าชื่อเต็มปรากฏมากกว่า 3 ครั้ง ให้แทนบางส่วน
		if fullNameCount > 3 {
			// แยก text ตามชื่อเต็ม แล้วประกอบกลับพร้อมแทนค่า
			parts := strings.Split(result, fullName)
			var newParts []string

			for i, part := range parts {
				newParts = append(newParts, part)

				// ถ้าไม่ใช่ part สุดท้าย ต้องใส่ชื่อกลับ
				if i < len(parts)-1 {
					// Pattern: 1=FullName, 2=FirstName, 3=เธอ, 4=FirstName, 5=FirstName, 6=เธอ...
					occurrence := i + 1
					switch {
					case occurrence == 1:
						// ครั้งแรก: ใช้ชื่อเต็ม
						newParts = append(newParts, fullName)
					case occurrence%3 == 0:
						// ทุกครั้งที่ 3: ใช้ "เธอ"
						newParts = append(newParts, "เธอ")
					default:
						// ครั้งอื่นๆ: ใช้ first name
						newParts = append(newParts, firstName)
					}
				}
			}

			result = strings.Join(newParts, "")
		}
	}

	// Cleanup: ลบ "เธอ เธอ" ซ้ำ และ whitespace เกิน
	result = cleanupDoublePronouns(result)

	return result
}

// cleanupDoublePronouns ลบสรรพนามที่ซ้ำติดกัน และ whitespace เกิน
func cleanupDoublePronouns(text string) string {
	// แก้ "เธอ เธอ" → "เธอ"
	result := strings.ReplaceAll(text, "เธอ เธอ", "เธอ")

	// แก้ช่องว่างเกิน (multiple spaces → single space)
	spaceRegex := regexp.MustCompile(`\s{2,}`)
	result = spaceRegex.ReplaceAllString(result, " ")

	return result
}

// removeLeadingActorName ลบชื่อนักแสดงที่นำหน้าประโยค
// เช่น "Megami Jun, มุ่งมั่น..." → "มุ่งมั่น..."
// เช่น "Megami Jun แสดง..." → "แสดง..."
func removeLeadingActorName(text string, casts []models.CastMetadata) string {
	if len(casts) == 0 || text == "" {
		return text
	}

	result := strings.TrimSpace(text)

	for _, cast := range casts {
		name := cast.Name

		// ลอง match แบบ case-insensitive และ normalize spaces
		lowerResult := strings.ToLower(result)
		lowerName := strings.ToLower(name)

		// Pattern ต่างๆ ที่อาจเกิด (check lowercase)
		patterns := []struct {
			check  string // lowercase pattern to check
			remove string // actual pattern to remove
		}{
			{lowerName + ", ", name + ", "},
			{lowerName + " ", name + " "},
			{lowerName + "、", name + "、"},
			{lowerName + " " + lowerName + ", ", name + " " + name + ", "},
			{lowerName + " " + lowerName + " ", name + " " + name + " "},
		}

		for _, p := range patterns {
			if strings.HasPrefix(lowerResult, p.check) {
				// ลบโดยใช้ length ของ pattern
				result = strings.TrimSpace(result[len(p.remove):])
				break
			}
		}

		// ถ้ายังขึ้นต้นด้วยชื่อ ลองใช้ regex-like approach
		if strings.HasPrefix(strings.ToLower(result), lowerName) {
			// หา position หลังชื่อ
			afterName := result[len(name):]
			// ถ้าตัวถัดไปเป็น space, comma, หรือ Thai character → ลบชื่อออก
			if len(afterName) > 0 {
				firstChar := []rune(afterName)[0]
				if firstChar == ' ' || firstChar == ',' || firstChar == '、' ||
					(firstChar >= 0x0E00 && firstChar <= 0x0E7F) { // Thai Unicode range
					result = strings.TrimSpace(afterName)
				}
			}
		}
	}

	return result
}

// filterEmptyHighlights กรอง highlights ที่เป็นแค่ชื่อนักแสดงหรือสั้นเกินไป
func filterEmptyHighlights(highlights []string, casts []models.CastMetadata) []string {
	if len(highlights) == 0 {
		return highlights
	}

	castNames := buildCastNameSet(casts)

	var filtered []string
	for _, h := range highlights {
		trimmed := strings.TrimSpace(h)

		// ข้ามถ้าว่างเปล่า
		if trimmed == "" {
			continue
		}

		// ข้ามถ้าเป็นแค่ชื่อนักแสดง
		if castNames[strings.ToLower(trimmed)] {
			continue
		}

		// ข้ามถ้าสั้นเกินไป (น้อยกว่า 15 ตัวอักษร)
		if len([]rune(trimmed)) < 15 {
			continue
		}

		filtered = append(filtered, h)
	}

	return filtered
}

// filterEmptyKeyMoments กรอง KeyMoments ที่ชื่อเป็นแค่ชื่อนักแสดง (เฉพาะชื่อเต็ม)
func filterEmptyKeyMoments(moments []models.KeyMoment, casts []models.CastMetadata) []models.KeyMoment {
	if len(moments) == 0 {
		return moments
	}

	// สร้าง set เฉพาะชื่อเต็ม (ไม่รวม parts)
	fullNames := make(map[string]bool)
	for _, cast := range casts {
		fullNames[strings.ToLower(cast.Name)] = true
		if cast.NameTH != "" {
			fullNames[strings.ToLower(cast.NameTH)] = true
		}
	}

	var filtered []models.KeyMoment
	for _, m := range moments {
		trimmed := strings.TrimSpace(m.Name)

		// ข้ามถ้าว่างเปล่า
		if trimmed == "" {
			continue
		}

		// ข้ามถ้าเป็นแค่ชื่อเต็มนักแสดง (exact match)
		if fullNames[strings.ToLower(trimmed)] {
			continue
		}

		filtered = append(filtered, m)
	}

	return filtered
}

// buildCastNameSet สร้าง set ของชื่อ cast (full name และ parts)
func buildCastNameSet(casts []models.CastMetadata) map[string]bool {
	castNames := make(map[string]bool)
	for _, cast := range casts {
		castNames[strings.ToLower(cast.Name)] = true
		if cast.NameTH != "" {
			castNames[strings.ToLower(cast.NameTH)] = true
		}
		for _, part := range strings.Fields(cast.Name) {
			castNames[strings.ToLower(part)] = true
		}
	}
	return castNames
}

// filterInvalidFAQs กรอง FAQ ที่คำถามไม่สมบูรณ์ (แค่ชื่อ หรือสั้นเกินไป)
func filterInvalidFAQs(faqs []models.FAQItem, casts []models.CastMetadata) []models.FAQItem {
	if len(faqs) == 0 {
		return faqs
	}

	castNames := buildCastNameSet(casts)

	var filtered []models.FAQItem
	for _, faq := range faqs {
		question := strings.TrimSpace(faq.Question)

		// ข้ามถ้าคำถามว่างเปล่า
		if question == "" {
			continue
		}

		// ข้ามถ้าคำถามเป็นแค่ชื่อนักแสดง (+ ?)
		questionWithoutMark := strings.TrimSuffix(question, "?")
		questionWithoutMark = strings.TrimSpace(questionWithoutMark)
		if castNames[strings.ToLower(questionWithoutMark)] {
			continue
		}

		// ข้ามถ้าคำถามสั้นเกินไป (น้อยกว่า 15 ตัวอักษร ไม่นับ ?)
		if len([]rune(questionWithoutMark)) < 15 {
			continue
		}

		// ข้ามถ้าคำถามไม่มี keyword ที่สำคัญ (อะไร, ไหม, ยังไง, เท่าไหร่, ที่ไหน, ใคร, ทำไม)
		hasQuestionWord := strings.Contains(question, "อะไร") ||
			strings.Contains(question, "ไหม") ||
			strings.Contains(question, "ยังไง") ||
			strings.Contains(question, "เท่าไหร่") ||
			strings.Contains(question, "ที่ไหน") ||
			strings.Contains(question, "ใคร") ||
			strings.Contains(question, "ทำไม") ||
			strings.Contains(question, "เกี่ยวกับ") ||
			strings.Contains(question, "คือ") ||
			strings.Contains(question, "มี") ||
			strings.Contains(question, "ดี")

		if !hasQuestionWord {
			continue
		}

		filtered = append(filtered, faq)
	}

	return filtered
}

// containsThai ตรวจสอบว่า string มีตัวอักษรภาษาไทยหรือไม่
func containsThai(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Thai) {
			return true
		}
	}
	return false
}

// containsEnglish ตรวจสอบว่า string มีตัวอักษรภาษาอังกฤษหรือไม่
func containsEnglish(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

// convertParagraphMarkers แปลง [PARA] marker เป็น \n\n
// AI ถูกสั่งให้ใช้ [PARA] คั่นย่อหน้าแทน \n\n เพื่อหลีกเลี่ยงปัญหา JSON encoding
func convertParagraphMarkers(text string) string {
	// แปลง [PARA] เป็น \n\n
	result := strings.ReplaceAll(text, "[PARA]", "\n\n")
	// ลบ \n\n ที่ซ้ำกัน
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}
	// Trim leading/trailing whitespace
	result = strings.TrimSpace(result)
	return result
}

// extractEnglishPart ดึงเฉพาะส่วนที่เป็นภาษาอังกฤษออกมา
func extractEnglishPart(s string) string {
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == ' ' {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

// extractThaiPart ดึงเฉพาะส่วนที่เป็นภาษาไทยออกมา
func extractThaiPart(s string) string {
	var result strings.Builder
	for _, r := range s {
		// Thai Unicode range: 0x0E00 - 0x0E7F
		if r >= 0x0E00 && r <= 0x0E7F {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// mixedNameRegex - จับ pattern ที่มี Thai + English หรือ English + Thai ติดกัน
// Pattern: (Thai chars 1-10)(space?)(English chars 2-15) หรือ (English chars 2-15)(space?)(Thai chars 1-10)
// จำกัดความยาวเพื่อจับเฉพาะชื่อนักแสดง ไม่ใช่ประโยคทั้งหมด
// เช่น จับ "เมกามิ Jun" แต่ไม่จับ "Mami กับการทดลองสุดพิเศษ"
var mixedNameRegex = regexp.MustCompile(`[\p{Thai}]{1,10}\s*[A-Za-z]{2,15}|[A-Za-z]{2,15}\s*[\p{Thai}]{1,10}`)

// buildCastNameMap สร้าง map ของชื่อ cast สำหรับ lookup
// key = ส่วนของชื่อ (first name, last name, full name) lowercase
// value = ชื่อเต็ม EN
func buildCastNameMap(casts []models.CastMetadata) map[string]string {
	nameMap := make(map[string]string)

	for _, cast := range casts {
		fullName := cast.Name
		fullNameLower := strings.ToLower(fullName)

		// เพิ่ม full name
		nameMap[fullNameLower] = fullName

		// แยกชื่อเป็นส่วนๆ
		nameParts := strings.Fields(fullName)
		for _, part := range nameParts {
			partLower := strings.ToLower(part)
			// เก็บ mapping: part -> full name
			// ถ้ามีซ้ำ (เช่น หลาย cast มี first name เดียวกัน) จะ overwrite
			// แต่ส่วนใหญ่จะไม่มีปัญหาเพราะชื่อนักแสดงมักไม่ซ้ำกัน
			nameMap[partLower] = fullName
		}
	}

	return nameMap
}

// findMatchingCastName หาชื่อ cast ที่ตรงกับ English part ของ mixed name
func findMatchingCastName(mixedName string, castNameMap map[string]string) (string, bool) {
	englishPart := extractEnglishPart(mixedName)
	if englishPart == "" {
		return "", false
	}

	englishPartLower := strings.ToLower(englishPart)

	// ลองหา exact match ก่อน
	if fullName, ok := castNameMap[englishPartLower]; ok {
		return fullName, true
	}

	// ลองหา partial match (ถ้า englishPart เป็นส่วนหนึ่งของชื่อ)
	for key, fullName := range castNameMap {
		if strings.Contains(key, englishPartLower) || strings.Contains(englishPartLower, key) {
			return fullName, true
		}
	}

	return "", false
}

// sanitizeTextWithCastNames แทนที่ชื่อนักแสดงที่ผิดในข้อความ
// ใช้ regex หา mixed-language names แล้ว match กับ cast จาก metadata
// FIX: เก็บ Thai part ที่ไม่ใช่ชื่อไว้ (เช่น "Mami กับ" → "Zemba Mami กับ")
func sanitizeTextWithCastNames(text string, castNameMap map[string]string) (string, int) {
	if len(castNameMap) == 0 {
		return text, 0
	}

	replacementCount := 0

	result := mixedNameRegex.ReplaceAllStringFunc(text, func(match string) string {
		// ตรวจสอบว่าเป็น mixed language จริงๆ
		if !containsThai(match) || !containsEnglish(match) {
			return match
		}

		// ดึง English part
		englishPart := extractEnglishPart(match)
		if englishPart == "" {
			return match
		}

		// หาชื่อ cast ที่ตรงกับ English part
		if correctName, found := findMatchingCastName(match, castNameMap); found {
			// ตรวจสอบว่า Thai part เป็นส่วนหนึ่งของชื่อจริงๆ หรือเป็นคำอื่น
			thaiPart := extractThaiPart(match)

			// ถ้า Thai part ยาวกว่า 4 ตัวอักษร น่าจะเป็นคำปกติ ไม่ใช่ชื่อ
			// เช่น "กับการทดลอง" vs "เมกามิ"
			if len([]rune(thaiPart)) > 4 {
				// เก็บ Thai part ไว้ ไม่แทนที่
				return match
			}

			// Thai part สั้น น่าจะเป็นชื่อที่เขียนผิด เช่น "จุน" "มามิ"
			replacementCount++
			return correctName
		}

		// ถ้าหาไม่เจอ ให้คงเดิม
		return match
	})

	return result, replacementCount
}

// sanitizeAIOutput ทำความสะอาด output จาก AI โดย:
// 1. แทนที่ชื่อนักแสดงที่ผสมภาษา (mixed-language)
// 2. ลบชื่อที่ซ้ำติดกัน (repeated names)
// 3. แทนชื่อที่ใช้บ่อยเกินไปด้วยสรรพนาม (pronoun substitution)
func (h *SEOHandler) sanitizeAIOutput(aiOutput *ports.AIOutput, casts []models.CastMetadata) {
	castNameMap := buildCastNameMap(casts)

	// Helper function to sanitize with all steps
	totalReplacements := 0
	sanitize := func(text string) string {
		// Step 1: แก้ mixed-language names (e.g., "เมกามิ Jun" → "Megami Jun")
		result := text
		if len(castNameMap) > 0 {
			var count int
			result, count = sanitizeTextWithCastNames(result, castNameMap)
			totalReplacements += count
		}

		// Step 2: ลบชื่อซ้ำติดกัน (e.g., "Megami Megami Jun" → "Megami Jun")
		result = removeRepeatedNames(result)

		return result
	}

	// Helper for long text fields - includes pronoun substitution + paragraph markers conversion
	sanitizeLongText := func(text string) string {
		result := sanitize(text)
		// Step 3: แทนชื่อที่ใช้บ่อยด้วยสรรพนาม (เธอ, first name)
		result = replaceExcessiveNamesWithPronouns(result, casts)
		// Step 4: แปลง [PARA] markers เป็น \n\n (AI ใช้ [PARA] เพื่อหลีกเลี่ยง JSON encoding issues)
		result = convertParagraphMarkers(result)
		return result
	}

	originalTitle := aiOutput.Title

	// Sanitize short text fields (no pronoun substitution)
	aiOutput.Title = sanitize(aiOutput.Title)
	aiOutput.MetaTitle = sanitize(aiOutput.MetaTitle)

	// Ensure metaTitle มี "ซับไทย" (SEO keyword สำคัญ)
	if !strings.Contains(aiOutput.MetaTitle, "ซับไทย") {
		// เพิ่ม "ซับไทย" หลัง ] แรก หรือต่อท้าย
		if idx := strings.Index(aiOutput.MetaTitle, "]"); idx != -1 {
			aiOutput.MetaTitle = aiOutput.MetaTitle[:idx+1] + " ซับไทย" + aiOutput.MetaTitle[idx+1:]
		} else {
			aiOutput.MetaTitle = aiOutput.MetaTitle + " [ซับไทย]"
		}
	}

	aiOutput.MetaDescription = sanitize(aiOutput.MetaDescription)
	aiOutput.ThumbnailAlt = sanitize(aiOutput.ThumbnailAlt)

	// Sanitize long text fields (with pronoun substitution for natural reading)
	aiOutput.Summary = sanitizeLongText(aiOutput.Summary)
	aiOutput.SummaryShort = sanitize(aiOutput.SummaryShort) // TTS ใช้ชื่อเต็ม
	aiOutput.DetailedReview = sanitizeLongText(aiOutput.DetailedReview)
	aiOutput.ExpertAnalysis = sanitizeLongText(aiOutput.ExpertAnalysis)
	aiOutput.DialogueAnalysis = sanitizeLongText(aiOutput.DialogueAnalysis)
	aiOutput.CharacterInsight = sanitizeLongText(aiOutput.CharacterInsight)
	aiOutput.CharacterDynamic = sanitizeLongText(aiOutput.CharacterDynamic)
	aiOutput.PlotAnalysis = sanitizeLongText(aiOutput.PlotAnalysis)
	aiOutput.Recommendation = sanitizeLongText(aiOutput.Recommendation)
	aiOutput.ActorPerformanceTrend = sanitizeLongText(aiOutput.ActorPerformanceTrend)
	aiOutput.ComparisonNote = sanitizeLongText(aiOutput.ComparisonNote)
	aiOutput.CinematographyAnalysis = sanitizeLongText(aiOutput.CinematographyAnalysis)
	aiOutput.CharacterJourney = sanitizeLongText(aiOutput.CharacterJourney)
	aiOutput.ThematicExplanation = sanitizeLongText(aiOutput.ThematicExplanation)
	aiOutput.ActorEvolution = sanitizeLongText(aiOutput.ActorEvolution)
	aiOutput.ViewingTips = sanitizeLongText(aiOutput.ViewingTips)
	aiOutput.AudienceMatch = sanitizeLongText(aiOutput.AudienceMatch)
	aiOutput.ReplayValue = sanitizeLongText(aiOutput.ReplayValue)

	// Sanitize array fields - Highlights
	for i := range aiOutput.Highlights {
		// Step 1: ลบชื่อนักแสดงที่นำหน้าออกก่อน
		aiOutput.Highlights[i] = removeLeadingActorName(aiOutput.Highlights[i], casts)
		// Step 2: sanitize mixed-language และชื่อซ้ำ
		aiOutput.Highlights[i] = sanitize(aiOutput.Highlights[i])
	}

	// Filter out highlights that are just actor names or too short
	aiOutput.Highlights = filterEmptyHighlights(aiOutput.Highlights, casts)
	for i := range aiOutput.GalleryAlts {
		aiOutput.GalleryAlts[i] = sanitize(aiOutput.GalleryAlts[i])
	}
	for i := range aiOutput.Keywords {
		aiOutput.Keywords[i] = sanitize(aiOutput.Keywords[i])
	}
	for i := range aiOutput.LongTailKeywords {
		aiOutput.LongTailKeywords[i] = sanitize(aiOutput.LongTailKeywords[i])
	}
	// BestMoments - ลบชื่อนักแสดงที่นำหน้าออก
	for i := range aiOutput.BestMoments {
		// Step 1: ลบชื่อนักแสดงที่นำหน้าออกก่อน
		aiOutput.BestMoments[i] = removeLeadingActorName(aiOutput.BestMoments[i], casts)
		// Step 2: sanitize mixed-language และชื่อซ้ำ
		aiOutput.BestMoments[i] = sanitize(aiOutput.BestMoments[i])
	}

	// Filter out BestMoments that are just actor names
	aiOutput.BestMoments = filterEmptyHighlights(aiOutput.BestMoments, casts)

	for i := range aiOutput.KeyMoments {
		aiOutput.KeyMoments[i].Name = sanitize(aiOutput.KeyMoments[i].Name)
	}

	// Filter out KeyMoments that are just actor names
	aiOutput.KeyMoments = filterEmptyKeyMoments(aiOutput.KeyMoments, casts)

	for i := range aiOutput.CastBios {
		aiOutput.CastBios[i].Bio = sanitize(aiOutput.CastBios[i].Bio)
	}
	for i := range aiOutput.TopQuotes {
		aiOutput.TopQuotes[i].Context = sanitize(aiOutput.TopQuotes[i].Context)
	}
	for i := range aiOutput.FAQItems {
		aiOutput.FAQItems[i].Question = sanitize(aiOutput.FAQItems[i].Question)
		aiOutput.FAQItems[i].Answer = sanitize(aiOutput.FAQItems[i].Answer)
	}

	// Filter out FAQ items with invalid questions (just names or too short)
	aiOutput.FAQItems = filterInvalidFAQs(aiOutput.FAQItems, casts)

	for i := range aiOutput.EmotionalArc {
		aiOutput.EmotionalArc[i].Description = sanitize(aiOutput.EmotionalArc[i].Description)
	}

	// Log if title was changed
	if originalTitle != aiOutput.Title {
		h.logger.Info("Sanitized mixed-language cast name in title",
			"original", originalTitle,
			"sanitized", aiOutput.Title,
		)
	}

	if totalReplacements > 0 {
		h.logger.Info("AI output sanitized for mixed-language cast names",
			"total_replacements", totalReplacements,
			"casts", len(casts),
		)
	}
}
