package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"seo-worker/domain/models"
	"seo-worker/domain/ports"
)

// ============================================================================
// Constants & Configuration
// ============================================================================

const (
	maxRetries       = 3
	retryBaseDelay   = time.Second
	maxOutputTokens  = 4096 // Per chunk (ไม่ใช่ 8192 เพราะแบ่งเป็น 3 chunks แล้ว)
	defaultTemp      = 0.7  // สไตล์การเขียนคงที่ทุก chunk

	// Safe Moments Strategy for JAV
	safeKeyMomentsLimit = 600 // Hard limit: 10 นาทีแรกเท่านั้น (วินาที)
	minKeyMoments       = 3   // จำนวน key moments ขั้นต่ำ (Public Schema)
	maxKeyMomentsPublic = 5   // จำนวน key moments สูงสุดสำหรับ Public (Google)
	maxKeyMomentsInternal = 20 // จำนวน key moments สูงสุดสำหรับ Internal (Members)
)

// keywordBlacklist - คำต้องห้ามใน keyMoments name (explicit content)
var keywordBlacklist = []string{
	"เซ็กซ์", "เซ็ก", "sex", "ร่วมเพศ", "มีเพศสัมพันธ์",
	"ออรัล", "oral", "อมควย", "อมนม",
	"เย็ด", "เอา", "เสียว", "กระเด้า",
	"หี", "ควย", "นม", "หัวนม",
	"น้ำแตก", "แตก", "cum", "orgasm",
	"ท่าหมา", "doggy", "cowgirl", "missionary",
	"69", "threesome", "gangbang",
}

// seoKeywordBlacklist - คำต้องห้ามใน SEO keywords (สำหรับ Google)
var seoKeywordBlacklist = []string{
	"หนังโป๊", "โป๊", "porn", "xxx", "av",
	"เย็ด", "เอากัน", "ร่วมรัก",
	"หนังx", "หนังเอ็กซ์", "หนังผู้ใหญ่",
	"creampie", "แตกใน", "หลั่งใน",
	"blowjob", "อมควย",
}

// explicitTermReplacements - คำที่ต้องแทนที่ด้วยคำสุภาพ
var explicitTermReplacements = map[string]string{
	"หลั่งใน":          "ใกล้ชิดแบบพิเศษ",
	"แตกใน":           "ใกล้ชิดแบบพิเศษ",
	"การหลั่งภายใน":      "ความใกล้ชิดแบบพิเศษ",
	"หลั่งภายใน":        "ใกล้ชิดแบบพิเศษ",
	"ฉากหลั่งใน":        "ฉากโรแมนติกแบบใกล้ชิด",
	"ฉากแตกใน":         "ฉากจบแบบพิเศษ",
	"ฉากเซ็กส์":         "ฉากรักใคร่",
	"ฉากร่วมเพศ":        "ฉากรักใคร่",
	"ฉากร่วมรัก":        "ฉากโรแมนติก",
	"อวัยวะเพศ":         "ส่วนสงวน",
	"ช่องคลอด":         "ร่างกาย",
	"Creampie":        "ฉากจบแบบพิเศษ",
	"creampie":        "ฉากจบแบบพิเศษ",
}

// ============================================================================
// Helper Functions
// ============================================================================

func writeDebugFile(path, content string) error {
	_ = os.MkdirAll("output", 0755)
	return os.WriteFile(path, []byte(content), 0644)
}

func toPtr[T any](v T) *T {
	return &v
}

// ============================================================================
// GeminiClient
// ============================================================================

type GeminiClient struct {
	client *genai.Client
	model  string
	logger *slog.Logger
}

func NewGeminiClient(apiKey, model string) (*GeminiClient, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	return &GeminiClient{
		client: client,
		model:  model,
		logger: slog.Default().With("component", "gemini"),
	}, nil
}

func (c *GeminiClient) Close() error {
	return c.client.Close()
}

// ============================================================================
// Main Entry Point: 3-Chunk Pipeline
// ============================================================================

