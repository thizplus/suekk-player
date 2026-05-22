package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"

	"seo-worker/domain/models"
	"seo-worker/domain/ports"
	v3 "seo-worker/infrastructure/ai/review/v3"
)

// ============================================================================
// Gemini Client V3: 6-Chunk Intent-Driven Pipeline
// ============================================================================
//
// Execution Flow:
//
//                    ┌─────────────┐
//                    │  CHUNK 1    │  (Sequential - Foundation)
//                    │  ~10 sec    │  Search Intent Answer
//                    └──────┬──────┘
//                           │
//           ┌───────────────┴───────────────┐
//           ▼                               ▼
//    ┌─────────────┐               ┌─────────────┐
//    │  CHUNK 2    │               │  CHUNK 3    │  (Parallel)
//    │  ~8 sec     │               │  ~12 sec    │
//    │  Facts      │               │  Story      │
//    └──────┬──────┘               └──────┬──────┘
//           │                             │
//           └───────────────┬─────────────┘
//                           │
//                    ┌──────▼──────┐
//                    │  CHUNK 4    │  (Sequential - needs 1,3)
//                    │  ~12 sec    │  Review
//                    └──────┬──────┘
//                           │
//           ┌───────────────┴───────────────┐
//           ▼                               ▼
//    ┌─────────────┐               ┌─────────────┐
//    │  CHUNK 5    │               │  CHUNK 6    │  (Parallel)
//    │  ~8 sec     │               │  ~8 sec     │
//    │  FAQ        │               │  SEO        │
//    └─────────────┘               └─────────────┘
//
// Total Time: ~40 sec (vs ~60 sec sequential)
// ============================================================================

