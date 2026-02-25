package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"

	"seo-worker/domain/ports"
)

// ============================================================================
// Gemini Client V2: 7-Chunk Parallel Execution
// ============================================================================
//
// Execution Flow:
//
//                    ┌─────────────┐
//                    │  CHUNK 1    │  (Sequential - Foundation)
//                    │  ~15 sec    │
//                    └──────┬──────┘
//                           │
//           ┌───────────────┼───────────────┐
//           ▼               ▼               ▼
//    ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
//    │  CHUNK 2    │ │  CHUNK 3    │ │  CHUNK 4    │  (Parallel)
//    │  ~12 sec    │ │  ~12 sec    │ │  ~15 sec    │
//    └──────┬──────┘ └──────┬──────┘ └──────┬──────┘
//           │               │               │
//           └───────────────┴───────────────┘
//                           │
//                    ┌──────▼──────┐
//                    │  CHUNK 5    │  (Sequential - needs 2,3,4)
//                    │  ~10 sec    │
//                    └──────┬──────┘
//                           │
//           ┌───────────────┴───────────────┐
//           ▼                               ▼
//    ┌─────────────┐               ┌─────────────┐
//    │  CHUNK 6    │               │  CHUNK 7    │  (Parallel)
//    │  ~10 sec    │               │  ~15 sec    │
//    └─────────────┘               └─────────────┘
//
// Total Time: ~55 sec (vs ~90 sec sequential)
// ============================================================================

