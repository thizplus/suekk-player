package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 6 (V2): Technical & FAQ
// Focus: ข้อมูลเทคนิคและ FAQ
// Persona: Technical Writer / Customer Support
// ============================================================================

// buildChunk6SchemaV2 สร้าง JSON Schema สำหรับ Chunk 6 V2
func (c *GeminiClient) buildChunk6SchemaV2() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			// Trustworthiness
			"translationMethod": {
				Type:        genai.TypeString,
				Description: "วิธีการแปล 30-50 คำ",
			},
			"translationNote": {
				Type:        genai.TypeString,
				Description: "หมายเหตุการแปล 30-50 คำ",
			},
			"subtitleQuality": {
				Type:        genai.TypeString,
				Description: "คุณภาพซับ 30-50 คำ",
			},
			"videoQuality": {
				Type:        genai.TypeString,
				Description: "คุณภาพวิดีโอ 20-30 คำ",
			},
			"audioQuality": {
				Type:        genai.TypeString,
				Description: "คุณภาพเสียง 20-30 คำ",
			},
			"technicalFaq": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"question": {Type: genai.TypeString},
						"answer":   {Type: genai.TypeString, Description: "คำตอบ 40-60 คำ"},
					},
					Required: []string{"question", "answer"},
				},
				Description: "FAQ เทคนิค 2-3 ข้อ",
			},

			// FAQ
			"faqItems": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"question": {Type: genai.TypeString, Description: "⚠️ ต้องเป็นประโยคคำถามที่สมบูรณ์ (มี อะไร/ไหม/ยังไง/ที่ไหน)"},
						"answer":   {Type: genai.TypeString, Description: "คำตอบ 50-80 คำ ⚠️ ห้ามใช้คำหยาบ"},
					},
					Required: []string{"question", "answer"},
				},
				Description: "FAQ ทั่วไป 5-8 ข้อ ⚠️ คำถามต้องสมบูรณ์ ไม่ใช่แค่ชื่อนักแสดง!",
			},

			// SEO
			"keywords": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "SEO keywords 5-10 คำ ⚠️ ห้ามคำว่า 'หนังโป๊', 'xxx', 'av'",
			},
			"longTailKeywords": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "Long-tail keywords 3-5 วลี",
			},
		},
		Required: []string{
			"translationMethod", "translationNote", "subtitleQuality",
			"videoQuality", "audioQuality", "technicalFaq",
			"faqItems", "keywords", "longTailKeywords",
		},
	}
}

