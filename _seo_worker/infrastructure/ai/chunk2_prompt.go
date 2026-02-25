package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 2: E-E-A-T Analysis
// Focus: Expertise & Authoritativeness
// ⚠️ CRITICAL: ใช้ context จาก Chunk 1 - ห้าม re-summarize
// ============================================================================

// buildChunk2Schema สร้าง JSON Schema สำหรับ Chunk 2
func (c *GeminiClient) buildChunk2Schema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			// === [E] Expertise Section ===
			"dialogueAnalysis": {
				Type:        genai.TypeString,
				Description: "วิเคราะห์บทสนทนา สรรพนาม หางเสียง อารมณ์ที่เปลี่ยนไป (100-150 คำ)",
			},
			"characterInsight": {
				Type:        genai.TypeString,
				Description: "วิเคราะห์บุคลิกตัวละครผ่านคำพูดและการแสดง (100-150 คำ)",
			},
			"languageNotes": {
				Type:        genai.TypeString,
				Description: "หมายเหตุภาษา เช่น ใช้หางเสียงสุภาพ ค่ะ/ครับ (50 คำ)",
			},
			"actorPerformanceTrend": {
				Type:        genai.TypeString,
				Description: "เปรียบเทียบการแสดงกับผลงานก่อนหน้า (100 คำ)",
			},
			"comparisonNote": {
				Type:        genai.TypeString,
				Description: "เปรียบเทียบกับเรื่องอื่นในค่ายเดียวกัน ต้องระบุ VIDEO CODE จริง (50 คำ)",
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
				Description: "3-5 ประโยคเด็ดจากซับ ⚠️ เลือกเฉพาะ 10 นาทีแรก (600 วินาที) เน้นบทสนทนาที่น่าสนใจ",
			},
			"expertAnalysis": {
				Type:        genai.TypeString,
				Description: "บทวิเคราะห์ผู้เชี่ยวชาญ ห้ามว่าง (100 คำ)",
			},
			"detailedReview": {
				Type:        genai.TypeString,
				Description: "⚠️ บทวิเคราะห์ยาว 800-1000 คำ (3,000-4,000 ตัวอักษร) เน้นวิเคราะห์การแสดง เทคนิค และความเสียว เขียน 6-7 ย่อหน้า ห้ามสั้น!",
			},
			"castBios": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"castId": {Type: genai.TypeString},
						"bio":    {Type: genai.TypeString, Description: "bio 50-100 คำ"},
					},
					Required: []string{"castId", "bio"},
				},
				Description: "bio สำหรับแต่ละ cast",
			},
			"tagDescriptions": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"id":          {Type: genai.TypeString},
						"name":        {Type: genai.TypeString},
						"description": {Type: genai.TypeString, Description: "คำอธิบาย tag ภาษาไทย ⚠️ ห้ามใช้คำว่า: หลั่ง, แตกใน, อวัยวะเพศ, ช่องคลอด"},
					},
					Required: []string{"id", "name", "description"},
				},
				Description: "คำอธิบายภาษาไทยสำหรับแต่ละ tag ⚠️ ใช้ภาษาอ้อมค้อม/สุภาพ เช่น Creampie='ฉากจบแบบโรแมนติกที่แนบชิด'",
			},

			// === [A] Authoritativeness Section ===
			"characterDynamic": {
				Type:        genai.TypeString,
				Description: "ความสัมพันธ์ตัวละคร (50 คำ)",
			},
			"plotAnalysis": {
				Type:        genai.TypeString,
				Description: "วิเคราะห์โครงเรื่อง (100 คำ)",
			},
			"recommendation": {
				Type:        genai.TypeString,
				Description: "เหมาะสำหรับคนที่ชอบ... (50 คำ)",
			},
			"recommendedFor": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "กลุ่มเป้าหมาย เช่น แฟนหนังแนว Medical, คนชอบ Drama",
			},
			"settingDescription": {
				Type:        genai.TypeString,
				Description: "บริบทฉาก (50 คำ)",
			},
			"moodTone": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "อารมณ์เรื่อง เช่น ดราม่า, โรแมนติก",
			},
			"thematicKeywords": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "Keywords สำหรับ semantic search รวม location-specific keywords",
			},

			// === [SEO] Internal Linking ===
			"contextualLinks": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"text":        {Type: genai.TypeString, Description: "ประโยคเชื่อมโยงแบบธรรมชาติ (ไม่ใส่ลิงก์) เช่น 'ถ้าคุณประทับใจการแสดงแนว Medical ของ Zemba Mami คุณอาจจะสนใจ'"},
						"linkedSlug":  {Type: genai.TypeString, Description: "Slug ของ article ที่แนะนำ"},
						"linkedTitle": {Type: genai.TypeString, Description: "Title ที่แสดง เช่น 'DLDSS-470 ที่เน้นการสำรวจอารมณ์ในอีกรูปแบบหนึ่ง'"},
					},
					Required: []string{"text", "linkedSlug", "linkedTitle"},
				},
				Description: "2-3 ประโยคเชื่อมโยงไป related articles (SEO Internal Linking) - ประโยคต้องเป็นธรรมชาติ ไม่เหมือนสแปม",
			},
		},
		Required: []string{
			// Expertise (ห้ามว่าง)
			"dialogueAnalysis", "characterInsight", "topQuotes",
			"expertAnalysis", "detailedReview", "castBios", "tagDescriptions",
			"comparisonNote",
			// Authoritativeness
			"characterDynamic", "plotAnalysis", "recommendation",
			"recommendedFor", "settingDescription", "moodTone", "thematicKeywords",
			// NOTE: contextualLinks is OPTIONAL - only generated if RelatedArticles exist
		},
	}
}