// GenerateArticleContentV3 รัน 6-chunk pipeline แบบ Intent-Driven
func (c *GeminiClient) GenerateArticleContentV3(ctx context.Context, input *ports.AIInput) (*models.ArticleContentV3, error) {
	videoCode := input.VideoMetadata.RealCode
	if videoCode == "" {
		videoCode = input.VideoMetadata.Code
	}

	// ตรวจสอบภาษา - ถ้า "en" ให้ใช้ English pipeline
	language := input.Language
	if language == "" {
		language = "th"
	}

	c.logger.InfoContext(ctx, "Starting 6-chunk V3 generation (Intent-Driven)",
		"video_code", videoCode,
		"model", c.model,
		"language", language,
	)

	// ถ้าเป็นภาษาอังกฤษ ให้ใช้ v3 package ที่รองรับ EN
	if language == "en" {
		return c.generateArticleContentV3EN(ctx, input, videoCode)
	}

	startTime := time.Now()

	// ===== Phase 1: Chunk 1 (Foundation - Search Intent) =====
	c.logger.InfoContext(ctx, "[Phase 1] Generating Chunk 1: Search Intent Answer...")
	chunk1, err := c.generateChunk1V3WithRetry(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("chunk1 failed: %w", err)
	}
	c.logger.InfoContext(ctx, "[Phase 1] Chunk 1 completed",
		"quickAnswer_len", len(chunk1.QuickAnswer),
		"verdict_len", len(chunk1.Verdict),
	)

	// Save state after Phase 1
	state := &ChunkStateV3{
		VideoCode: videoCode,
		Chunk1:    chunk1,
		LastChunk: 1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	c.saveStateV3(state)

	// ===== Phase 2: Chunks 2, 3 (Parallel - Facts + Story) =====
	c.logger.InfoContext(ctx, "[Phase 2] Generating Chunks 2,3 in parallel...")
	chunk2, chunk3, err := c.generateChunks23ParallelV3(ctx, input, chunk1)
	if err != nil {
		return nil, &PartialGenerationErrorV3{
			Message:       "phase 2 failed",
			PartialPath:   fmt.Sprintf("output/state_v3_%s.json", videoCode),
			FailedChunk:   2,
			CompletedUpTo: 1,
			Cause:         err,
		}
	}
	c.logger.InfoContext(ctx, "[Phase 2] Chunks 2,3 completed",
		"facts_code", chunk2.Code,
		"synopsis_len", len(chunk3.Synopsis),
	)

	// Save state after Phase 2
	state.Chunk2 = chunk2
	state.Chunk3 = chunk3
	state.LastChunk = 3
	state.UpdatedAt = time.Now()
	c.saveStateV3(state)

	// ===== Phase 3: Chunk 4 (Sequential - Review) =====
	c.logger.InfoContext(ctx, "[Phase 3] Generating Chunk 4: Review Section...")
	chunk4, err := c.generateChunk4V3WithRetry(ctx, input, chunk1, chunk3)
	if err != nil {
		return nil, &PartialGenerationErrorV3{
			Message:       "chunk4 failed",
			PartialPath:   fmt.Sprintf("output/state_v3_%s.json", videoCode),
			FailedChunk:   4,
			CompletedUpTo: 3,
			Cause:         err,
		}
	}
	c.logger.InfoContext(ctx, "[Phase 3] Chunk 4 completed",
		"reviewSummary_len", len(chunk4.ReviewSummary),
		"strengths", len(chunk4.Strengths),
		"weaknesses", len(chunk4.Weaknesses),
	)

	// Save state after Phase 3
	state.Chunk4 = chunk4
	state.LastChunk = 4
	state.UpdatedAt = time.Now()
	c.saveStateV3(state)

	// ===== Phase 4: Chunks 5, 6 (Parallel - FAQ + SEO) =====
	c.logger.InfoContext(ctx, "[Phase 4] Generating Chunks 5,6 in parallel...")
	chunk5, chunk6, err := c.generateChunks56ParallelV3(ctx, input, chunk1, chunk3, chunk4)
	if err != nil {
		return nil, &PartialGenerationErrorV3{
			Message:       "phase 4 failed",
			PartialPath:   fmt.Sprintf("output/state_v3_%s.json", videoCode),
			FailedChunk:   5,
			CompletedUpTo: 4,
			Cause:         err,
		}
	}
	c.logger.InfoContext(ctx, "[Phase 4] Chunks 5,6 completed",
		"faqItems", len(chunk5.FAQItems),
		"titleAggressive_len", len(chunk6.TitleAggressive),
		"rating", chunk6.Rating,
	)

	// ===== Get Variation (for logging/debugging) =====
	contentVariation := GetContentVariation(input.VideoMetadata.ID)
	titleStyleIndices := GetTitleStyleIndices(input.VideoMetadata.ID)

	variation := &VariationMeta{
		OpeningStyle: contentVariation.OpeningStyle,
		ReviewTone:   contentVariation.ReviewTone,
		FAQStyle:     contentVariation.FAQStyle,
		// Title/SEO style indices (for pattern distribution debugging)
		TitleAggressiveIdx: titleStyleIndices.AggressiveIdx,
		TitleBalancedIdx:   titleStyleIndices.BalancedIdx,
		IntentSetIdx:       titleStyleIndices.IntentSetIdx,
		IntentOrderSwapped: titleStyleIndices.IntentOrderSwapped,
	}

	// ===== Adjust Rating (realistic distribution) =====
	ratingAdj := adjustRatingV3(chunk6.Rating, chunk4, input.VideoMetadata.ID)

	c.logger.InfoContext(ctx, "Content variation applied",
		"video_id", input.VideoMetadata.ID,
		"opening_style", variation.OpeningStyle,
		"review_tone", variation.ReviewTone,
		"faq_style", variation.FAQStyle,
		"title_aggressive_idx", variation.TitleAggressiveIdx,
		"title_balanced_idx", variation.TitleBalancedIdx,
		"intent_set_idx", variation.IntentSetIdx,
		"intent_order_swapped", variation.IntentOrderSwapped,
		"original_rating", ratingAdj.OriginalRating,
		"adjusted_rating", ratingAdj.AdjustedRating,
		"adjust_reason", ratingAdj.AdjustReason,
	)

	// ===== Aggregate =====
	output := AggregateChunksV3(
		input.VideoMetadata.ID,
		"th",
		chunk1, chunk2, chunk3, chunk4, chunk5, chunk6,
		variation,
		ratingAdj,
	)

	// Clean up state file on full success
	os.Remove(fmt.Sprintf("output/state_v3_%s.json", videoCode))

	elapsed := time.Since(startTime)
	c.logger.InfoContext(ctx, "6-chunk V3 generation completed successfully",
		"video_code", videoCode,
		"elapsed", elapsed.String(),
	)

	return output, nil
}

// ============================================================================
// English V3 Pipeline - delegates to review/v3 package
// ============================================================================

func (c *GeminiClient) generateArticleContentV3EN(ctx context.Context, input *ports.AIInput, videoCode string) (*models.ArticleContentV3, error) {
	// สร้าง v3 client ชั่วคราว (ใช้ Gemini client และ model เดิม)
	v3Client, err := v3.NewClient(c.apiKey, c.model)
	if err != nil {
		return nil, fmt.Errorf("failed to create v3 client: %w", err)
	}
	defer v3Client.Close()

	// เรียก v3 package ที่รองรับภาษาอังกฤษ
	return v3Client.GenerateArticleContent(ctx, input)
}

// ============================================================================
// Phase 2: Parallel execution of Chunks 2, 3 (Facts + Story)
// ============================================================================

func (c *GeminiClient) generateChunks23ParallelV3(
	ctx context.Context,
	input *ports.AIInput,
	chunk1 *Chunk1OutputV3,
) (*Chunk2OutputV3, *Chunk3OutputV3, error) {
	var wg sync.WaitGroup
	var chunk2 *Chunk2OutputV3
	var chunk3 *Chunk3OutputV3
	var err2, err3 error

	wg.Add(2)

	// Chunk 2: Structured Facts
	go func() {
		defer wg.Done()
		chunk2, err2 = c.generateChunk2V3WithRetry(ctx, input)
	}()

	// Chunk 3: Story Recap
	go func() {
		defer wg.Done()
		chunk3, err3 = c.generateChunk3V3WithRetry(ctx, input, chunk1)
	}()

	wg.Wait()

	// Check for errors
	if err2 != nil {
		return nil, nil, fmt.Errorf("chunk2 failed: %w", err2)
	}
	if err3 != nil {
		return nil, nil, fmt.Errorf("chunk3 failed: %w", err3)
	}

	return chunk2, chunk3, nil
}

// ============================================================================
// Phase 4: Parallel execution of Chunks 5, 6 (FAQ + SEO)
// ============================================================================

func (c *GeminiClient) generateChunks56ParallelV3(
	ctx context.Context,
	input *ports.AIInput,
	chunk1 *Chunk1OutputV3,
	chunk3 *Chunk3OutputV3,
	chunk4 *Chunk4OutputV3,
) (*Chunk5OutputV3, *Chunk6OutputV3, error) {
	var wg sync.WaitGroup
	var chunk5 *Chunk5OutputV3
	var chunk6 *Chunk6OutputV3
	var err5, err6 error

	wg.Add(2)

	// Chunk 5: FAQ Intent Block
	go func() {
		defer wg.Done()
		chunk5, err5 = c.generateChunk5V3WithRetry(ctx, input, chunk1, chunk3, chunk4)
	}()

	// Chunk 6: SEO Output
	go func() {
		defer wg.Done()
		chunk6, err6 = c.generateChunk6V3WithRetry(ctx, input, chunk1, chunk3, chunk4)
	}()

	wg.Wait()

	// Check for errors
	if err5 != nil {
		return nil, nil, fmt.Errorf("chunk5 failed: %w", err5)
	}
	if err6 != nil {
		return nil, nil, fmt.Errorf("chunk6 failed: %w", err6)
	}

	return chunk5, chunk6, nil
}

// ============================================================================
// Individual Chunk Generators with Retry
// ============================================================================

func (c *GeminiClient) generateChunk1V3WithRetry(ctx context.Context, input *ports.AIInput) (*Chunk1OutputV3, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk1V3(ctx, input)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 1 V3] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk1 v3 failed after %d retries: %w", maxRetries, lastErr)
}

