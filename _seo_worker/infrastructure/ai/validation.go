package ai

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"seo-worker/domain/models"
)

// ============================================================================
// Validation Rules for 7-Chunk Architecture
// ============================================================================

// ValidationError represents a validation failure
type ValidationError struct {
	Chunk   int
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("chunk %d: %s - %s", e.Chunk, e.Field, e.Message)
}

// ValidationWarning represents a soft issue (log only, don't reject)
type ValidationWarning struct {
	Chunk   int
	Field   string
	Message string
}

// ValidationResult contains errors (hard failures) and warnings (soft issues)
type ValidationResult struct {
	Errors   []ValidationError
	Warnings []ValidationWarning
}

func (r *ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

func (r *ValidationResult) IsValid() bool {
	return len(r.Errors) == 0
}

// ============================================================================
// Paragraph Formatting Rules
// ============================================================================

// ParagraphRules - minimum paragraphs required for each field
var ParagraphRules = map[string]int{
	"summary":                4, // อย่างน้อย 4 ย่อหน้า
	"detailedReview":         5, // อย่างน้อย 5 ย่อหน้า
	"characterJourney":       3, // อย่างน้อย 3 ย่อหน้า
	"cinematographyAnalysis": 3, // อย่างน้อย 3 ย่อหน้า
	"thematicExplanation":    2, // อย่างน้อย 2 ย่อหน้า
}

// validateParagraphStructure ตรวจสอบว่าเนื้อหายาวมีการแบ่งย่อหน้า
func validateParagraphStructure(text string, fieldName string, minParagraphs int) *ValidationError {
	paragraphs := strings.Split(text, "\n\n")

	// ต้องมีอย่างน้อย minParagraphs ย่อหน้า
	if len(paragraphs) < minParagraphs {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("ต้องมีอย่างน้อย %d ย่อหน้า (พบ %d)", minParagraphs, len(paragraphs)),
		}
	}

	// แต่ละย่อหน้าต้องไม่ยาวเกินไป (< 200 คำ)
	for i, p := range paragraphs {
		wordCount := len(strings.Fields(p))
		if wordCount > 200 {
			return &ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("ย่อหน้าที่ %d ยาวเกินไป (%d คำ, max 200)", i+1, wordCount),
			}
		}
	}

	return nil
}

// ensureParagraphBreaks เพิ่ม \n\n ถ้าเนื้อหายาวเกินไปไม่มีการแบ่ง
func ensureParagraphBreaks(text string, targetParagraphs int) string {
	// ถ้ามี \n\n อยู่แล้ว ไม่ต้องทำอะไร
	if strings.Contains(text, "\n\n") {
		return text
	}

	// แบ่งตามประโยค (. หรือ 。)
	sentences := splitSentences(text)
	if len(sentences) < targetParagraphs {
		return text
	}

	// รวมประโยคเป็นย่อหน้า
	sentencesPerParagraph := len(sentences) / targetParagraphs
	var paragraphs []string
	var currentParagraph []string

	for i, sentence := range sentences {
		currentParagraph = append(currentParagraph, sentence)

		if len(currentParagraph) >= sentencesPerParagraph && i < len(sentences)-1 {
			paragraphs = append(paragraphs, strings.Join(currentParagraph, " "))
			currentParagraph = nil
		}
	}

	// เพิ่มย่อหน้าสุดท้าย
	if len(currentParagraph) > 0 {
		paragraphs = append(paragraphs, strings.Join(currentParagraph, " "))
	}

	return strings.Join(paragraphs, "\n\n")
}

// splitSentences แยกประโยค
func splitSentences(text string) []string {
	// Simple sentence splitter for Thai/Japanese/English
	re := regexp.MustCompile(`[.。!?！？]+\s*`)
	parts := re.Split(text, -1)

	var sentences []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) > 0 {
			sentences = append(sentences, p)
		}
	}
	return sentences
}

// ============================================================================
// FAQ Validation
// ============================================================================

// questionWords - คำที่ควรมีในคำถาม FAQ
var questionWords = []string{
	"อะไร", "ไหม", "ยังไง", "ที่ไหน", "เมื่อไหร่", "ใคร", "ทำไม",
	"หรือไม่", "หรือเปล่า", "แบบไหน", "เป็นอย่างไร", "ได้ไหม",
}

