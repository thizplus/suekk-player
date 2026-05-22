package v3

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"seo-worker/domain/models"
	"seo-worker/domain/ports"
	en "seo-worker/infrastructure/ai/review/v3/en"
	th "seo-worker/infrastructure/ai/review/v3/th"
)

// ============================================================================
// V3 Client: 6-Chunk Intent-Driven Pipeline
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

const (
	maxRetries     = 3
	retryBaseDelay = 2 * time.Second
)

// Client is the V3 AI generation client
type Client struct {
	client *genai.Client
	model  string
	logger *slog.Logger
}

// NewClient creates a new V3 client
func NewClient(apiKey, model string) (*Client, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &Client{
		client: client,
		model:  model,
		logger: slog.Default().With("component", "v3_client"),
	}, nil
}

// Close closes the client
func (c *Client) Close() error {
	return c.client.Close()
}

// GenerateArticleContent runs the 6-chunk V3 pipeline
func (c *Client) GenerateArticleContent(ctx context.Context, input *ports.AIInput) (*models.ArticleContentV3, error) {
	videoCode := input.VideoMetadata.RealCode
	if videoCode == "" {
		videoCode = input.VideoMetadata.Code
	}

	// Determine language (default: th)
	language := input.Language
	if language == "" {
		language = "th"
	}

	c.logger.InfoContext(ctx, "Starting 6-chunk V3 generation (Intent-Driven)",
		"video_code", videoCode,
		"model", c.model,
		"language", language,
	)

	// Route to language-specific pipeline
	if language == "en" {
		return c.generateArticleContentEN(ctx, input, videoCode)
	}
	return c.generateArticleContentTH(ctx, input, videoCode)
}

