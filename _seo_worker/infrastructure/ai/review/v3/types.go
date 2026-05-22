package v3

import (
	"time"

	"seo-worker/domain/models"
)

// ============================================================================
// V3 Internal Chunk Types
// ============================================================================
// These are internal types used during generation.
// Final output is converted to models.ArticleContentV3

// Chunk1Output - Search Intent Answer
type Chunk1Output struct {
	QuickAnswer string `json:"quickAnswer"` // 2-3 ประโยค ตอบทันที
	MainHook    string `json:"mainHook"`    // 1 ประโยคดึงดูด
	Verdict     string `json:"verdict"`     // ควรดูไหม 1 ประโยค
}

// Chunk2Output - Structured Facts
type Chunk2Output struct {
	Code              string   `json:"code"`
	Studio            string   `json:"studio"`
	Cast              []string `json:"cast"`
	Duration          string   `json:"duration"`
	DurationMinutes   int      `json:"durationMinutes"`
	Genre             []string `json:"genre"`
	ReleaseYear       string   `json:"releaseYear"`
	SubtitleAvailable bool     `json:"subtitleAvailable"`
}

// Chunk3Output - Story Recap
type Chunk3Output struct {
	Synopsis            string   `json:"synopsis"`
	StoryFlow           string   `json:"storyFlow"`
	KeyScenes           []string `json:"keyScenes"`
	FeaturedScene       string   `json:"featuredScene"` // ฉากเด่น ~100 คำ (Scroll Depth Trigger)
	Tone                string   `json:"tone"`
	RelationshipDynamic string   `json:"relationshipDynamic"`
}

// Chunk4Output - Review Section
type Chunk4Output struct {
	ReviewSummary  string   `json:"reviewSummary"`
	Strengths      []string `json:"strengths"`
	Weaknesses     []string `json:"weaknesses"`
	WhoShouldWatch string   `json:"whoShouldWatch"`
	VerdictReason  string   `json:"verdictReason"`
}

// Chunk5Output - FAQ Intent Block
type Chunk5Output struct {
	FAQItems []models.FAQItem `json:"faqItems"`
}

// Chunk6Output - SEO Output
type Chunk6Output struct {
	TitleAggressive string   `json:"titleAggressive"`
	TitleBalanced   string   `json:"titleBalanced"`
	MetaDescription string   `json:"metaDescription"`
	Slug            string   `json:"slug"`
	Keywords        []string `json:"keywords"`
	SearchIntents   []string `json:"searchIntents"`
	Rating          float64  `json:"rating"`
}

// ============================================================================
// State Management
// ============================================================================

// ChunkState for resume capability
type ChunkState struct {
	JobID     string `json:"job_id"`
	VideoCode string `json:"video_code"`

	Chunk1 *Chunk1Output `json:"chunk1,omitempty"`
	Chunk2 *Chunk2Output `json:"chunk2,omitempty"`
	Chunk3 *Chunk3Output `json:"chunk3,omitempty"`
	Chunk4 *Chunk4Output `json:"chunk4,omitempty"`
	Chunk5 *Chunk5Output `json:"chunk5,omitempty"`
	Chunk6 *Chunk6Output `json:"chunk6,omitempty"`

	LastChunk int       `json:"last_chunk"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ============================================================================
// Aggregator: Merge chunks into models.ArticleContentV3
// ============================================================================

func AggregateChunks(
	videoID string,
	language string,
	chunk1 *Chunk1Output,
	chunk2 *Chunk2Output,
	chunk3 *Chunk3Output,
	chunk4 *Chunk4Output,
	chunk5 *Chunk5Output,
	chunk6 *Chunk6Output,
	variation *models.VariationMeta,
	ratingAdj *models.RatingAdjustment,
) *models.ArticleContentV3 {
	now := time.Now()

	// Calculate word count
	wordCount := countWords(chunk1.QuickAnswer) +
		countWords(chunk1.MainHook) +
		countWords(chunk1.Verdict) +
		countWords(chunk3.Synopsis) +
		countWords(chunk3.StoryFlow) +
		countWords(chunk3.FeaturedScene) +
		countWords(chunk4.ReviewSummary) +
		countWords(chunk4.WhoShouldWatch) +
		countWords(chunk4.VerdictReason)

	for _, s := range chunk4.Strengths {
		wordCount += countWords(s)
	}
	for _, w := range chunk4.Weaknesses {
		wordCount += countWords(w)
	}
	for _, faq := range chunk5.FAQItems {
		wordCount += countWords(faq.Answer)
	}

	// Use adjusted rating if available
	rating := chunk6.Rating
	if ratingAdj != nil && ratingAdj.AdjustedRating > 0 {
		rating = ratingAdj.AdjustedRating
	}

	// Convert Chunk2Output to models.FactsV3
	facts := models.FactsV3{
		Code:              chunk2.Code,
		Studio:            chunk2.Studio,
		Cast:              chunk2.Cast,
		Duration:          chunk2.Duration,
		DurationMinutes:   chunk2.DurationMinutes,
		Genre:             chunk2.Genre,
		ReleaseYear:       chunk2.ReleaseYear,
		SubtitleAvailable: chunk2.SubtitleAvailable,
	}

	return &models.ArticleContentV3{
		VideoID:  videoID,
		Language: language,

		// Chunk 1
		QuickAnswer: chunk1.QuickAnswer,
		MainHook:    chunk1.MainHook,
		Verdict:     chunk1.Verdict,

		// Chunk 2
		Facts: facts,

		// Chunk 3
		Synopsis:            chunk3.Synopsis,
		StoryFlow:           chunk3.StoryFlow,
		KeyScenes:           chunk3.KeyScenes,
		FeaturedScene:       chunk3.FeaturedScene,
		Tone:                chunk3.Tone,
		RelationshipDynamic: chunk3.RelationshipDynamic,

		// Chunk 4
		ReviewSummary:  chunk4.ReviewSummary,
		Strengths:      chunk4.Strengths,
		Weaknesses:     chunk4.Weaknesses,
		WhoShouldWatch: chunk4.WhoShouldWatch,
		VerdictReason:  chunk4.VerdictReason,

		// Chunk 5
		FAQItems: chunk5.FAQItems,

		// Chunk 6
		TitleAggressive: chunk6.TitleAggressive,
		TitleBalanced:   chunk6.TitleBalanced,
		Title:           chunk6.TitleBalanced,   // API compat: title = TitleBalanced
		MetaTitle:       chunk6.TitleAggressive, // API compat: metaTitle = TitleAggressive
		MetaDescription: chunk6.MetaDescription,
		Slug:            chunk6.Slug,
		Keywords:        chunk6.Keywords,
		SearchIntents:   chunk6.SearchIntents,
		Rating:          rating,

		// Debug/Analytics
		WordCount:        wordCount,
		Variation:        variation,
		RatingAdjustment: ratingAdj,

		// Timestamps
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// countWords counts words in text
func countWords(text string) int {
	if text == "" {
		return 0
	}
	words := 0
	inWord := false
	for _, r := range text {
		if r == ' ' || r == '\n' || r == '\t' {
			inWord = false
		} else if !inWord {
			inWord = true
			words++
		}
	}
	return words
}
