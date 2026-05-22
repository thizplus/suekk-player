package ai

import (
	"time"

	"seo-worker/domain/models"
)

// NOTE: models.ArticleContentV3, models.FactsV3, models.VariationMeta, models.RatingAdjustment
// are now the canonical types used across the system (defined in domain/models/article_v3.go)

// ============================================================================
// Chunk Types V3: Intent-Driven Architecture (6 Chunks)
// ============================================================================
//
// เปลี่ยนจาก Essay Style → Intent-Driven Style
// เน้น: Answer First → Facts → Story → Review → FAQ → SEO
//
// Chunk 1: Search Intent Answer - ตอบ query ทันที
// Chunk 2: Structured Facts     - ข้อมูลสำหรับ table/JSON-LD
// Chunk 3: Story Recap          - เรื่องย่อจาก SRT
// Chunk 4: Review Section       - รีวิว + Pros/Cons
// Chunk 5: FAQ Intent Block     - 5 คำถามตรง intent
// Chunk 6: SEO Output           - Title/Meta
//
// ============================================================================

// ============================================================================
// Chunk 1: Search Intent Answer
// Focus: ตอบ search query ให้เร็วที่สุด (Google Snippet Target)
// ============================================================================

type Chunk1OutputV3 struct {
	QuickAnswer string `json:"quickAnswer"` // 2-3 ประโยค ตอบทันที
	MainHook    string `json:"mainHook"`    // 1 ประโยคดึงดูด
	Verdict     string `json:"verdict"`     // ควรดูไหม 1 ประโยค
}

// ============================================================================
// Chunk 2: Structured Facts
// Focus: ข้อมูล facts สำหรับ table และ JSON-LD
// ============================================================================

type Chunk2OutputV3 struct {
	Code              string   `json:"code"`
	Studio            string   `json:"studio"`
	Cast              []string `json:"cast"`
	Duration          string   `json:"duration"`          // ISO 8601 (PT1H30M)
	DurationMinutes   int      `json:"durationMinutes"`   // จำนวนนาที (สำหรับ Frontend schema)
	Genre             []string `json:"genre"`
	ReleaseYear       string   `json:"releaseYear"`
	SubtitleAvailable bool     `json:"subtitleAvailable"`
}

// ============================================================================
// Chunk 3: Story Recap
// Focus: สรุปเรื่องจาก SRT (Unique Content)
// ============================================================================

type Chunk3OutputV3 struct {
	Synopsis            string   `json:"synopsis"`            // 150-250 คำ
	StoryFlow           string   `json:"storyFlow"`           // timeline: เริ่ม → กลาง → จบ
	KeyScenes           []string `json:"keyScenes"`           // 3-5 ฉากสำคัญ
	FeaturedScene       string   `json:"featuredScene"`       // ฉากเด่น ~100 คำ (Scroll Depth Trigger)
	Tone                string   `json:"tone"`                // อารมณ์เรื่อง
	RelationshipDynamic string   `json:"relationshipDynamic"` // ความสัมพันธ์ตัวละคร
}

// ============================================================================
// Chunk 4: Review Section
// Focus: รีวิว Verdict Style (ไม่ใช่ Essay)
// ============================================================================

type Chunk4OutputV3 struct {
	ReviewSummary  string   `json:"reviewSummary"`  // 200-300 คำ (3 ย่อหน้า)
	Strengths      []string `json:"strengths"`      // 3-5 จุดเด่น
	Weaknesses     []string `json:"weaknesses"`     // 2-3 จุดอ่อน
	WhoShouldWatch string   `json:"whoShouldWatch"` // เหมาะกับใคร
	VerdictReason  string   `json:"verdictReason"`  // เหตุผลสรุป
}

// ============================================================================
// Chunk 5: FAQ Intent Block
// Focus: 5 คำถามที่ตรง search intent + conversion
// ============================================================================

type Chunk5OutputV3 struct {
	FAQItems []models.FAQItem `json:"faqItems"` // 5 คำถามตรง intent
}

// FAQ Intent ที่ต้องมี:
// 1. "[CODE] เกี่ยวกับอะไร?"
// 2. "[CODE] มีซับไทยไหม?"
// 3. "ดู [CODE] แล้วคุ้มไหม?"
// 4. "[CODE] นักแสดงใครเด่น?"
// 5. "ดู [CODE] ซับไทยได้ที่ไหน?" ← Conversion FAQ