func (c *GeminiClient) generateChunk2V3WithRetry(ctx context.Context, input *ports.AIInput) (*Chunk2OutputV3, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk2V3(ctx, input)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 2 V3] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk2 v3 failed after %d retries: %w", maxRetries, lastErr)
}

func (c *GeminiClient) generateChunk3V3WithRetry(ctx context.Context, input *ports.AIInput, chunk1 *Chunk1OutputV3) (*Chunk3OutputV3, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk3V3(ctx, input, chunk1)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 3 V3] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk3 v3 failed after %d retries: %w", maxRetries, lastErr)
}

func (c *GeminiClient) generateChunk4V3WithRetry(ctx context.Context, input *ports.AIInput, chunk1 *Chunk1OutputV3, chunk3 *Chunk3OutputV3) (*Chunk4OutputV3, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk4V3(ctx, input, chunk1, chunk3)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 4 V3] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk4 v3 failed after %d retries: %w", maxRetries, lastErr)
}

func (c *GeminiClient) generateChunk5V3WithRetry(
	ctx context.Context,
	input *ports.AIInput,
	chunk1 *Chunk1OutputV3,
	chunk3 *Chunk3OutputV3,
	chunk4 *Chunk4OutputV3,
) (*Chunk5OutputV3, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk5V3(ctx, input, chunk1, chunk3, chunk4)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 5 V3] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk5 v3 failed after %d retries: %w", maxRetries, lastErr)
}

