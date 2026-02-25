package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/models"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 7 (V2): Deep Analysis
// Focus: วิเคราะห์เชิงลึก (Cinematography, Character Journey)
// Persona: Film Critic / Cultural Analyst
// ============================================================================

// buildChunk7SchemaV2 สร้าง JSON Schema สำหรับ Chunk 7 V2
func (c *GeminiClient) buildChunk7SchemaV2() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			// Section 1: Cinematography & Atmosphere
			"cinematographyAnalysis": {
				Type:        genai.TypeString,
				Description: "วิเคราะห์งานภาพ 250-350 คำ แบ่ง 3-4 ย่อหน้า (คั่นด้วย \\n\\n)",
			},
			"visualStyle": {
				Type:        genai.TypeString,
				Description: "สไตล์ภาพโดยรวม 50-80 คำ",
			},
			"atmosphereNotes": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "จุดสังเกตบรรยากาศ 3-5 จุด",
			},

			// Section 2: Character Emotional Journey
			"characterJourney": {
				Type:        genai.TypeString,
				Description: "พัฒนาการทางอารมณ์ 300-400 คำ แบ่ง 3-5 ย่อหน้า (คั่นด้วย \\n\\n)",
			},
			"emotionalArc": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"phase":       {Type: genai.TypeString, Description: "ช่วงเวลา"},
						"emotion":     {Type: genai.TypeString, Description: "อารมณ์หลัก"},
						"description": {Type: genai.TypeString, Description: "บรรยาย 30-50 คำ"},
					},
					Required: []string{"phase", "emotion", "description"},
				},
				Description: "3-4 จุดสำคัญของ emotional arc",
			},

			// Section 3: Educational Context
			"thematicExplanation": {
				Type:        genai.TypeString,
				Description: "อธิบายธีม 200-300 คำ แบ่ง 2-3 ย่อหน้า (คั่นด้วย \\n\\n)",
			},
			"culturalContext": {
				Type:        genai.TypeString,
				Description: "บริบทวัฒนธรรมญี่ปุ่น 100-150 คำ",
			},
			"genreInsights": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "ข้อมูลเชิงลึกแนวเรื่อง 3-5 ข้อ",
			},

			// Section 4: Comparative Analysis
			"studioComparison": {
				Type:        genai.TypeString,
				Description: "เปรียบเทียบกับค่าย 150-200 คำ",
			},
			"actorEvolution": {
				Type:        genai.TypeString,
				Description: "พัฒนาการนักแสดง 150-200 คำ",
			},
			"genreRanking": {
				Type:        genai.TypeString,
				Description: "ตำแหน่งในแนว 50-80 คำ",
			},

			// Section 5: Viewing Experience
			"viewingTips": {
				Type:        genai.TypeString,
				Description: "คำแนะนำการรับชม 150-200 คำ",
			},
			"bestMoments": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "ช่วงเวลาดีที่สุด 3-5 จุด ⚠️ ต้องมีคำอธิบาย ไม่ใช่แค่ชื่อนักแสดง!",
			},
			"audienceMatch": {
				Type:        genai.TypeString,
				Description: "เหมาะกับใคร 80-100 คำ",
			},
			"replayValue": {
				Type:        genai.TypeString,
				Description: "ความคุ้มค่าดูซ้ำ 50-80 คำ",
			},
		},
		Required: []string{
			// Section 1
			"cinematographyAnalysis", "visualStyle", "atmosphereNotes",
			// Section 2
			"characterJourney", "emotionalArc",
			// Section 3
			"thematicExplanation", "culturalContext", "genreInsights",
			// Section 4
			"studioComparison", "actorEvolution", "genreRanking",
			// Section 5
			"viewingTips", "bestMoments", "audienceMatch", "replayValue",
		},
	}
}

