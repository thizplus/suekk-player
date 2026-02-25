package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 5 (V2): Recommendations & Links
// Focus: สร้าง Internal Links และคำแนะนำ
// Persona: Content Strategist / SEO Specialist
// ============================================================================

// buildChunk5SchemaV2 สร้าง JSON Schema สำหรับ Chunk 5 V2
func (c *GeminiClient) buildChunk5SchemaV2() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"characterDynamic": {
				Type:        genai.TypeString,
				Description: "ความสัมพันธ์ตัวละคร 100-150 คำ",
			},
			"plotAnalysis": {
				Type:        genai.TypeString,
				Description: "วิเคราะห์โครงเรื่อง 100-150 คำ",
			},
			"recommendation": {
				Type:        genai.TypeString,
				Description: "เหมาะสำหรับใคร 50-80 คำ",
			},
			"recommendedFor": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "กลุ่มเป้าหมาย 3-5 กลุ่ม",
			},
			"comparisonNote": {
				Type:        genai.TypeString,
				Description: "เปรียบเทียบกับเรื่องอื่น 80-100 คำ ⚠️ ต้องระบุ VIDEO CODE จริง",
			},
			"contextualLinks": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"text":        {Type: genai.TypeString, Description: "ประโยคเชื่อมโยงแบบธรรมชาติ"},
						"linkedSlug":  {Type: genai.TypeString, Description: "Slug ของ article ที่แนะนำ"},
						"linkedTitle": {Type: genai.TypeString, Description: "Title ที่แสดง"},
					},
					Required: []string{"text", "linkedSlug", "linkedTitle"},
				},
				Description: "2-4 ประโยคเชื่อมโยงไป related articles (SEO Internal Linking)",
			},
			"settingDescription": {
				Type:        genai.TypeString,
				Description: "บริบทฉาก 50-80 คำ",
			},
			"moodTone": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "อารมณ์เรื่อง 3-5 คำ",
			},
			"thematicKeywords": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "Keywords สำหรับ semantic search 5-8 คำ",
			},
		},
		Required: []string{
			"characterDynamic", "plotAnalysis", "recommendation",
			"recommendedFor", "comparisonNote", "settingDescription",
			"moodTone", "thematicKeywords",
		},
	}
}