// generateArticleContentTH runs the Thai V3 pipeline
func (c *Client) generateArticleContentTH(ctx context.Context, input *ports.AIInput, videoCode string) (*models.ArticleContentV3, error) {

	startTime := time.Now()

	// Build prompt params
	params := c.buildPromptParams(input)

	// ===== Phase 1: Chunk 1 (Foundation - Search Intent) =====
	c.logger.InfoContext(ctx, "[Phase 1] Generating Chunk 1: Search Intent Answer...")
	chunk1, err := c.generateChunk1WithRetry(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("chunk1 failed: %w", err)
	}
	c.logger.InfoContext(ctx, "[Phase 1] Chunk 1 completed",
		"quickAnswer_len", len(chunk1.QuickAnswer),
		"verdict_len", len(chunk1.Verdict),
	)

	// Save state after Phase 1
	state := &ChunkState{
		VideoCode: videoCode,
		Chunk1:    chunk1,
		LastChunk: 1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	c.saveState(state)

	// ===== Phase 2: Chunks 2, 3 (Parallel - Facts + Story) =====
	c.logger.InfoContext(ctx, "[Phase 2] Generating Chunks 2,3 in parallel...")
	chunk2, chunk3, err := c.generateChunks23Parallel(ctx, params, chunk1)
	if err != nil {
		return nil, &PartialGenerationError{
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
	c.saveState(state)

	// ===== Phase 3: Chunk 4 (Sequential - Review) =====
	c.logger.InfoContext(ctx, "[Phase 3] Generating Chunk 4: Review Section...")
	chunk4, err := c.generateChunk4WithRetry(ctx, params, chunk1, chunk3)
	if err != nil {
		return nil, &PartialGenerationError{
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
	c.saveState(state)

	// ===== Phase 4: Chunks 5, 6 (Parallel - FAQ + SEO) =====
	c.logger.InfoContext(ctx, "[Phase 4] Generating Chunks 5,6 in parallel...")
	chunk5, chunk6, err := c.generateChunks56Parallel(ctx, params, chunk1, chunk3, chunk4)
	if err != nil {
		return nil, &PartialGenerationError{
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

	variation := ToVariationMeta(contentVariation, titleStyleIndices)

	// ===== Adjust Rating (realistic distribution) =====
	ratingAdj := AdjustRating(chunk6.Rating, chunk4, input.VideoMetadata.ID)

	c.logger.InfoContext(ctx, "Content variation applied",
		"video_id", input.VideoMetadata.ID,
		"opening_style", variation.OpeningStyle,
		"review_tone", variation.ReviewTone,
		"faq_style", variation.FAQStyle,
		"original_rating", ratingAdj.OriginalRating,
		"adjusted_rating", ratingAdj.AdjustedRating,
		"adjust_reason", ratingAdj.AdjustReason,
	)

	// ===== Aggregate =====
	output := AggregateChunks(
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

// generateArticleContentEN runs the English V3 pipeline
func (c *Client) generateArticleContentEN(ctx context.Context, input *ports.AIInput, videoCode string) (*models.ArticleContentV3, error) {
	startTime := time.Now()

	// Build prompt params for English
	params := c.buildPromptParamsEN(input)

	// ===== Phase 1: Chunk 1 (Foundation - Search Intent) =====
	c.logger.InfoContext(ctx, "[Phase 1/EN] Generating Chunk 1: Search Intent Answer...")
	chunk1, err := c.generateChunk1ENWithRetry(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("chunk1 failed: %w", err)
	}
	c.logger.InfoContext(ctx, "[Phase 1/EN] Chunk 1 completed",
		"quickAnswer_len", len(chunk1.QuickAnswer),
		"verdict_len", len(chunk1.Verdict),
	)

	// Save state after Phase 1
	state := &ChunkState{
		VideoCode: videoCode,
		Chunk1:    chunk1,
		LastChunk: 1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	c.saveState(state)

	// ===== Phase 2: Chunks 2, 3 (Parallel - Facts + Story) =====
	c.logger.InfoContext(ctx, "[Phase 2/EN] Generating Chunks 2,3 in parallel...")
	chunk2, chunk3, err := c.generateChunks23ParallelEN(ctx, params, chunk1)
	if err != nil {
		return nil, &PartialGenerationError{
			Message:       "phase 2 failed",
			PartialPath:   fmt.Sprintf("output/state_v3_%s.json", videoCode),
			FailedChunk:   2,
			CompletedUpTo: 1,
			Cause:         err,
		}
	}
	c.logger.InfoContext(ctx, "[Phase 2/EN] Chunks 2,3 completed",
		"facts_code", chunk2.Code,
		"synopsis_len", len(chunk3.Synopsis),
	)

	// Save state after Phase 2
	state.Chunk2 = chunk2
	state.Chunk3 = chunk3
	state.LastChunk = 3
	state.UpdatedAt = time.Now()
	c.saveState(state)

	// ===== Phase 3: Chunk 4 (Sequential - Review) =====
	c.logger.InfoContext(ctx, "[Phase 3/EN] Generating Chunk 4: Review Section...")
	chunk4, err := c.generateChunk4ENWithRetry(ctx, params, chunk1, chunk3)
	if err != nil {
		return nil, &PartialGenerationError{
			Message:       "chunk4 failed",
			PartialPath:   fmt.Sprintf("output/state_v3_%s.json", videoCode),
			FailedChunk:   4,
			CompletedUpTo: 3,
			Cause:         err,
		}
	}
	c.logger.InfoContext(ctx, "[Phase 3/EN] Chunk 4 completed",
		"reviewSummary_len", len(chunk4.ReviewSummary),
		"strengths", len(chunk4.Strengths),
		"weaknesses", len(chunk4.Weaknesses),
	)

	// Save state after Phase 3
	state.Chunk4 = chunk4
	state.LastChunk = 4
	state.UpdatedAt = time.Now()
	c.saveState(state)

	// ===== Phase 4: Chunks 5, 6 (Parallel - FAQ + SEO) =====
	c.logger.InfoContext(ctx, "[Phase 4/EN] Generating Chunks 5,6 in parallel...")
	chunk5, chunk6, err := c.generateChunks56ParallelEN(ctx, params, chunk1, chunk3, chunk4)
	if err != nil {
		return nil, &PartialGenerationError{
			Message:       "phase 4 failed",
			PartialPath:   fmt.Sprintf("output/state_v3_%s.json", videoCode),
			FailedChunk:   5,
			CompletedUpTo: 4,
			Cause:         err,
		}
	}
	c.logger.InfoContext(ctx, "[Phase 4/EN] Chunks 5,6 completed",
		"faqItems", len(chunk5.FAQItems),
		"titleAggressive_len", len(chunk6.TitleAggressive),
		"rating", chunk6.Rating,
	)

	// ===== Get Variation (for logging/debugging) =====
	contentVariation := GetContentVariation(input.VideoMetadata.ID)
	titleStyleIndices := GetTitleStyleIndices(input.VideoMetadata.ID)

	variation := ToVariationMeta(contentVariation, titleStyleIndices)

	// ===== Adjust Rating (realistic distribution) =====
	ratingAdj := AdjustRating(chunk6.Rating, chunk4, input.VideoMetadata.ID)

	c.logger.InfoContext(ctx, "Content variation applied (EN)",
		"video_id", input.VideoMetadata.ID,
		"opening_style", variation.OpeningStyle,
		"review_tone", variation.ReviewTone,
		"faq_style", variation.FAQStyle,
		"original_rating", ratingAdj.OriginalRating,
		"adjusted_rating", ratingAdj.AdjustedRating,
		"adjust_reason", ratingAdj.AdjustReason,
	)

	// ===== Aggregate =====
	output := AggregateChunks(
		input.VideoMetadata.ID,
		"en",
		chunk1, chunk2, chunk3, chunk4, chunk5, chunk6,
		variation,
		ratingAdj,
	)

	// Clean up state file on full success
	os.Remove(fmt.Sprintf("output/state_v3_%s.json", videoCode))

	elapsed := time.Since(startTime)
	c.logger.InfoContext(ctx, "6-chunk V3 generation completed successfully (EN)",
		"video_code", videoCode,
		"elapsed", elapsed.String(),
	)

	return output, nil
}

// ============================================================================
// Prompt Params Builder
// ============================================================================

func (c *Client) buildPromptParams(input *ports.AIInput) *th.PromptParams {
	castNames := make([]string, len(input.Casts))
	for i, cast := range input.Casts {
		castNames[i] = cast.Name
	}

	tags := make([]string, len(input.Tags))
	for i, tag := range input.Tags {
		tags[i] = tag.Name
	}

	makerName := ""
	if input.VideoMetadata.Maker != nil {
		makerName = input.VideoMetadata.Maker.Name
	}

	return &th.PromptParams{
		VideoCode:    input.VideoMetadata.RealCode,
		VideoID:      input.VideoMetadata.ID,
		Duration:     input.VideoMetadata.Duration,
		ReleaseDate:  input.VideoMetadata.ReleaseDate,
		CastNames:    castNames,
		Tags:         tags,
		MakerName:    makerName,
		SRTContent:   input.SRTContent,
		GalleryCount: input.GalleryCount,
	}
}

func (c *Client) buildPromptParamsEN(input *ports.AIInput) *en.PromptParams {
	castNames := make([]string, len(input.Casts))
	for i, cast := range input.Casts {
		castNames[i] = cast.Name
	}

	tags := make([]string, len(input.Tags))
	for i, tag := range input.Tags {
		tags[i] = tag.Name
	}

	makerName := ""
	if input.VideoMetadata.Maker != nil {
		makerName = input.VideoMetadata.Maker.Name
	}

	return &en.PromptParams{
		VideoCode:    input.VideoMetadata.RealCode,
		VideoID:      input.VideoMetadata.ID,
		Duration:     input.VideoMetadata.Duration,
		ReleaseDate:  input.VideoMetadata.ReleaseDate,
		CastNames:    castNames,
		Tags:         tags,
		MakerName:    makerName,
		SRTContent:   input.SRTContent,
		GalleryCount: input.GalleryCount,
	}
}

// ============================================================================
// Phase 2: Parallel execution of Chunks 2, 3 (Facts + Story)
// ============================================================================

func (c *Client) generateChunks23Parallel(
	ctx context.Context,
	params *th.PromptParams,
	chunk1 *Chunk1Output,
) (*Chunk2Output, *Chunk3Output, error) {
	var wg sync.WaitGroup
	var chunk2 *Chunk2Output
	var chunk3 *Chunk3Output
	var err2, err3 error

	wg.Add(2)

	// Chunk 2: Structured Facts
	go func() {
		defer wg.Done()
		chunk2, err2 = c.generateChunk2WithRetry(ctx, params)
	}()

	// Chunk 3: Story Recap
	go func() {
		defer wg.Done()
		chunk3, err3 = c.generateChunk3WithRetry(ctx, params, chunk1)
	}()

	wg.Wait()

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

func (c *Client) generateChunks56Parallel(
	ctx context.Context,
	params *th.PromptParams,
	chunk1 *Chunk1Output,
	chunk3 *Chunk3Output,
	chunk4 *Chunk4Output,
) (*Chunk5Output, *Chunk6Output, error) {
	var wg sync.WaitGroup
	var chunk5 *Chunk5Output
	var chunk6 *Chunk6Output
	var err5, err6 error

	wg.Add(2)

	// Chunk 5: FAQ Intent Block
	go func() {
		defer wg.Done()
		chunk5, err5 = c.generateChunk5WithRetry(ctx, params, chunk1, chunk3, chunk4)
	}()

	// Chunk 6: SEO Output
	go func() {
		defer wg.Done()
		chunk6, err6 = c.generateChunk6WithRetry(ctx, params, chunk1, chunk3, chunk4)
	}()

	wg.Wait()

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

func (c *Client) generateChunk1WithRetry(ctx context.Context, params *th.PromptParams) (*Chunk1Output, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk1(ctx, params)
		if err == nil {
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

func (c *Client) generateChunk2WithRetry(ctx context.Context, params *th.PromptParams) (*Chunk2Output, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk2(ctx, params)
		if err == nil {
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

func (c *Client) generateChunk3WithRetry(ctx context.Context, params *th.PromptParams, chunk1 *Chunk1Output) (*Chunk3Output, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk3(ctx, params, chunk1)
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

func (c *Client) generateChunk4WithRetry(ctx context.Context, params *th.PromptParams, chunk1 *Chunk1Output, chunk3 *Chunk3Output) (*Chunk4Output, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk4(ctx, params, chunk1, chunk3)
		if err == nil {
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

func (c *Client) generateChunk5WithRetry(ctx context.Context, params *th.PromptParams, chunk1 *Chunk1Output, chunk3 *Chunk3Output, chunk4 *Chunk4Output) (*Chunk5Output, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk5(ctx, params, chunk1, chunk3, chunk4)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 5] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk5 failed after %d retries: %w", maxRetries, lastErr)
}

func (c *Client) generateChunk6WithRetry(ctx context.Context, params *th.PromptParams, chunk1 *Chunk1Output, chunk3 *Chunk3Output, chunk4 *Chunk4Output) (*Chunk6Output, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk6(ctx, params, chunk1, chunk3, chunk4)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 6] Failed, retrying",
			"attempt", i+1,
			"error", err,
		)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk6 failed after %d retries: %w", maxRetries, lastErr)
}

// ============================================================================
// Individual Chunk Generators
// ============================================================================

func (c *Client) generateChunk1(ctx context.Context, params *th.PromptParams) (*Chunk1Output, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = th.BuildChunk1Schema()

	prompt := th.BuildChunk1Prompt(params)
	prompt = sanitizeUTF8(prompt)

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
		debugPath := fmt.Sprintf("output/chunk1_debug_%s.json", params.VideoCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk1: %w", err)
	}

	return &chunk, nil
}

func (c *Client) generateChunk2(ctx context.Context, params *th.PromptParams) (*Chunk2Output, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = th.BuildChunk2Schema()

	prompt := th.BuildChunk2Prompt(params)
	prompt = sanitizeUTF8(prompt)

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
		debugPath := fmt.Sprintf("output/chunk2_debug_%s.json", params.VideoCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk2: %w", err)
	}

	return &chunk, nil
}

func (c *Client) generateChunk3(ctx context.Context, params *th.PromptParams, chunk1 *Chunk1Output) (*Chunk3Output, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = th.BuildChunk3Schema()

	chunk1Ctx := &th.Chunk1Context{
		QuickAnswer: chunk1.QuickAnswer,
		Verdict:     chunk1.Verdict,
	}

	prompt := th.BuildChunk3Prompt(params, chunk1Ctx)
	prompt = sanitizeUTF8(prompt)

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
		return nil, fmt.Errorf("failed to parse chunk3: %w", err)
	}

	return &chunk, nil
}

func (c *Client) generateChunk4(ctx context.Context, params *th.PromptParams, chunk1 *Chunk1Output, chunk3 *Chunk3Output) (*Chunk4Output, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = th.BuildChunk4Schema()

	chunk1Ctx := &th.Chunk1Context{
		QuickAnswer: chunk1.QuickAnswer,
		Verdict:     chunk1.Verdict,
	}
	chunk3Ctx := &th.Chunk3Context{
		Synopsis:  chunk3.Synopsis,
		StoryFlow: chunk3.StoryFlow,
		Tone:      chunk3.Tone,
		KeyScenes: chunk3.KeyScenes,
	}

	prompt := th.BuildChunk4Prompt(params, chunk1Ctx, chunk3Ctx)
	prompt = sanitizeUTF8(prompt)

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
		debugPath := fmt.Sprintf("output/chunk4_debug_%s.json", params.VideoCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk4: %w", err)
	}

	return &chunk, nil
}

func (c *Client) generateChunk5(ctx context.Context, params *th.PromptParams, chunk1 *Chunk1Output, chunk3 *Chunk3Output, chunk4 *Chunk4Output) (*Chunk5Output, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = th.BuildChunk5Schema()

	chunk1Ctx := &th.Chunk1Context{
		QuickAnswer: chunk1.QuickAnswer,
		Verdict:     chunk1.Verdict,
	}
	chunk3Ctx := &th.Chunk3Context{
		Synopsis:  chunk3.Synopsis,
		StoryFlow: chunk3.StoryFlow,
		Tone:      chunk3.Tone,
		KeyScenes: chunk3.KeyScenes,
	}
	chunk4Ctx := &th.Chunk4Context{
		ReviewSummary:  chunk4.ReviewSummary,
		Strengths:      chunk4.Strengths,
		Weaknesses:     chunk4.Weaknesses,
		WhoShouldWatch: chunk4.WhoShouldWatch,
		VerdictReason:  chunk4.VerdictReason,
	}

	prompt := th.BuildChunk5Prompt(params, chunk1Ctx, chunk3Ctx, chunk4Ctx)
	prompt = sanitizeUTF8(prompt)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk5Output
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk5_debug_%s.json", params.VideoCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk5: %w", err)
	}

	// Post-process: Sanitize FAQ items
	chunk.FAQItems = c.sanitizeFAQItems(chunk.FAQItems)

	return &chunk, nil
}

func (c *Client) generateChunk6(ctx context.Context, params *th.PromptParams, chunk1 *Chunk1Output, chunk3 *Chunk3Output, chunk4 *Chunk4Output) (*Chunk6Output, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = th.BuildChunk6Schema()

	chunk1Ctx := &th.Chunk1Context{
		QuickAnswer: chunk1.QuickAnswer,
		Verdict:     chunk1.Verdict,
	}
	chunk3Ctx := &th.Chunk3Context{
		Synopsis:  chunk3.Synopsis,
		StoryFlow: chunk3.StoryFlow,
		Tone:      chunk3.Tone,
		KeyScenes: chunk3.KeyScenes,
	}
	chunk4Ctx := &th.Chunk4Context{
		ReviewSummary:  chunk4.ReviewSummary,
		Strengths:      chunk4.Strengths,
		Weaknesses:     chunk4.Weaknesses,
		WhoShouldWatch: chunk4.WhoShouldWatch,
		VerdictReason:  chunk4.VerdictReason,
	}

	prompt := th.BuildChunk6Prompt(params, chunk1Ctx, chunk3Ctx, chunk4Ctx)
	prompt = sanitizeUTF8(prompt)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk6Output
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk6_debug_%s.json", params.VideoCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk6: %w", err)
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
// English Chunk Generators
// ============================================================================

// generateChunks23ParallelEN runs English Chunks 2,3 in parallel
func (c *Client) generateChunks23ParallelEN(
	ctx context.Context,
	params *en.PromptParams,
	chunk1 *Chunk1Output,
) (*Chunk2Output, *Chunk3Output, error) {
	var wg sync.WaitGroup
	var chunk2 *Chunk2Output
	var chunk3 *Chunk3Output
	var err2, err3 error

	wg.Add(2)

	go func() {
		defer wg.Done()
		chunk2, err2 = c.generateChunk2ENWithRetry(ctx, params)
	}()

	go func() {
		defer wg.Done()
		chunk3, err3 = c.generateChunk3ENWithRetry(ctx, params, chunk1)
	}()

	wg.Wait()

	if err2 != nil {
		return nil, nil, fmt.Errorf("chunk2 failed: %w", err2)
	}
	if err3 != nil {
		return nil, nil, fmt.Errorf("chunk3 failed: %w", err3)
	}

	return chunk2, chunk3, nil
}

// generateChunks56ParallelEN runs English Chunks 5,6 in parallel
func (c *Client) generateChunks56ParallelEN(
	ctx context.Context,
	params *en.PromptParams,
	chunk1 *Chunk1Output,
	chunk3 *Chunk3Output,
	chunk4 *Chunk4Output,
) (*Chunk5Output, *Chunk6Output, error) {
	var wg sync.WaitGroup
	var chunk5 *Chunk5Output
	var chunk6 *Chunk6Output
	var err5, err6 error

	wg.Add(2)

	go func() {
		defer wg.Done()
		chunk5, err5 = c.generateChunk5ENWithRetry(ctx, params, chunk1, chunk3, chunk4)
	}()

	go func() {
		defer wg.Done()
		chunk6, err6 = c.generateChunk6ENWithRetry(ctx, params, chunk1, chunk3, chunk4)
	}()

	wg.Wait()

	if err5 != nil {
		return nil, nil, fmt.Errorf("chunk5 failed: %w", err5)
	}
	if err6 != nil {
		return nil, nil, fmt.Errorf("chunk6 failed: %w", err6)
	}

	return chunk5, chunk6, nil
}

// English chunk generators with retry

func (c *Client) generateChunk1ENWithRetry(ctx context.Context, params *en.PromptParams) (*Chunk1Output, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk1EN(ctx, params)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 1/EN] Failed, retrying", "attempt", i+1, "error", err)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk1 EN failed after %d retries: %w", maxRetries, lastErr)
}

func (c *Client) generateChunk2ENWithRetry(ctx context.Context, params *en.PromptParams) (*Chunk2Output, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk2EN(ctx, params)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 2/EN] Failed, retrying", "attempt", i+1, "error", err)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk2 EN failed after %d retries: %w", maxRetries, lastErr)
}

func (c *Client) generateChunk3ENWithRetry(ctx context.Context, params *en.PromptParams, chunk1 *Chunk1Output) (*Chunk3Output, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk3EN(ctx, params, chunk1)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 3/EN] Failed, retrying", "attempt", i+1, "error", err)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk3 EN failed after %d retries: %w", maxRetries, lastErr)
}

func (c *Client) generateChunk4ENWithRetry(ctx context.Context, params *en.PromptParams, chunk1 *Chunk1Output, chunk3 *Chunk3Output) (*Chunk4Output, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk4EN(ctx, params, chunk1, chunk3)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 4/EN] Failed, retrying", "attempt", i+1, "error", err)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk4 EN failed after %d retries: %w", maxRetries, lastErr)
}

func (c *Client) generateChunk5ENWithRetry(ctx context.Context, params *en.PromptParams, chunk1 *Chunk1Output, chunk3 *Chunk3Output, chunk4 *Chunk4Output) (*Chunk5Output, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk5EN(ctx, params, chunk1, chunk3, chunk4)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 5/EN] Failed, retrying", "attempt", i+1, "error", err)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk5 EN failed after %d retries: %w", maxRetries, lastErr)
}

func (c *Client) generateChunk6ENWithRetry(ctx context.Context, params *en.PromptParams, chunk1 *Chunk1Output, chunk3 *Chunk3Output, chunk4 *Chunk4Output) (*Chunk6Output, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		chunk, err := c.generateChunk6EN(ctx, params, chunk1, chunk3, chunk4)
		if err == nil {
			return chunk, nil
		}
		lastErr = err
		c.logger.WarnContext(ctx, "[Chunk 6/EN] Failed, retrying", "attempt", i+1, "error", err)
		time.Sleep(retryBaseDelay * time.Duration(i+1))
	}
	return nil, fmt.Errorf("chunk6 EN failed after %d retries: %w", maxRetries, lastErr)
}

// Individual English chunk generators

func (c *Client) generateChunk1EN(ctx context.Context, params *en.PromptParams) (*Chunk1Output, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = en.BuildChunk1Schema()

	prompt := en.BuildChunk1Prompt(params)
	prompt = sanitizeUTF8(prompt)

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
		debugPath := fmt.Sprintf("output/chunk1_en_debug_%s.json", params.VideoCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk1 EN: %w", err)
	}

	return &chunk, nil
}

func (c *Client) generateChunk2EN(ctx context.Context, params *en.PromptParams) (*Chunk2Output, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = en.BuildChunk2Schema()

	prompt := en.BuildChunk2Prompt(params)
	prompt = sanitizeUTF8(prompt)

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
		debugPath := fmt.Sprintf("output/chunk2_en_debug_%s.json", params.VideoCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk2 EN: %w", err)
	}

	return &chunk, nil
}

func (c *Client) generateChunk3EN(ctx context.Context, params *en.PromptParams, chunk1 *Chunk1Output) (*Chunk3Output, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = en.BuildChunk3Schema()

	chunk1Ctx := &en.Chunk1Context{
		QuickAnswer: chunk1.QuickAnswer,
		Verdict:     chunk1.Verdict,
	}

	prompt := en.BuildChunk3Prompt(params, chunk1Ctx)
	prompt = sanitizeUTF8(prompt)

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
		return nil, fmt.Errorf("failed to parse chunk3 EN: %w", err)
	}

	return &chunk, nil
}

func (c *Client) generateChunk4EN(ctx context.Context, params *en.PromptParams, chunk1 *Chunk1Output, chunk3 *Chunk3Output) (*Chunk4Output, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = en.BuildChunk4Schema()

	chunk1Ctx := &en.Chunk1Context{
		QuickAnswer: chunk1.QuickAnswer,
		Verdict:     chunk1.Verdict,
	}
	chunk3Ctx := &en.Chunk3Context{
		Synopsis:  chunk3.Synopsis,
		StoryFlow: chunk3.StoryFlow,
		Tone:      chunk3.Tone,
		KeyScenes: chunk3.KeyScenes,
	}

	prompt := en.BuildChunk4Prompt(params, chunk1Ctx, chunk3Ctx)
	prompt = sanitizeUTF8(prompt)

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
		debugPath := fmt.Sprintf("output/chunk4_en_debug_%s.json", params.VideoCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk4 EN: %w", err)
	}

	return &chunk, nil
}

func (c *Client) generateChunk5EN(ctx context.Context, params *en.PromptParams, chunk1 *Chunk1Output, chunk3 *Chunk3Output, chunk4 *Chunk4Output) (*Chunk5Output, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = en.BuildChunk5Schema()

	chunk1Ctx := &en.Chunk1Context{
		QuickAnswer: chunk1.QuickAnswer,
		Verdict:     chunk1.Verdict,
	}
	chunk3Ctx := &en.Chunk3Context{
		Synopsis:  chunk3.Synopsis,
		StoryFlow: chunk3.StoryFlow,
		Tone:      chunk3.Tone,
		KeyScenes: chunk3.KeyScenes,
	}
	chunk4Ctx := &en.Chunk4Context{
		ReviewSummary:  chunk4.ReviewSummary,
		Strengths:      chunk4.Strengths,
		Weaknesses:     chunk4.Weaknesses,
		WhoShouldWatch: chunk4.WhoShouldWatch,
		VerdictReason:  chunk4.VerdictReason,
	}

	prompt := en.BuildChunk5Prompt(params, chunk1Ctx, chunk3Ctx, chunk4Ctx)
	prompt = sanitizeUTF8(prompt)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk5Output
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk5_en_debug_%s.json", params.VideoCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk5 EN: %w", err)
	}

	chunk.FAQItems = c.sanitizeFAQItems(chunk.FAQItems)
	return &chunk, nil
}

