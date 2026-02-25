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
	IsSuperSafe bool    `json:"is_super_safe"` // NEW: < 0.15 + face + no_mosaic (Public SEO)
	IsSafe      bool    `json:"is_safe"`       // < 0.30
	NsfwScore   float64 `json:"nsfw_score"`
	Error       string  `json:"error,omitempty"`

	// Quality scores (for sorting)
	FaceScore      float64 `json:"face_score"`
	AestheticScore float64 `json:"aesthetic_score"`

	// Detailed scores (for debugging)
	FalconsaiScore float64 `json:"falconsai_score"` // Score from Falconsai model
	NudenetScore   float64 `json:"nudenet_score"`   // Score from NudeNet model
	Classification string  `json:"classification"`  // super_safe, safe, nsfw, error
	Reason         string  `json:"reason"`          // Why this classification was chosen

	// Mosaic/Censorship detection
	MosaicDetected bool    `json:"mosaic_detected"` // True if mosaic/pixelation censorship detected
	MosaicScore    float64 `json:"mosaic_score"`    // Mosaic detection score (0-1)

	// POV (Point of View) detection
	POVDetected bool    `json:"pov_detected"` // True if POV adult composition detected
	POVScore    float64 `json:"pov_score"`    // POV detection score (0-1)
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
	OriginalImages    int     `json:"original_images"`    // Before dedup
	DuplicatesRemoved int     `json:"duplicates_removed"` // Removed by dedup
	SuperSafeCount    int     `json:"super_safe_count"`   // < 0.15 + face + no_mosaic + no_pov
	SafeCount         int     `json:"safe_count"`         // borderline (0.15-0.3) or POV
	NsfwCount         int     `json:"nsfw_count"`
	ErrorCount        int     `json:"error_count"`
	MosaicCount       int     `json:"mosaic_count"` // Images with censorship/mosaic detected
	POVCount          int     `json:"pov_count"`    // Images with POV composition detected
	AvgNsfwScore      float64 `json:"avg_nsfw_score"`
	AvgFaceScore      float64 `json:"avg_face_score"` // Average face detection score
	ProcessingTime    float64 `json:"processing_time_sec"`
}

// ClassifierConfig configuration สำหรับ classifier
type ClassifierConfig struct {
	PythonPath    string  // Path to python executable (default: "python")
	ScriptPath    string  // Path to classify_batch.py
	NsfwThreshold float64 // Score above this = NSFW (default: 0.3)
	Timeout       int     // Timeout in seconds (default: 300 for POV + Mosaic)
	MaxNsfwImages int     // Max NSFW images to keep (default: 10)
	MaxSafeImages int     // Max Safe images to keep (default: 10)
	MinSafeImages int     // Minimum safe images required (default: 12)

	// Three-Tier config
	SuperSafeThreshold float64 // Score below this + face = super safe (default: 0.15)
	MinFaceScore       float64 // Minimum face score for super safe (default: 0.1)
	MinSuperSafeImages int     // Minimum super safe images required (default: 10)

	// Debug options
	Verbose bool // If true, enable detailed per-image logging

	// Performance options (skip slow detections)
	SkipMosaic bool // If true, skip slow mosaic detection
	SkipPOV    bool // If true, skip slow POV detection

	// Deduplication options
	SkipDedup      bool // If true, skip image deduplication
	DedupThreshold int  // Hamming distance threshold for dedup (0=identical, 8=default)
}

// DefaultConfig returns default classifier configuration
func DefaultConfig() ClassifierConfig {
	return ClassifierConfig{
		PythonPath:         "python",
		ScriptPath:         "infrastructure/classifier/classify_batch.py",
		NsfwThreshold:      0.3,
		Timeout:            300,
		MaxNsfwImages:      20,
		MaxSafeImages:      10,
		MinSafeImages:      12,
		SuperSafeThreshold: 0.15,
		MinFaceScore:       0.1,
		MinSuperSafeImages: 10,
		SkipDedup:          true, // ปิด dedup - ใช้ MaxFrames limit แทน
		DedupThreshold:     8,    // Hamming distance threshold (ไม่ใช้ถ้า SkipDedup=true)
	}
}

// SeparatedImages ภาพที่แยกแล้วตาม classification (Three-Tier)
type SeparatedImages struct {
	SuperSafe []ClassificationResult `json:"super_safe"` // < 0.15 + face (Public SEO)
	Safe      []ClassificationResult `json:"safe"`       // 0.15-0.3 or no face (Lazy load)
	Nsfw      []ClassificationResult `json:"nsfw"`       // >= 0.3 (Member only)
	Error     []ClassificationResult `json:"error"`
}
