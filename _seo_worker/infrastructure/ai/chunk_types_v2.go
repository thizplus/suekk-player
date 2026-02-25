package ai

import (
	"fmt"
	"time"

	"seo-worker/domain/models"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk Types V2: 7-Chunk Architecture
// ============================================================================
//
// โครงสร้างใหม่แบ่งเป็น 7 Chunks เพื่อคุณภาพสูงสุด:
//
// Chunk 1: Core Identity (Foundation) - สร้าง "แก่น" และ CoreContext
// Chunk 2: Scene & Moments           - วิเคราะห์ฉากและ timestamps [Parallel]
// Chunk 3: Expertise                 - วิเคราะห์ภาษาและบทสนทนา [Parallel]
// Chunk 4: Authority                 - DetailedReview, CastBios, Tags [Parallel]
// Chunk 5: Recommendations           - Links, คำแนะนำ (รอ 2,3,4)
// Chunk 6: Technical & FAQ           - ข้อมูลเทคนิค, FAQ [Parallel]
// Chunk 7: Deep Analysis             - Cinematography, Character Journey [Parallel]
//
// ============================================================================

// ============================================================================
// Chunk 1: Core Identity (Foundation)
// Focus: สร้าง "แก่น" ของบทความ ใช้เป็น Context สำหรับ Chunk อื่น
// Persona: นักเขียน SEO มืออาชีพ
// ============================================================================

type Chunk1OutputV2 struct {
	// Core SEO
	Title           string `json:"title"`           // H1 ภาษาไทย 50-60 ตัวอักษร
	MetaTitle       string `json:"metaTitle"`       // Meta title สำหรับ Google
	MetaDescription string `json:"metaDescription"` // 150-160 ตัวอักษร

	// Content Summary
	Summary      string `json:"summary"`      // สรุป 400-500 คำ (แบ่ง 4-6 ย่อหน้า)
	SummaryShort string `json:"summaryShort"` // สรุปสั้นสำหรับ TTS

	// Thumbnail
	ThumbnailAlt string `json:"thumbnailAlt"` // Alt text สำหรับ thumbnail

	// Quality
	QualityScore int `json:"qualityScore"` // 1-10

	// Theme/Tone (for CoreContext)
	MainTheme string `json:"mainTheme"` // ธีมหลัก เช่น "ชีวิตสองด้าน"
	MainTone  string `json:"mainTone"`  // โทนหลัก เช่น "ผ่อนคลาย"
}

// ============================================================================
// Chunk 2: Scene & Moments
// Focus: วิเคราะห์ฉากและช่วงเวลาสำคัญ
// Persona: ผู้กำกับภาพยนตร์ / Scene Analyst
// ============================================================================

type Chunk2OutputV2 struct {
	Highlights     []string           `json:"highlights"`     // 5-8 ฉากสำคัญ (แต่ละจุด 15-30 คำ)
	KeyMoments     []models.KeyMoment `json:"keyMoments"`     // 3-5 Timestamps สำคัญ (Public)
	SceneLocations []string           `json:"sceneLocations"` // 3-5 สถานที่ในเรื่อง
	GalleryAlts    []string           `json:"galleryAlts"`    // Alt text สำหรับรูป
}

// ============================================================================
// Chunk 3: Expertise (Linguistic Analysis)
// Focus: วิเคราะห์บทสนทนาและภาษา
// Persona: นักภาษาศาสตร์ / นักวิจารณ์ภาพยนตร์
// ============================================================================

type Chunk3OutputV2 struct {
	DialogueAnalysis      string           `json:"dialogueAnalysis"`      // วิเคราะห์บทสนทนา 100-150 คำ
	CharacterInsight      string           `json:"characterInsight"`      // วิเคราะห์บุคลิกตัวละคร 100-150 คำ
	TopQuotes             []ports.TopQuote `json:"topQuotes"`             // 4-5 ประโยคเด็ด + timestamp + context
	LanguageNotes         string           `json:"languageNotes"`         // หมายเหตุภาษา 50-80 คำ
	ActorPerformanceTrend string           `json:"actorPerformanceTrend"` // แนวโน้มการแสดง 80-100 คำ
}

// ============================================================================
// Chunk 4: Authority (Entity Bios)
// Focus: สร้างเนื้อหาที่แสดง Authority (Cast, Tags, Review)
// Persona: นักเขียนชีวประวัติ / Encyclopedia Writer
// ============================================================================

type Chunk4OutputV2 struct {
	DetailedReview  string           `json:"detailedReview"`  // รีวิวละเอียด 500-700 คำ (5-7 ย่อหน้า)
	CastBios        []ports.CastBio  `json:"castBios"`        // ชีวประวัตินักแสดง 80-120 คำ/คน
	TagDescriptions []models.TagDesc `json:"tagDescriptions"` // คำอธิบาย Tags 30-50 คำ/tag
	ExpertAnalysis  string           `json:"expertAnalysis"`  // บทวิเคราะห์ผู้เชี่ยวชาญ 150-200 คำ
}

// ============================================================================
// Chunk 5: Recommendations & Links
// Focus: สร้าง Internal Links และคำแนะนำ
// Persona: Content Strategist / SEO Specialist
// ============================================================================

type Chunk5OutputV2 struct {
	CharacterDynamic   string                  `json:"characterDynamic"`   // ความสัมพันธ์ตัวละคร 100-150 คำ
	PlotAnalysis       string                  `json:"plotAnalysis"`       // วิเคราะห์โครงเรื่อง 100-150 คำ
	Recommendation     string                  `json:"recommendation"`     // เหมาะสำหรับใคร 50-80 คำ
	RecommendedFor     []string                `json:"recommendedFor"`     // กลุ่มเป้าหมาย 3-5 กลุ่ม
	ComparisonNote     string                  `json:"comparisonNote"`     // เปรียบเทียบกับเรื่องอื่น 80-100 คำ
	ContextualLinks    []models.ContextualLink `json:"contextualLinks"`    // ลิงก์ไปบทความอื่น 2-4 ลิงก์
	SettingDescription string                  `json:"settingDescription"` // บริบทฉาก 50-80 คำ
	MoodTone           []string                `json:"moodTone"`           // อารมณ์เรื่อง 3-5 คำ
	ThematicKeywords   []string                `json:"thematicKeywords"`   // Keywords สำหรับ semantic search 5-8 คำ
}

// ============================================================================
// Chunk 6: Technical & FAQ
// Focus: ข้อมูลเทคนิคและ FAQ
// Persona: Technical Writer / Customer Support
// ============================================================================

type Chunk6OutputV2 struct {
	// Trustworthiness
	TranslationMethod string           `json:"translationMethod"` // วิธีแปล 30-50 คำ
	TranslationNote   string           `json:"translationNote"`   // หมายเหตุการแปล 30-50 คำ
	SubtitleQuality   string           `json:"subtitleQuality"`   // คุณภาพซับ 30-50 คำ
	VideoQuality      string           `json:"videoQuality"`      // คุณภาพวิดีโอ 20-30 คำ
	AudioQuality      string           `json:"audioQuality"`      // คุณภาพเสียง 20-30 คำ
	TechnicalFAQ      []models.FAQItem `json:"technicalFaq"`      // FAQ เทคนิค 2-3 ข้อ

	// FAQ
	FAQItems []models.FAQItem `json:"faqItems"` // FAQ ทั่วไป 5-8 ข้อ

	// SEO
	Keywords         []string `json:"keywords"`         // SEO keywords 5-10 คำ
	LongTailKeywords []string `json:"longTailKeywords"` // Long-tail keywords 3-5 วลี
}

// ============================================================================
// Chunk 7: Deep Analysis
// Focus: วิเคราะห์เชิงลึก (Cinematography, Character Journey)
// Persona: Film Critic / Cultural Analyst
// ============================================================================

type Chunk7OutputV2 struct {
	// Section 1: Cinematography & Atmosphere
	CinematographyAnalysis string   `json:"cinematographyAnalysis"` // วิเคราะห์งานภาพ 250-350 คำ (3-4 ย่อหน้า)
	VisualStyle            string   `json:"visualStyle"`            // สไตล์ภาพโดยรวม 50-80 คำ
	AtmosphereNotes        []string `json:"atmosphereNotes"`        // จุดสังเกตบรรยากาศ 3-5 จุด

	// Section 2: Character Emotional Journey
	CharacterJourney string                  `json:"characterJourney"` // พัฒนาการทางอารมณ์ 300-400 คำ (3-5 ย่อหน้า)
	EmotionalArc     []EmotionalArcPointV2   `json:"emotionalArc"`     // จุดสำคัญใน emotional arc 3-4 จุด

	// Section 3: Educational Context
	ThematicExplanation string   `json:"thematicExplanation"` // อธิบายธีม 200-300 คำ (2-3 ย่อหน้า)
	CulturalContext     string   `json:"culturalContext"`     // บริบทวัฒนธรรม 100-150 คำ
	GenreInsights       []string `json:"genreInsights"`       // ข้อมูลเชิงลึกแนว 3-5 ข้อ

	// Section 4: Comparative Analysis
	StudioComparison string `json:"studioComparison"` // เปรียบเทียบกับค่าย 150-200 คำ
	ActorEvolution   string `json:"actorEvolution"`   // พัฒนาการนักแสดง 150-200 คำ
	GenreRanking     string `json:"genreRanking"`     // ตำแหน่งในแนว 50-80 คำ

	// Section 5: Viewing Experience
	ViewingTips   string   `json:"viewingTips"`   // คำแนะนำการรับชม 150-200 คำ
	BestMoments   []string `json:"bestMoments"`   // ช่วงเวลาดีที่สุด 3-5 จุด (พร้อมคำอธิบาย)
	AudienceMatch string   `json:"audienceMatch"` // เหมาะกับใคร 80-100 คำ
	ReplayValue   string   `json:"replayValue"`   // ความคุ้มค่าดูซ้ำ 50-80 คำ
}

// EmotionalArcPointV2 จุดสำคัญใน emotional arc ของตัวละคร
type EmotionalArcPointV2 struct {
	Phase       string `json:"phase"`       // ช่วงเวลา เช่น "เริ่มต้น", "ไคลแมกซ์"
	Emotion     string `json:"emotion"`     // อารมณ์หลัก
	Description string `json:"description"` // บรรยาย 30-50 คำ
}

// ============================================================================
// Context Structures
// ============================================================================

// CoreContext - Context หลักจาก Chunk 1 ส่งไปทุก Chunk
type CoreContext struct {
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	MainTheme string `json:"mainTheme"`
	MainTone  string `json:"mainTone"`

	// Entity-Consistency: รายชื่อที่ต้องใช้ตรงกันทุก Chunk
	Entities EntityList `json:"entities"`
}

// EntityList - รายชื่อ entity ที่ต้องใช้ตรงกันทุก Chunk
type EntityList struct {
	Actors    []ActorEntity `json:"actors"`
	Locations []string      `json:"locations"` // สถานที่หลักในเรื่อง
	Keywords  []string      `json:"keywords"`  // คำสำคัญ
}

// ActorEntity - ข้อมูลนักแสดงสำหรับ Entity-Consistency
type ActorEntity struct {
	FullName  string `json:"fullName"`  // ชื่อเต็ม (ใช้ครั้งแรก)
	FirstName string `json:"firstName"` // ชื่อต้น (ใช้ครั้งถัดไป)
	Role      string `json:"role"`      // บทบาทในเรื่อง
}

// ExtendedContext - Context ขยายสำหรับ Chunk 6, 7 (รวมข้อมูลจาก Chunk 2, 4)
type ExtendedContext struct {
	// จาก CoreContext
	Title     string     `json:"title"`
	Summary   string     `json:"summary"`
	Entities  EntityList `json:"entities"`

	// จาก Chunk 2: ฉากสำคัญที่ถูกเลือก
	TopHighlights []string `json:"topHighlights"` // Top 3 highlights
	KeyScenes     []string `json:"keyScenes"`     // Scene locations

	// จาก Chunk 4: บทวิเคราะห์ผู้เชี่ยวชาญ
	ExpertSummary string `json:"expertSummary"` // สรุป detailedReview 100 คำ
	MainInsight   string `json:"mainInsight"`   // expertAnalysis
}

// ============================================================================
// State Management (Partial Success)
// ============================================================================

// ChunkStateV2 เก็บสถานะการ generate สำหรับ resume (รองรับ 7 chunks)
type ChunkStateV2 struct {
	JobID     string `json:"job_id"`
	VideoCode string `json:"video_code"`

	// Chunk outputs
	Chunk1 *Chunk1OutputV2 `json:"chunk1,omitempty"`
	Chunk2 *Chunk2OutputV2 `json:"chunk2,omitempty"`
	Chunk3 *Chunk3OutputV2 `json:"chunk3,omitempty"`
	Chunk4 *Chunk4OutputV2 `json:"chunk4,omitempty"`
	Chunk5 *Chunk5OutputV2 `json:"chunk5,omitempty"`
	Chunk6 *Chunk6OutputV2 `json:"chunk6,omitempty"`
	Chunk7 *Chunk7OutputV2 `json:"chunk7,omitempty"`

	// Context
	CoreContext     *CoreContext     `json:"coreContext,omitempty"`
	ExtendedContext *ExtendedContext `json:"extendedContext,omitempty"`

	// State tracking
	LastChunk int       `json:"last_chunk"` // 0, 1, 2, 3, 4, 5, 6, 7 (completed up to)
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ============================================================================
// Error Types (V2)
// ============================================================================

// PartialGenerationErrorV2 แยก partial fail ออกจาก full fail
type PartialGenerationErrorV2 struct {
	Message       string // Error message
	PartialPath   string // Path to saved state file
	FailedChunk   int    // Which chunk failed (1-7)
	CompletedUpTo int    // Chunks completed before failure
	Cause         error  // Underlying error
}

func (e *PartialGenerationErrorV2) Error() string {
	return fmt.Sprintf("%s (partial state saved: %s, failed at chunk %d)",
		e.Message, e.PartialPath, e.FailedChunk)
}

func (e *PartialGenerationErrorV2) Unwrap() error {
	return e.Cause
}

// ============================================================================
// Aggregation Result (V2)
// ============================================================================

// AggregationResultV2 รองรับ Partial Success
type AggregationResultV2 struct {
	Output       *ports.AIOutput `json:"output,omitempty"`
	PartialState *ChunkStateV2   `json:"partial_state,omitempty"`
	IsComplete   bool            `json:"is_complete"`
	FailedChunk  int             `json:"failed_chunk,omitempty"` // 0 = none, 1-7
	Error        string          `json:"error,omitempty"`
}

// ============================================================================
// Context Building Functions
// ============================================================================

// BuildCoreContext สร้าง CoreContext จาก Chunk 1 output และ input
func BuildCoreContext(chunk1 *Chunk1OutputV2, casts []models.CastMetadata, sceneLocations []string) *CoreContext {
	// สร้าง actor entities
	actors := make([]ActorEntity, len(casts))
	for i, cast := range casts {
		// แยก first name (คำแรก)
		firstName := cast.Name
		parts := splitName(cast.Name)
		if len(parts) > 0 {
			firstName = parts[0]
		}

		actors[i] = ActorEntity{
			FullName:  cast.Name,
			FirstName: firstName,
			Role:      "", // จะถูก fill จาก context
		}
	}

	return &CoreContext{
		Title:     chunk1.Title,
		Summary:   chunk1.Summary,
		MainTheme: chunk1.MainTheme,
		MainTone:  chunk1.MainTone,
		Entities: EntityList{
			Actors:    actors,
			Locations: sceneLocations,
			Keywords:  []string{}, // จะถูก fill หลังจาก chunk 5
		},
	}
}

// BuildExtendedContext สร้าง ExtendedContext สำหรับ Chunk 6, 7
func BuildExtendedContext(core *CoreContext, chunk2 *Chunk2OutputV2, chunk4 *Chunk4OutputV2) *ExtendedContext {
	// Top 3 highlights
	topHighlights := chunk2.Highlights
	if len(topHighlights) > 3 {
		topHighlights = topHighlights[:3]
	}

	// สรุป detailedReview (100 คำแรก)
	expertSummary := truncateToWords(chunk4.DetailedReview, 100)

	return &ExtendedContext{
		Title:         core.Title,
		Summary:       truncateToWords(core.Summary, 200), // จำกัด 200 คำ
		Entities:      core.Entities,
		TopHighlights: topHighlights,
		KeyScenes:     chunk2.SceneLocations,
		ExpertSummary: expertSummary,
		MainInsight:   chunk4.ExpertAnalysis,
	}
}

// ============================================================================
// Aggregator: Merge 7 chunks into AIOutput
// ============================================================================

func AggregateChunksV2(
	chunk1 *Chunk1OutputV2,
	chunk2 *Chunk2OutputV2,
	chunk3 *Chunk3OutputV2,
	chunk4 *Chunk4OutputV2,
	chunk5 *Chunk5OutputV2,
	chunk6 *Chunk6OutputV2,
	chunk7 *Chunk7OutputV2,
) *ports.AIOutput {
	output := &ports.AIOutput{
		// === From Chunk 1: Core Identity ===
		Title:           chunk1.Title,
		MetaTitle:       chunk1.MetaTitle,
		MetaDescription: chunk1.MetaDescription,
		Summary:         chunk1.Summary,
		SummaryShort:    chunk1.SummaryShort,
		ThumbnailAlt:    chunk1.ThumbnailAlt,
		QualityScore:    chunk1.QualityScore,

		// === From Chunk 2: Scene & Moments ===
		Highlights:     chunk2.Highlights,
		KeyMoments:     chunk2.KeyMoments,
		SceneLocations: chunk2.SceneLocations,
		GalleryAlts:    chunk2.GalleryAlts,

		// === From Chunk 3: Expertise ===
		DialogueAnalysis:      chunk3.DialogueAnalysis,
		CharacterInsight:      chunk3.CharacterInsight,
		TopQuotes:             chunk3.TopQuotes,
		LanguageNotes:         chunk3.LanguageNotes,
		ActorPerformanceTrend: chunk3.ActorPerformanceTrend,

		// === From Chunk 4: Authority ===
		DetailedReview:  chunk4.DetailedReview,
		CastBios:        chunk4.CastBios,
		TagDescriptions: chunk4.TagDescriptions,
		ExpertAnalysis:  chunk4.ExpertAnalysis,

		// === From Chunk 5: Recommendations ===
		CharacterDynamic:   chunk5.CharacterDynamic,
		PlotAnalysis:       chunk5.PlotAnalysis,
		Recommendation:     chunk5.Recommendation,
		RecommendedFor:     chunk5.RecommendedFor,
		ComparisonNote:     chunk5.ComparisonNote,
		ContextualLinks:    chunk5.ContextualLinks,
		SettingDescription: chunk5.SettingDescription,
		MoodTone:           chunk5.MoodTone,
		ThematicKeywords:   chunk5.ThematicKeywords,

		// === From Chunk 6: Technical & FAQ ===
		TranslationMethod: chunk6.TranslationMethod,
		TranslationNote:   chunk6.TranslationNote,
		SubtitleQuality:   chunk6.SubtitleQuality,
		VideoQuality:      chunk6.VideoQuality,
		AudioQuality:      chunk6.AudioQuality,
		TechnicalFAQ:      chunk6.TechnicalFAQ,
		FAQItems:          chunk6.FAQItems,
		Keywords:          chunk6.Keywords,
		LongTailKeywords:  chunk6.LongTailKeywords,
	}

	// === From Chunk 7: Deep Analysis (Optional) ===
	if chunk7 != nil {
		output.CinematographyAnalysis = chunk7.CinematographyAnalysis
		output.VisualStyle = chunk7.VisualStyle
		output.AtmosphereNotes = chunk7.AtmosphereNotes
		output.CharacterJourney = chunk7.CharacterJourney
		output.EmotionalArc = convertEmotionalArcV2(chunk7.EmotionalArc)
		output.ThematicExplanation = chunk7.ThematicExplanation
		output.CulturalContext = chunk7.CulturalContext
		output.GenreInsights = chunk7.GenreInsights
		output.StudioComparison = chunk7.StudioComparison
		output.ActorEvolution = chunk7.ActorEvolution
		output.GenreRanking = chunk7.GenreRanking
		output.ViewingTips = chunk7.ViewingTips
		output.BestMoments = chunk7.BestMoments
		output.AudienceMatch = chunk7.AudienceMatch
		output.ReplayValue = chunk7.ReplayValue
	}

	return output
}

// ============================================================================
// Helper Functions
// ============================================================================

// convertEmotionalArcV2 แปลง internal type เป็น ports type
func convertEmotionalArcV2(arc []EmotionalArcPointV2) []ports.EmotionalArcPoint {
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

// splitName แยกชื่อเป็นส่วนๆ
func splitName(name string) []string {
	var parts []string
	current := ""
	for _, r := range name {
		if r == ' ' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// truncateToWords ตัด text ให้เหลือ n คำแรก
func truncateToWords(text string, maxWords int) string {
	words := splitName(text) // reuse splitName for word splitting
	if len(words) <= maxWords {
		return text
	}
	result := ""
	for i := 0; i < maxWords; i++ {
		if i > 0 {
			result += " "
		}
		result += words[i]
	}
	return result + "..."
}
