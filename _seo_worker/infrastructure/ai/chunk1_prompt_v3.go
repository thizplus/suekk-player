package ai

import (
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 1 V3: Search Intent Answer
// Focus: ตอบ search query ให้เร็วที่สุด (Google Snippet Target)
// ============================================================================

func (c *GeminiClient) buildChunk1SchemaV3() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"quickAnswer": {
				Type:        genai.TypeString,
				Description: "2-3 ประโยคบอกเล่า ห้ามมีเครื่องหมาย ? ในประโยคแรก ขึ้นต้นด้วย '[CODE] ซับไทย เป็นเรื่อง...'",
			},
			"mainHook": {
				Type:        genai.TypeString,
				Description: "1 ประโยคดึงดูดให้อยากอ่านต่อ/ดูต่อ",
			},
			"verdict": {
				Type:        genai.TypeString,
				Description: "1 ประโยคสรุป soft บอกว่าเหมาะกับใคร (ห้าม clickbait เช่น 'พลาดแล้วเสียใจ')",
			},
		},
		Required: []string{"quickAnswer", "mainHook", "verdict"},
	}
}

func (c *GeminiClient) buildChunk1PromptV3(input *ports.AIInput) string {
	// Cast names
	castNames := make([]string, len(input.Casts))
	for i, cast := range input.Casts {
		castNames[i] = cast.Name
	}
	castNamesStr := strings.Join(castNames, ", ")

	// Get content variation based on video ID (deterministic)
	variation := GetContentVariation(input.VideoMetadata.ID)
	openingStylePrompt := GetOpeningStylePrompt(variation.OpeningStyle)

	return fmt.Sprintf(`[ROLE]
คุณคือ Google Featured Snippet Writer

[GOAL]
ตอบคำค้น "%s review" และ "%s ซับไทย" ให้เร็วที่สุด
เหมือนเพื่อนถามว่า "เรื่องนี้ดีไหม?" แล้วคุณตอบทันที

[INPUT]
- Code: %s
- Duration: %d นาที
- Cast: %s

SRT Transcript (ใช้ทำความเข้าใจเนื้อเรื่อง):
%s

---

[OUTPUT]

1. **quickAnswer** (2-3 ประโยค) - DECLARATIVE STATEMENT เท่านั้น!
   ✅ ต้องขึ้นต้นด้วยประโยคบอกเล่า:
   - "[CODE] ซับไทย เป็นเรื่องราวดราม่าเกี่ยวกับ..."
   - "ผลงานนี้เล่าถึง [ตัวละคร] ที่..."
   - "[นักแสดง] กลับมาในบท..."

   🚫 ห้ามเด็ดขาด - ห้ามมี "?" ในประโยคแรก:
   - "กำลังหา...ใช่ไหม?" ❌
   - "อยากรู้ไหมว่า...?" ❌
   - "เคยสงสัยไหม...?" ❌
   - "...หรือเปล่า?" ❌

   Google ไม่ชอบ question hook!

2. **mainHook** (1 ประโยค)
   ดึงดูดให้อยากดู:
   - ใช้คำกระตุ้น
   - สร้างความอยากรู้

3. **verdict** (1 ประโยค) - SOFT RECOMMENDATION
   ✅ บอกว่าเหมาะกับใคร:
   - "เหมาะกับคนที่ชอบแนว..."
   - "ถ้าชอบดราม่าเข้มข้น เรื่องนี้ตอบโจทย์"

   ❌ ห้าม aggressive/clickbait:
   - "พลาดแล้วจะเสียใจ!"
   - "ต้องดู!"
   - "ห้ามพลาด!"

---

[RULES]
- ❌ ห้ามเขียนยาว
- ❌ ห้ามเกริ่นนำ
- ❌ ห้ามเขียนแบบ essay
- ❌ ห้าม clickbait tone
- ✅ ตอบตรงๆ declarative statement
- ✅ ใช้ชื่อนักแสดง: %s

🆕 [FIRST 150 WORDS RULE - สำคัญมาก!]
quickAnswer ต้องมี keywords เหล่านี้แบบธรรมชาติ:
- CODE: %s
- "ซับไทย"
- "รีวิว"
- ชื่อนักแสดง: %s

⚠️ [ANTI-STUFFING RULE]
- ❌ ห้ามใช้ CODE (%s) เกิน 3 ครั้งใน 200 คำแรก
- ✅ ใช้ synonym แทน: "เรื่องนี้", "ผลงานนี้", "เรื่องราว"
- ✅ density ต้องธรรมชาติ ไม่ยัดคำ
- ❌ ห้าม keyword stuffing

🚨 [OPENING STYLE - ใช้แบบ %s] 🚨
%s

🚨 [ANTI-SIMILARITY RULE - สำคัญที่สุด!] 🚨
❌ ห้ามใช้ pattern เปิดซ้ำกับบทความอื่น
❌ ห้ามขึ้นต้นด้วย "[CODE] เป็น..." ทุกหน้า (ยกเว้น Style A)
❌ ห้ามใช้ sentence structure เดิมซ้ำ

✅ ต้องใช้ sentence structure ต่างกันจริง:
- Subject position ต่างกัน (ตัวละคร vs ค่าย vs แนว)
- Verb tense ต่างกัน (ปัจจุบัน vs อดีต)
- Sentence length ต่างกัน (สั้น vs ยาว)

✅ Variation ที่ต้องทำ (ทุกแบบต้องเป็น declarative):
- บางหน้า: เริ่มด้วย direct "[CODE] ซับไทย เป็นเรื่องราวเกี่ยวกับ..."
- บางหน้า: เริ่มด้วย context "จากค่าย Y ที่เน้น... ผลงานนี้..."
- บางหน้า: เริ่มด้วย cast "[ชื่อนักแสดง] กลับมาในบท..."
- บางหน้า: เริ่มด้วย genre "แนวดราม่าความสัมพันธ์จาก..."

❌ ห้ามเริ่มด้วยคำถามหรือ clickbait hook

⚠️ ใช้ Opening Style ที่กำหนดให้ข้างบนเท่านั้น!
`,
		input.VideoMetadata.RealCode,
		input.VideoMetadata.RealCode,
		input.VideoMetadata.RealCode,
		input.VideoMetadata.Duration/60,
		castNamesStr,
		truncateSRT(input.SRTContent, 2000),
		castNamesStr,
		// First 150 Words Rule arguments
		input.VideoMetadata.RealCode,
		castNamesStr,
		// Anti-stuffing rule argument
		input.VideoMetadata.RealCode,
		// Opening style
		variation.OpeningStyle,
		openingStylePrompt,
	)
}