// buildChunk7PromptV2 สร้าง prompt สำหรับ Chunk 7 V2
func (c *GeminiClient) buildChunk7PromptV2(input *ports.AIInput, extCtx *ExtendedContext) string {
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

	// Top highlights
	highlightsStr := strings.Join(extCtx.TopHighlights, "\n- ")

	// Entities
	entitiesJSON, _ := json.Marshal(extCtx.Entities)

	// Duration
	durationStr := formatDurationThai(input.VideoMetadata.Duration)

	return fmt.Sprintf(`[PERSONA]
คุณคือ "Film Critic / Cultural Analyst ระดับพรีเมียม"
- เชี่ยวชาญ Cinematography และ Visual Aesthetics
- วิเคราะห์ Character Arc และ Emotional Journey
- เข้าใจบริบทวัฒนธรรมญี่ปุ่น
- เขียนคำแนะนำการรับชมที่มีคุณค่า

[TASK]
หน้าที่: วิเคราะห์เชิงลึกเพื่อเพิ่ม Text สำหรับ SEO
ผลลัพธ์: Cinematography, CharacterJourney, ThematicExplanation, ViewingTips

---

## ⚠️ Extended Context จาก Chunks ก่อนหน้า

### Title:
%s

### Summary:
%s

### Top Highlights (วิเคราะห์ฉากเหล่านี้):
- %s

### Key Scenes:
%s

### Expert Summary:
%s

### Main Insight:
%s

### Entities:
%s

---

## ข้อมูล Video

- Code: %s
- Duration: %s
- Casts: %s
- Studio/Maker: %s
- Tags:
%s

---
%s
---

## ⚠️ CRITICAL RULES

### 1. เนื้อหายาวต้องแบ่งย่อหน้าด้วย [PARA]
- cinematographyAnalysis: 3-4 ย่อหน้า (คั่นด้วย [PARA])
- characterJourney: 3-5 ย่อหน้า (คั่นด้วย [PARA])
- thematicExplanation: 2-3 ย่อหน้า (คั่นด้วย [PARA])

### 2. bestMoments ห้ามขึ้นต้นด้วยชื่อ + ลดการใช้ชื่อซ้ำ!
- ❌ ห้าม: "Megami Jun แสดงให้เห็น..." (ขึ้นต้นด้วยชื่อ)
- ❌ ห้าม: "Megami Jun ใช้เทคนิค..." (ขึ้นต้นด้วยชื่อ)
- ✅ ต้อง: "ช่วงที่แสดงความมุ่งมั่นในการทำงาน โดดเด่นด้วยการแสดงออกที่เป็นธรรมชาติ"
- ✅ ต้อง: "ฉากที่หญิงสาวสร้างบรรยากาศผ่อนคลาย ทำให้ผู้ชมรู้สึกสบายใจไปด้วย"
- ✅ ต้อง: "การใช้ Close-up เน้นความอ่อนโยนในการดูแลลูกค้า"
- ⚠️ เน้น "อะไรเกิดขึ้น/เทคนิคอะไร" > "ใครทำ"
- ⚠️ ถ้าต้องอ้างถึงนักแสดง ให้วางชื่อไว้กลางหรือท้ายประโยค

### 2.1 ลดการใช้ชื่อซ้ำด้วย Pronoun/Role Substitution
- ⚠️ ใช้ชื่อเต็มไม่เกิน 2 ครั้งใน 5 bestMoments!
- ✅ แทนด้วยสรรพนาม: "เธอ", "หญิงสาว", "นางเอก"
- ✅ แทนด้วยบทบาท: "คนไข้สาว", "พนักงาน", "ตัวละครหลัก"
- ❌ ห้าม: "ช่วงที่ Zemba Mami..." ซ้ำทุกข้อ (Spammy!)
- ✅ ต้อง: "ช่วงที่เธอต้องเผชิญ...", "เมื่อหญิงสาวพยายาม..."

- แต่ละ bestMoment ต้องยาว 15-30 คำ

### 3. ใช้ศัพท์เทคนิค
- Cinematography: Lighting, Close-up, POV, Color Grading
- Psychology: Anxiety, Trust, Catharsis, Emotional Arc
- ⚠️ ศัพท์เทคนิคช่วย SEO!

### 4. Entity-Consistency
- ✅ ใช้ชื่อตาม entities.actors
- ✅ ครั้งแรก: fullName, ครั้งถัดไป: firstName หรือ "เธอ"

### 5. ห้ามใช้คำหยาบ
- ❌ ห้าม: "เซ็กส์", "ร่วมเพศ", "หลั่ง", "แตกใน"
- ✅ ใช้: "ความใกล้ชิด", "ช่วงเวลาส่วนตัว", "ฉากโรแมนติก"

---

## Output Requirements

### 🎬 Section 1: Cinematography (วิเคราะห์งานภาพ)
1. **cinematographyAnalysis**: 250-350 คำ, 3-4 ย่อหน้า (คั่นด้วย [PARA])
   - การจัดแสง (Lighting)
   - มุมกล้อง (Camera Angles)
   - สีโทน (Color Grading)
2. **visualStyle**: 50-80 คำ
3. **atmosphereNotes**: 3-5 จุด

### 🎭 Section 2: Character Journey (พัฒนาการตัวละคร)
4. **characterJourney**: 300-400 คำ, 3-5 ย่อหน้า (คั่นด้วย [PARA])
   - เริ่มต้น → พัฒนา → ไคลแมกซ์ → จบ
5. **emotionalArc**: 3-4 จุด

### 📚 Section 3: Educational (บริบทเชิงลึก)
6. **thematicExplanation**: 200-300 คำ, 2-3 ย่อหน้า (คั่นด้วย [PARA])
7. **culturalContext**: 100-150 คำ
8. **genreInsights**: 3-5 ข้อ

### ⚖️ Section 4: Comparative (การเปรียบเทียบ)
9. **studioComparison**: 150-200 คำ
10. **actorEvolution**: 150-200 คำ
11. **genreRanking**: 50-80 คำ

### 👁️ Section 5: Viewing (คำแนะนำ)
12. **viewingTips**: 150-200 คำ
13. **bestMoments**: 3-5 จุด (แต่ละจุด 15-30 คำ)
14. **audienceMatch**: 80-100 คำ
15. **replayValue**: 50-80 คำ

---

## ⛔ ข้อห้าม (DON'T)
- ❌ เนื้อหายาวไม่มี [PARA] คั่นย่อหน้า (REJECT!)
- ❌ bestMoments ที่เป็นแค่ชื่อนักแสดง
- ❌ Copy จาก Summary ตรงๆ
- ❌ ใช้คำหยาบ
- ❌ เขียนแบบ generic
- ❌ ผสมภาษาในชื่อนักแสดง
`,
		extCtx.Title,
		extCtx.Summary,
		highlightsStr,
		strings.Join(extCtx.KeyScenes, ", "),
		extCtx.ExpertSummary,
		extCtx.MainInsight,
		string(entitiesJSON),
		input.VideoMetadata.RealCode,
		durationStr,
		castNamesStr,
		getMakerNameV2(input.VideoMetadata.Maker),
		tagsStr.String(),
		GlobalConstraintsV2+GlobalConstraintsForArrays, // Global Rules
	)
}

// getMakerNameV2 safely extracts maker name
func getMakerNameV2(maker *models.MakerMetadata) string {
	if maker == nil {
		return "ไม่ระบุค่าย"
	}
	return maker.Name
}
