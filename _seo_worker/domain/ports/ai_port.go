package ports

import (
	"context"

	"seo-worker/domain/models"
)

// AIPort - Interface สำหรับ AI Content Generation (Gemini)
type AIPort interface {
	// GenerateArticleContent รับ SRT + Metadata แล้วสร้าง content
	// ใช้ Gemini JSON Mode เพื่อป้องกัน parsing error
	GenerateArticleContent(ctx context.Context, input *AIInput) (*AIOutput, error)

	// GenerateArticleContentV2 รัน 7-chunk pipeline แบบ parallel
	// ใช้ Atomic Chunking + Context Feeding + Entity-Consistency
	// ~55 sec (vs ~90 sec sequential)
	GenerateArticleContentV2(ctx context.Context, input *AIInput) (*AIOutput, error)
}

// AIInput - ข้อมูลที่ส่งให้ AI
type AIInput struct {
	SRTContent      string                   // Full SRT text
	VideoMetadata   *models.VideoMetadata    // From api.subth.com
	Casts           []models.CastMetadata    // Cast info
	Tags            []models.TagMetadata     // Tag info (สำหรับ generate tag descriptions)
	PreviousWorks   []models.PreviousWork    // For context
	GalleryCount    int                      // จำนวน gallery images (สำหรับสร้าง alt)
	RelatedArticles []RelatedArticleForAI    // Related articles (สำหรับสร้าง contextual links)
}

// RelatedArticleForAI - ข้อมูล related article สำหรับ AI สร้าง contextual links
type RelatedArticleForAI struct {
	Slug         string   `json:"slug"`         // URL slug
	Title        string   `json:"title"`        // Video title
	RealCode     string   `json:"realCode"`     // เช่น DLDSS-470
	CastNames    []string `json:"castNames"`    // รายชื่อนักแสดง
	Tags         []string `json:"tags"`         // Tags ของ video นั้น
	ThumbnailUrl string   `json:"thumbnailUrl"` // Thumbnail URL สำหรับแสดงภาพ
}

