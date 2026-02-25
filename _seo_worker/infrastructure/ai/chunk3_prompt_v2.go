package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 3 (V2): Expertise (Linguistic Analysis)
// Focus: วิเคราะห์บทสนทนาและภาษา
// Persona: นักภาษาศาสตร์ / นักวิจารณ์ภาพยนตร์
// ============================================================================

// buildChunk3SchemaV2 สร้าง JSON Schema สำหรับ Chunk 3 V2
func (c *GeminiClient) buildChunk3SchemaV2() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"dialogueAnalysis": {
				Type:        genai.TypeString,
				Description: "วิเคราะห์บทสนทนา: สรรพนาม หางเสียง อารมณ์ที่เปลี่ยนไป (100-150 คำ)",
			},
			"characterInsight": {
				Type:        genai.TypeString,
				Description: "วิเคราะห์บุคลิกตัวละครจากวิธีพูด (100-150 คำ)",
			},
			"topQuotes": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"text":      {Type: genai.TypeString, Description: "ประโยคภาษาไทย (ไม่ใช่ประโยคลามก)"},
						"timestamp": {Type: genai.TypeInteger, Description: "เวลา (วินาที) ⚠️ ต้องอยู่ภายใน 600 วินาทีแรก"},
						"emotion":   {Type: genai.TypeString, Description: "อารมณ์"},
						"context":   {Type: genai.TypeString, Description: "บริบท"},
					},
					Required: []string{"text", "timestamp", "emotion", "context"},
				},
				Description: "4-5 ประโยคเด็ดจากซับ ⚠️ เลือกเฉพาะ 10 นาทีแรก",
			},
			"languageNotes": {
				Type:        genai.TypeString,
				Description: "หมายเหตุเกี่ยวกับภาษา เช่น การใช้ภาษาสุภาพ/เป็นกันเอง (50-80 คำ)",
			},
			"actorPerformanceTrend": {
				Type:        genai.TypeString,
				Description: "แนวโน้มการแสดง เปรียบเทียบกับผลงานก่อนหน้า (80-100 คำ)",
			},
		},
		Required: []string{
			"dialogueAnalysis", "characterInsight", "topQuotes",
			"languageNotes", "actorPerformanceTrend",
		},
	}
}

// buildChunk3PromptV2 สร้าง prompt สำหรับ Chunk 3 V2
func (c *GeminiClient) buildChunk3PromptV2(input *ports.AIInput, coreCtx *CoreContext) string {
	// สร้าง cast info
	var castsInfo strings.Builder
	for _, cast := range input.Casts {
		castsInfo.WriteString(fmt.Sprintf("- %s\n", cast.Name))
	}

	// Previous works
	var prevWorks strings.Builder
	for _, work := range input.PreviousWorks {
		prevWorks.WriteString(fmt.Sprintf("- %s (%s)\n", work.Title, work.VideoCode))
	}
	if prevWorks.Len() == 0 {
		prevWorks.WriteString("(ไม่มีข้อมูลผลงานก่อนหน้า)")
	}

	// Entities
	entitiesJSON, _ := json.Marshal(coreCtx.Entities)

	return fmt.Sprintf(`[PERSONA]
คุณคือ "นักภาษาศาสตร์ และ นักวิจารณ์ภาพยนตร์มืออาชีพ"
- เชี่ยวชาญการวิเคราะห์ภาษาและการสื่อสาร
- สังเกตรูปแบบการพูด หางเสียง สรรพนาม
- วิเคราะห์บุคลิกตัวละครจากวิธีสื่อสาร

[TASK]
หน้าที่: วิเคราะห์บทสนทนาและภาษาอย่างเชี่ยวชาญ
ผลลัพธ์: DialogueAnalysis, CharacterInsight, TopQuotes, LanguageNotes

---

## ⚠️ Context จาก Chunk 1

### Title:
%s

### Summary:
%s

### Theme/Tone:
- Theme: %s
- Tone: %s

### Entities (ใช้ชื่อตามนี้เท่านั้น):
%s

---

## ข้อมูล Input

### SRT Transcript (วิเคราะห์บทสนทนา):
%s

### Casts:
%s

### Previous Works (สำหรับเปรียบเทียบการแสดง):
%s

---

## ⚠️ CRITICAL RULES

### 1. topQuotes ต้องอยู่ใน 600 วินาทีแรก
- ⚠️ ดึง timestamp จาก SRT
- ⚠️ เลือกเฉพาะประโยคที่ timestamp ≤ 600
- ⚠️ ไม่ใช่ประโยคลามก

### 2. ใช้ศัพท์ทางภาษาศาสตร์
- ✅ สรรพนาม, หางเสียง, Keigo, ภาษาสุภาพ/เป็นกันเอง
- ✅ พัฒนาการอารมณ์จากบทสนทนา

### 3. Entity-Consistency
- ✅ ใช้ชื่อนักแสดงตาม entities.actors เท่านั้น
- ✅ ครั้งแรก: ใช้ fullName, ครั้งถัดไป: ใช้ firstName หรือ "เธอ"

---

## Output Requirements

1. **dialogueAnalysis**: วิเคราะห์บทสนทนา 100-150 คำ
   - รูปแบบสรรพนาม
   - หางเสียงที่ใช้ (ค่ะ/คะ/จ๊ะ)
   - อารมณ์ที่เปลี่ยนแปลง

2. **characterInsight**: วิเคราะห์บุคลิก 100-150 คำ
   - จากวิธีพูด
   - จากการตอบสนอง
   - จากภาษากาย (ถ้ามีใน context)

3. **topQuotes**: 4-5 ประโยคเด็ด
   - text: ประโยคภาษาไทย
   - timestamp: ≤ 600 วินาที
   - emotion: อารมณ์
   - context: บริบท

4. **languageNotes**: หมายเหตุภาษา 50-80 คำ

5. **actorPerformanceTrend**: แนวโน้มการแสดง 80-100 คำ
   - เปรียบเทียบกับผลงานก่อนหน้า (ถ้ามี)

---

## ⛔ ข้อห้าม (DON'T)
- ❌ topQuotes timestamp > 600 วินาที (ถูก filter!)
- ❌ ประโยคลามกใน topQuotes
- ❌ ผสมภาษาในชื่อนักแสดง
- ❌ วิเคราะห์แบบผิวเผิน
`,
		coreCtx.Title,
		coreCtx.Summary,
		coreCtx.MainTheme,
		coreCtx.MainTone,
		string(entitiesJSON),
		truncateSRT(input.SRTContent, 3000),
		castsInfo.String(),
		prevWorks.String(),
	)
}