func (c *GeminiClient) GenerateArticleContent(ctx context.Context, input *ports.AIInput) (*ports.AIOutput, error) {
	videoCode := input.VideoMetadata.RealCode
	if videoCode == "" {
		videoCode = input.VideoMetadata.Code
	}

	c.logger.InfoContext(ctx, "Starting 4-chunk generation",
		"video_code", videoCode,
		"model", c.model,
	)

	// ===== Chunk 1: Core SEO =====
	c.logger.InfoContext(ctx, "[Chunk 1/4] Generating Core SEO...")
	chunk1, err := c.generateChunk1WithRetry(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("chunk1 failed: %w", err)
	}
	c.logger.InfoContext(ctx, "[Chunk 1/4] Completed",
		"title_len", len(chunk1.Title),
		"summary_len", len(chunk1.Summary),
		"highlights", len(chunk1.Highlights),
		"key_moments", len(chunk1.KeyMoments),
	)

	// Save state after Chunk 1
	state := &ChunkState{
		VideoCode: videoCode,
		Chunk1:    chunk1,
		LastChunk: 1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	c.saveState(state)

	// ===== Chunk 2: E-E-A-T Analysis (ใช้ context จาก Chunk 1) =====
	c.logger.InfoContext(ctx, "[Chunk 2/4] Generating E-E-A-T Analysis...")
	chunk2, err := c.generateChunk2WithRetry(ctx, input, chunk1)
	if err != nil {
		// Partial success: save state and return partial error
		return nil, &PartialGenerationError{
			Message:       "chunk2 failed after retries",
			PartialPath:   fmt.Sprintf("output/state_%s.json", videoCode),
			FailedChunk:   2,
			CompletedUpTo: 1,
			Cause:         err,
		}
	}
	c.logger.InfoContext(ctx, "[Chunk 2/4] Completed",
		"detailed_review_len", len(chunk2.DetailedReview),
		"cast_bios", len(chunk2.CastBios),
		"tag_descriptions", len(chunk2.TagDescriptions),
	)

	// Save state after Chunk 2
	state.Chunk2 = chunk2
	state.LastChunk = 2
	state.UpdatedAt = time.Now()
	c.saveState(state)

	// ===== Chunk 3: Technical + FAQ (ใช้ context จาก Chunk 1) =====
	c.logger.InfoContext(ctx, "[Chunk 3/4] Generating Technical + FAQ...")
	chunk3, err := c.generateChunk3WithRetry(ctx, input, chunk1)
	if err != nil {
		// Partial success: save state and return partial error
		return nil, &PartialGenerationError{
			Message:       "chunk3 failed after retries",
			PartialPath:   fmt.Sprintf("output/state_%s.json", videoCode),
			FailedChunk:   3,
			CompletedUpTo: 2,
			Cause:         err,
		}
	}
	c.logger.InfoContext(ctx, "[Chunk 3/4] Completed",
		"faq_items", len(chunk3.FAQItems),
		"keywords", len(chunk3.Keywords),
	)

	// Save state after Chunk 3
	state.Chunk3 = chunk3
	state.LastChunk = 3
	state.UpdatedAt = time.Now()
	c.saveState(state)

	// ===== Chunk 4: Deep Analysis (ใช้ context จาก Chunk 1 + Chunk 2) =====
	c.logger.InfoContext(ctx, "[Chunk 4/4] Generating Deep Analysis...")
	chunk4, err := c.generateChunk4WithRetry(ctx, input, chunk1, chunk2)
	if err != nil {
		// Partial success: save state and return partial error
		return nil, &PartialGenerationError{
			Message:       "chunk4 failed after retries",
			PartialPath:   fmt.Sprintf("output/state_%s.json", videoCode),
			FailedChunk:   4,
			CompletedUpTo: 3,
			Cause:         err,
		}
	}
	c.logger.InfoContext(ctx, "[Chunk 4/4] Completed",
		"cinematography_len", len(chunk4.CinematographyAnalysis),
		"character_journey_len", len(chunk4.CharacterJourney),
		"thematic_explanation_len", len(chunk4.ThematicExplanation),
	)

	// ===== Aggregate =====
	output := AggregateChunks(chunk1, chunk2, chunk3, chunk4)

	// Clean up state file on full success
	os.Remove(fmt.Sprintf("output/state_%s.json", videoCode))

	c.logger.InfoContext(ctx, "4-chunk generation completed successfully",
		"video_code", videoCode,
	)

	return output, nil
}

// ============================================================================
// Chunk Generators with Retry
// ============================================================================

func (c *GeminiClient) generateChunk1WithRetry(ctx context.Context, input *ports.AIInput) (*Chunk1Output, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk1(ctx, input)
		if err == nil {
			// Validate
			if valErr := c.validateChunk1(chunk); valErr != nil {
				lastErr = valErr
				c.logger.WarnContext(ctx, "[Chunk 1] Validation failed, retrying",
					"attempt", i+1,
					"error", valErr,
				)
				time.Sleep(retryBaseDelay * time.Duration(i+1))
				continue
			}
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 1] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk1 failed after %d retries: %w", maxRetries, lastErr)
}

func (c *GeminiClient) generateChunk2WithRetry(ctx context.Context, input *ports.AIInput, chunk1 *Chunk1Output) (*Chunk2Output, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk2(ctx, input, chunk1)
		if err == nil {
			// Validate
			if valErr := c.validateChunk2(chunk); valErr != nil {
				lastErr = valErr
				c.logger.WarnContext(ctx, "[Chunk 2] Validation failed, retrying",
					"attempt", i+1,
					"error", valErr,
				)
				time.Sleep(retryBaseDelay * time.Duration(i+1))
				continue
			}
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 2] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk2 failed after %d retries: %w", maxRetries, lastErr)
}

func (c *GeminiClient) generateChunk3WithRetry(ctx context.Context, input *ports.AIInput, chunk1 *Chunk1Output) (*Chunk3Output, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk3(ctx, input, chunk1)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 3] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk3 failed after %d retries: %w", maxRetries, lastErr)
}