// ============================================================================
// Chunk 6: SEO Output
// Focus: Title (Aggressive + Balanced) และ Meta
// ============================================================================

type Chunk6OutputV3 struct {
	TitleAggressive string   `json:"titleAggressive"` // CTR focus
	TitleBalanced   string   `json:"titleBalanced"`   // H1, Google friendly
	MetaDescription string   `json:"metaDescription"` // 150-160 chars
	Slug            string   `json:"slug"`
	Keywords        []string `json:"keywords"`      // 5-8 keywords
	SearchIntents   []string `json:"searchIntents"` // 4-5 intent phrases (for internal linking)
	Rating          float64  `json:"rating"`        // 1-5 scale (for Review JSON-LD)
}

// ============================================================================
// Aggregated Output V3
// ============================================================================

type ArticleContentV3 struct {
	VideoID  string `json:"videoId"`
	Language string `json:"language"` // "th" or "en"

	// === Chunk 1: Search Intent ===
	QuickAnswer string `json:"quickAnswer"`
	MainHook    string `json:"mainHook"`
	Verdict     string `json:"verdict"`

	// === Chunk 2: Structured Facts ===
	Facts Chunk2OutputV3 `json:"facts"`

	// === Chunk 3: Story Recap ===
	Synopsis            string   `json:"synopsis"`
	StoryFlow           string   `json:"storyFlow"`
	KeyScenes           []string `json:"keyScenes"`
	FeaturedScene       string   `json:"featuredScene"` // ฉากเด่น ~100 คำ (Scroll Depth Trigger)
	Tone                string   `json:"tone"`
	RelationshipDynamic string   `json:"relationshipDynamic"`

	// === Chunk 4: Review ===
	ReviewSummary  string   `json:"reviewSummary"`
	Strengths      []string `json:"strengths"`
	Weaknesses     []string `json:"weaknesses"`
	WhoShouldWatch string   `json:"whoShouldWatch"`
	VerdictReason  string   `json:"verdictReason"`

	// === Chunk 5: FAQ ===
	FAQItems []models.FAQItem `json:"faqItems"`

	// === Chunk 6: SEO ===
	TitleAggressive string   `json:"titleAggressive"`
	TitleBalanced   string   `json:"titleBalanced"`
	MetaDescription string   `json:"metaDescription"`
	Slug            string   `json:"slug"`
	Keywords        []string `json:"keywords"`
	SearchIntents   []string `json:"searchIntents"` // 4-5 intent phrases
	Rating          float64  `json:"rating"`        // 1-5 scale

	// === From Metadata (not AI generated) ===
	CastProfiles  []models.CastProfile  `json:"castProfiles,omitempty"`
	RelatedVideos []models.RelatedVideo `json:"relatedVideos,omitempty"`
	GalleryImages []models.GalleryImage `json:"galleryImages,omitempty"`

	// === TTS (keep from V2) ===
	AudioSummaryURL string `json:"audioSummaryUrl,omitempty"`
	AudioDuration   int    `json:"audioDuration,omitempty"`

	// === Debug/Analytics (สำหรับ debug ranking) ===
	WordCount        int               `json:"wordCount,omitempty"`        // จำนวนคำทั้งหมด
	Variation        *VariationMeta    `json:"variation,omitempty"`        // Style ที่ใช้ generate
	RatingAdjustment *RatingAdjustment `json:"ratingAdjustment,omitempty"` // Rating adjustment log

	// === Timestamps ===
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// VariationMeta เก็บ style ที่ใช้ generate content (สำหรับ debug)
type VariationMeta struct {
	OpeningStyle string `json:"openingStyle"` // A, B, C, D
	ReviewTone   string `json:"reviewTone"`   // Analytical, Conversational, Comparative
	FAQStyle     string `json:"faqStyle"`     // Formal, Casual

	// Title/SEO style indices (for debugging pattern distribution)
	TitleAggressiveIdx int  `json:"titleAggressiveIdx,omitempty"` // 0-3
	TitleBalancedIdx   int  `json:"titleBalancedIdx,omitempty"`   // 0-3
	IntentSetIdx       int  `json:"intentSetIdx,omitempty"`       // 0-5
	IntentOrderSwapped bool `json:"intentOrderSwapped,omitempty"` // true/false
}

// RatingAdjustment เก็บ log การปรับ rating (สำหรับ debug)
type RatingAdjustment struct {
	OriginalRating float64 `json:"originalRating"`
	AdjustedRating float64 `json:"adjustedRating"`
	AdjustReason   string  `json:"adjustReason"` // no_adjustment, high_rating_with_weaknesses, weaknesses_exceed_strengths
}

// ============================================================================
// State Management V3
// ============================================================================

type ChunkStateV3 struct {
	JobID     string `json:"job_id"`
	VideoCode string `json:"video_code"`

	// Chunk outputs
	Chunk1 *Chunk1OutputV3 `json:"chunk1,omitempty"`
	Chunk2 *Chunk2OutputV3 `json:"chunk2,omitempty"`
	Chunk3 *Chunk3OutputV3 `json:"chunk3,omitempty"`
	Chunk4 *Chunk4OutputV3 `json:"chunk4,omitempty"`
	Chunk5 *Chunk5OutputV3 `json:"chunk5,omitempty"`
	Chunk6 *Chunk6OutputV3 `json:"chunk6,omitempty"`

	// State tracking
	LastChunk int       `json:"last_chunk"` // 0-6
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ============================================================================
// Aggregator V3: Merge 6 chunks into models.ArticleContentV3
// ============================================================================

func AggregateChunksV3(
	videoID string,
	language string,
	chunk1 *Chunk1OutputV3,
	chunk2 *Chunk2OutputV3,
	chunk3 *Chunk3OutputV3,
	chunk4 *Chunk4OutputV3,
	chunk5 *Chunk5OutputV3,
	chunk6 *Chunk6OutputV3,
	variation *VariationMeta,
	ratingAdj *RatingAdjustment,
) *models.ArticleContentV3 {
	now := time.Now()

	// Calculate word count (รวมทุก content field)
	wordCount := countWords(chunk1.QuickAnswer) +
		countWords(chunk1.MainHook) +
		countWords(chunk1.Verdict) +
		countWords(chunk3.Synopsis) +
		countWords(chunk3.StoryFlow) +
		countWords(chunk3.FeaturedScene) +
		countWords(chunk4.ReviewSummary) +
		countWords(chunk4.WhoShouldWatch) +
		countWords(chunk4.VerdictReason)

	// เพิ่ม Strengths
	for _, s := range chunk4.Strengths {
		wordCount += countWords(s)
	}
	// เพิ่ม Weaknesses
	for _, w := range chunk4.Weaknesses {
		wordCount += countWords(w)
	}
	// เพิ่ม FAQ answers
	for _, faq := range chunk5.FAQItems {
		wordCount += countWords(faq.Answer)
	}

	// Use adjusted rating if available
	rating := chunk6.Rating
	if ratingAdj != nil && ratingAdj.AdjustedRating > 0 {
		rating = ratingAdj.AdjustedRating
	}

	// Convert Chunk2OutputV3 to models.FactsV3
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

	// Convert variation to models.VariationMeta
	var variationMeta *models.VariationMeta
	if variation != nil {
		variationMeta = &models.VariationMeta{
			OpeningStyle:       variation.OpeningStyle,
			ReviewTone:         variation.ReviewTone,
			FAQStyle:           variation.FAQStyle,
			TitleAggressiveIdx: variation.TitleAggressiveIdx,
			TitleBalancedIdx:   variation.TitleBalancedIdx,
			IntentSetIdx:       variation.IntentSetIdx,
			IntentOrderSwapped: variation.IntentOrderSwapped,
		}
	}

	// Convert ratingAdj to models.RatingAdjustment
	var ratingAdjustment *models.RatingAdjustment
	if ratingAdj != nil {
		ratingAdjustment = &models.RatingAdjustment{
			OriginalRating: ratingAdj.OriginalRating,
			AdjustedRating: ratingAdj.AdjustedRating,
			AdjustReason:   ratingAdj.AdjustReason,
		}
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
		Variation:        variationMeta,
		RatingAdjustment: ratingAdjustment,

		// Timestamps
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// countWords นับจำนวนคำใน text
func countWords(text string) int {
	if text == "" {
		return 0
	}
	// Simple word count: split by whitespace
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
