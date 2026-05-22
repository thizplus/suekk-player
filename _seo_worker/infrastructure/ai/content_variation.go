package ai

import (
	"hash/fnv"
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

// Opening Styles (4 แบบ)
var OpeningStyles = map[string]string{
	"A": `Style A - Direct: เริ่มด้วยข้อมูลตรงๆ
ตัวอย่าง: "เรื่องนี้เล่าถึง [ตัวละคร] ที่ [สถานการณ์]..."`,

	"B": `Style B - Question: เริ่มด้วยคำถาม
ตัวอย่าง: "เคยสงสัยไหมว่าถ้า [สถานการณ์]? เรื่องนี้ตอบคำถามนั้น..."`,

	"C": `Style C - Hook: เริ่มด้วย hook ดึงดูด
ตัวอย่าง: "ถ้าคุณชอบแนว [แนว] นี่คือเรื่องที่ต้องดู..."`,

	"D": `Style D - Context: เริ่มด้วยบริบท
ตัวอย่าง: "จากค่าย [ค่าย] ที่เน้นแนว [แนว] มาตลอด ผลงานนี้ก็ไม่ผิดหวัง..."`,
}

// Review Tones (3 แบบ)
var ReviewTones = map[string]string{
	"Analytical": `Analytical: วิเคราะห์เชิงลึก จุดเด่น จุดอ่อน ให้คะแนน
- ใช้ภาษาเป็นทางการ
- มีเหตุผลประกอบ
- เน้นข้อมูล`,

	"Conversational": `Conversational: เล่าให้ฟังแบบเพื่อนคุยกัน
- ใช้ภาษาไม่เป็นทางการ
- มีอารมณ์ร่วม
- เน้นความรู้สึก`,

	"Comparative": `Comparative: เทียบกับเรื่องอื่นในแนวเดียวกัน
- เปรียบเทียบกับผลงานอื่น
- หาจุดเด่นที่ต่าง
- ให้มุมมองกว้าง`,
}

// FAQ Styles (2 แบบ)
var FAQStyles = map[string]string{
	"Formal": `Formal: คำถามเป็นทางการ
ตัวอย่าง: "[CODE] เกี่ยวกับอะไร?"`,

	"Casual": `Casual: คำถามไม่เป็นทางการ
ตัวอย่าง: "เล่าให้ฟังหน่อยว่า [CODE] เป็นแนวไหน?"`,
}

// GetContentVariation คืน variation ตาม videoID (deterministic)
func GetContentVariation(videoID string) ContentVariation {
	hash := hashString(videoID)

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

// hashString สร้าง hash จาก string (deterministic)
func hashString(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// GetOpeningStylePrompt คืน prompt สำหรับ opening style
func GetOpeningStylePrompt(style string) string {
	if prompt, ok := OpeningStyles[style]; ok {
		return prompt
	}
	return OpeningStyles["A"]
}

// GetReviewTonePrompt คืน prompt สำหรับ review tone
func GetReviewTonePrompt(tone string) string {
	if prompt, ok := ReviewTones[tone]; ok {
		return prompt
	}
	return ReviewTones["Analytical"]
}

// GetFAQStylePrompt คืน prompt สำหรับ FAQ style
func GetFAQStylePrompt(style string) string {
	if prompt, ok := FAQStyles[style]; ok {
		return prompt
	}
	return FAQStyles["Formal"]
}

// TitleStyleIndices เก็บ indices สำหรับ title styles (for logging)
type TitleStyleIndices struct {
	AggressiveIdx      int
	BalancedIdx        int
	IntentSetIdx       int
	IntentOrderSwapped bool
}

// GetTitleStyleIndices คืน style indices สำหรับ Chunk 6 (deterministic)
func GetTitleStyleIndices(videoID string) TitleStyleIndices {
	h := hashString(videoID)

	return TitleStyleIndices{
		AggressiveIdx:      int(h % 4),       // 4 aggressive styles
		BalancedIdx:        int((h / 10) % 4), // 4 balanced styles
		IntentSetIdx:       int((h / 100) % 6), // 6 template sets
		IntentOrderSwapped: (h/1000)%2 == 1,
	}
}