func (c *GeminiClient) generateChunk4WithRetry(ctx context.Context, input *ports.AIInput, chunk1 *Chunk1Output, chunk2 *Chunk2Output) (*Chunk4Output, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk4(ctx, input, chunk1, chunk2)
		if err == nil {
			// Validate
			if valErr := c.validateChunk4(chunk); valErr != nil {
				lastErr = valErr
				c.logger.WarnContext(ctx, "[Chunk 4] Validation failed, retrying",
					"attempt", i+1,
					"error", valErr,
				)
				time.Sleep(retryBaseDelay * time.Duration(i+1))
				continue
			}
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 4] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk4 failed after %d retries: %w", maxRetries, lastErr)
}

// ============================================================================
// Individual Chunk Generators
// ============================================================================

func (c *GeminiClient) generateChunk1(ctx context.Context, input *ports.AIInput) (*Chunk1Output, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = c.buildChunk1Schema()

	prompt := c.buildChunk1Prompt(input)
	prompt = sanitizeUTF8(prompt) // Fix invalid UTF-8

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk1Output
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		// Save debug file
		debugPath := fmt.Sprintf("output/chunk1_debug_%s.json", input.VideoMetadata.RealCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk1: %w", err)
	}

	// Post-process: Safe Moments filtering for JAV content
	chunk.KeyMoments = c.processKeyMomentsSafe(chunk.KeyMoments, input.VideoMetadata.Duration)

	return &chunk, nil
}

func (c *GeminiClient) generateChunk2(ctx context.Context, input *ports.AIInput, chunk1 *Chunk1Output) (*Chunk2Output, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = c.buildChunk2Schema()

	prompt := c.buildChunk2Prompt(input, chunk1)
	prompt = sanitizeUTF8(prompt) // Fix invalid UTF-8

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk2Output
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk2_debug_%s.json", input.VideoMetadata.RealCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk2: %w", err)
	}

	// Post-process: Filter topQuotes ที่ timestamp > 600 วินาที
	chunk.TopQuotes = c.filterTopQuotesSafe(chunk.TopQuotes)

	// Post-process: Sanitize tagDescriptions ให้สุภาพ
	chunk.TagDescriptions = c.sanitizeTagDescriptions(chunk.TagDescriptions)

	return &chunk, nil
}

func (c *GeminiClient) generateChunk3(ctx context.Context, input *ports.AIInput, chunk1 *Chunk1Output) (*Chunk3Output, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = c.buildChunk3Schema()

	prompt := c.buildChunk3Prompt(input, chunk1)
	prompt = sanitizeUTF8(prompt) // Fix invalid UTF-8

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk3Output
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk3_debug_%s.json", input.VideoMetadata.RealCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk3: %w", err)
	}

	// Post-process: Filter keywords ที่ไม่เหมาะสมสำหรับ Google
	chunk.Keywords = c.filterSEOKeywords(chunk.Keywords)
	chunk.LongTailKeywords = c.filterSEOKeywords(chunk.LongTailKeywords)

	// Post-process: Sanitize faqItems ให้สุภาพ
	chunk.FAQItems = c.sanitizeFAQItems(chunk.FAQItems)

	return &chunk, nil
}