func (c *GeminiClient) generateChunk6V3WithRetry(
	ctx context.Context,
	input *ports.AIInput,
	chunk1 *Chunk1OutputV3,
	chunk3 *Chunk3OutputV3,
	chunk4 *Chunk4OutputV3,
) (*Chunk6OutputV3, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk6V3(ctx, input, chunk1, chunk3, chunk4)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 6 V3] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk6 v3 failed after %d retries: %w", maxRetries, lastErr)
}

// ============================================================================
// Individual Chunk Generators
// ============================================================================

func (c *GeminiClient) generateChunk1V3(ctx context.Context, input *ports.AIInput) (*Chunk1OutputV3, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = c.buildChunk1SchemaV3()

	prompt := c.buildChunk1PromptV3(input)
	prompt = sanitizeUTF8(prompt)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk1OutputV3
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk1v3_debug_%s.json", input.VideoMetadata.RealCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk1v3: %w", err)
	}

	return &chunk, nil
}

func (c *GeminiClient) generateChunk2V3(ctx context.Context, input *ports.AIInput) (*Chunk2OutputV3, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = c.buildChunk2SchemaV3()

	prompt := c.buildChunk2PromptV3(input)
	prompt = sanitizeUTF8(prompt)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk2OutputV3
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk2v3_debug_%s.json", input.VideoMetadata.RealCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk2v3: %w", err)
	}

	return &chunk, nil
}

func (c *GeminiClient) generateChunk3V3(ctx context.Context, input *ports.AIInput, chunk1 *Chunk1OutputV3) (*Chunk3OutputV3, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = c.buildChunk3SchemaV3()

	prompt := c.buildChunk3PromptV3(input, chunk1)
	prompt = sanitizeUTF8(prompt)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk3OutputV3
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		return nil, fmt.Errorf("failed to parse chunk3v3: %w", err)
	}

	return &chunk, nil
}

func (c *GeminiClient) generateChunk4V3(ctx context.Context, input *ports.AIInput, chunk1 *Chunk1OutputV3, chunk3 *Chunk3OutputV3) (*Chunk4OutputV3, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = c.buildChunk4SchemaV3()

	prompt := c.buildChunk4PromptV3(input, chunk1, chunk3)
	prompt = sanitizeUTF8(prompt)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk4OutputV3
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk4v3_debug_%s.json", input.VideoMetadata.RealCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk4v3: %w", err)
	}

	return &chunk, nil
}

func (c *GeminiClient) generateChunk5V3(
	ctx context.Context,
	input *ports.AIInput,
	chunk1 *Chunk1OutputV3,
	chunk3 *Chunk3OutputV3,
	chunk4 *Chunk4OutputV3,
) (*Chunk5OutputV3, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = c.buildChunk5SchemaV3()

	prompt := c.buildChunk5PromptV3(input, chunk1, chunk3, chunk4)
	prompt = sanitizeUTF8(prompt)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk5OutputV3
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk5v3_debug_%s.json", input.VideoMetadata.RealCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk5v3: %w", err)
	}

	// Post-process: Sanitize FAQ items
	chunk.FAQItems = c.sanitizeFAQItems(chunk.FAQItems)

	return &chunk, nil
}

