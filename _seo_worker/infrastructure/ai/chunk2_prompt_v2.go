package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 2 (V2): Scene & Moments
// Focus: วิเคราะห์ฉากและช่วงเวลาสำคัญ
// Persona: ผู้กำกับภาพยนตร์ / Scene Analyst
// ============================================================================

// buildChunk2SchemaV2 สร้าง JSON Schema สำหรับ Chunk 2 V2
func (c *GeminiClient) buildChunk2SchemaV2() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"highlights": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "5-8 ฉากสำคัญ แต่ละจุด 15-30 คำ ⚠️ ต้องบอกว่าเกิดอะไรขึ้น ไม่ใช่แค่ชื่อนักแสดง",
			},
			"keyMoments": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"name":        {Type: genai.TypeString, Description: "ชื่อฉากสุภาพ ⚠️ ห้ามใช้คำหยาบ"},
						"startOffset": {Type: genai.TypeInteger, Description: "เวลาเริ่ม (วินาที)"},
						"endOffset":   {Type: genai.TypeInteger, Description: "เวลาจบ (วินาที) ต้อง > startOffset"},
					},
					Required: []string{"name", "startOffset", "endOffset"},
				},
				Description: "3-5 key moments กระจายทั่วทั้งวิดีโอ ใช้ชื่อสุภาพ",
			},
			"sceneLocations": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "3-5 สถานที่ในเรื่อง เช่น ['ห้องตรวจ', 'คลินิก']",
			},
			"galleryAlts": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "Alt text สำหรับรูป: [รหัส] - [ชื่อนักแสดง] - [บริบทกว้างๆ]",
			},
		},
		Required: []string{"highlights", "keyMoments", "sceneLocations", "galleryAlts"},
	}
}

// buildChunk2PromptV2 สร้าง prompt สำหรับ Chunk 2 V2
func (c *GeminiClient) buildChunk2PromptV2(input *ports.AIInput, coreCtx *CoreContext) string {
	// สร้าง cast names string
	castNames := make([]string, len(input.Casts))
	for i, cast := range input.Casts {
		castNames[i] = cast.Name
	}
	castNamesStr := strings.Join(castNames, ", ")

	// Serialize entities
	entitiesJSON, _ := json.Marshal(coreCtx.Entities)

	return fmt.Sprintf(`[PERSONA]
คุณคือ "ผู้กำกับภาพยนตร์ / Scene Analyst"
- เชี่ยวชาญการวิเคราะห์ฉากและ Timing
- สังเกตรายละเอียดและบรรยากาศ

[TASK]
หน้าที่: วิเคราะห์ฉากสำคัญและช่วงเวลาที่น่าสนใจ
ผลลัพธ์: Highlights, KeyMoments, SceneLocations, GalleryAlts

---

## ⚠️ Context จาก Chunk 1 (ห้ามแต่งเนื้อเรื่องใหม่!)

### Title:
%s

### Summary (อ้างอิงฉากจากที่นี่):
%s

### Theme/Tone:
- Theme: %s
- Tone: %s

### Entities (ใช้ชื่อตามนี้เท่านั้น):
%s

---

## ข้อมูล Input

### SRT Transcript (ใช้ดึง timestamps):
%s

### Video Metadata:
- Code: %s
- Duration: %d seconds
- Casts: %s
- Gallery Count: %d

---
%s
---

## ⚠️ CRITICAL RULES

### 1. highlights ห้ามขึ้นต้นด้วยชื่อ + ลดการใช้ชื่อซ้ำ!
- ❌ ห้าม: "Megami Jun, มุ่งมั่นที่จะ..." (ขึ้นต้นด้วยชื่อ)
- ❌ ห้าม: "Megami Jun ใช้เทคนิค..." (ขึ้นต้นด้วยชื่อ)
- ✅ ต้อง: "ฉากนวดที่สร้างบรรยากาศผ่อนคลายอย่างเป็นธรรมชาติ"
- ✅ ต้อง: "ช่วงเวลาที่หญิงสาวแสดงความเอาใจใส่ต่อลูกค้าอย่างจริงใจ"
- ✅ ต้อง: "การใช้เทคนิคนวดที่หลากหลายเพื่อคลายความเมื่อยล้า"
- ⚠️ เน้น "อะไรเกิดขึ้น" > "ใครทำ"
- ⚠️ ถ้าต้องอ้างถึงนักแสดง ให้วางชื่อไว้กลางหรือท้ายประโยค

### 1.1 ลดการใช้ชื่อซ้ำด้วย Pronoun/Role Substitution
- ⚠️ ใช้ชื่อเต็มไม่เกิน 3 ครั้งใน 8 highlights!
- ✅ แทนด้วยสรรพนาม: "เธอ", "หญิงสาว"
- ✅ แทนด้วยบทบาท: "คนไข้สาว", "พนักงานนวด", "นางเอก", "ตัวละครหลัก"
- ❌ ห้าม: "เมื่อ Zemba Mami พยายาม..." ซ้ำทุกข้อ
- ✅ ต้อง: "เมื่อหญิงสาวพยายาม...", "ช่วงที่เธอต้องเผชิญ..."

- แต่ละ highlight ต้องยาว 15-30 คำ

### 2. keyMoments ต้องกระจายทั่ววิดีโอ
- ⚠️ ดึง timestamp จาก SRT โดยตรง
- ⚠️ กระจาย 3-5 moments ตลอดวิดีโอ
- ⚠️ ใช้ชื่อสุภาพ ห้ามใช้คำหยาบ

### 3. Entity-Consistency
- ✅ ใช้ชื่อนักแสดงตาม entities.actors เท่านั้น
- ❌ ห้ามแต่งชื่อใหม่หรือผสมภาษา

---

## Output Requirements

1. **highlights**: 5-8 ฉากสำคัญ แต่ละจุด 15-30 คำ
2. **keyMoments**: 3-5 timestamps พร้อมชื่อสุภาพ
3. **sceneLocations**: 3-5 สถานที่ในเรื่อง
4. **galleryAlts**: %d alt texts รูปแบบ "[%s] - [ชื่อนักแสดง] - [บริบท]"

---

## ⛔ ข้อห้าม (DON'T)
- ❌ highlights ที่เป็นแค่ชื่อนักแสดง (ถูก filter ออก!)
- ❌ keyMoments ที่ timestamps < 30 วินาที duration
- ❌ ใช้คำหยาบใน keyMoments name
- ❌ ผสมภาษาในชื่อนักแสดง
`,
		coreCtx.Title,
		coreCtx.Summary,
		coreCtx.MainTheme,
		coreCtx.MainTone,
		string(entitiesJSON),
		truncateSRT(input.SRTContent, 2000), // ส่งแค่ส่วนหนึ่ง
		input.VideoMetadata.RealCode,
		input.VideoMetadata.Duration,
		castNamesStr,
		input.GalleryCount,
		GlobalConstraintsV2+GlobalConstraintsForArrays, // Global Rules
		input.GalleryCount,
		input.VideoMetadata.RealCode,
	)
}