func (c *GeminiClient) generateChunk4(ctx context.Context, input *ports.AIInput, chunk1 *Chunk1Output, chunk2 *Chunk2Output) (*Chunk4Output, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = c.buildChunk4Schema()

	prompt := c.buildChunk4Prompt(input, chunk1, chunk2)
	prompt = sanitizeUTF8(prompt) // Fix invalid UTF-8

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk4Output
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk4_debug_%s.json", input.VideoMetadata.RealCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk4: %w", err)
	}

	// Post-process: Sanitize all text fields
	chunk.CinematographyAnalysis = c.sanitizeText(chunk.CinematographyAnalysis)
	chunk.CharacterJourney = c.sanitizeText(chunk.CharacterJourney)
	chunk.ThematicExplanation = c.sanitizeText(chunk.ThematicExplanation)
	chunk.ViewingTips = c.sanitizeText(chunk.ViewingTips)
	chunk.AudienceMatch = c.sanitizeText(chunk.AudienceMatch)

	return &chunk, nil
}

// ============================================================================
// Model Configuration
// ============================================================================

func (c *GeminiClient) configureModel(model *genai.GenerativeModel) {
	model.ResponseMIMEType = "application/json"
	model.Temperature = toPtr(float32(defaultTemp))
	model.TopP = toPtr(float32(0.95))
	model.TopK = toPtr(int32(40))
	model.MaxOutputTokens = toPtr(int32(maxOutputTokens))
}

// ============================================================================
// Response Extraction
// ============================================================================

func (c *GeminiClient) extractJSON(resp *genai.GenerateContentResponse) (string, error) {
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from gemini")
	}

	candidate := resp.Candidates[0]
	c.logger.Info("[DEBUG] Gemini response",
		"finish_reason", candidate.FinishReason,
		"parts_count", len(candidate.Content.Parts),
	)

	part := candidate.Content.Parts[0]
	jsonStr, ok := part.(genai.Text)
	if !ok {
		return "", fmt.Errorf("unexpected response type: %T", part)
	}

	// Sanitize JSON: fix huge numbers that would overflow int64
	sanitized := c.sanitizeJSONNumbers(string(jsonStr))

	return sanitized, nil
}

// sanitizeUTF8 ลบ invalid UTF-8 characters ออกจาก string
// ป้องกัน error: "proto: field contains invalid UTF-8"
func sanitizeUTF8(s string) string {
	// strings.ToValidUTF8 แทนที่ invalid UTF-8 sequences ด้วย replacement character
	// แต่เราลบทิ้งเลยดีกว่า (ใส่ "" แทน)
	return strings.ToValidUTF8(s, "")
}

// sanitizeJSONNumbers แก้ตัวเลขที่ใหญ่เกินใน JSON
// Gemini บางครั้งส่งตัวเลขที่ overflow int64 ทำให้ JSON parse fail
func (c *GeminiClient) sanitizeJSONNumbers(jsonStr string) string {
	// Regex: หาตัวเลขที่ยาวเกิน 15 หลัก (int64 max = 9223372036854775807 = 19 หลัก)
	// และแทนที่ด้วย 0 เพราะน่าจะเป็น bug จาก Gemini
	re := regexp.MustCompile(`:\s*(\d{16,})`)
	sanitized := re.ReplaceAllStringFunc(jsonStr, func(match string) string {
		// Extract the number part
		numStr := strings.TrimLeft(match, ": ")
		c.logger.Warn("[Sanitize] Found huge number, replacing with 0",
			"original_length", len(numStr),
			"preview", numStr[:min(20, len(numStr))],
		)
		return ": 0"
	})
	return sanitized
}

// ============================================================================
// Safe Moments Post-Processing (JAV-specific)
// ============================================================================