// buildChunk6PromptV2 สร้าง prompt สำหรับ Chunk 6 V2
func (c *GeminiClient) buildChunk6PromptV2(input *ports.AIInput, extCtx *ExtendedContext) string {
	// Cast names
	castNames := make([]string, len(input.Casts))
	for i, cast := range input.Casts {
		castNames[i] = cast.Name
	}
	castNamesStr := strings.Join(castNames, ", ")

	// Tags
	var tagsStr strings.Builder
	for _, tag := range input.Tags {
		tagsStr.WriteString(fmt.Sprintf("- %s\n", tag.Name))
	}

	// Top highlights จาก Extended Context
	highlightsStr := strings.Join(extCtx.TopHighlights, "\n- ")

	// Entities
	entitiesJSON, _ := json.Marshal(extCtx.Entities)

	// Duration formatted
	durationStr := formatDurationThai(input.VideoMetadata.Duration)

	return fmt.Sprintf(`[PERSONA]
คุณคือ "Technical Writer / Customer Support Specialist"
- เชี่ยวชาญการเขียน FAQ ที่ตอบคำถามที่คนค้นหาจริง
- วิเคราะห์คุณภาพเทคนิคของวิดีโอและซับไตเติ้ล
- เขียน SEO keywords ที่มี search intent ชัดเจน

[TASK]
หน้าที่: สร้าง FAQ และข้อมูลเทคนิค
ผลลัพธ์: TranslationInfo, FAQ, Keywords

---

## ⚠️ Extended Context จาก Chunks ก่อนหน้า

### Title:
%s

### Summary (สรุป):
%s

### Top Highlights:
- %s

### Key Scenes:
%s

### Expert Summary:
%s

### Entities (ใช้ตรวจสอบชื่อ):
%s

---

## ข้อมูล Video

- Code: %s
- Duration: %s
- Casts: %s
- Tags:
%s

### ตัวอย่างจาก SRT:
%s

---

## ⚠️⚠️⚠️ CRITICAL RULES (สำคัญมาก!) ⚠️⚠️⚠️

### 1. FAQ คำถามต้องสมบูรณ์
- ✅ ต้องเป็นประโยคคำถามที่มี: อะไร, ไหม, ยังไง, ที่ไหน, เมื่อไหร่, ใคร, ทำไม
- ❌ ห้ามแค่ชื่อนักแสดง เช่น "Megami Jun?" (REJECT!)
- ❌ ห้ามแค่รหัสเรื่อง เช่น "%s?" (REJECT!)

### 2. FAQ ต้องหลากหลาย (ห้ามนำข้อมูลจากบทนำมาตอบอย่างเดียว)
ต้องมี FAQ หลายประเภท:
1. คำถามเกี่ยวกับเนื้อเรื่อง (Plot)
2. คำถามเกี่ยวกับนักแสดง (Cast)
3. คำถามเกี่ยวกับการรับชม (Where to watch)
4. คำถามเกี่ยวกับคุณภาพ (Subtitle/Video quality)
5. คำถามเกี่ยวกับกลุ่มเป้าหมาย (Who should watch)

### 3. FAQ Actor Verification
- ⚠️ ชื่อนักแสดงในคำถามต้องตรงกับ entities.actors
- ❌ ห้ามแต่งชื่อนักแสดงขึ้นมาเอง!
- ✅ ใช้เฉพาะชื่อจาก: %s

### 4. ตัวอย่าง FAQ ที่ดี
✅ "%s เกี่ยวกับอะไร?"
✅ "%s แสดงเป็นใครในเรื่องนี้?"
✅ "ดู %s ซับไทยได้ที่ไหน?"
✅ "เรื่องนี้เหมาะกับใคร?"
✅ "%s ซับไทยคุณภาพดีไหม?"

### 5. Keywords ห้ามคำหยาบ
- ❌ ห้าม: "หนังโป๊", "xxx", "av", "หนังเอ็กซ์"
- ✅ ใช้: "หนังญี่ปุ่น", "หนังรัก", "ซีรีส์ญี่ปุ่น"

---

## Output Requirements

### Technical Info
1. **translationMethod**: 30-50 คำ
2. **translationNote**: 30-50 คำ
3. **subtitleQuality**: 30-50 คำ
4. **videoQuality**: 20-30 คำ
5. **audioQuality**: 20-30 คำ
6. **technicalFaq**: 2-3 ข้อ (FAQ เทคนิค)

### FAQ
7. **faqItems**: 5-8 ข้อ
   - ⚠️ คำถามต้องสมบูรณ์
   - ⚠️ คำตอบ 50-80 คำ
   - ⚠️ ห้ามใช้คำหยาบ

### SEO
8. **keywords**: 5-10 คำ
9. **longTailKeywords**: 3-5 วลี

---

## ⛔ ข้อห้าม (DON'T)
- ❌ FAQ ที่คำถามแค่ชื่อนักแสดง/รหัสเรื่อง
- ❌ FAQ ที่คำถามไม่มี อะไร/ไหม/ยังไง
- ❌ FAQ ที่ใช้ชื่อนักแสดงที่ไม่อยู่ใน entities
- ❌ Keywords ที่มีคำหยาบ
- ❌ ผสมภาษาในชื่อนักแสดง
`,
		extCtx.Title,
		extCtx.Summary,
		highlightsStr,
		strings.Join(extCtx.KeyScenes, ", "),
		extCtx.ExpertSummary,
		string(entitiesJSON),
		input.VideoMetadata.RealCode,
		durationStr,
		castNamesStr,
		tagsStr.String(),
		truncateSRT(input.SRTContent, 500),
		input.VideoMetadata.RealCode,
		castNamesStr,
		input.VideoMetadata.RealCode,
		castNames[0], // ใช้ชื่อนักแสดงคนแรก
		input.VideoMetadata.RealCode,
		input.VideoMetadata.RealCode,
	)
}