// buildChunk2Prompt สร้าง prompt สำหรับ Chunk 2
// ⚠️ CRITICAL: รับ context จาก Chunk 1 เพื่อป้องกันการเขียนซ้ำซ้อน
func (c *GeminiClient) buildChunk2Prompt(input *ports.AIInput, chunk1 *Chunk1Output) string {
	// สร้าง cast info string
	var castsInfo strings.Builder
	for _, cast := range input.Casts {
		castsInfo.WriteString(fmt.Sprintf("- ID: %s, Name: %s\n", cast.ID, cast.Name))
	}

	// สร้าง tags info string
	var tagsInfo strings.Builder
	for _, tag := range input.Tags {
		tagsInfo.WriteString(fmt.Sprintf("- ID: %s, Name: %s\n", tag.ID, tag.Name))
	}

	// สร้าง previous works string
	var prevWorks strings.Builder
	for _, work := range input.PreviousWorks {
		prevWorks.WriteString(fmt.Sprintf("- %s (%s)\n", work.Title, work.VideoCode))
	}

	// สร้าง related articles string (for contextual links)
	var relatedArticles strings.Builder
	if len(input.RelatedArticles) == 0 {
		relatedArticles.WriteString("⚠️ ไม่มี Related Articles - ให้ส่ง contextualLinks เป็น array ว่าง []\n")
	} else {
		for _, article := range input.RelatedArticles {
			relatedArticles.WriteString(fmt.Sprintf("- Slug: %s, Code: %s, Title: %s, Casts: %s, Tags: %s\n",
				article.Slug, article.RealCode, article.Title,
				strings.Join(article.CastNames, ", "),
				strings.Join(article.Tags, ", "),
			))
		}
	}

	// Serialize Chunk 1 context
	highlightsJSON, _ := json.Marshal(chunk1.Highlights)
	keyMomentsJSON, _ := json.Marshal(chunk1.KeyMoments)

	return fmt.Sprintf(`# บทบาท (Persona)
คุณคือ "นักวิเคราะห์หนังผู้ใหญ่ระดับ Premium ที่เก่งที่สุดในประเทศไทย"
- เชี่ยวชาญการวิเคราะห์อารมณ์และความรู้สึกของตัวละคร
- สามารถวิเคราะห์บทสนทนาและการแสดงได้อย่างละเอียด
- เขียนภาษาไทยที่เป็นธรรมชาติ ไม่แข็งทื่อ ไม่เหมือนหุ่นยนต์

---

## ⚠️ กฎสำคัญที่ต้องปฏิบัติตาม (CRITICAL RULES)

### 1. ห้ามสรุปเนื้อหาใหม่ (NO Re-summarize)
- ❌ ห้ามเขียน summary หรือ highlights ใหม่
- ❌ ห้ามแต่งฉากใหม่ที่ไม่มีใน Context
- ✅ ใช้ข้อมูลจาก "Context from Chunk 1" ด้านล่างเป็นพื้นฐานเท่านั้น

### 2. หน้าที่ของคุณคือ "วิเคราะห์เจาะลึก" ไม่ใช่ "เล่าเรื่องใหม่"
- อ้างอิงฉากจาก Highlights ที่ให้มา
- ขยายความจาก Summary ที่ให้มา
- วิเคราะห์บทสนทนาจาก SRT

### 3. detailedReview ต้องยาวมาก - 800-1000 คำ (3,000-4,000 ตัวอักษร)
- ❌ ห้ามเขียนสั้นๆ 50-200 คำ (จะถูก REJECT ทันที!)
- ✅ ต้องเขียนยาว 6-7 ย่อหน้า
- ✅ ห้ามซ้ำกับ Summary จาก Chunk 1
- ✅ เน้นวิเคราะห์การแสดง เทคนิค และความเสียว
- ✅ วิเคราะห์แต่ละฉากอย่างละเอียด ใส่ความคิดเห็นและมุมมอง

### 4. expertAnalysis ห้ามว่าง
- ต้องวิเคราะห์เทคนิคการแสดงหรือจุดเด่นของเรื่อง

### 5. tagDescriptions ห้ามปล่อย description ว่าง
- ต้อง generate คำอธิบายภาษาไทยสำหรับทุก tag

---

## Context from Chunk 1 (ใช้เป็นฐานในการวิเคราะห์)

### Summary (ห้ามเขียนใหม่ - ใช้อ้างอิงเท่านั้น):
%s

### Highlights (อ้างอิงฉากจากที่นี่):
%s

### Key Moments (อ้างอิง timestamps จากที่นี่):
%s

---

## ข้อมูล Input เพิ่มเติม

### SRT Transcript (ใช้วิเคราะห์บทสนทนา):
%s

### Video Metadata:
- Code: %s
- Duration: %d seconds

### Casts (ต้อง generate bio สำหรับแต่ละคน):
%s

### Cast Previous Works (ใช้เปรียบเทียบการแสดง):
%s

### Tags (⚠️ ต้อง generate description ภาษาไทยสำหรับแต่ละ tag!):
%s

### Related Articles (สำหรับสร้าง Contextual Links):
%s

---

## Output Requirements (Chunk 2: E-E-A-T Analysis)

### [E] Expertise Section (ห้ามว่าง!)
1. **dialogueAnalysis**: วิเคราะห์บทสนทนา สรรพนาม หางเสียง อารมณ์ (100-150 คำ)
2. **characterInsight**: วิเคราะห์บุคลิกตัวละคร (100-150 คำ)
3. **topQuotes**: 3-5 ประโยคเด็ดจากซับ
    - ⚠️ **เลือกเฉพาะ timestamp ภายใน 600 วินาทีแรก (10 นาที)**
    - เน้นบทสนทนาที่น่าสนใจ ไม่ใช่ประโยคลามก
    - พร้อม timestamp (วินาที), emotion, context
4. **languageNotes**: หมายเหตุภาษา (50 คำ)
5. **actorPerformanceTrend**: เปรียบเทียบการแสดงกับผลงานก่อนหน้า (100 คำ)
6. **comparisonNote**: ⚠️ ต้องระบุ VIDEO CODE จริง (50 คำ)
7. **expertAnalysis**: บทวิเคราะห์ผู้เชี่ยวชาญ **ห้ามว่าง** (100 คำ)
8. **detailedReview**: ⚠️ บทวิเคราะห์ **800-1000 คำ (3,000-4,000 ตัวอักษร)** - ห้ามสั้น! เขียน 6-7 ย่อหน้า
9. **castBios**: bio สำหรับแต่ละ cast (50-100 คำต่อคน)
10. **tagDescriptions**: ⚠️ คำอธิบายภาษาไทยสำหรับแต่ละ tag
    - ห้ามว่าง
    - ⚠️ **ห้ามใช้คำเหล่านี้:** หลั่ง, แตกใน, อวัยวะเพศ, ช่องคลอด, น้ำกาม
    - ✅ ตัวอย่างที่ดี:
      * "Creampie" → "ฉากจบแบบโรแมนติกที่แนบชิดสุดพิเศษ"
      * "Big Tits" → "นักแสดงที่มีสัดส่วนโดดเด่น"
      * "Blowjob" → "ฉากแสดงความรักใคร่ทางปาก"
    - ❌ ตัวอย่างที่ห้าม:
      * "Creampie" → "ฉากที่มีการหลั่งภายใน..." ← ห้าม!
      * "Big Tits" → "หน้าอกขนาดใหญ่" ← ตรงเกินไป

### [A] Authoritativeness Section
11. **characterDynamic**: ความสัมพันธ์ตัวละคร (50 คำ)
12. **plotAnalysis**: วิเคราะห์โครงเรื่อง (100 คำ)
13. **recommendation**: เหมาะสำหรับ... (50 คำ)
14. **recommendedFor**: Array เช่น ["แฟนหนังแนว Medical", "คนชอบ Drama"]
15. **settingDescription**: บริบทฉาก (50 คำ)
16. **moodTone**: Array อารมณ์ ["ดราม่า", "ซีเรียส", "อีโรติก"]
17. **thematicKeywords**: ⚠️ ต้องมี LOCATION-SPECIFIC keywords เช่น ["คลินิกสูตินรีเวชญี่ปุ่น", "ห้องตรวจ"]

### [SEO] Internal Linking (OPTIONAL)
18. **contextualLinks**: 2-3 ประโยคเชื่อมโยงไป Related Articles
    - ⚠️ **ถ้าไม่มี Related Articles → ให้ส่ง array ว่าง []**
    - ⚠️ **ห้ามแต่ง slug ขึ้นมาเอง** - ใช้เฉพาะ slug จาก Related Articles ที่ให้มา
    - ใช้ข้อมูลจาก Related Articles (cast เดียวกัน, แนวเดียวกัน) สร้างประโยคเชื่อมโยง
    - ประโยคต้องเป็นธรรมชาติ ไม่เหมือนสแปม
    - ✅ ตัวอย่างที่ดี:
      * text: "ถ้าคุณประทับใจการแสดงแนว Medical ของ Zemba Mami ในเรื่องนี้ คุณอาจจะสนใจ"
      * linkedSlug: "dldss-470"
      * linkedTitle: "DLDSS-470 ที่เน้นการสำรวจอารมณ์ในอีกรูปแบบหนึ่ง"
    - ❌ ตัวอย่างที่ห้าม:
      * "ดูเรื่องนี้ด้วย: DLDSS-470" ← เหมือนสแปม
      * "แนะนำ: DLDSS-470" ← ไม่มี context
      * แต่ง slug ขึ้นมาเองที่ไม่อยู่ใน Related Articles ← ห้ามเด็ดขาด!

---

## ⚠️ ข้อห้าม (DON'T)
- ❌ เขียน summary ใหม่ (ใช้จาก Chunk 1)
- ❌ แต่งฉากที่ไม่มีใน Highlights
- ❌ expertAnalysis ว่าง
- ❌ tagDescriptions.description ว่าง
- ❌ detailedReview สั้นกว่า 800 คำ (REJECT ทันที!)
- ❌ detailedReview ซ้ำกับ Summary
- ❌ **ผสมภาษาในชื่อนักแสดง** เช่น "เซ็นมะ Mami", "Zemba มามิ" ← ผิด! REJECT ทันที!
- ✅ **ใช้ชื่อ ENGLISH ตามที่ให้มาเท่านั้น** - copy จาก Casts info โดยตรง ห้ามแปลง!
`,
		chunk1.Summary,
		string(highlightsJSON),
		string(keyMomentsJSON),
		input.SRTContent,
		input.VideoMetadata.RealCode,
		input.VideoMetadata.Duration,
		castsInfo.String(),
		prevWorks.String(),
		tagsInfo.String(),
		relatedArticles.String(),
	)
}