// processKeyMomentsSafe ประมวลผล keyMoments ให้ปลอดภัย
// 1. กรอง explicit keywords
// 2. จำกัดเวลาไม่เกิน 600 วินาที (10 นาทีแรก)
// 3. เรียงลำดับตาม startOffset
// 4. ลบ timestamps ที่ซ้อนทับกัน
func (c *GeminiClient) processKeyMomentsSafe(moments []models.KeyMoment, videoDuration int) []models.KeyMoment {
	if len(moments) == 0 {
		return moments
	}

	c.logger.Info("[Safe Moments] Processing",
		"input_count", len(moments),
		"video_duration", videoDuration,
	)

	// Step 1: Filter by keyword blacklist
	filtered := make([]models.KeyMoment, 0, len(moments))
	for _, m := range moments {
		if !c.containsBlacklistedKeyword(m.Name) {
			filtered = append(filtered, m)
		} else {
			c.logger.Debug("[Safe Moments] Filtered out",
				"name", m.Name,
				"reason", "blacklisted keyword",
			)
		}
	}

	// Step 2: Filter by time limit (600 seconds)
	safeFiltered := make([]models.KeyMoment, 0, len(filtered))
	for _, m := range filtered {
		if m.StartOffset <= safeKeyMomentsLimit {
			safeFiltered = append(safeFiltered, m)
		} else {
			c.logger.Debug("[Safe Moments] Filtered out",
				"name", m.Name,
				"start_offset", m.StartOffset,
				"reason", "exceeds 600s limit",
			)
		}
	}

	// Step 3: Sort by startOffset
	sort.Slice(safeFiltered, func(i, j int) bool {
		return safeFiltered[i].StartOffset < safeFiltered[j].StartOffset
	})

	// Step 4: Remove overlapping timestamps (keep only distinct 30-second buckets)
	deduped := make([]models.KeyMoment, 0, len(safeFiltered))
	seenBuckets := make(map[int]bool)
	for _, m := range safeFiltered {
		bucket := m.StartOffset / 30 // 30 วินาที bucket
		if !seenBuckets[bucket] {
			seenBuckets[bucket] = true
			deduped = append(deduped, m)
		} else {
			c.logger.Debug("[Safe Moments] Filtered out duplicate bucket",
				"name", m.Name,
				"bucket", bucket*30,
			)
		}
	}

	// Step 5: Ensure minimum coverage - add static seed moments if needed
	if len(deduped) < minKeyMoments {
		deduped = c.addSeedMoments(deduped, videoDuration)
	}

	// Step 6: Limit to maxKeyMomentsPublic (สำหรับ Google Schema)
	// Note: Internal moments (สำหรับ Members) จะใช้ maxKeyMomentsInternal
	if len(deduped) > maxKeyMomentsPublic {
		deduped = deduped[:maxKeyMomentsPublic]
	}

	c.logger.Info("[Safe Moments] Completed",
		"output_count", len(deduped),
		"mode", "public", // สำหรับ Google Schema
	)

	return deduped
}