func (c *Client) generateChunk6EN(ctx context.Context, params *en.PromptParams, chunk1 *Chunk1Output, chunk3 *Chunk3Output, chunk4 *Chunk4Output) (*Chunk6Output, error) {
	model := c.client.GenerativeModel(c.model)
	c.configureModel(model)
	model.ResponseSchema = en.BuildChunk6Schema()

	chunk1Ctx := &en.Chunk1Context{
		QuickAnswer: chunk1.QuickAnswer,
		Verdict:     chunk1.Verdict,
	}
	chunk3Ctx := &en.Chunk3Context{
		Synopsis:  chunk3.Synopsis,
		StoryFlow: chunk3.StoryFlow,
		Tone:      chunk3.Tone,
		KeyScenes: chunk3.KeyScenes,
	}
	chunk4Ctx := &en.Chunk4Context{
		ReviewSummary:  chunk4.ReviewSummary,
		Strengths:      chunk4.Strengths,
		Weaknesses:     chunk4.Weaknesses,
		WhoShouldWatch: chunk4.WhoShouldWatch,
		VerdictReason:  chunk4.VerdictReason,
	}

	prompt := en.BuildChunk6Prompt(params, chunk1Ctx, chunk3Ctx, chunk4Ctx)
	prompt = sanitizeUTF8(prompt)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	jsonString, err := c.extractJSON(resp)
	if err != nil {
		return nil, err
	}

	var chunk Chunk6Output
	if err := json.Unmarshal([]byte(jsonString), &chunk); err != nil {
		debugPath := fmt.Sprintf("output/chunk6_en_debug_%s.json", params.VideoCode)
		_ = writeDebugFile(debugPath, jsonString)
		return nil, fmt.Errorf("failed to parse chunk6 EN: %w", err)
	}

	chunk.Keywords = c.filterSEOKeywords(chunk.Keywords)
	chunk.SearchIntents = c.filterSEOKeywords(chunk.SearchIntents)

	if chunk.Rating < 1.0 {
		chunk.Rating = 1.0
	} else if chunk.Rating > 5.0 {
		chunk.Rating = 5.0
	}

	return &chunk, nil
}

