package ai

import (
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Title Style Variations (Anti-Template Footprint)
// ============================================================================
// ใช้ hash(videoID) เลือก style เพื่อให้ title มีความหลากหลาย
// ไม่ให้ Google เห็น pattern ซ้ำทุกหน้า

var titleAggressiveStyles = []string{
	// Style A: Question Hook
	`ใช้รูปแบบ: "[CODE] ซับไทย [คำถาม]?"
   ตัวอย่าง:
   - "%s ซับไทย ฉากไหนเด็ดสุด?"
   - "%s ซับไทย ดูแล้วคุ้มไหม?"
   - "%s ซับไทย %s เล่นดีขนาดไหน?"`,

	// Style B: Number Hook
	`ใช้รูปแบบ: "[CODE] ซับไทย [ตัวเลข] [สิ่งที่น่าสนใจ]"
   ตัวอย่าง:
   - "%s ซับไทย 5 ฉากที่ต้องดู"
   - "%s ซับไทย 3 เหตุผลที่ห้ามพลาด"
   - "%s ซับไทย %s กับฉากเด็ดที่สุด"`,

	// Style C: Emotional Hook
	`ใช้รูปแบบ: "[CODE] ซับไทย [Emotion/Reaction]"
   ตัวอย่าง:
   - "%s ซับไทย ดูจบแล้วอยากดูซ้ำ"
   - "%s ซับไทย เนื้อเรื่องโดนใจมาก"
   - "%s ซับไทย %s พีคมาก"`,

	// Style D: Comparison/Unique
	`ใช้รูปแบบ: "[CODE] ซับไทย [จุดเด่นที่ต่าง]"
   ตัวอย่าง:
   - "%s ซับไทย ต่างจากเรื่องอื่นตรงไหน"
   - "%s ซับไทย ไม่เหมือนเรื่องไหนที่เคยดู"
   - "%s ซับไทย %s ในบทบาทใหม่"`,
}

var titleBalancedStyles = []string{
	// Style A: Standard Review
	`ใช้รูปแบบ: "รีวิว [CODE] ซับไทย [ข้อมูลเพิ่ม]"
   ตัวอย่าง:
   - "รีวิว %s ซับไทย เนื้อเรื่องและฉากเด่น"
   - "รีวิว %s ซับไทย ดูแล้วคุ้มไหม"`,

	// Style B: Guide Style
	`ใช้รูปแบบ: "[CODE] ซับไทย [Guide]"
   ตัวอย่าง:
   - "%s ซับไทย รีวิว เรื่องย่อ และฉากเด่น"
   - "%s ซับไทย คู่มือก่อนดู สิ่งที่ควรรู้"`,

	// Style C: Summary Style
	`ใช้รูปแบบ: "[CODE] ซับไทย สรุป [อะไร]"
   ตัวอย่าง:
   - "%s ซับไทย สรุปเนื้อเรื่องและความเห็น"
   - "%s ซับไทย สรุปรีวิว จุดเด่น จุดด้อย"`,

	// Style D: Complete Style
	`ใช้รูปแบบ: "[CODE] ซับไทย [ครบจบ]"
   ตัวอย่าง:
   - "%s ซับไทย รีวิวครบ เรื่องย่อ นักแสดง"
   - "%s ซับไทย ข้อมูลครบ ก่อนตัดสินใจดู"`,
}

// searchIntents template sets (6 sets for better distribution)
var searchIntentsTemplateSets = []struct {
	Templates []string
}{
	// Set A
	{Templates: []string{"%s ซับไทย", "%s รีวิว"}},
	// Set B
	{Templates: []string{"ดู %s ซับไทย", "รีวิวจริง %s"}},
	// Set C
	{Templates: []string{"%s ดูที่ไหน", "%s เรื่องย่อ"}},
	// Set D
	{Templates: []string{"%s ซับไทย ดีไหม", "รีวิว %s ซับไทย"}},
	// Set E
	{Templates: []string{"ดู %s ได้ที่ไหน", "%s สรุป"}},
	// Set F
	{Templates: []string{"%s แนะนำ", "%s ความเห็น"}},
}

// TitleStyleInfo for logging/debugging
type TitleStyleInfo struct {
	AggressiveStyleIdx int
	BalancedStyleIdx   int
	IntentSetIdx       int
	IntentOrderSwapped bool
}

// ============================================================================
// Chunk 6 V3: SEO Output
// Focus: Title (Aggressive + Balanced) และ Meta
// ============================================================================

func (c *GeminiClient) buildChunk6SchemaV3() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"titleAggressive": {
				Type:        genai.TypeString,
				Description: "Title แบบ CTR focus ดึงดูดให้คลิก 50-60 ตัวอักษร",
			},
			"titleBalanced": {
				Type:        genai.TypeString,
				Description: "Title แบบ Google friendly สำหรับ H1 50-60 ตัวอักษร",
			},
			"metaDescription": {
				Type:        genai.TypeString,
				Description: "Meta description 150-160 ตัวอักษร",
			},
			"slug": {
				Type:        genai.TypeString,
				Description: "URL slug เช่น 'dass-541-sub-thai-review'",
			},
			"keywords": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "5-8 SEO keywords",
			},
			"searchIntents": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "4-5 search intent phrases สำหรับ internal linking",
			},
			"rating": {
				Type:        genai.TypeNumber,
				Description: "คะแนน 1-5 (ทศนิยม 1 ตำแหน่ง) สำหรับ Review JSON-LD",
			},
		},
		Required: []string{"titleAggressive", "titleBalanced", "metaDescription", "slug", "keywords", "searchIntents", "rating"},
	}
}