// containsBlacklistedKeyword ตรวจสอบว่ามีคำต้องห้ามหรือไม่
func (c *GeminiClient) containsBlacklistedKeyword(text string) bool {
	textLower := strings.ToLower(text)
	for _, keyword := range keywordBlacklist {
		if strings.Contains(textLower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// addSeedMoments เพิ่ม static seed moments เมื่อมี moments ไม่พอ
// Static seeds: ใช้ชื่อสุภาพแบบวิชาการ/รีวิว ตาม E-E-A-T guidelines
func (c *GeminiClient) addSeedMoments(existing []models.KeyMoment, videoDuration int) []models.KeyMoment {
	seedMoments := []models.KeyMoment{
		{Name: "บทนำและการแนะนำตัวละครหลัก", StartOffset: 0, EndOffset: 90},
		{Name: "บทสนทนาเปิดเรื่องและการสร้างสถานการณ์", StartOffset: 120, EndOffset: 210},
		{Name: "การพัฒนาความสัมพันธ์ระหว่างตัวละคร", StartOffset: 240, EndOffset: 330},
		{Name: "จุดเปลี่ยนสำคัญของเนื้อเรื่อง", StartOffset: 360, EndOffset: 450},
		{Name: "ไคลแมกซ์ของบทบาทและอารมณ์", StartOffset: 480, EndOffset: 570},
	}

	// Collect existing start offsets to avoid overlap
	existingStarts := make(map[int]bool)
	for _, m := range existing {
		bucket := m.StartOffset / 60 // 60 วินาที bucket for seeds
		existingStarts[bucket] = true
	}

	// Add seeds that don't overlap
	result := append([]models.KeyMoment{}, existing...)
	for _, seed := range seedMoments {
		if len(result) >= minKeyMoments {
			break
		}
		bucket := seed.StartOffset / 60
		if !existingStarts[bucket] && seed.EndOffset <= videoDuration {
			result = append(result, seed)
			existingStarts[bucket] = true
			c.logger.Debug("[Safe Moments] Added seed moment",
				"name", seed.Name,
				"start", seed.StartOffset,
			)
		}
	}

	// Re-sort
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartOffset < result[j].StartOffset
	})

	return result
}

// ============================================================================
// Additional Post-Processing Filters
// ============================================================================

// filterTopQuotesSafe กรอง topQuotes ที่ timestamp > 600 วินาที
func (c *GeminiClient) filterTopQuotesSafe(quotes []ports.TopQuote) []ports.TopQuote {
	if len(quotes) == 0 {
		return quotes
	}

	filtered := make([]ports.TopQuote, 0, len(quotes))
	for _, q := range quotes {
		if q.Timestamp <= safeKeyMomentsLimit {
			filtered = append(filtered, q)
		} else {
			c.logger.Debug("[Safe Filter] Filtered out topQuote",
				"text", q.Text[:min(50, len(q.Text))],
				"timestamp", q.Timestamp,
				"reason", "exceeds 600s limit",
			)
		}
	}

	c.logger.Info("[Safe Filter] TopQuotes filtered",
		"input", len(quotes),
		"output", len(filtered),
	)

	return filtered
}

// filterSEOKeywords กรองคำที่ไม่เหมาะสมสำหรับ Google
func (c *GeminiClient) filterSEOKeywords(keywords []string) []string {
	if len(keywords) == 0 {
		return keywords
	}

	filtered := make([]string, 0, len(keywords))
	for _, kw := range keywords {
		if !c.containsSEOBlacklistedKeyword(kw) {
			filtered = append(filtered, kw)
		} else {
			c.logger.Debug("[SEO Filter] Filtered out keyword",
				"keyword", kw,
				"reason", "contains blacklisted word",
			)
		}
	}

	c.logger.Info("[SEO Filter] Keywords filtered",
		"input", len(keywords),
		"output", len(filtered),
	)

	return filtered
}

// containsSEOBlacklistedKeyword ตรวจสอบว่ามีคำต้องห้ามสำหรับ SEO หรือไม่
func (c *GeminiClient) containsSEOBlacklistedKeyword(text string) bool {
	textLower := strings.ToLower(text)
	for _, keyword := range seoKeywordBlacklist {
		if strings.Contains(textLower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// sanitizeText แทนที่คำไม่สุภาพด้วยคำสุภาพ
func (c *GeminiClient) sanitizeText(text string) string {
	result := text
	for explicit, polite := range explicitTermReplacements {
		if strings.Contains(result, explicit) {
			result = strings.ReplaceAll(result, explicit, polite)
			c.logger.Debug("[Sanitize] Replaced explicit term",
				"from", explicit,
				"to", polite,
			)
		}
	}
	return result
}

// sanitizeTagDescriptions แทนที่คำไม่สุภาพใน tagDescriptions
func (c *GeminiClient) sanitizeTagDescriptions(tags []models.TagDesc) []models.TagDesc {
	for i := range tags {
		tags[i].Description = c.sanitizeText(tags[i].Description)
	}
	return tags
}

// sanitizeFAQItems แทนที่คำไม่สุภาพใน faqItems
func (c *GeminiClient) sanitizeFAQItems(items []models.FAQItem) []models.FAQItem {
	for i := range items {
		items[i].Answer = c.sanitizeText(items[i].Answer)
	}
	return items
}

// ============================================================================
// Validation
// ============================================================================

func (c *GeminiClient) validateChunk1(chunk *Chunk1Output) error {
	var errors []string

	// ตรวจสอบความยาว summary (400 คำ ≈ 1,500 chars, tolerance 800)
	summaryChars := len([]rune(chunk.Summary))
	if summaryChars < 800 {
		errors = append(errors, fmt.Sprintf("summary: %d chars (min 800)", summaryChars))
	}

	// ตรวจสอบ highlights
	if len(chunk.Highlights) < 3 {
		errors = append(errors, fmt.Sprintf("highlights: %d items (min 3)", len(chunk.Highlights)))
	}

	// ตรวจสอบ key moments (หลังจาก Safe Moments processing แล้ว)
	// Note: อนุญาตให้ keyMoments ว่างได้ (Context Discovery rule - ถ้าวิดีโอไม่มี safe scenes)
	// แต่ถ้ามีแล้วต้องมีอย่างน้อย 3 ตัว
	if len(chunk.KeyMoments) > 0 && len(chunk.KeyMoments) < minKeyMoments {
		errors = append(errors, fmt.Sprintf("keyMoments: %d items (min %d or 0)", len(chunk.KeyMoments), minKeyMoments))
	}

	// ตรวจสอบ timestamps - เบาลงเพราะ Safe Moments processing ทำแล้ว
	for i, km := range chunk.KeyMoments {
		if km.EndOffset <= km.StartOffset {
			errors = append(errors, fmt.Sprintf("keyMoments[%d]: endOffset(%d) <= startOffset(%d)", i, km.EndOffset, km.StartOffset))
			break
		}
		// ตรวจสอบ duration อย่างน้อย 30 วินาที (ลดลงจาก 45)
		if km.EndOffset-km.StartOffset < 30 {
			errors = append(errors, fmt.Sprintf("keyMoments[%d]: duration %ds < 30s", i, km.EndOffset-km.StartOffset))
			break
		}
		// ตรวจสอบว่า startOffset ไม่เกิน 600 วินาที (Safe Moments limit)
		if km.StartOffset > safeKeyMomentsLimit {
			errors = append(errors, fmt.Sprintf("keyMoments[%d]: startOffset %d exceeds safe limit %d", i, km.StartOffset, safeKeyMomentsLimit))
			break
		}
	}

	// ตรวจสอบ title
	if len(chunk.Title) < 20 {
		errors = append(errors, fmt.Sprintf("title: %d chars (min 20)", len(chunk.Title)))
	}

	// ตรวจสอบ galleryAlts
	if len(chunk.GalleryAlts) == 0 {
		errors = append(errors, "galleryAlts: empty")
	}

	if len(errors) > 0 {
		return &ChunkValidationError{
			Chunk:   1,
			Field:   "multiple",
			Message: strings.Join(errors, "; "),
		}
	}

	return nil
}

func (c *GeminiClient) validateChunk2(chunk *Chunk2Output) error {
	var errors []string

	// ตรวจสอบความยาว detailedReview (600 คำ ≈ 2,000 chars, tolerance 1,000)
	detailedChars := len([]rune(chunk.DetailedReview))
	if detailedChars < 1000 {
		errors = append(errors, fmt.Sprintf("detailedReview: %d chars (min 1000)", detailedChars))
	}

	// ตรวจสอบ expertAnalysis (100 คำ ≈ 300 chars, tolerance 100)
	expertChars := len([]rune(chunk.ExpertAnalysis))
	if expertChars < 100 {
		errors = append(errors, fmt.Sprintf("expertAnalysis: %d chars (min 100)", expertChars))
	}

	// ตรวจสอบ dialogueAnalysis
	dialogueChars := len([]rune(chunk.DialogueAnalysis))
	if dialogueChars < 100 {
		errors = append(errors, fmt.Sprintf("dialogueAnalysis: %d chars (min 100)", dialogueChars))
	}

	// ตรวจสอบ topQuotes
	if len(chunk.TopQuotes) < 3 {
		errors = append(errors, fmt.Sprintf("topQuotes: %d items (min 3)", len(chunk.TopQuotes)))
	}

	// ตรวจสอบ tagDescriptions ไม่มี description ว่าง
	for i, td := range chunk.TagDescriptions {
		if len(strings.TrimSpace(td.Description)) < 10 {
			errors = append(errors, fmt.Sprintf("tagDescriptions[%d]: description empty or too short", i))
			break // แค่ตัวแรกที่ผิดก็พอ
		}
	}

	if len(errors) > 0 {
		return &ChunkValidationError{
			Chunk:   2,
			Field:   "multiple",
			Message: strings.Join(errors, "; "),
		}
	}

	return nil
}

func (c *GeminiClient) validateChunk4(chunk *Chunk4Output) error {
	var errors []string

	// ตรวจสอบ cinematographyAnalysis (300 คำ ≈ 900 chars, tolerance 500)
	cinematographyChars := len([]rune(chunk.CinematographyAnalysis))
	if cinematographyChars < 500 {
		errors = append(errors, fmt.Sprintf("cinematographyAnalysis: %d chars (min 500)", cinematographyChars))
	}

	// ตรวจสอบ characterJourney (400 คำ ≈ 1,200 chars, tolerance 600)
	characterChars := len([]rune(chunk.CharacterJourney))
	if characterChars < 600 {
		errors = append(errors, fmt.Sprintf("characterJourney: %d chars (min 600)", characterChars))
	}

	// ตรวจสอบ thematicExplanation (300 คำ ≈ 900 chars, tolerance 400)
	thematicChars := len([]rune(chunk.ThematicExplanation))
	if thematicChars < 400 {
		errors = append(errors, fmt.Sprintf("thematicExplanation: %d chars (min 400)", thematicChars))
	}

	// ตรวจสอบ viewingTips (200 คำ ≈ 600 chars, tolerance 300)
	viewingChars := len([]rune(chunk.ViewingTips))
	if viewingChars < 300 {
		errors = append(errors, fmt.Sprintf("viewingTips: %d chars (min 300)", viewingChars))
	}

	// ตรวจสอบ emotionalArc
	if len(chunk.EmotionalArc) < 3 {
		errors = append(errors, fmt.Sprintf("emotionalArc: %d items (min 3)", len(chunk.EmotionalArc)))
	}

	// ตรวจสอบ atmosphereNotes
	if len(chunk.AtmosphereNotes) < 3 {
		errors = append(errors, fmt.Sprintf("atmosphereNotes: %d items (min 3)", len(chunk.AtmosphereNotes)))
	}

	// ตรวจสอบ bestMoments
	if len(chunk.BestMoments) < 3 {
		errors = append(errors, fmt.Sprintf("bestMoments: %d items (min 3)", len(chunk.BestMoments)))
	}

	if len(errors) > 0 {
		return &ChunkValidationError{
			Chunk:   4,
			Field:   "multiple",
			Message: strings.Join(errors, "; "),
		}
	}

	return nil
}

// ============================================================================
// State Management
// ============================================================================

func (c *GeminiClient) saveState(state *ChunkState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	path := fmt.Sprintf("output/state_%s.json", state.VideoCode)
	return writeDebugFile(path, string(data))
}

func (c *GeminiClient) loadState(videoCode string) (*ChunkState, error) {
	path := fmt.Sprintf("output/state_%s.json", videoCode)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state ChunkState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// ResumeFromState ทำต่อจาก state ที่บันทึกไว้
func (c *GeminiClient) ResumeFromState(ctx context.Context, input *ports.AIInput, videoCode string) (*ports.AIOutput, error) {
	state, err := c.loadState(videoCode)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	c.logger.InfoContext(ctx, "Resuming from saved state",
		"video_code", videoCode,
		"last_chunk", state.LastChunk,
	)

	var chunk2 *Chunk2Output
	var chunk3 *Chunk3Output
	var chunk4 *Chunk4Output

	// Resume based on last completed chunk
	switch state.LastChunk {
	case 1:
		// Need to generate chunk 2, 3, and 4
		chunk2, err = c.generateChunk2WithRetry(ctx, input, state.Chunk1)
		if err != nil {
			return nil, err
		}
		state.Chunk2 = chunk2
		state.LastChunk = 2
		c.saveState(state)
		fallthrough

	case 2:
		// Need to generate chunk 3 and 4
		if state.Chunk2 != nil {
			chunk2 = state.Chunk2
		}
		chunk3, err = c.generateChunk3WithRetry(ctx, input, state.Chunk1)
		if err != nil {
			return nil, err
		}
		state.Chunk3 = chunk3
		state.LastChunk = 3
		c.saveState(state)
		fallthrough

	case 3:
		// Need to generate chunk 4
		if state.Chunk3 != nil {
			chunk3 = state.Chunk3
		}
		chunk4, err = c.generateChunk4WithRetry(ctx, input, state.Chunk1, state.Chunk2)
		if err != nil {
			return nil, err
		}
	}

	// Use saved chunks if available
	if state.Chunk3 != nil && chunk3 == nil {
		chunk3 = state.Chunk3
	}

	// Aggregate
	output := AggregateChunks(state.Chunk1, state.Chunk2, chunk3, chunk4)

	// Clean up state file
	os.Remove(fmt.Sprintf("output/state_%s.json", videoCode))

	return output, nil
}

// ============================================================================
// Interface Verification
// ============================================================================

var _ ports.AIPort = (*GeminiClient)(nil)