// AIOutput - ผลลัพธ์จาก AI (E-E-A-T Framework)
type AIOutput struct {
	// === Core SEO ===
	Title           string   `json:"title"`           // H1 title (50-60 chars)
	MetaTitle       string   `json:"metaTitle"`       // Meta title with [Code] [ซับไทย] (50-60 chars)
	MetaDescription string   `json:"metaDescription"` // 150-160 chars
	Summary         string   `json:"summary"`         // 400+ words
	Highlights      []string `json:"highlights"`
	DetailedReview  string   `json:"detailedReview"` // 600+ words
	QualityScore    int      `json:"qualityScore"`

	// Key Moments with timestamps from SRT
	KeyMoments []models.KeyMoment `json:"keyMoments"`

	// Cast bios generated from previous works
	CastBios []CastBio `json:"castBios"`

	// Tag descriptions
	TagDescriptions []models.TagDesc `json:"tagDescriptions"`

	// FAQ
	FAQItems []models.FAQItem `json:"faqItems"`

	// Gallery alt texts
	GalleryAlts []string `json:"galleryAlts"`

	// Thumbnail alt
	ThumbnailAlt string `json:"thumbnailAlt"`

	// === [E] Experience Section ===
	SceneLocations []string `json:"sceneLocations"` // ["ห้องตรวจ", "คลินิก"]

	// === [E] Expertise Section ===
	DialogueAnalysis      string     `json:"dialogueAnalysis"`      // วิเคราะห์บทสนทนา
	CharacterInsight      string     `json:"characterInsight"`      // วิเคราะห์บุคลิกตัวละคร
	TopQuotes             []TopQuote `json:"topQuotes"`             // ประโยคเด็ด
	LanguageNotes         string     `json:"languageNotes"`         // หมายเหตุภาษา
	ActorPerformanceTrend string     `json:"actorPerformanceTrend"` // เปรียบเทียบการแสดง
	ComparisonNote        string     `json:"comparisonNote"`        // เปรียบเทียบกับเรื่องอื่น

	// === [A] Authoritativeness Section ===
	SummaryShort       string   `json:"summaryShort"`       // สรุปสั้น 2-3 บรรทัด (สำหรับ TTS)
	CharacterDynamic   string   `json:"characterDynamic"`   // ความสัมพันธ์ตัวละคร
	PlotAnalysis       string   `json:"plotAnalysis"`       // วิเคราะห์โครงเรื่อง
	Recommendation     string   `json:"recommendation"`     // เหมาะสำหรับ...
	RecommendedFor     []string `json:"recommendedFor"`     // ["แฟนหนัง X", "คนชอบ Y"]
	ThematicKeywords   []string `json:"thematicKeywords"`   // Keywords semantic search
	SettingDescription string   `json:"settingDescription"` // บริบทฉาก
	MoodTone           []string `json:"moodTone"`           // ["ดราม่า", "โรแมนติก"]

	// === [T] Trustworthiness Section ===
	TranslationMethod string           `json:"translationMethod"` // วิธีการแปล
	TranslationNote   string           `json:"translationNote"`   // หมายเหตุการแปล (Human Touch)
	SubtitleQuality   string           `json:"subtitleQuality"`   // คุณภาพซับ
	TechnicalFAQ      []models.FAQItem `json:"technicalFaq"`      // FAQ เทคนิค

	// === Technical Specs ===
	VideoQuality string `json:"videoQuality"` // คุณภาพวิดีโอ เช่น "1080p Full HD"
	AudioQuality string `json:"audioQuality"` // คุณภาพเสียง เช่น "ระบบเสียงสเตอริโอคมชัด"

	// === SEO Enhancement ===
	ExpertAnalysis   string   `json:"expertAnalysis"`   // บทวิเคราะห์ผู้เชี่ยวชาญ
	Keywords         []string `json:"keywords"`         // SEO keywords
	LongTailKeywords []string `json:"longTailKeywords"` // Long-tail keywords

	// === Internal Linking (SEO) ===
	ContextualLinks []models.ContextualLink `json:"contextualLinks"` // ประโยคเชื่อมโยงไป related articles

	// === Chunk 4: Deep Analysis (SEO Text boost) ===
	// Section 1: Cinematography & Atmosphere
	CinematographyAnalysis string   `json:"cinematographyAnalysis,omitempty"` // วิเคราะห์งานภาพ 300-500 คำ
	VisualStyle            string   `json:"visualStyle,omitempty"`            // สไตล์ภาพโดยรวม
	AtmosphereNotes        []string `json:"atmosphereNotes,omitempty"`        // จุดสังเกตบรรยากาศ

	// Section 2: Character Emotional Journey
	CharacterJourney string              `json:"characterJourney,omitempty"` // พัฒนาการทางอารมณ์ 400-600 คำ
	EmotionalArc     []EmotionalArcPoint `json:"emotionalArc,omitempty"`     // จุดสำคัญ emotional arc

	// Section 3: Educational Context
	ThematicExplanation string   `json:"thematicExplanation,omitempty"` // อธิบายธีม 300-500 คำ
	CulturalContext     string   `json:"culturalContext,omitempty"`     // บริบทวัฒนธรรม
	GenreInsights       []string `json:"genreInsights,omitempty"`       // ข้อมูลเชิงลึกแนวเรื่อง

	// Section 4: Comparative Analysis
	StudioComparison string `json:"studioComparison,omitempty"` // เปรียบเทียบกับค่าย
	ActorEvolution   string `json:"actorEvolution,omitempty"`   // พัฒนาการนักแสดง
	GenreRanking     string `json:"genreRanking,omitempty"`     // ตำแหน่งในแนว

	// Section 5: Viewing Experience
	ViewingTips   string   `json:"viewingTips,omitempty"`   // คำแนะนำการรับชม
	BestMoments   []string `json:"bestMoments,omitempty"`   // ช่วงเวลาดีที่สุด
	AudienceMatch string   `json:"audienceMatch,omitempty"` // เหมาะกับใคร
	ReplayValue   string   `json:"replayValue,omitempty"`   // ความคุ้มค่าดูซ้ำ
}

type CastBio struct {
	CastID string `json:"castId"`
	Bio    string `json:"bio"`
}

// TopQuote - ประโยคเด็ดจากซับไตเติ้ล
type TopQuote struct {
	Text      string `json:"text"`      // ประโยคไทย
	Timestamp int    `json:"timestamp"` // seconds
	Emotion   string `json:"emotion"`   // อารมณ์
	Context   string `json:"context"`   // บริบท
}

// EmotionalArcPoint - จุดสำคัญใน emotional arc ของตัวละคร
type EmotionalArcPoint struct {
	Phase       string `json:"phase"`       // ช่วงเวลา เช่น "เริ่มต้น", "ไคลแมกซ์"
	Emotion     string `json:"emotion"`     // อารมณ์หลัก
	Description string `json:"description"` // บรรยาย
}
