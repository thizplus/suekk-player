package ai

import (
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 4 V3: Review Section
// Focus: รีวิว Verdict Style (ไม่ใช่ Essay)
// ============================================================================

func (c *GeminiClient) buildChunk4SchemaV3() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"reviewSummary": {
				Type:        genai.TypeString,
				Description: "รีวิว 200-300 คำ แบ่ง 3 ย่อหน้า (คั่นด้วย [PARA]) เขียนแบบ reviewer ไม่ใช่ essay",
			},
			"strengths": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "3-5 จุดเด่น แต่ละข้อ 10-20 คำ",
			},
			"weaknesses": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "2-3 จุดอ่อน แต่ละข้อ 10-20 คำ (ต้องมีอย่างน้อย 1 ข้อ)",
			},
			"whoShouldWatch": {
				Type:        genai.TypeString,
				Description: "เหมาะกับใคร 50-80 คำ",
			},
			"verdictReason": {
				Type:        genai.TypeString,
				Description: "เหตุผลที่ควร/ไม่ควรดู 30-50 คำ",
			},
		},
		Required: []string{"reviewSummary", "strengths", "weaknesses", "whoShouldWatch", "verdictReason"},
	}
}

func (c *GeminiClient) buildChunk4PromptV3(input *ports.AIInput, chunk1 *Chunk1OutputV3, chunk3 *Chunk3OutputV3) string {
	// Cast names
	castNames := make([]string, len(input.Casts))
	for i, cast := range input.Casts {
		castNames[i] = cast.Name
	}
	castNamesStr := strings.Join(castNames, ", ")

	// Get content variation based on video ID (deterministic)
	variation := GetContentVariation(input.VideoMetadata.ID)
	reviewTonePrompt := GetReviewTonePrompt(variation.ReviewTone)

	return fmt.Sprintf(`[ROLE]
คุณคือ Reviewer (ไม่ใช่นักเขียน essay)
เขียนเหมือนคนรีวิวให้เพื่อน ตรงๆ ไม่อ้อม

[GOAL]
รีวิวแบบ Verdict Style
บอกให้ชัดว่าดียังไง แย่ยังไง เหมาะกับใคร

🚨 [REVIEW TONE - ใช้แบบ %s] 🚨
%s

[CONTEXT]
Quick Answer: %s
Verdict: %s
Synopsis: %s
Tone: %s

[INPUT]
- Code: %s
- Cast: %s

---

[OUTPUT]

1. **reviewSummary** (200-300 คำ, 3 ย่อหน้า)
   ใช้ [PARA] คั่นย่อหน้า

   โครงสร้าง:
   - ย่อหน้า 1: ภาพรวม + จุดเด่นหลัก
   - ย่อหน้า 2: การแสดง + เทคนิค
   - ย่อหน้า 3: สรุป + ใครควรดู

   ❌ ห้ามเขียนแบบ essay วิชาการ
   ✅ เขียนเหมือนรีวิวให้เพื่อน

2. **strengths** (3-5 ข้อ)
   จุดเด่น แต่ละข้อ 10-20 คำ

   ตัวอย่าง:
   - "การแสดงของ [ชื่อ] เป็นธรรมชาติ สื่ออารมณ์ได้ดี"
   - "บรรยากาศผ่อนคลาย เหมาะดูพักผ่อน"
   - "เคมีระหว่างตัวละครดี มีความ romantic"

3. **weaknesses** (2-3 ข้อ)
   จุดอ่อน แต่ละข้อ 10-20 คำ
   ⚠️ ต้องมีอย่างน้อย 1 ข้อ (ไม่มีเรื่องไหนสมบูรณ์แบบ)

   ตัวอย่าง:
   - "โครงเรื่องเรียบง่าย ไม่มี plot twist"
   - "จังหวะช้าไปสำหรับบางคน"
   - "บทสนทนาบางช่วงยืดเยื้อ"

4. **whoShouldWatch** (50-80 คำ)
   บอกชัดๆ ว่า:
   - เหมาะกับ: [กลุ่มคน]
   - ไม่เหมาะกับ: [กลุ่มคน]

5. **verdictReason** (30-50 คำ)
   เหตุผลสรุป 1-2 ประโยค

   ตัวอย่าง:
   "เรื่องนี้เด่นที่เคมีตัวละครและบรรยากาศ แต่โครงเรื่องเรียบง่าย เหมาะกับคนที่อยากดูอะไรเบาๆ มากกว่าความเข้มข้น"

---

[RULES]
- ✅ ตรงๆ ไม่อ้อม
- ✅ verdict ต้องชัด
- ✅ ใช้ชื่อ: %s
- ❌ ห้ามใช้คำหยาบ
- ❌ ห้ามเขียนแบบ essay
- ⚠️ ใช้ Review Tone ที่กำหนดให้ข้างบนเท่านั้น!
`,
		// Review tone variation
		variation.ReviewTone,
		reviewTonePrompt,
		chunk1.QuickAnswer,
		chunk1.Verdict,
		truncateToWords(chunk3.Synopsis, 100),
		chunk3.Tone,
		input.VideoMetadata.RealCode,
		castNamesStr,
		castNamesStr,
	)
}