// validateFAQQuestion ตรวจสอบว่า FAQ question สมบูรณ์
func validateFAQQuestion(question string, casts []models.CastMetadata) *ValidationError {
	trimmed := strings.TrimSpace(question)

	// ต้องมีความยาวอย่างน้อย 15 ตัวอักษร
	if len([]rune(trimmed)) < 15 {
		return &ValidationError{
			Field:   "faqItems[].question",
			Message: fmt.Sprintf("คำถามสั้นเกินไป (%d ตัวอักษร, min 15)", len([]rune(trimmed))),
		}
	}

	// ต้องมีคำถาม (question words)
	hasQuestionWord := false
	for _, qw := range questionWords {
		if strings.Contains(trimmed, qw) {
			hasQuestionWord = true
			break
		}
	}

	// ถ้าไม่มี question word และลงท้ายด้วย ? ก็ยอมรับ
	if !hasQuestionWord && !strings.HasSuffix(trimmed, "?") {
		return &ValidationError{
			Field:   "faqItems[].question",
			Message: "คำถามไม่สมบูรณ์ (ต้องมี อะไร/ไหม/ยังไง หรือลงท้ายด้วย ?)",
		}
	}

	// ไม่ควรเป็นแค่ชื่อนักแสดง
	for _, cast := range casts {
		if strings.ToLower(trimmed) == strings.ToLower(cast.Name) ||
			strings.ToLower(trimmed) == strings.ToLower(cast.Name)+"?" {
			return &ValidationError{
				Field:   "faqItems[].question",
				Message: fmt.Sprintf("คำถามเป็นแค่ชื่อนักแสดง '%s'", cast.Name),
			}
		}
	}

	return nil
}

// validateFAQActors ตรวจสอบว่าชื่อนักแสดงใน FAQ ตรงกับ entities
func validateFAQActors(faqs []models.FAQItem, entities *EntityList) []ValidationWarning {
	var warnings []ValidationWarning

	// สร้าง set ของชื่อที่ถูกต้อง
	validNames := make(map[string]bool)
	for _, actor := range entities.Actors {
		validNames[strings.ToLower(actor.FullName)] = true
		validNames[strings.ToLower(actor.FirstName)] = true
	}

	for i, faq := range faqs {
		// ดึงชื่อที่ดูเหมือนชื่อนักแสดง (Capitalized words)
		possibleNames := extractPossibleNames(faq.Question)

		for _, name := range possibleNames {
			// ถ้าดูเหมือนชื่อคน แต่ไม่อยู่ใน entities
			if looksLikeActorName(name) && !validNames[strings.ToLower(name)] {
				warnings = append(warnings, ValidationWarning{
					Field:   fmt.Sprintf("faqItems[%d].question", i),
					Message: fmt.Sprintf("พบชื่อ '%s' ที่ไม่ได้อยู่ใน cast list", name),
				})
			}
		}
	}

	return warnings
}

// extractPossibleNames ดึงคำที่อาจเป็นชื่อนักแสดง
func extractPossibleNames(text string) []string {
	var names []string

	// Pattern: 2+ คำที่ขึ้นต้นด้วยตัวใหญ่ติดกัน
	words := strings.Fields(text)
	for i := 0; i < len(words)-1; i++ {
		// ถ้าทั้ง 2 คำขึ้นต้นด้วยตัวใหญ่
		if startsWithUpper(words[i]) && startsWithUpper(words[i+1]) {
			names = append(names, words[i]+" "+words[i+1])
		}
	}

	return names
}

// startsWithUpper ตรวจสอบว่าเริ่มต้นด้วยตัวใหญ่
func startsWithUpper(s string) bool {
	for _, r := range s {
		return unicode.IsUpper(r)
	}
	return false
}

// looksLikeActorName ตรวจสอบว่าเป็นชื่อนักแสดงหรือไม่
func looksLikeActorName(s string) bool {
	words := strings.Fields(s)
	if len(words) < 2 {
		return false
	}

	// Common words ที่ไม่ใช่ชื่อคน
	commonWords := map[string]bool{
		"SubTH": true, "Full": true, "HD": true,
		"FNS": true, "IPX": true, "DLDSS": true, "SONE": true,
	}

	for _, word := range words {
		if commonWords[word] {
			return false
		}
		// ต้องขึ้นต้นด้วยตัวใหญ่
		if len(word) > 0 && !unicode.IsUpper(rune(word[0])) {
			return false
		}
	}

	return true
}

// ============================================================================
// Highlights/BestMoments Validation
// ============================================================================