// GenerateArticleContentV2 รัน 7-chunk pipeline แบบ parallel
func (c *GeminiClient) GenerateArticleContentV2(ctx context.Context, input *ports.AIInput) (*ports.AIOutput, error) {
	videoCode := input.VideoMetadata.RealCode
	if videoCode == "" {
		videoCode = input.VideoMetadata.Code
	}

	c.logger.InfoContext(ctx, "Starting 7-chunk V2 generation",
		"video_code", videoCode,
		"model", c.model,
	)

	startTime := time.Now()

	// ===== Phase 1: Chunk 1 (Foundation) =====
	c.logger.InfoContext(ctx, "[Phase 1] Generating Chunk 1: Core Identity...")
	chunk1, err := c.generateChunk1V2WithRetry(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("chunk1 failed: %w", err)
	}
	c.logger.InfoContext(ctx, "[Phase 1] Chunk 1 completed",
		"title_len", len(chunk1.Title),
		"summary_len", len(chunk1.Summary),
	)

	// Build CoreContext
	coreCtx := BuildCoreContext(chunk1, input.Casts, []string{})

	// Save state after Phase 1
	state := &ChunkStateV2{
		VideoCode:   videoCode,
		Chunk1:      chunk1,
		CoreContext: coreCtx,
		LastChunk:   1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	c.saveStateV2(state)

	// ===== Phase 2: Chunks 2, 3, 4 (Parallel) =====
	c.logger.InfoContext(ctx, "[Phase 2] Generating Chunks 2,3,4 in parallel...")
	chunk2, chunk3, chunk4, err := c.generateChunks234Parallel(ctx, input, coreCtx)
	if err != nil {
		return nil, &PartialGenerationErrorV2{
			Message:       "phase 2 failed",
			PartialPath:   fmt.Sprintf("output/state_%s.json", videoCode),
			FailedChunk:   2, // Could be 2, 3, or 4
			CompletedUpTo: 1,
			Cause:         err,
		}
	}
	c.logger.InfoContext(ctx, "[Phase 2] Chunks 2,3,4 completed",
		"highlights", len(chunk2.Highlights),
		"topQuotes", len(chunk3.TopQuotes),
		"detailedReview_len", len(chunk4.DetailedReview),
	)

	// Update CoreContext with scene locations from Chunk 2
	coreCtx.Entities.Locations = chunk2.SceneLocations

	// Save state after Phase 2
	state.Chunk2 = chunk2
	state.Chunk3 = chunk3
	state.Chunk4 = chunk4
	state.CoreContext = coreCtx
	state.LastChunk = 4
	state.UpdatedAt = time.Now()
	c.saveStateV2(state)

	// ===== Phase 3: Chunk 5 (Sequential - needs 2,3,4) =====
	c.logger.InfoContext(ctx, "[Phase 3] Generating Chunk 5: Recommendations...")
	chunk5, err := c.generateChunk5V2WithRetry(ctx, input, coreCtx, chunk2, chunk3, chunk4)
	if err != nil {
		return nil, &PartialGenerationErrorV2{
			Message:       "chunk5 failed",
			PartialPath:   fmt.Sprintf("output/state_%s.json", videoCode),
			FailedChunk:   5,
			CompletedUpTo: 4,
			Cause:         err,
		}
	}
	c.logger.InfoContext(ctx, "[Phase 3] Chunk 5 completed",
		"contextualLinks", len(chunk5.ContextualLinks),
		"moodTone", len(chunk5.MoodTone),
	)

	// Save state after Phase 3
	state.Chunk5 = chunk5
	state.LastChunk = 5
	state.UpdatedAt = time.Now()
	c.saveStateV2(state)

	// Build ExtendedContext for Phase 4
	extCtx := BuildExtendedContext(coreCtx, chunk2, chunk4)
	state.ExtendedContext = extCtx

	// ===== Phase 4: Chunks 6, 7 (Parallel) =====
	c.logger.InfoContext(ctx, "[Phase 4] Generating Chunks 6,7 in parallel...")
	chunk6, chunk7, err := c.generateChunks67Parallel(ctx, input, extCtx)
	if err != nil {
		return nil, &PartialGenerationErrorV2{
			Message:       "phase 4 failed",
			PartialPath:   fmt.Sprintf("output/state_%s.json", videoCode),
			FailedChunk:   6, // Could be 6 or 7
			CompletedUpTo: 5,
			Cause:         err,
		}
	}
	c.logger.InfoContext(ctx, "[Phase 4] Chunks 6,7 completed",
		"faqItems", len(chunk6.FAQItems),
		"cinematography_len", len(chunk7.CinematographyAnalysis),
	)

	// ===== Aggregate =====
	output := AggregateChunksV2(chunk1, chunk2, chunk3, chunk4, chunk5, chunk6, chunk7)

	// Clean up state file on full success
	os.Remove(fmt.Sprintf("output/state_%s.json", videoCode))

	elapsed := time.Since(startTime)
	c.logger.InfoContext(ctx, "7-chunk V2 generation completed successfully",
		"video_code", videoCode,
		"elapsed", elapsed.String(),
	)

	return output, nil
}

// ============================================================================
// Phase 2: Parallel execution of Chunks 2, 3, 4
// ============================================================================

func (c *GeminiClient) generateChunks234Parallel(
	ctx context.Context,
	input *ports.AIInput,
	coreCtx *CoreContext,
) (*Chunk2OutputV2, *Chunk3OutputV2, *Chunk4OutputV2, error) {
	var wg sync.WaitGroup
	var chunk2 *Chunk2OutputV2
	var chunk3 *Chunk3OutputV2
	var chunk4 *Chunk4OutputV2
	var err2, err3, err4 error

	wg.Add(3)

	// Chunk 2: Scene & Moments
	go func() {
		defer wg.Done()
		chunk2, err2 = c.generateChunk2V2WithRetry(ctx, input, coreCtx)
	}()

	// Chunk 3: Expertise
	go func() {
		defer wg.Done()
		chunk3, err3 = c.generateChunk3V2WithRetry(ctx, input, coreCtx)
	}()

	// Chunk 4: Authority
	go func() {
		defer wg.Done()
		chunk4, err4 = c.generateChunk4V2WithRetry(ctx, input, coreCtx)
	}()

	wg.Wait()

	// Check for errors
	if err2 != nil {
		return nil, nil, nil, fmt.Errorf("chunk2 failed: %w", err2)
	}
	if err3 != nil {
		return nil, nil, nil, fmt.Errorf("chunk3 failed: %w", err3)
	}
	if err4 != nil {
		return nil, nil, nil, fmt.Errorf("chunk4 failed: %w", err4)
	}

	return chunk2, chunk3, chunk4, nil
}

// ============================================================================
// Phase 4: Parallel execution of Chunks 6, 7
// ============================================================================

func (c *GeminiClient) generateChunks67Parallel(
	ctx context.Context,
	input *ports.AIInput,
	extCtx *ExtendedContext,
) (*Chunk6OutputV2, *Chunk7OutputV2, error) {
	var wg sync.WaitGroup
	var chunk6 *Chunk6OutputV2
	var chunk7 *Chunk7OutputV2
	var err6, err7 error

	wg.Add(2)

	// Chunk 6: Technical & FAQ
	go func() {
		defer wg.Done()
		chunk6, err6 = c.generateChunk6V2WithRetry(ctx, input, extCtx)
	}()

	// Chunk 7: Deep Analysis
	go func() {
		defer wg.Done()
		chunk7, err7 = c.generateChunk7V2WithRetry(ctx, input, extCtx)
	}()

	wg.Wait()

	// Check for errors
	if err6 != nil {
		return nil, nil, fmt.Errorf("chunk6 failed: %w", err6)
	}
	if err7 != nil {
		return nil, nil, fmt.Errorf("chunk7 failed: %w", err7)
	}

	return chunk6, chunk7, nil
}

// ============================================================================
// Individual Chunk Generators with Retry
// ============================================================================

func (c *GeminiClient) generateChunk1V2WithRetry(ctx context.Context, input *ports.AIInput) (*Chunk1OutputV2, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk1V2(ctx, input)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 1 V2] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk1 failed after %d retries: %w", maxRetries, lastErr)
}

