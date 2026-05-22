package models

import "time"

// ============================================================================
// Article Content V3: Intent-Driven Architecture
// ============================================================================
//
// เปลี่ยนจาก Essay Style (V2) → Intent-Driven Style (V3)
// 6 Chunks: Search Intent → Facts → Story → Review → FAQ → SEO
//
// ============================================================================

// ArticleContentV3 - โครงสร้างใหม่สำหรับ Intent-Driven Content
type ArticleContentV3 struct {
	VideoID  string `json:"videoId"`
	Language string `json:"language"` // "th" or "en"

	// === Chunk 1: Search Intent Answer ===
	QuickAnswer string `json:"quickAnswer"` // 2-3 ประโยค ตอบทันที (Google snippet target)
	MainHook    string `json:"mainHook"`    // 1 ประโยคดึงดูด
	Verdict     string `json:"verdict"`     // ควรดูไหม 1 ประโยค

	// === Chunk 2: Structured Facts ===
	Facts FactsV3 `json:"facts"`

	// === Chunk 3: Story Recap ===
	Synopsis            string   `json:"synopsis"`            // 150-250 คำ
	StoryFlow           string   `json:"storyFlow"`           // timeline: เริ่ม → กลาง → จบ
	KeyScenes           []string `json:"keyScenes"`           // 3-5 ฉากสำคัญ
	FeaturedScene       string   `json:"featuredScene"`       // ฉากเด่น ~100 คำ (Scroll Depth Trigger)
	Tone                string   `json:"tone"`                // อารมณ์เรื่อง
	RelationshipDynamic string   `json:"relationshipDynamic"` // ความสัมพันธ์ตัวละคร

	// === Chunk 4: Review ===
	ReviewSummary  string   `json:"reviewSummary"`  // 200-300 คำ (3 ย่อหน้า)
	Strengths      []string `json:"strengths"`      // 3-5 จุดเด่น
	Weaknesses     []string `json:"weaknesses"`     // 2-3 จุดอ่อน
	WhoShouldWatch string   `json:"whoShouldWatch"` // เหมาะกับใคร
	VerdictReason  string   `json:"verdictReason"`  // เหตุผลสรุป

	// === Chunk 5: FAQ ===
	FAQItems []FAQItem `json:"faqItems"` // 5 คำถาม intent-driven

	// === Chunk 6: SEO ===
	TitleAggressive string   `json:"titleAggressive"` // Meta title (CTR focus)
	TitleBalanced   string   `json:"titleBalanced"`   // H1 (Google friendly)
	Title           string   `json:"title"`           // Alias for TitleBalanced (API compat)
	MetaTitle       string   `json:"metaTitle"`       // Alias for TitleAggressive (API compat)
	MetaDescription string   `json:"metaDescription"` // 150-160 chars
	Slug            string   `json:"slug"`
	Keywords        []string `json:"keywords"`      // 5-8 keywords
	SearchIntents   []string `json:"searchIntents"` // 4-5 intent phrases
	Rating          float64  `json:"rating"`        // 1-5 scale

	// === From Metadata (not AI generated) ===
	CastProfiles        []CastProfile  `json:"castProfiles,omitempty"`
	MakerInfo           *MakerInfo     `json:"makerInfo,omitempty"`
	RelatedVideos       []RelatedVideo `json:"relatedVideos,omitempty"`
	TagDescriptions     []TagDesc      `json:"tagDescriptions,omitempty"`
	GalleryImages       []GalleryImage `json:"galleryImages,omitempty"`        // Public gallery (safe)
	MemberGalleryImages []GalleryImage `json:"memberGalleryImages,omitempty"`  // Member-only gallery (nsfw)
	MemberGalleryCount  int            `json:"memberGalleryCount,omitempty"`   // จำนวนภาพ member

	// === Media ===
	ThumbnailURL    string `json:"thumbnailUrl"`
	ThumbnailAlt    string `json:"thumbnailAlt,omitempty"`
	AudioSummaryURL string `json:"audioSummaryUrl,omitempty"`
	AudioDuration   int    `json:"audioDuration,omitempty"`

	// === Debug/Analytics ===
	WordCount        int               `json:"wordCount,omitempty"`
	Variation        *VariationMeta    `json:"variation,omitempty"`
	RatingAdjustment *RatingAdjustment `json:"ratingAdjustment,omitempty"`

	// === Timestamps ===
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// FactsV3 - ข้อมูล facts สำหรับ table และ JSON-LD
type FactsV3 struct {
	Code              string   `json:"code"`
	Studio            string   `json:"studio"`
	Cast              []string `json:"cast"`
	Duration          string   `json:"duration"`          // ISO 8601 (PT1H30M)
	DurationMinutes   int      `json:"durationMinutes"`   // จำนวนนาที (สำหรับ Frontend schema)
	Genre             []string `json:"genre"`
	ReleaseYear       string   `json:"releaseYear"`
	SubtitleAvailable bool     `json:"subtitleAvailable"`
}

// VariationMeta - เก็บ style ที่ใช้ generate content (สำหรับ debug)
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

// RatingAdjustment - เก็บ log การปรับ rating (สำหรับ debug)
type RatingAdjustment struct {
	OriginalRating float64 `json:"originalRating"`
	AdjustedRating float64 `json:"adjustedRating"`
	AdjustReason   string  `json:"adjustReason"` // no_adjustment, high_rating_with_weaknesses, etc.
}
