package classifier

// ═══════════════════════════════════════════════════════════════════════════════
// NSFW Classifier Types
// Shared types for NSFW classification results
// Three-Tier System: SuperSafe (< 0.15 + face) | Safe (0.15-0.3) | NSFW (>= 0.3)
// ═══════════════════════════════════════════════════════════════════════════════

// Three-Tier Thresholds
const (
	SuperSafeThreshold = 0.15 // < 0.15 + face = super safe (Public SEO)
	SafeThreshold      = 0.30 // < 0.30 = safe
	MinFaceScore       = 0.1  // ต้องมีหน้าคนในภาพสำหรับ super_safe
)

// ClassificationResult ผลการ classify ของแต่ละภาพ
type ClassificationResult struct {
	Filename    string  `json:"filename"`
	IsSuperSafe bool    `json:"is_super_safe"` // NEW: < 0.15 + face (Public SEO)
	IsSafe      bool    `json:"is_safe"`       // < 0.30
	NsfwScore   float64 `json:"nsfw_score"`
	Error       string  `json:"error,omitempty"`

	// Quality scores (for sorting)
	FaceScore      float64 `json:"face_score"`
	AestheticScore float64 `json:"aesthetic_score"`
}

// BatchResult ผลลัพธ์จากการ classify ทั้ง batch
type BatchResult struct {
	Results    map[string]ClassificationResult `json:"results"`
	Stats      ClassificationStats             `json:"stats"`
	OutputPath string                          `json:"output_path"`
}

// ClassificationStats สถิติการ classify (สำหรับ logging/tuning)
type ClassificationStats struct {
	TotalImages       int     `json:"total_images"`
	SuperSafeCount    int     `json:"super_safe_count"` // NEW
	SafeCount         int     `json:"safe_count"`       // borderline (0.15-0.3)
	NsfwCount         int     `json:"nsfw_count"`
	ErrorCount        int     `json:"error_count"`
	AvgNsfwScore      float64 `json:"avg_nsfw_score"`
	ProcessingTime    float64 `json:"processing_time_sec"`
}

// ClassifierConfig configuration สำหรับ classifier
type ClassifierConfig struct {
	PythonPath    string  // Path to python executable (default: "python")
	ScriptPath    string  // Path to classify_batch.py
	NsfwThreshold float64 // Score above this = NSFW (default: 0.3)
	Timeout       int     // Timeout in seconds (default: 90)
	MaxNsfwImages int     // Max NSFW images to keep (default: 30)
	MinSafeImages int     // Minimum safe images required (default: 12)

	// Three-Tier config
	SuperSafeThreshold float64 // Score below this + face = super safe (default: 0.15)
	MinFaceScore       float64 // Minimum face score for super safe (default: 0.1)
	MinSuperSafeImages int     // Minimum super safe images required (default: 10)
}

// DefaultConfig returns default classifier configuration
func DefaultConfig() ClassifierConfig {
	return ClassifierConfig{
		PythonPath:         "python",
		ScriptPath:         "infrastructure/classifier/classify_batch.py",
		NsfwThreshold:      0.3,
		Timeout:            90,
		MaxNsfwImages:      30,
		MinSafeImages:      12,
		SuperSafeThreshold: 0.15,
		MinFaceScore:       0.1,
		MinSuperSafeImages: 10,
	}
}

// SeparatedImages ภาพที่แยกแล้วตาม classification (Three-Tier)
type SeparatedImages struct {
	SuperSafe []ClassificationResult `json:"super_safe"` // < 0.15 + face (Public SEO)
	Safe      []ClassificationResult `json:"safe"`       // 0.15-0.3 or no face (Lazy load)
	Nsfw      []ClassificationResult `json:"nsfw"`       // >= 0.3 (Member only)
	Error     []ClassificationResult `json:"error"`
}