func (c *GeminiClient) buildChunk6PromptV3(input *ports.AIInput, chunk1 *Chunk1OutputV3, chunk3 *Chunk3OutputV3, chunk4 *Chunk4OutputV3) string {
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

	// Tags for keywords
	var tagsStr strings.Builder
	for _, tag := range input.Tags {
		tagsStr.WriteString(fmt.Sprintf("- %s\n", tag.Name))
	}

	// Get deterministic style index based on videoID
	h := hashString(input.VideoMetadata.ID)
	aggressiveStyleIdx := int(h % uint32(len(titleAggressiveStyles)))
	balancedStyleIdx := int((h / 10) % uint32(len(titleBalancedStyles)))
	intentSetIdx := int((h / 100) % uint32(len(searchIntentsTemplateSets)))

	// Deterministic order shuffle for searchIntents templates
	intentOrderSwapped := (h/1000)%2 == 1

	// Build style-specific examples
	code := input.VideoMetadata.RealCode
	aggressiveStyle := fmt.Sprintf(titleAggressiveStyles[aggressiveStyleIdx], code, code, code, firstCast)
	balancedStyle := fmt.Sprintf(titleBalancedStyles[balancedStyleIdx], code, code, code, code)

	// Build searchIntents template instruction (with order shuffle)
	intentSet := searchIntentsTemplateSets[intentSetIdx]
	templates := intentSet.Templates
	if intentOrderSwapped && len(templates) >= 2 {
		// Swap order
		templates = []string{templates[1], templates[0]}
	}
	var intentTemplateStr strings.Builder
	for _, tmpl := range templates {
		intentTemplateStr.WriteString(fmt.Sprintf("   - \"%s\"\n", fmt.Sprintf(tmpl, code)))
	}

	return fmt.Sprintf(`[ROLE]
คุณคือ SEO Title Specialist

[GOAL]
สร้าง Title 2 แบบ:
- Aggressive: ดึง CTR สูง
- Balanced: Google friendly

[CONTEXT]
Quick Answer: %s
Verdict: %s
Tone: %s
Strengths: %s

[INPUT]
- Code: %s
- Cast: %s
- Tags:
%s

---

[OUTPUT]

1. **titleAggressive** (50-60 ตัวอักษร)
   CTR focus - ดึงดูดให้คลิก

   🎯 STYLE ที่ต้องใช้:
   %s

   ⚠️ ใช้เป็น Meta Title
   ⚠️ ห้ามใช้ pattern เดียวกับบทความอื่น

2. **titleBalanced** (50-60 ตัวอักษร)
   Google friendly - สำหรับ H1

   🎯 STYLE ที่ต้องใช้:
   %s

   ⚠️ ใช้เป็น H1
   ⚠️ ต้อง professional

3. **metaDescription** (150-160 ตัวอักษร)
   ดึงดูด + มี keyword

   รูปแบบ:
   "[สรุปเรื่อง 1 ประโยค] [จุดเด่น] [CTA]"

   ตัวอย่าง:
   "%s ซับไทย เรื่องราวของ [ตัวละคร] ที่ [สถานการณ์] จุดเด่นคือ [อะไร] เหมาะกับคนชอบ [แนว] ดูเลยที่นี่"

4. **slug**
   URL-friendly

   รูปแบบ: "[code]-sub-thai" หรือ "[code]"

   ตัวอย่าง: "%s" (lowercase)

5. **keywords** (5-8 คำ)
   SEO keywords ที่คนค้นหา

   ต้องมี:
   - [CODE] ซับไทย
   - [CODE] รีวิว
   - [CODE] เรื่องย่อ
   - [ชื่อนักแสดง]
   - [แนวเรื่อง]

6. **searchIntents** (4-5 คำ) 🆕
   Search intent phrases สำหรับ internal linking

   ⚠️ สำคัญ: ต้อง UNIQUE ไม่ใช่ template เดิมทุกหน้า!

   สร้าง 2 ข้อจาก template ที่กำหนด:
%s
   สร้าง 2-3 ข้อจาก context (unique!):
   - "%s แนว%sไหม"
   - "%s เล่นดีไหมใน %s"
   - "%s [keyScene จากเรื่อง]"

   ❌ ห้าม copy template เดิมทุกหน้า
   ✅ ต้อง unique จาก content

7. **rating** (1-5) 🆕
   คะแนนรีวิว 1-5 scale (ทศนิยม 1 ตำแหน่ง)

   พิจารณาจาก:
   - Verdict (ดี/ไม่ดี)
   - จำนวน Strengths vs Weaknesses
   - Overall quality

   ตัวอย่าง: 4.2, 3.8, 4.5

---

[RULES]
- ✅ titleAggressive ต้องดึงดูด ใช้ style ที่กำหนด
- ✅ titleBalanced ต้อง professional ใช้ style ที่กำหนด
- ✅ metaDescription ต้องมี CTA
- ✅ slug ต้อง lowercase ไม่มีช่องว่าง
- ✅ searchIntents ต้องตรง keyword cluster
- ✅ rating ต้อง realistic (ไม่ให้ 5 ทุกเรื่อง)
- ❌ ห้ามใช้คำหยาบ
- ❌ ห้าม keywords: "หนังโป๊", "xxx", "av"

🚨 [ANTI-DUPLICATE TITLE RULE - สำคัญมาก!] 🚨
❌ ห้ามใช้โครงสร้าง title เดิมกับบทความอื่น
❌ ห้ามใช้ pattern "[CODE] ซับไทย รีวิวจริง..." ทุกหน้า
❌ ห้าม copy ตัวอย่างตรง ๆ ต้องปรับให้เหมาะกับ content

✅ Title ต้อง unique ตาม:
- ใช้ hook จาก verdict/strengths จริง
- ใช้ชื่อนักแสดงถ้าเด่น
- ใช้ tone/แนวเรื่องที่ต่าง
- ไม่ซ้ำ structure กับตัวอย่าง
`,
		chunk1.QuickAnswer,
		chunk1.Verdict,
		chunk3.Tone,
		strings.Join(chunk4.Strengths, ", "),
		input.VideoMetadata.RealCode,
		castNamesStr,
		tagsStr.String(),
		aggressiveStyle,
		balancedStyle,
		code,
		strings.ToLower(code),
		intentTemplateStr.String(),
		code,
		chunk3.Tone,
		firstCast,
		code,
		code,
	)
}
