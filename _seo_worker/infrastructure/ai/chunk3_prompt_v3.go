package ai

import (
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 3 V3: Story Recap
// Focus: สรุปเรื่องจาก SRT (Unique Content)
// ============================================================================

func (c *GeminiClient) buildChunk3SchemaV3() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"synopsis": {
				Type:        genai.TypeString,
				Description: "เรื่องย่อ 150-250 คำ แบ่ง 2-3 ย่อหน้า (คั่นด้วย [PARA])",
			},
			"storyFlow": {
				Type:        genai.TypeString,
				Description: "Timeline เรื่อง: เริ่มต้น → พัฒนา → ไคลแมกซ์ (80-100 คำ)",
			},
			"keyScenes": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "3-5 ฉากสำคัญ แต่ละข้อ 15-25 คำ",
			},
			"featuredScene": {
				Type:        genai.TypeString,
				Description: "ฉากเด่นที่สุด 120-150 คำ ต้องมี: บรรยากาศ, อารมณ์, context, dialogue (ห้ามแค่ quote อย่างเดียว)",
			},
			"tone": {
				Type:        genai.TypeString,
				Description: "อารมณ์เรื่อง 2-3 คำ เช่น 'ผ่อนคลาย โรแมนติก'",
			},
			"relationshipDynamic": {
				Type:        genai.TypeString,
				Description: "ความสัมพันธ์ตัวละคร 50-80 คำ",
			},
		},
		Required: []string{"synopsis", "storyFlow", "keyScenes", "featuredScene", "tone", "relationshipDynamic"},
	}
}

func (c *GeminiClient) buildChunk3PromptV3(input *ports.AIInput, chunk1 *Chunk1OutputV3) string {
	// Cast names
	castNames := make([]string, len(input.Casts))
	for i, cast := range input.Casts {
		castNames[i] = cast.Name
	}
	castNamesStr := strings.Join(castNames, ", ")

	return fmt.Sprintf(`[ROLE]
คุณคือ Story Summarizer

[GOAL]
สรุปเรื่องจาก SRT transcript
นี่คือ unique content ที่คู่แข่งไม่มี

[CONTEXT จาก Chunk 1]
Quick Answer: %s
Verdict: %s

[INPUT]
- Code: %s
- Cast: %s
- Duration: %d นาที

SRT Transcript:
%s

---

[OUTPUT]

1. **synopsis** (150-250 คำ, 2-3 ย่อหน้า)
   - ใช้ [PARA] คั่นย่อหน้า
   - เล่าเรื่องตามลำดับ
   - เน้น character behavior จาก dialogue

   ตัวอย่างโครงสร้าง:
   "เรื่องเริ่มต้นเมื่อ [ตัวละคร] [สถานการณ์]... [PARA] เมื่อเวลาผ่านไป [พัฒนาการ]... [PARA] ในที่สุด [สรุป]..."

2. **storyFlow** (80-100 คำ)
   Timeline:
   - เริ่มต้น: [อะไรเกิดขึ้น]
   - พัฒนา: [ความสัมพันธ์เปลี่ยนไปอย่างไร]
   - ไคลแมกซ์: [จุดสูงสุด]

3. **keyScenes** (3-5 ฉาก)
   - แต่ละฉาก 15-25 คำ
   - ❌ ห้ามขึ้นต้นด้วยชื่อนักแสดง
   - ✅ ขึ้นต้นด้วย "ฉากที่...", "ช่วงเวลา...", "การ..."

4. **featuredScene** (120-150 คำ) 🔥 Scroll Depth Trigger!
   เลือก 1 ฉากที่โดดเด่นที่สุดแล้วเขียนขยายแบบละเอียด

   ✅ ต้องมีครบ 4 องค์ประกอบ:
   a) **บรรยากาศ** - สถานที่ แสง เสียง อารมณ์ฉาก
   b) **Context** - เกิดอะไรขึ้นก่อนหน้า ทำไมฉากนี้สำคัญ
   c) **Action/Dialogue** - ตัวละครทำอะไร พูดอะไร (quote สั้นๆ)
   d) **ผลกระทบ** - ฉากนี้เปลี่ยนแปลงอะไรในเรื่อง

   ❌ ห้ามแค่ quote dialogue อย่างเดียว
   ❌ ห้าม generic เช่น "ฉากนี้น่าตื่นเต้น"
   ✅ ต้องเฉพาะเจาะจงกับเรื่องนี้จริงๆ

   โครงสร้างตัวอย่าง:
   "ในห้องที่แสงส่องลอดผ้าม่านบางๆ [บรรยากาศ]...
   หลังจากที่ [context]... [ตัวละคร] ก็พูดว่า '...' [dialogue]...
   ช่วงเวลานั้นเปลี่ยนทุกอย่าง เพราะ... [ผลกระทบ]"

5. **tone** (2-3 คำ)
   เช่น: "ผ่อนคลาย โรแมนติก", "เข้มข้น ดราม่า"

6. **relationshipDynamic** (50-80 คำ)
   - ความสัมพันธ์ระหว่างตัวละคร
   - Power dynamics

---

[RULES]
- ✅ ใช้ข้อมูลจาก SRT เท่านั้น
- ✅ ใช้ชื่อนักแสดง: %s
- ❌ ห้ามแต่งเรื่องเพิ่ม
- ❌ ห้ามใช้คำหยาบ
`,
		chunk1.QuickAnswer,
		chunk1.Verdict,
		input.VideoMetadata.RealCode,
		castNamesStr,
		input.VideoMetadata.Duration/60,
		truncateSRT(input.SRTContent, 3000),
		castNamesStr,
	)
}