// validateHighlight ตรวจสอบว่า highlight มีเนื้อหา ไม่ใช่แค่ชื่อ
func validateHighlight(highlight string, casts []models.CastMetadata) *ValidationError {
	trimmed := strings.TrimSpace(highlight)

	// ต้องมีความยาวอย่างน้อย 15 ตัวอักษร
	if len([]rune(trimmed)) < 15 {
		return &ValidationError{
			Field:   "highlights[]",
			Message: fmt.Sprintf("highlight สั้นเกินไป (%d ตัวอักษร, min 15)", len([]rune(trimmed))),
		}
	}

	// ไม่ควรเป็นแค่ชื่อนักแสดง
	for _, cast := range casts {
		if strings.ToLower(trimmed) == strings.ToLower(cast.Name) {
			return &ValidationError{
				Field:   "highlights[]",
				Message: fmt.Sprintf("highlight เป็นแค่ชื่อนักแสดง '%s'", cast.Name),
			}
		}
	}

	return nil
}

// ============================================================================
// Name Spam Detection
// ============================================================================

// countNameOccurrences นับจำนวนครั้งที่ชื่อปรากฏใน text
func countNameOccurrences(text string, name string) int {
	return strings.Count(strings.ToLower(text), strings.ToLower(name))
}

// validateNameSpam ตรวจสอบว่ามีการ spam ชื่อมากเกินไปหรือไม่
func validateNameSpam(text string, casts []models.CastMetadata, maxPerHundredWords int) *ValidationWarning {
	wordCount := len(strings.Fields(text))
	if wordCount == 0 {
		return nil
	}

	for _, cast := range casts {
		count := countNameOccurrences(text, cast.Name)
		// คำนวณ per 100 words
		per100 := float64(count) / (float64(wordCount) / 100.0)

		if per100 > float64(maxPerHundredWords) {
			return &ValidationWarning{
				Field:   "text",
				Message: fmt.Sprintf("ชื่อ '%s' ปรากฏบ่อยเกินไป (%.1f ครั้ง/100 คำ, max %d)", cast.Name, per100, maxPerHundredWords),
			}
		}
	}

	return nil
}

// ============================================================================
// Chunk Validators
// ============================================================================

// ValidateChunk1V2 validates Chunk 1 output
func ValidateChunk1V2(chunk *Chunk1OutputV2) *ValidationResult {
	result := &ValidationResult{}

	// summary length
	summaryRunes := len([]rune(chunk.Summary))
	if summaryRunes < 800 {
		result.Errors = append(result.Errors, ValidationError{
			Chunk:   1,
			Field:   "summary",
			Message: fmt.Sprintf("summary สั้นเกินไป (%d ตัวอักษร, min 800)", summaryRunes),
		})
	}

	// summary paragraphs
	if minParagraphs, ok := ParagraphRules["summary"]; ok {
		if err := validateParagraphStructure(chunk.Summary, "summary", minParagraphs); err != nil {
			err.Chunk = 1
			result.Warnings = append(result.Warnings, ValidationWarning{
				Chunk:   1,
				Field:   err.Field,
				Message: err.Message,
			})
		}
	}

	// title length
	if len(chunk.Title) < 20 {
		result.Errors = append(result.Errors, ValidationError{
			Chunk:   1,
			Field:   "title",
			Message: fmt.Sprintf("title สั้นเกินไป (%d ตัวอักษร, min 20)", len(chunk.Title)),
		})
	}

	// mainTheme and mainTone
	if len(chunk.MainTheme) == 0 {
		result.Errors = append(result.Errors, ValidationError{
			Chunk:   1,
			Field:   "mainTheme",
			Message: "mainTheme ว่างเปล่า",
		})
	}

	if len(chunk.MainTone) == 0 {
		result.Errors = append(result.Errors, ValidationError{
			Chunk:   1,
			Field:   "mainTone",
			Message: "mainTone ว่างเปล่า",
		})
	}

	return result
}

// ValidateChunk2V2 validates Chunk 2 output
func ValidateChunk2V2(chunk *Chunk2OutputV2, casts []models.CastMetadata) *ValidationResult {
	result := &ValidationResult{}

	// highlights
	if len(chunk.Highlights) < 3 {
		result.Errors = append(result.Errors, ValidationError{
			Chunk:   2,
			Field:   "highlights",
			Message: fmt.Sprintf("highlights น้อยเกินไป (%d, min 3)", len(chunk.Highlights)),
		})
	}

	// validate each highlight
	for _, h := range chunk.Highlights {
		if err := validateHighlight(h, casts); err != nil {
			err.Chunk = 2
			result.Warnings = append(result.Warnings, ValidationWarning{
				Chunk:   2,
				Field:   err.Field,
				Message: err.Message,
			})
		}
	}

	return result
}

