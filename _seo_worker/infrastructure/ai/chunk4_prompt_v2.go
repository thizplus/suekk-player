package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 4 (V2): Authority (Entity Bios)
// Focus: สร้างเนื้อหาที่แสดง Authority (Cast, Tags, Review)
// Persona: นักเขียนชีวประวัติ / Encyclopedia Writer
// ============================================================================

// buildChunk4SchemaV2 สร้าง JSON Schema สำหรับ Chunk 4 V2
func (c *GeminiClient) buildChunk4SchemaV2() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"detailedReview": {
				Type:        genai.TypeString,
				Description: "⚠️ รีวิวละเอียด 500-700 คำ แบ่ง 5-7 ย่อหน้า (คั่นด้วย \\n\\n) เน้นวิเคราะห์การแสดง เทคนิค",
			},
			"castBios": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"castId": {Type: genai.TypeString},
						"bio":    {Type: genai.TypeString, Description: "bio 80-120 คำ"},
					},
					Required: []string{"castId", "bio"},
				},
				Description: "ชีวประวัตินักแสดงแต่ละคน",
			},
			"tagDescriptions": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"id":          {Type: genai.TypeString},
						"name":        {Type: genai.TypeString},
						"description": {Type: genai.TypeString, Description: "คำอธิบาย 30-50 คำ ⚠️ ห้ามใช้คำว่า: หลั่ง, แตกใน, อวัยวะเพศ"},
					},
					Required: []string{"id", "name", "description"},
				},
				Description: "คำอธิบายภาษาไทยสำหรับแต่ละ tag",
			},
			"expertAnalysis": {
				Type:        genai.TypeString,
				Description: "บทวิเคราะห์ผู้เชี่ยวชาญ 150-200 คำ ⚠️ ห้ามว่าง!",
			},
		},
		Required: []string{"detailedReview", "castBios", "tagDescriptions", "expertAnalysis"},
	}
}