func (c *GeminiClient) generateChunk6V3(
	ctx context.Context,
	input *ports.AIInput,
	chunk1 *Chunk1OutputV3,
	chunk3 *Chunk3OutputV3,
	chunk4 *Chunk4OutputV3,
) (*Chunk6OutputV3, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = c.buildChunk6SchemaV3()

	prompt := c.buildChunk6PromptV3(input, chunk1, chunk3, chunk4)
	prompt = sanitizeUTF8(prompt)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk6OutputV3
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk6v3_debug_%s.json", input.VideoMetadata.RealCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk6v3: %w", err)
	}

	// Post-process: Filter keywords and validate rating
	chunk.Keywords = c.filterSEOKeywords(chunk.Keywords)
	chunk.SearchIntents = c.filterSEOKeywords(chunk.SearchIntents)

	// Ensure rating is within valid range
	if chunk.Rating < 1.0 {
		chunk.Rating = 1.0
	} else if chunk.Rating > 5.0 {
		chunk.Rating = 5.0
	}

	return &chunk, nil
}

// ============================================================================
// State Management V3
// ============================================================================

func (c *GeminiClient) saveStateV3(state *ChunkStateV3) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	path := fmt.Sprintf("output/state_v3_%s.json", state.VideoCode)
	return writeDebugFile(path, string(data))
}