// ValidateChunk4V2 validates Chunk 4 output
func ValidateChunk4V2(chunk *Chunk4OutputV2, casts []models.CastMetadata) *ValidationResult {
	result := &ValidationResult{}

	// detailedReview length
	reviewRunes := len([]rune(chunk.DetailedReview))
	if reviewRunes < 1000 {
		result.Errors = append(result.Errors, ValidationError{
			Chunk:   4,
			Field:   "detailedReview",
			Message: fmt.Sprintf("detailedReview สั้นเกินไป (%d ตัวอักษร, min 1000)", reviewRunes),
		})
	}

	// detailedReview paragraphs
	if minParagraphs, ok := ParagraphRules["detailedReview"]; ok {
		if err := validateParagraphStructure(chunk.DetailedReview, "detailedReview", minParagraphs); err != nil {
			err.Chunk = 4
			result.Warnings = append(result.Warnings, ValidationWarning{
				Chunk:   4,
				Field:   err.Field,
				Message: err.Message,
			})
		}
	}

	// expertAnalysis
	if len([]rune(chunk.ExpertAnalysis)) < 100 {
		result.Errors = append(result.Errors, ValidationError{
			Chunk:   4,
			Field:   "expertAnalysis",
			Message: "expertAnalysis สั้นเกินไป (min 100 ตัวอักษร)",
		})
	}

	// name spam check
	if warning := validateNameSpam(chunk.DetailedReview, casts, 5); warning != nil {
		warning.Chunk = 4
		warning.Field = "detailedReview"
		result.Warnings = append(result.Warnings, *warning)
	}

	return result
}

// ValidateChunk6V2 validates Chunk 6 output
func ValidateChunk6V2(chunk *Chunk6OutputV2, casts []models.CastMetadata) *ValidationResult {
	result := &ValidationResult{}

	// FAQ items
	if len(chunk.FAQItems) < 5 {
		result.Errors = append(result.Errors, ValidationError{
			Chunk:   6,
			Field:   "faqItems",
			Message: fmt.Sprintf("faqItems น้อยเกินไป (%d, min 5)", len(chunk.FAQItems)),
		})
	}

	// validate each FAQ
	for _, faq := range chunk.FAQItems {
		if err := validateFAQQuestion(faq.Question, casts); err != nil {
			err.Chunk = 6
			result.Warnings = append(result.Warnings, ValidationWarning{
				Chunk:   6,
				Field:   err.Field,
				Message: err.Message,
			})
		}
	}

	return result
}

// ValidateChunk7V2 validates Chunk 7 output
func ValidateChunk7V2(chunk *Chunk7OutputV2, casts []models.CastMetadata) *ValidationResult {
	result := &ValidationResult{}

	// cinematographyAnalysis length
	cinemaRunes := len([]rune(chunk.CinematographyAnalysis))
	if cinemaRunes < 500 {
		result.Errors = append(result.Errors, ValidationError{
			Chunk:   7,
			Field:   "cinematographyAnalysis",
			Message: fmt.Sprintf("cinematographyAnalysis สั้นเกินไป (%d ตัวอักษร, min 500)", cinemaRunes),
		})
	}

	// cinematographyAnalysis paragraphs
	if minParagraphs, ok := ParagraphRules["cinematographyAnalysis"]; ok {
		if err := validateParagraphStructure(chunk.CinematographyAnalysis, "cinematographyAnalysis", minParagraphs); err != nil {
			err.Chunk = 7
			result.Warnings = append(result.Warnings, ValidationWarning{
				Chunk:   7,
				Field:   err.Field,
				Message: err.Message,
			})
		}
	}

	// characterJourney length
	journeyRunes := len([]rune(chunk.CharacterJourney))
	if journeyRunes < 600 {
		result.Errors = append(result.Errors, ValidationError{
			Chunk:   7,
			Field:   "characterJourney",
			Message: fmt.Sprintf("characterJourney สั้นเกินไป (%d ตัวอักษร, min 600)", journeyRunes),
		})
	}

	// characterJourney paragraphs
	if minParagraphs, ok := ParagraphRules["characterJourney"]; ok {
		if err := validateParagraphStructure(chunk.CharacterJourney, "characterJourney", minParagraphs); err != nil {
			err.Chunk = 7
			result.Warnings = append(result.Warnings, ValidationWarning{
				Chunk:   7,
				Field:   err.Field,
				Message: err.Message,
			})
		}
	}

	// bestMoments validation
	for _, moment := range chunk.BestMoments {
		if err := validateHighlight(moment, casts); err != nil {
			err.Chunk = 7
			err.Field = "bestMoments[]"
			result.Warnings = append(result.Warnings, ValidationWarning{
				Chunk:   7,
				Field:   err.Field,
				Message: err.Message,
			})
		}
	}

	return result
}
