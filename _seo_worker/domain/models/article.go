package models

import "time"

// ArticleContent - ข้อมูล SEO Article (E-E-A-T Framework)
type ArticleContent struct {
	VideoID string `json:"videoId"`

	// === Core SEO ===
	Title           string `json:"title"`           // H1 title (50-60 chars)
	MetaTitle       string `json:"metaTitle"`       // Meta title with [Code] [ซับไทย] (50-60 chars)
	MetaDescription string `json:"metaDescription"` // 150-160 chars
	Slug            string `json:"slug"`            // URL-friendly

	// === Schema.org VideoObject ===
	VideoName        string `json:"videoName"`
	VideoDescription string `json:"videoDescription"`
	ThumbnailURL     string `json:"thumbnailUrl"`
	ThumbnailAlt     string `json:"thumbnailAlt"` // AI generated
	UploadDate       string `json:"uploadDate"`   // ISO 8601
	Duration         string `json:"duration"`     // ISO 8601 (PT1H30M)
	ContentURL       string `json:"contentUrl"`
	EmbedURL         string `json:"embedUrl"`

	// === Key Moments (hasPart) ===
	KeyMoments []KeyMoment `json:"keyMoments"`

	// === Article Content ===
	Summary        string   `json:"summary"`        // 500 words (AI)
	Highlights     []string `json:"highlights"`     // 5-10 key scenes (AI)
	DetailedReview string   `json:"detailedReview"` // Long-form 800-1000 words (AI)

	// === Cast & Crew ===
	CastProfiles  []CastProfile  `json:"castProfiles"`
	MakerInfo     *MakerInfo     `json:"makerInfo,omitempty"`
	PreviousWorks []PreviousWork `json:"previousWorks,omitempty"` // ผลงานก่อนหน้าของ cast

	// === Related Content ===
	RelatedVideos    []RelatedVideo    `json:"relatedVideos,omitempty"`
	TagDescriptions  []TagDesc         `json:"tagDescriptions,omitempty"`
	ContextualLinks  []ContextualLink  `json:"contextualLinks,omitempty"`  // SEO Internal Linking (AI generated)

	// === [E] Experience Section ===
	SceneLocations []string `json:"sceneLocations,omitempty"` // สถานที่ในเรื่อง

	// === [E] Expertise Section ===
	DialogueAnalysis      string     `json:"dialogueAnalysis,omitempty"`      // วิเคราะห์บทสนทนา
	CharacterInsight      string     `json:"characterInsight,omitempty"`      // วิเคราะห์บุคลิกตัวละคร
	TopQuotes             []TopQuote `json:"topQuotes,omitempty"`             // ประโยคเด็ด
	LanguageNotes         string     `json:"languageNotes,omitempty"`         // หมายเหตุภาษา
	ActorPerformanceTrend string     `json:"actorPerformanceTrend,omitempty"` // เปรียบเทียบการแสดง
	ComparisonNote        string     `json:"comparisonNote,omitempty"`        // เปรียบเทียบกับเรื่องอื่น

	// === [A] Authoritativeness Section ===
	SummaryShort       string   `json:"summaryShort,omitempty"`       // สรุปสั้น (สำหรับ TTS)
	CharacterDynamic   string   `json:"characterDynamic,omitempty"`   // ความสัมพันธ์ตัวละคร
	PlotAnalysis       string   `json:"plotAnalysis,omitempty"`       // วิเคราะห์โครงเรื่อง
	Recommendation     string   `json:"recommendation,omitempty"`     // เหมาะสำหรับ...
	RecommendedFor     []string `json:"recommendedFor,omitempty"`     // กลุ่มเป้าหมาย
	ThematicKeywords   []string `json:"thematicKeywords,omitempty"`   // Keywords semantic search
	SettingDescription string   `json:"settingDescription,omitempty"` // บริบทฉาก
	MoodTone           []string `json:"moodTone,omitempty"`           // อารมณ์เรื่อง

	// === [T] Trustworthiness Section ===
	TranslationMethod string    `json:"translationMethod,omitempty"` // วิธีการแปล
	TranslationNote   string    `json:"translationNote,omitempty"`   // หมายเหตุการแปล
	SubtitleQuality   string    `json:"subtitleQuality,omitempty"`   // คุณภาพซับ
	TechnicalFAQ      []FAQItem `json:"technicalFaq,omitempty"`      // FAQ เทคนิค

	// === Technical Specs (เพิ่มความน่าเชื่อถือเชิงเทคนิค) ===
	VideoQuality string `json:"videoQuality,omitempty"` // คุณภาพวิดีโอ เช่น "1080p Full HD"
	AudioQuality string `json:"audioQuality,omitempty"` // คุณภาพเสียง เช่น "Stereo 320kbps"

	// === SEO Enhancement ===
	ExpertAnalysis   string   `json:"expertAnalysis,omitempty"`   // บทวิเคราะห์ผู้เชี่ยวชาญ
	QualityScore     int      `json:"qualityScore"`               // AI rating 1-10
	Keywords         []string `json:"keywords,omitempty"`         // SEO keywords
	LongTailKeywords []string `json:"longTailKeywords,omitempty"` // Long-tail keywords
	ReadingTime      int      `json:"readingTime,omitempty"`      // minutes

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

	// === TTS ===
	AudioSummaryURL string `json:"audioSummaryUrl,omitempty"` // ElevenLabs output
	AudioDuration   int    `json:"audioDuration,omitempty"`   // seconds

	// === Gallery ===
	GalleryImages       []GalleryImage `json:"galleryImages,omitempty"`       // Public (safe - admin approved)
	MemberGalleryImages []GalleryImage `json:"memberGalleryImages,omitempty"` // Member only (safe + nsfw)
	MemberGalleryCount  int            `json:"memberGalleryCount,omitempty"`  // จำนวนภาพ member

	// === FAQ (AI Generated) ===
	FAQItems []FAQItem `json:"faqItems"`

	// === Timestamps ===
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type KeyMoment struct {
	Name        string `json:"name"`
	StartOffset int    `json:"startOffset"` // seconds
	EndOffset   int    `json:"endOffset"`   // seconds
	URL         string `json:"url"`         // ?t={startOffset}
}

type CastProfile struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	NameTH     string `json:"nameTH,omitempty"`
	Bio        string `json:"bio"` // AI generated
	ImageURL   string `json:"imageUrl,omitempty"`
	ProfileURL string `json:"profileUrl"`
}

type MakerInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ProfileURL  string `json:"profileUrl"`
}

type RelatedVideo struct {
	ID           string  `json:"id"`
	Code         string  `json:"code"`
	Title        string  `json:"title"`
	ThumbnailURL string  `json:"thumbnailUrl"`
	URL          string  `json:"url"`
	Similarity   float64 `json:"similarity,omitempty"` // from pgvector
}

type TagDesc struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"` // AI generated
	URL         string `json:"url"`
}

type GalleryImage struct {
	URL    string `json:"url"`
	Alt    string `json:"alt"` // AI generated from highlights
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// GalleryTier - ประเภทของ gallery (Manual Selection)
type GalleryTier string

const (
	GalleryTierSafe GalleryTier = "safe" // Admin approved - safe for public/SEO
	GalleryTierNSFW GalleryTier = "nsfw" // Admin approved - members only
)

// TieredGalleryImages - ภาพแยกตาม tier (Manual Selection)
type TieredGalleryImages struct {
	Safe []string // Admin approved - safe for public/SEO
	NSFW []string // Admin approved - members only
}

type FAQItem struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// ContextualLink - ลิงก์เชื่อมโยงในบริบท (SEO Internal Linking)
// AI สร้างประโยคเชื่อมโยงไปยัง related articles
type ContextualLink struct {
	Text         string `json:"text"`         // ประโยคเชื่อมโยง เช่น "ถ้าคุณประทับใจการแสดงแนว Medical ของ Zemba Mami คุณอาจจะสนใจ"
	LinkedSlug   string `json:"linkedSlug"`   // Slug ของ article ที่ลิงก์ไป เช่น "dldss-470"
	LinkedTitle  string `json:"linkedTitle"`  // Title สำหรับแสดง เช่น "DLDSS-470 ที่เน้นการสำรวจอารมณ์"
	ThumbnailUrl string `json:"thumbnailUrl"` // Thumbnail URL สำหรับแสดงภาพ
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

// VideoMetadata - ข้อมูล video จาก api.subth.com
type VideoMetadata struct {
	ID          string   `json:"id"`
	Code        string   `json:"code"`         // Internal code (e.g., utywgage) - ใช้ sync suekk/subth
	RealCode    string   `json:"realCode"`     // Real video code (e.g., DLDSS-471) - สกัดจาก title
	Title       string   `json:"title"`
	Duration    int      `json:"duration"` // seconds
	ReleaseDate string   `json:"releaseDate"`
	CastIDs     []string `json:"castIds"`
	MakerID     string   `json:"makerId"`
	TagIDs      []string `json:"tagIds"`
	CategoryID  string   `json:"categoryId"`
	Thumbnail   string   `json:"thumbnail"`
	HLSPath     string   `json:"hlsPath"`

	// Nested data (populated from /videos/:id response)
	Casts []CastMetadata  `json:"casts,omitempty"`
	Maker *MakerMetadata  `json:"maker,omitempty"`
	Tags  []TagMetadata   `json:"tags,omitempty"`
}

// CastMetadata - ข้อมูล cast จาก api.subth.com
type CastMetadata struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	NameTH   string `json:"nameTH,omitempty"`
	ImageURL string `json:"imageUrl,omitempty"`
	Slug     string `json:"slug"`
}

// PreviousWork - ผลงานก่อนหน้าของ cast
type PreviousWork struct {
	VideoID      string `json:"videoId"`
	VideoCode    string `json:"videoCode"`    // Internal code (e.g., "3993bp6j")
	Slug         string `json:"slug"`         // Article slug (e.g., "dass-541") - ใช้สร้าง URL
	Title        string `json:"title"`
	ThumbnailUrl string `json:"thumbnailUrl"` // Thumbnail URL for display
}

// EmbeddingData - Vector + Metadata สำหรับ pgvector
type EmbeddingData struct {
	VideoID   string    `json:"video_id"`
	Vector    []float32 `json:"vector"` // 1536 dims
	CastIDs   []string  `json:"cast_ids"`
	MakerID   string    `json:"maker_id"`
	TagIDs    []string  `json:"tag_ids"`
	CreatedAt time.Time `json:"created_at"`
}

// SuekkVideoInfo - ข้อมูล video จาก api.suekk.com
type SuekkVideoInfo struct {
	Code             string `json:"code"`
	Duration         int    `json:"duration"`         // seconds
	ThumbnailURL     string `json:"thumbnailUrl"`
	GalleryPath      string `json:"galleryPath"`
	GalleryCount     int    `json:"galleryCount"`
	GallerySafeCount int    `json:"gallerySafeCount"` // จำนวนภาพ safe (pre-classified)
	GalleryNsfwCount int    `json:"galleryNsfwCount"` // จำนวนภาพ nsfw (pre-classified)
}

// MakerMetadata - ข้อมูล maker จาก api.subth.com
type MakerMetadata struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// TagMetadata - ข้อมูล tag จาก api.subth.com
type TagMetadata struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	NameTH string `json:"nameTH,omitempty"`
	Slug   string `json:"slug"`
}

