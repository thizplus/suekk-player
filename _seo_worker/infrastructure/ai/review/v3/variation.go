package v3

import (
	"hash/fnv"

	"seo-worker/domain/models"
)

// ============================================================================
// Content Variation System
// เพื่อป้องกัน Google vector similarity detection
// ============================================================================

// ContentVariation กำหนด style ที่จะใช้ generate content
type ContentVariation struct {
	OpeningStyle string // A, B, C, D
	ReviewTone   string // Analytical, Conversational, Comparative
	FAQStyle     string // Formal, Casual
}

// GetContentVariation คืน variation ตาม videoID (deterministic)
func GetContentVariation(videoID string) ContentVariation {
	hash := HashString(videoID)

	// Opening Style: 4 แบบ (A, B, C, D)
	openingStyles := []string{"A", "B", "C", "D"}
	openingIdx := hash % uint32(len(openingStyles))

	// Review Tone: 3 แบบ
	reviewTones := []string{"Analytical", "Conversational", "Comparative"}
	reviewIdx := (hash / 4) % uint32(len(reviewTones))

	// FAQ Style: 2 แบบ
	faqStyles := []string{"Formal", "Casual"}
	faqIdx := (hash / 12) % uint32(len(faqStyles))

	return ContentVariation{
		OpeningStyle: openingStyles[openingIdx],
		ReviewTone:   reviewTones[reviewIdx],
		FAQStyle:     faqStyles[faqIdx],
	}
}

// TitleStyleIndices เก็บ indices สำหรับ title styles
type TitleStyleIndices struct {
	AggressiveIdx      int
	BalancedIdx        int
	IntentSetIdx       int
	IntentOrderSwapped bool
}

// GetTitleStyleIndices คืน style indices สำหรับ Chunk 6 (deterministic)
func GetTitleStyleIndices(videoID string) TitleStyleIndices {
	h := HashString(videoID)

	return TitleStyleIndices{
		AggressiveIdx:      int(h % 4),
		BalancedIdx:        int((h / 10) % 4),
		IntentSetIdx:       int((h / 100) % 6),
		IntentOrderSwapped: (h/1000)%2 == 1,
	}
}

// ToVariationMeta converts ContentVariation to models.VariationMeta
func ToVariationMeta(cv ContentVariation, tsi TitleStyleIndices) *models.VariationMeta {
	return &models.VariationMeta{
		OpeningStyle:       cv.OpeningStyle,
		ReviewTone:         cv.ReviewTone,
		FAQStyle:           cv.FAQStyle,
		TitleAggressiveIdx: tsi.AggressiveIdx,
		TitleBalancedIdx:   tsi.BalancedIdx,
		IntentSetIdx:       tsi.IntentSetIdx,
		IntentOrderSwapped: tsi.IntentOrderSwapped,
	}
}

// ============================================================================
// Rating Adjustment (Realistic Distribution)
// ============================================================================

// AdjustRating ปรับ rating ให้ realistic
func AdjustRating(originalRating float64, chunk4 *Chunk4Output, videoID string) *models.RatingAdjustment {
	adjusted := originalRating
	reason := "no_adjustment"

	strengthCount := len(chunk4.Strengths)
	weaknessCount := len(chunk4.Weaknesses)

	h := HashString(videoID)
	deterministicRand := float64(h%100) / 100.0
	deterministicRand2 := float64((h/100)%100) / 100.0

	// Rule 1: High rating but has multiple weaknesses
	if originalRating > 4.5 && weaknessCount > 1 {
		adjusted = 4.0 + deterministicRand*0.4
		reason = "high_rating_with_weaknesses"
	}

	// Rule 2: More weaknesses than strengths → lower rating
	if weaknessCount >= strengthCount && originalRating > 3.5 {
		adjusted = 2.5 + deterministicRand*1.0
		reason = "weaknesses_exceed_strengths"
	}

	// Rule 3: All ratings above 4.0 → deterministically adjust some down
	if reason == "no_adjustment" && originalRating > 4.0 {
		if deterministicRand2 < 0.4 {
			adjusted = 3.0 + deterministicRand*0.7
			reason = "distribution_balance"
		}
	}

	// Ensure rating is within valid range
	if adjusted < 1.0 {
		adjusted = 1.0
	} else if adjusted > 5.0 {
		adjusted = 5.0
	}

	// Round to 1 decimal place
	adjusted = float64(int(adjusted*10)) / 10

	return &models.RatingAdjustment{
		OriginalRating: originalRating,
		AdjustedRating: adjusted,
		AdjustReason:   reason,
	}
}

// ============================================================================
// Utilities
// ============================================================================

// HashString สร้าง hash จาก string (deterministic)
func HashString(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// TruncateSRT limits SRT content to maxLen characters
func TruncateSRT(srt string, maxLen int) string {
	if len(srt) <= maxLen {
		return srt
	}
	return srt[:maxLen] + "..."
}

// TruncateToWords limits text to approximately maxWords
func TruncateToWords(text string, maxWords int) string {
	words := 0
	lastSpace := 0
	for i, r := range text {
		if r == ' ' || r == '\n' {
			words++
			lastSpace = i
			if words >= maxWords {
				return text[:lastSpace] + "..."
			}
		}
	}
	return text
}
