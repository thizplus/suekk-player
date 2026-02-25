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

	// 1.7 Fetch ALL gallery images from Suekk storage (Three-Tier)
	var galleryImages []models.GalleryImage
	var memberGalleryImages []models.GalleryImage
	var coverURL string

	h.logger.InfoContext(ctx, "[DEBUG] Gallery fetch start (Three-Tier)",
		"gallery_path", suekkVideoInfo.GalleryPath,
		"gallery_count", suekkVideoInfo.GalleryCount,
		"gallery_safe_count", suekkVideoInfo.GallerySafeCount,
		"gallery_nsfw_count", suekkVideoInfo.GalleryNsfwCount,
	)

	if suekkVideoInfo.GalleryPath != "" {
		// ดึงภาพจากทุก tier (super_safe, safe, nsfw)
		tieredImages, err := h.suekkVideoFetcher.ListAllGalleryImages(ctx, suekkVideoInfo.GalleryPath)
		if err != nil {
			h.logger.WarnContext(ctx, "Failed to list tiered gallery images",
				"gallery_path", suekkVideoInfo.GalleryPath,
				"error", err,
			)
		} else if tieredImages != nil {
			h.logger.InfoContext(ctx, "Tiered gallery images fetched",
				"super_safe", len(tieredImages.SuperSafe),
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
				// Fallback: ใช้ super_safe URLs ตรงๆ (ไม่ copy)
				for _, url := range tieredImages.SuperSafe {
					galleryImages = append(galleryImages, models.GalleryImage{URL: url, Width: 1280, Height: 720})
				}
				for _, url := range tieredImages.Safe {
					memberGalleryImages = append(memberGalleryImages, models.GalleryImage{URL: url, Width: 1280, Height: 720})
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

	aiOutput, err := h.aiService.GenerateArticleContent(ctx, aiInput)
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
		ContextualLinks: h.filterValidContextualLinks(aiOutput.ContextualLinks, relatedArticles),

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
		GalleryImages:       galleryImages,       // Public (super_safe) - R2
		MemberGalleryImages: memberGalleryImages, // Member only (safe + nsfw) - R2
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

// filterValidContextualLinks กรอง contextual links ที่ valid (slug ต้องมีอยู่จริง)
// ป้องกัน AI แต่ง slug ขึ้นมาเอง
func (h *SEOHandler) filterValidContextualLinks(
	links []models.ContextualLink,
	validArticles []ports.RelatedArticleForAI,
) []models.ContextualLink {
	if len(links) == 0 || len(validArticles) == 0 {
		return nil
	}

	// สร้าง map ของ valid slugs
	validSlugs := make(map[string]bool)
	for _, article := range validArticles {
		validSlugs[article.Slug] = true
	}

	// กรองเฉพาะ links ที่ slug อยู่ใน valid slugs
	filtered := make([]models.ContextualLink, 0, len(links))
	for _, link := range links {
		if validSlugs[link.LinkedSlug] {
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
		// Slug = lowercase video code
		slug := strings.ToLower(work.VideoCode)

		related[i] = ports.RelatedArticleForAI{
			Slug:      slug,
			Title:     work.Title,
			RealCode:  work.VideoCode,
			CastNames: castNames, // Same cast as current video
			Tags:      tagNames,  // Use current video's tags (approximation)
		}
	}

	h.logger.Info("Built related articles for contextual links",
		"count", len(related),
	)

	return related
}

// ============================================================================
// Cast Name Sanitization - ป้องกัน AI ผสมภาษาในชื่อนักแสดง
// ============================================================================

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

// mixedNameRegex - จับ pattern ที่มี Thai + English หรือ English + Thai ติดกัน
// Pattern: (Thai chars)(space?)(English chars) หรือ (English chars)(space?)(Thai chars)
var mixedNameRegex = regexp.MustCompile(`[\p{Thai}]+\s*[A-Za-z]+|[A-Za-z]+\s*[\p{Thai}]+`)

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

		// หาชื่อ cast ที่ตรงกับ English part
		if correctName, found := findMatchingCastName(match, castNameMap); found {
			replacementCount++
			return correctName
		}

		// ถ้าหาไม่เจอ ให้คงเดิม
		return match
	})

	return result, replacementCount
}

// sanitizeAIOutput ทำความสะอาด output จาก AI โดยแทนที่ชื่อนักแสดงที่ผิด
// ใช้ regex ตรวจจับ mixed-language names แล้ว match กับ cast จาก metadata
func (h *SEOHandler) sanitizeAIOutput(aiOutput *ports.AIOutput, casts []models.CastMetadata) {
	castNameMap := buildCastNameMap(casts)

	if len(castNameMap) == 0 {
		return
	}

	// Helper function to sanitize and count
	totalReplacements := 0
	sanitize := func(text string) string {
		result, count := sanitizeTextWithCastNames(text, castNameMap)
		totalReplacements += count
		return result
	}

	originalTitle := aiOutput.Title

	// Sanitize text fields
	aiOutput.Title = sanitize(aiOutput.Title)
	aiOutput.MetaTitle = sanitize(aiOutput.MetaTitle)
	aiOutput.MetaDescription = sanitize(aiOutput.MetaDescription)
	aiOutput.Summary = sanitize(aiOutput.Summary)
	aiOutput.SummaryShort = sanitize(aiOutput.SummaryShort)
	aiOutput.DetailedReview = sanitize(aiOutput.DetailedReview)
	aiOutput.ThumbnailAlt = sanitize(aiOutput.ThumbnailAlt)
	aiOutput.ExpertAnalysis = sanitize(aiOutput.ExpertAnalysis)
	aiOutput.DialogueAnalysis = sanitize(aiOutput.DialogueAnalysis)
	aiOutput.CharacterInsight = sanitize(aiOutput.CharacterInsight)
	aiOutput.CharacterDynamic = sanitize(aiOutput.CharacterDynamic)
	aiOutput.PlotAnalysis = sanitize(aiOutput.PlotAnalysis)
	aiOutput.Recommendation = sanitize(aiOutput.Recommendation)
	aiOutput.ActorPerformanceTrend = sanitize(aiOutput.ActorPerformanceTrend)
	aiOutput.ComparisonNote = sanitize(aiOutput.ComparisonNote)
	aiOutput.CinematographyAnalysis = sanitize(aiOutput.CinematographyAnalysis)
	aiOutput.CharacterJourney = sanitize(aiOutput.CharacterJourney)
	aiOutput.ThematicExplanation = sanitize(aiOutput.ThematicExplanation)
	aiOutput.ActorEvolution = sanitize(aiOutput.ActorEvolution)
	aiOutput.ViewingTips = sanitize(aiOutput.ViewingTips)
	aiOutput.AudienceMatch = sanitize(aiOutput.AudienceMatch)
	aiOutput.ReplayValue = sanitize(aiOutput.ReplayValue)

	// Sanitize array fields
	for i := range aiOutput.Highlights {
		aiOutput.Highlights[i] = sanitize(aiOutput.Highlights[i])
	}
	for i := range aiOutput.GalleryAlts {
		aiOutput.GalleryAlts[i] = sanitize(aiOutput.GalleryAlts[i])
	}
	for i := range aiOutput.Keywords {
		aiOutput.Keywords[i] = sanitize(aiOutput.Keywords[i])
	}
	for i := range aiOutput.LongTailKeywords {
		aiOutput.LongTailKeywords[i] = sanitize(aiOutput.LongTailKeywords[i])
	}
	for i := range aiOutput.BestMoments {
		aiOutput.BestMoments[i] = sanitize(aiOutput.BestMoments[i])
	}
	for i := range aiOutput.KeyMoments {
		aiOutput.KeyMoments[i].Name = sanitize(aiOutput.KeyMoments[i].Name)
	}
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