func (c *GeminiClient) generateChunk2V2WithRetry(ctx context.Context, input *ports.AIInput, coreCtx *CoreContext) (*Chunk2OutputV2, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk2V2(ctx, input, coreCtx)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 2 V2] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk2 failed after %d retries: %w", maxRetries, lastErr)
}

func (c *GeminiClient) generateChunk3V2WithRetry(ctx context.Context, input *ports.AIInput, coreCtx *CoreContext) (*Chunk3OutputV2, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk3V2(ctx, input, coreCtx)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 3 V2] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk3 failed after %d retries: %w", maxRetries, lastErr)
}

func (c *GeminiClient) generateChunk4V2WithRetry(ctx context.Context, input *ports.AIInput, coreCtx *CoreContext) (*Chunk4OutputV2, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk4V2(ctx, input, coreCtx)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 4 V2] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk4 failed after %d retries: %w", maxRetries, lastErr)
}

func (c *GeminiClient) generateChunk5V2WithRetry(
	ctx context.Context,
	input *ports.AIInput,
	coreCtx *CoreContext,
	chunk2 *Chunk2OutputV2,
	chunk3 *Chunk3OutputV2,
	chunk4 *Chunk4OutputV2,
) (*Chunk5OutputV2, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk5V2(ctx, input, coreCtx, chunk2, chunk3, chunk4)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 5 V2] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk5 failed after %d retries: %w", maxRetries, lastErr)
}

func (c *GeminiClient) generateChunk6V2WithRetry(ctx context.Context, input *ports.AIInput, extCtx *ExtendedContext) (*Chunk6OutputV2, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk6V2(ctx, input, extCtx)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 6 V2] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk6 failed after %d retries: %w", maxRetries, lastErr)
}

func (c *GeminiClient) generateChunk7V2WithRetry(ctx context.Context, input *ports.AIInput, extCtx *ExtendedContext) (*Chunk7OutputV2, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk7V2(ctx, input, extCtx)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 7 V2] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk7 failed after %d retries: %w", maxRetries, lastErr)
}

// ============================================================================
// Individual Chunk Generators
// ============================================================================

func (c *GeminiClient) generateChunk1V2(ctx context.Context, input *ports.AIInput) (*Chunk1OutputV2, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = c.buildChunk1SchemaV2()

	prompt := c.buildChunk1PromptV2(input)
	prompt = sanitizeUTF8(prompt)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk1OutputV2
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk1v2_debug_%s.json", input.VideoMetadata.RealCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk1v2: %w", err)
	}

	return &chunk, nil
}

func (c *GeminiClient) generateChunk2V2(ctx context.Context, input *ports.AIInput, coreCtx *CoreContext) (*Chunk2OutputV2, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = c.buildChunk2SchemaV2()

	prompt := c.buildChunk2PromptV2(input, coreCtx)
	prompt = sanitizeUTF8(prompt)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk2OutputV2
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk2v2_debug_%s.json", input.VideoMetadata.RealCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk2v2: %w", err)
	}

	// Post-process: Safe Moments filtering
	chunk.KeyMoments = c.processKeyMomentsSafe(chunk.KeyMoments, input.VideoMetadata.Duration)

	return &chunk, nil
}

func (c *GeminiClient) generateChunk3V2(ctx context.Context, input *ports.AIInput, coreCtx *CoreContext) (*Chunk3OutputV2, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = c.buildChunk3SchemaV2()

	prompt := c.buildChunk3PromptV2(input, coreCtx)
	prompt = sanitizeUTF8(prompt)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk3OutputV2
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk3v2_debug_%s.json", input.VideoMetadata.RealCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk3v2: %w", err)
	}

	// Post-process: Filter topQuotes ที่ timestamp > 600 วินาที
	chunk.TopQuotes = c.filterTopQuotesSafe(chunk.TopQuotes)

	return &chunk, nil
}

func (c *GeminiClient) generateChunk4V2(ctx context.Context, input *ports.AIInput, coreCtx *CoreContext) (*Chunk4OutputV2, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = c.buildChunk4SchemaV2()

	prompt := c.buildChunk4PromptV2(input, coreCtx)
	prompt = sanitizeUTF8(prompt)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk4OutputV2
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk4v2_debug_%s.json", input.VideoMetadata.RealCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk4v2: %w", err)
	}

	// Post-process: Sanitize tagDescriptions
	chunk.TagDescriptions = c.sanitizeTagDescriptions(chunk.TagDescriptions)

	return &chunk, nil
}