// ============================================================================
// Model Configuration
// ============================================================================

func (c *Client) configureModel(model *genai.GenerativeModel) {
	model.SetTemperature(0.7)
	model.SetTopP(0.9)
	model.SetTopK(40)
	model.ResponseMIMEType = "application/json"

	model.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategorySexuallyExplicit,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: genai.HarmBlockNone,
		},
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

func (c *Client) extractJSON(resp *genai.GenerateContentResponse) (string, error) {
	if resp == nil || len(resp.Candidates) == 0 {
		return "", fmt.Errorf("empty response from API")
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return "", fmt.Errorf("no content parts in response")
	}

	var result strings.Builder
	for _, part := range candidate.Content.Parts {
		if textPart, ok := part.(genai.Text); ok {
			result.WriteString(string(textPart))
		}
	}

	return result.String(), nil
}

func (c *Client) sanitizeFAQItems(items []models.FAQItem) []models.FAQItem {
	sanitized := make([]models.FAQItem, 0, len(items))
	for _, item := range items {
		if item.Question != "" && item.Answer != "" {
			sanitized = append(sanitized, item)
		}
	}
	return sanitized
}

func (c *Client) filterSEOKeywords(keywords []string) []string {
	forbidden := []string{"หนังโป๊", "xxx", "av", "porn", "sex"}

	filtered := make([]string, 0, len(keywords))
	for _, kw := range keywords {
		kwLower := strings.ToLower(kw)
		isForbidden := false
		for _, f := range forbidden {
			if strings.Contains(kwLower, f) {
				isForbidden = true
				break
			}
		}
		if !isForbidden {
			filtered = append(filtered, kw)
		}
	}
	return filtered
}

// ============================================================================
// State Management
// ============================================================================

func (c *Client) saveState(state *ChunkState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	path := fmt.Sprintf("output/state_v3_%s.json", state.VideoCode)
	return writeDebugFile(path, string(data))
}

// ============================================================================
// Error Types
// ============================================================================

// PartialGenerationError is returned when generation fails mid-pipeline
type PartialGenerationError struct {
	Message       string
	PartialPath   string
	FailedChunk   int
	CompletedUpTo int
	Cause         error
}

func (e *PartialGenerationError) Error() string {
	return fmt.Sprintf("%s (chunk %d failed, completed up to %d). Partial state saved to: %s. Cause: %v",
		e.Message, e.FailedChunk, e.CompletedUpTo, e.PartialPath, e.Cause)
}

func (e *PartialGenerationError) Unwrap() error {
	return e.Cause
}

// ============================================================================
// Utilities
// ============================================================================

func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "")
}

func writeDebugFile(path, content string) error {
	if err := os.MkdirAll("output", 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}