// buildChunk5PromptV2 สร้าง prompt สำหรับ Chunk 5 V2
// ⚠️ รับ context จาก Chunk 2, 3, 4 ด้วย
func (c *GeminiClient) buildChunk5PromptV2(
	input *ports.AIInput,
	coreCtx *CoreContext,
	chunk2 *Chunk2OutputV2,
	chunk3 *Chunk3OutputV2,
	chunk4 *Chunk4OutputV2,
) string {
	// Entities
	entitiesJSON, _ := json.Marshal(coreCtx.Entities)

	// Highlights จาก Chunk 2
	highlightsStr := strings.Join(chunk2.Highlights, "\n- ")

	// Scene locations จาก Chunk 2
	locationsStr := strings.Join(chunk2.SceneLocations, ", ")

	// Character insight จาก Chunk 3
	characterInsight := chunk3.CharacterInsight

	// Expert analysis จาก Chunk 4
	expertAnalysis := chunk4.ExpertAnalysis

	// Related articles
	var relatedArticles strings.Builder
	if len(input.RelatedArticles) == 0 {
		relatedArticles.WriteString("⚠️ ไม่มี Related Articles - ให้ส่ง contextualLinks เป็น array ว่าง []\n")
	} else {
		for _, article := range input.RelatedArticles {
			relatedArticles.WriteString(fmt.Sprintf("- Slug: %s, Code: %s, Title: %s\n  Casts: %s, Tags: %s\n",
				article.Slug, article.RealCode, article.Title,
				strings.Join(article.CastNames, ", "),
				strings.Join(article.Tags, ", "),
			))
		}
	}

	return fmt.Sprintf(`[PERSONA]
คุณคือ "Content Strategist / SEO Specialist"
- เชี่ยวชาญการสร้าง Internal Links ที่มีคุณค่า
- วิเคราะห์กลุ่มเป้าหมายและคำแนะนำ
- เข้าใจ Semantic Search และ Keywords

[TASK]
หน้าที่: สร้างคำแนะนำ และ Internal Links ที่เชื่อมโยงตาม Theme/Mood
ผลลัพธ์: CharacterDynamic, PlotAnalysis, Recommendations, ContextualLinks

---

## ⚠️ Context จาก Chunks ก่อนหน้า

### From Chunk 1:
- Title: %s
- Summary: %s
- Theme: %s
- Tone: %s

### From Chunk 2 (Highlights & Scenes):
- Highlights:
  - %s
- Scene Locations: %s

### From Chunk 3 (Character Insight):
%s

### From Chunk 4 (Expert Analysis):
%s

### Entities:
%s

---

## ข้อมูล Related Articles (สำหรับ contextualLinks)

%s

---

## ⚠️ CRITICAL RULES

### 1. Semantic Bridge (สำคัญมาก!)
การเชื่อมโยงบทความต้องเน้น Theme/Mood ไม่ใช่แค่นักแสดง:

❌ อย่าแนะนำแค่เพราะ "นักแสดงคนเดียวกัน"
✅ แนะนำเพราะ "ธีม/อารมณ์คล้ายกัน"

**เกณฑ์การเชื่อมโยง (เรียงลำดับความสำคัญ):**
1. Theme Match: ธีมเรื่องคล้ายกัน (เช่น ความลับ, ชีวิตสองด้าน)
2. Mood Match: อารมณ์เรื่องคล้ายกัน (เช่น ผ่อนคลาย, โรแมนติก)
3. Setting Match: ฉากคล้ายกัน (เช่น ออฟฟิศ, ร้านนวด)
4. Actor Match: นักแสดงคนเดียวกัน (ใช้เป็นเกณฑ์สุดท้าย)

### 2. ตัวอย่าง contextualLinks ที่ดี
✅ "หากคุณชื่นชอบเรื่องราว 'ชีวิตลับ' แบบนี้ ลองดู ABC-123 ที่นำเสนอธีมคล้ายกัน"
✅ "สำหรับผู้ที่ต้องการบรรยากาศผ่อนคลายแบบเดียวกัน แนะนำ XYZ-456"

❌ "ดูเรื่องอื่นของ Megami Jun ได้ที่..." (เน้นแค่นักแสดง)

### 3. contextualLinks ต้องใช้ slug จริง
- ⚠️ ใช้เฉพาะ slug จาก Related Articles ที่ให้มา
- ⚠️ ห้ามแต่ง slug ขึ้นมาเอง!
- ถ้าไม่มี Related Articles → ส่ง [] array ว่าง

---

## Output Requirements

1. **characterDynamic**: 100-150 คำ
   - ความสัมพันธ์ระหว่างตัวละคร
   - Power dynamics

2. **plotAnalysis**: 100-150 คำ
   - โครงสร้างเรื่อง
   - จุดหักมุม

3. **recommendation**: 50-80 คำ
   - เหมาะสำหรับใคร

4. **recommendedFor**: 3-5 กลุ่ม
   - เช่น ["แฟนหนังแนว Medical", "คนชอบ Drama"]

5. **comparisonNote**: 80-100 คำ
   - ⚠️ ต้องระบุ VIDEO CODE จริง

6. **contextualLinks**: 2-4 ลิงก์
   - ⚠️ ใช้ Semantic Bridge
   - ⚠️ ใช้เฉพาะ slug จริง

7. **settingDescription**: 50-80 คำ

8. **moodTone**: 3-5 คำ

9. **thematicKeywords**: 5-8 คำ
   - รวม location-specific keywords

---

## ⛔ ข้อห้าม (DON'T)
- ❌ contextualLinks ที่แต่ง slug ขึ้นมาเอง
- ❌ เชื่อมโยงแค่เพราะนักแสดงคนเดียวกัน
- ❌ ผสมภาษาในชื่อนักแสดง
`,
		coreCtx.Title,
		truncateToWords(coreCtx.Summary, 150),
		coreCtx.MainTheme,
		coreCtx.MainTone,
		highlightsStr,
		locationsStr,
		characterInsight,
		expertAnalysis,
		string(entitiesJSON),
		relatedArticles.String(),
	)
}