func (c *GeminiClient) generateChunk5V2(
	ctx context.Context,
	input *ports.AIInput,
	coreCtx *CoreContext,
	chunk2 *Chunk2OutputV2,
	chunk3 *Chunk3OutputV2,
	chunk4 *Chunk4OutputV2,
) (*Chunk5OutputV2, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = c.buildChunk5SchemaV2()

	prompt := c.buildChunk5PromptV2(input, coreCtx, chunk2, chunk3, chunk4)
	prompt = sanitizeUTF8(prompt)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk5OutputV2
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk5v2_debug_%s.json", input.VideoMetadata.RealCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk5v2: %w", err)
	}

	return &chunk, nil
}

func (c *GeminiClient) generateChunk6V2(ctx context.Context, input *ports.AIInput, extCtx *ExtendedContext) (*Chunk6OutputV2, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = c.buildChunk6SchemaV2()

	prompt := c.buildChunk6PromptV2(input, extCtx)
	prompt = sanitizeUTF8(prompt)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk6OutputV2
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk6v2_debug_%s.json", input.VideoMetadata.RealCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk6v2: %w", err)
	}

	// Post-process: Filter keywords และ FAQ
	chunk.Keywords = c.filterSEOKeywords(chunk.Keywords)
	chunk.LongTailKeywords = c.filterSEOKeywords(chunk.LongTailKeywords)
	chunk.FAQItems = c.sanitizeFAQItems(chunk.FAQItems)

	return &chunk, nil
}

func (c *GeminiClient) generateChunk7V2(ctx context.Context, input *ports.AIInput, extCtx *ExtendedContext) (*Chunk7OutputV2, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = c.buildChunk7SchemaV2()

	prompt := c.buildChunk7PromptV2(input, extCtx)
	prompt = sanitizeUTF8(prompt)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk7OutputV2
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk7v2_debug_%s.json", input.VideoMetadata.RealCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk7v2: %w", err)
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
// State Management V2
// ============================================================================

func (c *GeminiClient) saveStateV2(state *ChunkStateV2) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	path := fmt.Sprintf("output/state_%s.json", state.VideoCode)
	return writeDebugFile(path, string(data))
}

func (c *GeminiClient) loadStateV2(videoCode string) (*ChunkStateV2, error) {
	path := fmt.Sprintf("output/state_%s.json", videoCode)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state ChunkStateV2
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// ResumeFromStateV2 ทำต่อจาก state ที่บันทึกไว้
func (c *GeminiClient) ResumeFromStateV2(ctx context.Context, input *ports.AIInput, videoCode string) (*ports.AIOutput, error) {
	state, err := c.loadStateV2(videoCode)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	c.logger.InfoContext(ctx, "Resuming V2 from saved state",
		"video_code", videoCode,
		"last_chunk", state.LastChunk,
	)

	// Resume based on last completed chunk
	var chunk2 *Chunk2OutputV2
	var chunk3 *Chunk3OutputV2
	var chunk4 *Chunk4OutputV2
	var chunk5 *Chunk5OutputV2
	var chunk6 *Chunk6OutputV2
	var chunk7 *Chunk7OutputV2

	switch state.LastChunk {
	case 1:
		// Need Phase 2 + Phase 3 + Phase 4
		chunk2, chunk3, chunk4, err = c.generateChunks234Parallel(ctx, input, state.CoreContext)
		if err != nil {
			return nil, err
		}
		state.Chunk2 = chunk2
		state.Chunk3 = chunk3
		state.Chunk4 = chunk4
		state.LastChunk = 4
		c.saveStateV2(state)
		fallthrough

	case 4:
		// Need Phase 3 + Phase 4
		if state.Chunk2 != nil {
			chunk2 = state.Chunk2
		}
		if state.Chunk3 != nil {
			chunk3 = state.Chunk3
		}
		if state.Chunk4 != nil {
			chunk4 = state.Chunk4
		}

		chunk5, err = c.generateChunk5V2WithRetry(ctx, input, state.CoreContext, chunk2, chunk3, chunk4)
		if err != nil {
			return nil, err
		}
		state.Chunk5 = chunk5
		state.LastChunk = 5
		c.saveStateV2(state)
		fallthrough

	case 5:
		// Need Phase 4
		if state.Chunk5 != nil {
			chunk5 = state.Chunk5
		}
		extCtx := BuildExtendedContext(state.CoreContext, state.Chunk2, state.Chunk4)

		chunk6, chunk7, err = c.generateChunks67Parallel(ctx, input, extCtx)
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
	if state.Chunk5 != nil && chunk5 == nil {
		chunk5 = state.Chunk5
	}

	// Aggregate
	output := AggregateChunksV2(state.Chunk1, chunk2, chunk3, chunk4, chunk5, chunk6, chunk7)

	// Clean up state file
	os.Remove(fmt.Sprintf("output/state_%s.json", videoCode))

	return output, nil
}