// CategoryMetadata - ข้อมูล category จาก api.subth.com
type CategoryMetadata struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	NameTH string `json:"nameTH,omitempty"`
	Slug   string `json:"slug"`
}

// ImageScore - คะแนนของภาพแต่ละภาพจาก Image Selector
type ImageScore struct {
	URL            string  `json:"url"`
	Filename       string  `json:"filename"`
	NSFWScore      float64 `json:"nsfw_score"`      // 0-1, lower is safer
	FaceScore      float64 `json:"face_score"`      // 0-1, higher means clearer face
	AestheticScore float64 `json:"aesthetic_score"` // 0-1, higher is better
	CombinedScore  float64 `json:"combined_score"`  // weighted combination
	IsSafe         bool    `json:"is_safe"`         // passes NSFW threshold
	IsBlurred      bool    `json:"is_blurred"`      // was this image blurred?
	BlurredPath    string  `json:"blurred_path"`    // local path to blurred image
}

// ImageSelectionResult - ผลลัพธ์จาก Image Selector
type ImageSelectionResult struct {
	Cover          *ImageScore  `json:"cover"`
	Gallery        []ImageScore `json:"gallery"`
	TotalImages    int          `json:"total_images"`
	SafeImages     int          `json:"safe_images"`
	BlurredImages  int          `json:"blurred_images"`
	ProcessingTime float64      `json:"processing_time"`
}