func (c *GeminiClient) loadStateV3(videoCode string) (*ChunkStateV3, error) {
	path := fmt.Sprintf("output/state_v3_%s.json", videoCode)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state ChunkStateV3
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// ResumeFromStateV3 ทำต่อจาก state ที่บันทึกไว้
func (c *GeminiClient) ResumeFromStateV3(ctx context.Context, input *ports.AIInput, videoCode string) (*models.ArticleContentV3, error) {
	state, err := c.loadStateV3(videoCode)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	c.logger.InfoContext(ctx, "Resuming V3 from saved state",
		"video_code", videoCode,
		"last_chunk", state.LastChunk,
	)

	var chunk2 *Chunk2OutputV3
	var chunk3 *Chunk3OutputV3
	var chunk4 *Chunk4OutputV3
	var chunk5 *Chunk5OutputV3
	var chunk6 *Chunk6OutputV3

	switch state.LastChunk {
	case 1:
		// Need Phase 2 + Phase 3 + Phase 4
		chunk2, chunk3, err = c.generateChunks23ParallelV3(ctx, input, state.Chunk1)
		if err != nil {
			return nil, err
		}
		state.Chunk2 = chunk2
		state.Chunk3 = chunk3
		state.LastChunk = 3
		c.saveStateV3(state)
		fallthrough

	case 3:
		// Need Phase 3 + Phase 4
		if state.Chunk2 != nil {
			chunk2 = state.Chunk2
		}
		if state.Chunk3 != nil {
			chunk3 = state.Chunk3
		}

		chunk4, err = c.generateChunk4V3WithRetry(ctx, input, state.Chunk1, chunk3)
		if err != nil {
			return nil, err
		}
		state.Chunk4 = chunk4
		state.LastChunk = 4
		c.saveStateV3(state)
		fallthrough

	case 4:
		// Need Phase 4
		if state.Chunk4 != nil {
			chunk4 = state.Chunk4
		}

		chunk5, chunk6, err = c.generateChunks56ParallelV3(ctx, input, state.Chunk1, state.Chunk3, chunk4)
		if err != nil {
			return nil, err
		}
	}

	// Use saved chunks if available
	if state.Chunk2 != nil && chunk2 == nil {
		chunk2 = state.Chunk2
	}
	if state.Chunk3 != nil && chunk3 == nil {
		chunk3 = state.Chunk3
	}
	if state.Chunk4 != nil && chunk4 == nil {
		chunk4 = state.Chunk4
	}

	// Get Variation (for logging/debugging)
	contentVariation := GetContentVariation(input.VideoMetadata.ID)
	titleStyleIndices := GetTitleStyleIndices(input.VideoMetadata.ID)

	variation := &VariationMeta{
		OpeningStyle:       contentVariation.OpeningStyle,
		ReviewTone:         contentVariation.ReviewTone,
		FAQStyle:           contentVariation.FAQStyle,
		TitleAggressiveIdx: titleStyleIndices.AggressiveIdx,
		TitleBalancedIdx:   titleStyleIndices.BalancedIdx,
		IntentSetIdx:       titleStyleIndices.IntentSetIdx,
		IntentOrderSwapped: titleStyleIndices.IntentOrderSwapped,
	}

	// Adjust Rating (realistic distribution)
	ratingAdj := adjustRatingV3(chunk6.Rating, chunk4, input.VideoMetadata.ID)

	// Aggregate
	output := AggregateChunksV3(
		input.VideoMetadata.ID,
		"th",
		state.Chunk1, chunk2, chunk3, chunk4, chunk5, chunk6,
		variation,
		ratingAdj,
	)

	// Clean up state file
	os.Remove(fmt.Sprintf("output/state_v3_%s.json", videoCode))

	return output, nil
}

// ============================================================================
// Error Types V3
// ============================================================================

type PartialGenerationErrorV3 struct {
	Message       string
	PartialPath   string
	FailedChunk   int
	CompletedUpTo int
	Cause         error
}

func (e *PartialGenerationErrorV3) Error() string {
	return fmt.Sprintf("%s (chunk %d failed, completed up to %d). Partial state saved to: %s. Cause: %v",
		e.Message, e.FailedChunk, e.CompletedUpTo, e.PartialPath, e.Cause)
}

func (e *PartialGenerationErrorV3) Unwrap() error {
	return e.Cause
}

// ============================================================================
// Rating Adjustment (Realistic Distribution)
// ============================================================================

// adjustRatingV3 ปรับ rating ให้ realistic
// Distribution target:
//   - 5% → 4.5+ (ยอดเยี่ยม)
//   - 30% → 3.8-4.4 (ดีมาก)
//   - 40% → 3.0-3.7 (ดี/ปานกลาง)
//   - 20% → 2.5-2.9 (พอใช้)
//   - 5% → < 2.5 (ไม่ค่อยดี)
//
// IMPORTANT: ใช้ deterministic seed ตาม videoID
// เพื่อให้ rating stable เมื่อ regenerate บทความเดิม
func adjustRatingV3(originalRating float64, chunk4 *Chunk4OutputV3, videoID string) *RatingAdjustment {
	adjusted := originalRating
	reason := "no_adjustment"

	strengthCount := len(chunk4.Strengths)
	weaknessCount := len(chunk4.Weaknesses)

	// Deterministic random based on videoID (stable across regenerations)
	h := hashString(videoID)
	deterministicRand := float64(h%100) / 100.0
	deterministicRand2 := float64((h/100)%100) / 100.0

	// Rule 1: High rating but has multiple weaknesses
	if originalRating > 4.5 && weaknessCount > 1 {
		adjusted = 4.0 + deterministicRand*0.4 // 4.0-4.4
		reason = "high_rating_with_weaknesses"
	}

	// Rule 2: More weaknesses than strengths → lower rating
	if weaknessCount >= strengthCount && originalRating > 3.5 {
		adjusted = 2.5 + deterministicRand*1.0 // 2.5-3.5
		reason = "weaknesses_exceed_strengths"
	}

	// Rule 3: All ratings above 4.0 → deterministically adjust some down
	if reason == "no_adjustment" && originalRating > 4.0 {
		// 40% chance (deterministic) to adjust down to mid-range
		if deterministicRand2 < 0.4 {
			adjusted = 3.0 + deterministicRand*0.7 // 3.0-3.7
			reason = "distribution_balance"
		}
	}

	// Ensure rating is within valid range
	if adjusted < 1.0 {
		adjusted = 1.0
	} else if adjusted > 5.0 {
		adjusted = 5.0
	}

	// Round to 1 decimal place
	adjusted = float64(int(adjusted*10)) / 10

	return &RatingAdjustment{
		OriginalRating: originalRating,
		AdjustedRating: adjusted,
		AdjustReason:   reason,
	}
}
