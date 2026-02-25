package ai

import (
	"fmt"
	"time"

	"seo-worker/domain/models"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 1: Core SEO
// Focus: เนื้อเรื่อง, SEO metadata, timestamps
//
// NOTE: Key Moments Strategy (Public vs Internal)
// ------------------------------------------------
// - **Public Key Moments** (ส่งให้ Google ผ่าน JSON-LD Schema):
//   - จำกัด 3-5 จุด, เฉพาะ 10 นาทีแรก, ชื่อสุภาพ/วิชาการ
//   - ตัวอย่าง: "การสัมภาษณ์นักแสดง", "บรรยากาศในคลินิก"
//
// - **Internal Key Moments** (โชว์เฉพาะสมาชิก หลัง Login):
//   - จำกัด 20-30 จุด, ครอบคลุมทั้งเรื่อง, ชื่อละเอียด
//   - ใช้ AI เจนแบบละเอียดทุกจุดพีค
//
// ปัจจุบัน Chunk1Output.KeyMoments = Public Moments เท่านั้น
// Internal Moments ต้อง implement แยกใน Member API
// ============================================================================

type Chunk1Output struct {
	// Core SEO
	Title           string `json:"title"`           // H1 ภาษาไทย 50-60 ตัวอักษร
	MetaTitle       string `json:"metaTitle"`       // Meta title สำหรับ Google
	MetaDescription string `json:"metaDescription"` // 150-160 ตัวอักษร

	// Content Summary
	Summary      string   `json:"summary"`      // สรุป 400+ คำ
	SummaryShort string   `json:"summaryShort"` // สรุปสั้นสำหรับ TTS
	Highlights   []string `json:"highlights"`   // 5-10 ฉากสำคัญ

	// Key Moments (timestamps จาก SRT)
	KeyMoments []models.KeyMoment `json:"keyMoments"`

	// Gallery & Scene
	GalleryAlts    []string `json:"galleryAlts"`    // Alt text สำหรับรูป
	SceneLocations []string `json:"sceneLocations"` // สถานที่ในเรื่อง
	ThumbnailAlt   string   `json:"thumbnailAlt"`   // Alt สำหรับ thumbnail

	// Quality
	QualityScore int `json:"qualityScore"` // 1-10
}

// ============================================================================
// Chunk 2: E-E-A-T Analysis
// Focus: Expertise & Authoritativeness
// ============================================================================

type Chunk2Output struct {
	// [E] Expertise Section
	DialogueAnalysis      string            `json:"dialogueAnalysis"`      // วิเคราะห์บทสนทนา 100-150 คำ
	CharacterInsight      string            `json:"characterInsight"`      // วิเคราะห์บุคลิก 100-150 คำ
	LanguageNotes         string            `json:"languageNotes"`         // หมายเหตุภาษา
	ActorPerformanceTrend string            `json:"actorPerformanceTrend"` // เปรียบเทียบการแสดง
	ComparisonNote        string            `json:"comparisonNote"`        // เปรียบเทียบกับเรื่องอื่น
	TopQuotes             []ports.TopQuote  `json:"topQuotes"`             // ประโยคเด็ด + timestamp
	ExpertAnalysis        string            `json:"expertAnalysis"`        // บทวิเคราะห์ผู้เชี่ยวชาญ
	DetailedReview        string            `json:"detailedReview"`        // รีวิว 600+ คำ
	CastBios              []ports.CastBio   `json:"castBios"`              // Bio แต่ละ cast
	TagDescriptions       []models.TagDesc  `json:"tagDescriptions"`       // คำอธิบาย tags

	// [A] Authoritativeness Section
	CharacterDynamic   string   `json:"characterDynamic"`   // ความสัมพันธ์ตัวละคร
	PlotAnalysis       string   `json:"plotAnalysis"`       // วิเคราะห์โครงเรื่อง
	Recommendation     string   `json:"recommendation"`     // เหมาะสำหรับ...
	RecommendedFor     []string `json:"recommendedFor"`     // กลุ่มเป้าหมาย
	SettingDescription string   `json:"settingDescription"` // บริบทฉาก
	MoodTone           []string `json:"moodTone"`           // อารมณ์เรื่อง
	ThematicKeywords   []string `json:"thematicKeywords"`   // Keywords semantic search

	// [SEO] Internal Linking
	ContextualLinks []models.ContextualLink `json:"contextualLinks"` // ประโยคเชื่อมโยงไป related articles
}

// ============================================================================
// Chunk 3: Technical + FAQ
// Focus: Trustworthiness & SEO keywords
// ============================================================================

type Chunk3Output struct {
	// [T] Trustworthiness Section
	TranslationMethod string           `json:"translationMethod"` // วิธีแปล
	TranslationNote   string           `json:"translationNote"`   // หมายเหตุการแปล
	SubtitleQuality   string           `json:"subtitleQuality"`   // คุณภาพซับ
	VideoQuality      string           `json:"videoQuality"`      // เช่น 1080p Full HD
	AudioQuality      string           `json:"audioQuality"`      // เช่น สเตอริโอ 320kbps
	TechnicalFAQ      []models.FAQItem `json:"technicalFaq"`      // FAQ เทคนิค 2-3 ข้อ

	// FAQ
	FAQItems []models.FAQItem `json:"faqItems"` // FAQ 3-5 ข้อ

	// SEO Keywords
	Keywords         []string `json:"keywords"`         // SEO keywords 5-10 คำ
	LongTailKeywords []string `json:"longTailKeywords"` // Long-tail 3-5 วลี
}

// ============================================================================
// Chunk 4: Deep Analysis (เพิ่ม Text สำหรับ SEO - Text/HTML Ratio)
// Focus: Cinematography, Character Journey, Educational, Comparison, Viewing Tips
// ============================================================================

type Chunk4Output struct {
	// === Section 1: Cinematography & Atmosphere ===
	CinematographyAnalysis string   `json:"cinematographyAnalysis"` // วิเคราะห์งานภาพ 300-500 คำ
	VisualStyle            string   `json:"visualStyle"`            // สไตล์ภาพโดยรวม 50-80 คำ
	AtmosphereNotes        []string `json:"atmosphereNotes"`        // 3-5 จุดสังเกตบรรยากาศ

	// === Section 2: Character Emotional Journey ===
	CharacterJourney string              `json:"characterJourney"` // พัฒนาการทางอารมณ์ 400-600 คำ
	EmotionalArc     []EmotionalArcPoint `json:"emotionalArc"`     // 3-4 จุดสำคัญ

	// === Section 3: Educational Context ===
	ThematicExplanation string   `json:"thematicExplanation"` // อธิบายธีม 300-500 คำ
	CulturalContext     string   `json:"culturalContext"`     // บริบทวัฒนธรรม 100-150 คำ
	GenreInsights       []string `json:"genreInsights"`       // 3-5 ข้อมูลเชิงลึก

	// === Section 4: Comparative Analysis ===
	StudioComparison string `json:"studioComparison"` // เปรียบเทียบกับค่าย 200-300 คำ
	ActorEvolution   string `json:"actorEvolution"`   // พัฒนาการนักแสดง 200-300 คำ
	GenreRanking     string `json:"genreRanking"`     // ตำแหน่งในแนว 50-100 คำ

	// === Section 5: Viewing Experience ===
	ViewingTips   string   `json:"viewingTips"`   // คำแนะนำ 200-300 คำ
	BestMoments   []string `json:"bestMoments"`   // 3-5 ช่วงเวลาดีที่สุด
	AudienceMatch string   `json:"audienceMatch"` // เหมาะกับใคร 100-150 คำ
	ReplayValue   string   `json:"replayValue"`   // ความคุ้มค่าดูซ้ำ 50-100 คำ
}

// EmotionalArcPoint จุดสำคัญใน emotional arc ของตัวละคร
type EmotionalArcPoint struct {
	Phase       string `json:"phase"`       // ช่วงเวลา เช่น "เริ่มต้น", "ไคลแมกซ์"
	Emotion     string `json:"emotion"`     // อารมณ์หลัก
	Description string `json:"description"` // บรรยาย 30-50 คำ
}

// ============================================================================
// State Management (Partial Success)
// ============================================================================

// ChunkState เก็บสถานะการ generate สำหรับ resume
type ChunkState struct {
	JobID      string        `json:"job_id"`
	VideoCode  string        `json:"video_code"`
	Chunk1     *Chunk1Output `json:"chunk1,omitempty"`
	Chunk2     *Chunk2Output `json:"chunk2,omitempty"`
	Chunk3     *Chunk3Output `json:"chunk3,omitempty"`
	Chunk4     *Chunk4Output `json:"chunk4,omitempty"`
	LastChunk  int           `json:"last_chunk"`  // 0, 1, 2, 3, 4 (completed up to)
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

// ============================================================================
// Error Types
// ============================================================================

// PartialGenerationError แยก partial fail ออกจาก full fail
// ใช้เมื่อ chunk บางส่วนสำเร็จแต่บางส่วนล้มเหลว
type PartialGenerationError struct {
	Message       string // Error message
	PartialPath   string // Path to saved state file
	FailedChunk   int    // Which chunk failed (1, 2, or 3)
	CompletedUpTo int    // Chunks completed before failure
	Cause         error  // Underlying error
}

func (e *PartialGenerationError) Error() string {
	return fmt.Sprintf("%s (partial state saved: %s, failed at chunk %d)",
		e.Message, e.PartialPath, e.FailedChunk)
}

func (e *PartialGenerationError) Unwrap() error {
	return e.Cause
}

// ChunkValidationError สำหรับ validation failures
type ChunkValidationError struct {
	Chunk   int
	Field   string
	Message string
}

func (e *ChunkValidationError) Error() string {
	return fmt.Sprintf("chunk %d validation failed: %s - %s", e.Chunk, e.Field, e.Message)
}

// ============================================================================
// Aggregation Result
// ============================================================================

// AggregationResult รองรับ Partial Success
type AggregationResult struct {
	Output       *ports.AIOutput `json:"output,omitempty"`
	PartialState *ChunkState     `json:"partial_state,omitempty"`
	IsComplete   bool            `json:"is_complete"`
	FailedChunk  int             `json:"failed_chunk,omitempty"` // 0 = none, 1, 2, 3
	Error        string          `json:"error,omitempty"`
}

// ============================================================================
// Aggregator: Merge 4 chunks into AIOutput
// ============================================================================

func AggregateChunks(chunk1 *Chunk1Output, chunk2 *Chunk2Output, chunk3 *Chunk3Output, chunk4 *Chunk4Output) *ports.AIOutput {
	output := &ports.AIOutput{
		// === From Chunk 1: Core SEO ===
		Title:           chunk1.Title,
		MetaTitle:       chunk1.MetaTitle,
		MetaDescription: chunk1.MetaDescription,
		Summary:         chunk1.Summary,
		SummaryShort:    chunk1.SummaryShort,
		Highlights:      chunk1.Highlights,
		KeyMoments:      chunk1.KeyMoments,
		GalleryAlts:     chunk1.GalleryAlts,
		SceneLocations:  chunk1.SceneLocations,
		ThumbnailAlt:    chunk1.ThumbnailAlt,
		QualityScore:    chunk1.QualityScore,

		// === From Chunk 2: E-E-A-T Analysis ===
		DialogueAnalysis:      chunk2.DialogueAnalysis,
		CharacterInsight:      chunk2.CharacterInsight,
		LanguageNotes:         chunk2.LanguageNotes,
		ActorPerformanceTrend: chunk2.ActorPerformanceTrend,
		ComparisonNote:        chunk2.ComparisonNote,
		TopQuotes:             chunk2.TopQuotes,
		ExpertAnalysis:        chunk2.ExpertAnalysis,
		DetailedReview:        chunk2.DetailedReview,
		CastBios:              chunk2.CastBios,
		TagDescriptions:       chunk2.TagDescriptions,
		CharacterDynamic:      chunk2.CharacterDynamic,
		PlotAnalysis:          chunk2.PlotAnalysis,
		Recommendation:        chunk2.Recommendation,
		RecommendedFor:        chunk2.RecommendedFor,
		SettingDescription:    chunk2.SettingDescription,
		MoodTone:              chunk2.MoodTone,
		ThematicKeywords:      chunk2.ThematicKeywords,
		ContextualLinks:       chunk2.ContextualLinks,

		// === From Chunk 3: Technical + FAQ ===
		TranslationMethod: chunk3.TranslationMethod,
		TranslationNote:   chunk3.TranslationNote,
		SubtitleQuality:   chunk3.SubtitleQuality,
		VideoQuality:      chunk3.VideoQuality,
		AudioQuality:      chunk3.AudioQuality,
		TechnicalFAQ:      chunk3.TechnicalFAQ,
		FAQItems:          chunk3.FAQItems,
		Keywords:          chunk3.Keywords,
		LongTailKeywords:  chunk3.LongTailKeywords,
	}

	// === From Chunk 4: Deep Analysis (Optional - for SEO Text boost) ===
	if chunk4 != nil {
		output.CinematographyAnalysis = chunk4.CinematographyAnalysis
		output.VisualStyle = chunk4.VisualStyle
		output.AtmosphereNotes = chunk4.AtmosphereNotes
		output.CharacterJourney = chunk4.CharacterJourney
		output.EmotionalArc = convertEmotionalArc(chunk4.EmotionalArc)
		output.ThematicExplanation = chunk4.ThematicExplanation
		output.CulturalContext = chunk4.CulturalContext
		output.GenreInsights = chunk4.GenreInsights
		output.StudioComparison = chunk4.StudioComparison
		output.ActorEvolution = chunk4.ActorEvolution
		output.GenreRanking = chunk4.GenreRanking
		output.ViewingTips = chunk4.ViewingTips
		output.BestMoments = chunk4.BestMoments
		output.AudienceMatch = chunk4.AudienceMatch
		output.ReplayValue = chunk4.ReplayValue
	}

	return output
}

// convertEmotionalArc แปลง internal type เป็น ports type
func convertEmotionalArc(arc []EmotionalArcPoint) []ports.EmotionalArcPoint {
	result := make([]ports.EmotionalArcPoint, len(arc))
	for i, p := range arc {
		result[i] = ports.EmotionalArcPoint{
			Phase:       p.Phase,
			Emotion:     p.Emotion,
			Description: p.Description,
		}
	}
	return result
}
