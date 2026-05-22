package ai

import (
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 5 V3: FAQ Intent Block
// Focus: 5 คำถามที่ตรง search intent + conversion
// ============================================================================

func (c *GeminiClient) buildChunk5SchemaV3() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"faqItems": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"question": {Type: genai.TypeString, Description: "คำถามเต็มรูปแบบ"},
						"answer":   {Type: genai.TypeString, Description: "คำตอบ 40-60 คำ"},
					},
					Required: []string{"question", "answer"},
				},
				Description: "5 คำถามที่ตรง search intent",
			},
		},
		Required: []string{"faqItems"},
	}
}

func (c *GeminiClient) buildChunk5PromptV3(input *ports.AIInput, chunk1 *Chunk1OutputV3, chunk3 *Chunk3OutputV3, chunk4 *Chunk4OutputV3) string {
	// Cast names
	castNames := make([]string, len(input.Casts))
	for i, cast := range input.Casts {
		castNames[i] = cast.Name
	}
	castNamesStr := strings.Join(castNames, ", ")
	firstCast := ""
	if len(castNames) > 0 {
		firstCast = castNames[0]
	}

	// Strengths summary
	strengthsStr := strings.Join(chunk4.Strengths, ", ")

	// Get content variation based on video ID (deterministic)
	variation := GetContentVariation(input.VideoMetadata.ID)
	faqStylePrompt := GetFAQStylePrompt(variation.FAQStyle)

	return fmt.Sprintf(`[ROLE]
คุณคือ FAQ Writer for Search Intent + Conversion

[GOAL]
สร้าง 5 FAQ ที่ตรง search intent
FAQ ข้อ 5 ต้องเป็น Conversion FAQ (ดึงให้สมัครสมาชิก)

🚨 [FAQ STYLE - ใช้แบบ %s] 🚨
%s

[CONTEXT]
Quick Answer: %s
Verdict: %s
Synopsis: %s
Strengths: %s
Who Should Watch: %s

[INPUT]
- Code: %s
- Cast: %s

---

[OUTPUT]

สร้าง **faqItems** จำนวน 5 ข้อ ตามนี้:

### FAQ 1: เรื่องเกี่ยวกับอะไร
Question: "%s เกี่ยวกับอะไร?"
Answer: [สรุปเนื้อเรื่อง 2-3 ประโยค จาก synopsis]

### FAQ 2: มีซับไทยไหม
Question: "%s มีซับไทยไหม?"
Answer: "มีซับไทยคุณภาพดี แปลครบทุกบทสนทนา พร้อมรับชมได้ทันที"

### FAQ 3: คุ้มไหม
Question: "ดู %s แล้วคุ้มไหม?"
Answer: [ตอบจาก verdict + เหตุผล 2-3 ประโยค]

### FAQ 4: นักแสดงใครเด่น
Question: "%s นักแสดงใครเด่น?"
Answer: "[ชื่อนักแสดง] โดดเด่นในบท [บทบาท] [เหตุผลที่เด่น 1-2 ประโยค]"

### FAQ 5: ดูที่ไหน (CONVERSION FAQ!)
Question: "ดู %s ซับไทยได้ที่ไหน?"
Answer: "รับชม %s ซับไทยเวอร์ชันเต็มได้ในระบบสมาชิก สมัครเพื่อดูเนื้อหาคุณภาพสูงพร้อมซับไทยครบทุกเรื่อง"

---

[RULES]
- ✅ คำถามต้องเต็มรูปแบบ (มี ? ท้าย)
- ✅ คำตอบ 40-60 คำ
- ✅ FAQ ข้อ 5 ต้องมี conversion copy
- ✅ ใช้ชื่อ: %s
- ❌ ห้ามใช้คำหยาบ
- ❌ ห้ามคำถามสั้นเกินไป
- ⚠️ ใช้ FAQ Style ที่กำหนดให้ข้างบนเท่านั้น!
`,
		// FAQ style variation
		variation.FAQStyle,
		faqStylePrompt,
		chunk1.QuickAnswer,
		chunk1.Verdict,
		truncateToWords(chunk3.Synopsis, 80),
		strengthsStr,
		chunk4.WhoShouldWatch,
		input.VideoMetadata.RealCode,
		castNamesStr,
		input.VideoMetadata.RealCode,
		input.VideoMetadata.RealCode,
		input.VideoMetadata.RealCode,
		firstCast,
		input.VideoMetadata.RealCode,
		input.VideoMetadata.RealCode,
		castNamesStr,
	)
}