// buildChunk4PromptV2 สร้าง prompt สำหรับ Chunk 4 V2
func (c *GeminiClient) buildChunk4PromptV2(input *ports.AIInput, coreCtx *CoreContext) string {
	// สร้าง cast info
	var castsInfo strings.Builder
	for _, cast := range input.Casts {
		castsInfo.WriteString(fmt.Sprintf("- ID: %s, Name: %s\n", cast.ID, cast.Name))
	}

	// สร้าง tags info
	var tagsInfo strings.Builder
	for _, tag := range input.Tags {
		tagsInfo.WriteString(fmt.Sprintf("- ID: %s, Name: %s\n", tag.ID, tag.Name))
	}

	// Previous works
	var prevWorks strings.Builder
	for _, work := range input.PreviousWorks {
		prevWorks.WriteString(fmt.Sprintf("- %s (%s)\n", work.Title, work.VideoCode))
	}

	// Entities
	entitiesJSON, _ := json.Marshal(coreCtx.Entities)

	return fmt.Sprintf(`[PERSONA]
คุณคือ "นักเขียนชีวประวัติ และ Encyclopedia Writer"
- เชี่ยวชาญการเขียน bio ที่ให้ข้อมูลครบถ้วน
- สามารถเขียนคำอธิบายแบบสารานุกรม

[TASK]
หน้าที่: สร้างเนื้อหาที่แสดง Authority (DetailedReview, CastBios, TagDescriptions)
ผลลัพธ์: เนื้อหาที่มีความน่าเชื่อถือสูง

---

## ⚠️ Context จาก Chunk 1

### Title:
%s

### Summary (อ้างอิงเนื้อเรื่อง):
%s

### Theme/Tone:
- Theme: %s
- Tone: %s

### Entities:
%s

---

## ข้อมูล Input

### Casts (ต้อง generate bio สำหรับแต่ละคน):
%s

### Cast Previous Works:
%s

### Tags (ต้อง generate description สำหรับแต่ละ tag):
%s

---

## ⚠️⚠️⚠️ CRITICAL RULES (ต้องปฏิบัติตามอย่างเคร่งครัด!) ⚠️⚠️⚠️

### 1. detailedReview ต้องแบ่งย่อหน้าด้วย [PARA] + ไม่ spam ชื่อ
- ✅ ใช้ [PARA] คั่นระหว่างย่อหน้า (ไม่ใช่ \n)
- ✅ ต้องมี 5-7 ย่อหน้า คั่นด้วย [PARA] (500-700 คำ)
- ⛔ **ห้ามเขียน "[ชื่อนักแสดง] ทำ [กริยา]" ซ้ำกันเกิน 2 ครั้งต่อย่อหน้า!**
- ⛔ **ห้ามขึ้นต้นประโยคด้วยชื่อเต็มติดกันเกิน 2 ประโยค!**
- ⛔ **ห้ามใช้ชื่อเต็มเกิน 5 ครั้งต่อ 100 คำ!**

### 2. Negative Prompting (สิ่งที่ห้ามทำ)
❌ ตัวอย่างที่ห้าม (REJECT ทันที!):
"Megami Jun นวดลูกค้า Megami Jun ถามลูกค้า Megami Jun ยิ้ม Megami Jun..."

✅ ตัวอย่างที่ถูกต้อง:
"Megami Jun รับบทเป็นพนักงานนวด เธอแสดงออกถึงความเอาใจใส่ผ่านท่าทางที่อ่อนโยน การนวดของเธอสะท้อนถึงความเป็นมืออาชีพ..."

### 3. วิธีหลีกเลี่ยงการ spam ชื่อ:
- ครั้งแรก: ใช้ชื่อเต็ม (fullName)
- ครั้งที่ 2: ใช้ firstName
- ครั้งที่ 3+: ใช้ "เธอ", "เขา", "ตัวละคร", "นางเอก"
- เน้นการบรรยายความรู้สึก บรรยากาศ แทนการระบุชื่อ

### 4. tagDescriptions ห้ามใช้คำหยาบ
- ❌ ห้าม: "หลั่ง", "แตกใน", "อวัยวะเพศ", "ช่องคลอด"
- ✅ ใช้คำอ้อมค้อม: "ฉากจบแบบพิเศษ", "ความใกล้ชิดแบบโรแมนติก"

### 5. expertAnalysis ห้ามว่าง
- ต้องมี 150-200 คำ
- วิเคราะห์เทคนิคการแสดง, จุดเด่น, คุณภาพการผลิต

---

## Output Requirements

1. **detailedReview**: 500-700 คำ แบ่ง 5-7 ย่อหน้า
   - ⚠️ แต่ละย่อหน้าใช้ชื่อเต็มไม่เกิน 2 ครั้ง!
   - ⚠️ คั่นด้วย [PARA] ไม่ใช่ \n

2. **castBios**: bio แต่ละคน 80-120 คำ
   - castId: ใช้ ID ตามที่ให้มา
   - bio: เขียนแบบชีวประวัติ

3. **tagDescriptions**: คำอธิบาย 30-50 คำ/tag
   - ⚠️ ห้ามว่าง!
   - ⚠️ ใช้ภาษาสุภาพ

4. **expertAnalysis**: 150-200 คำ
   - ⚠️ ห้ามว่าง!
   - วิเคราะห์คุณภาพการผลิต, การแสดง

---

## ⛔ ข้อห้าม (DON'T)
- ❌ detailedReview ไม่มี [PARA] คั่นย่อหน้า (REJECT!)
- ❌ detailedReview spam ชื่อนักแสดง
- ❌ tagDescriptions ว่างเปล่า
- ❌ expertAnalysis ว่างเปล่า
- ❌ ผสมภาษาในชื่อนักแสดง
`,
		coreCtx.Title,
		coreCtx.Summary,
		coreCtx.MainTheme,
		coreCtx.MainTone,
		string(entitiesJSON),
		castsInfo.String(),
		prevWorks.String(),
		tagsInfo.String(),
	)
}
