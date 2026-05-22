package th

import (
	"fmt"
	"hash/fnv"
	"strings"
)

// ============================================================================
// Thai Review Prompts (V3 Intent-Driven)
// ============================================================================

// PromptParams contains all data needed for prompt generation
type PromptParams struct {
	VideoCode     string
	VideoID       string
	Duration      int // seconds
	ReleaseDate   string
	CastNames     []string
	Tags          []string
	MakerName     string
	SRTContent    string
	GalleryCount  int
}

// Chunk1Context is the output from Chunk 1
type Chunk1Context struct {
	QuickAnswer string
	Verdict     string
}

// Chunk3Context is the output from Chunk 3
type Chunk3Context struct {
	Synopsis  string
	StoryFlow string
	Tone      string
	KeyScenes []string
}

// Chunk4Context is the output from Chunk 4
type Chunk4Context struct {
	ReviewSummary  string
	Strengths      []string
	Weaknesses     []string
	WhoShouldWatch string
	VerdictReason  string
}

// ============================================================================
// Opening Styles (Anti-Similarity)
// ============================================================================

var openingStyles = map[string]string{
	"A": `Style A - Direct: เริ่มด้วยข้อมูลตรงๆ
ตัวอย่าง: "เรื่องนี้เล่าถึง [ตัวละคร] ที่ [สถานการณ์]..."`,

	"B": `Style B - Question: เริ่มด้วยคำถาม
ตัวอย่าง: "เคยสงสัยไหมว่าถ้า [สถานการณ์]? เรื่องนี้ตอบคำถามนั้น..."`,

	"C": `Style C - Hook: เริ่มด้วย hook ดึงดูด
ตัวอย่าง: "ถ้าคุณชอบแนว [แนว] นี่คือเรื่องที่ต้องดู..."`,

	"D": `Style D - Context: เริ่มด้วยบริบท
ตัวอย่าง: "จากค่าย [ค่าย] ที่เน้นแนว [แนว] มาตลอด ผลงานนี้ก็ไม่ผิดหวัง..."`,
}

var reviewTones = map[string]string{
	"Analytical": `Analytical: วิเคราะห์เชิงลึก จุดเด่น จุดอ่อน ให้คะแนน
- ใช้ภาษาเป็นทางการ
- มีเหตุผลประกอบ
- เน้นข้อมูล`,

	"Conversational": `Conversational: เล่าให้ฟังแบบเพื่อนคุยกัน
- ใช้ภาษาไม่เป็นทางการ
- มีอารมณ์ร่วม
- เน้นความรู้สึก`,

	"Comparative": `Comparative: เทียบกับเรื่องอื่นในแนวเดียวกัน
- เปรียบเทียบกับผลงานอื่น
- หาจุดเด่นที่ต่าง
- ให้มุมมองกว้าง`,
}

var faqStyles = map[string]string{
	"Formal": `Formal: คำถามเป็นทางการ
ตัวอย่าง: "[CODE] เกี่ยวกับอะไร?"`,

	"Casual": `Casual: คำถามไม่เป็นทางการ
ตัวอย่าง: "เล่าให้ฟังหน่อยว่า [CODE] เป็นแนวไหน?"`,
}

// GetOpeningStylePrompt returns the prompt for opening style
func GetOpeningStylePrompt(style string) string {
	if prompt, ok := openingStyles[style]; ok {
		return prompt
	}
	return openingStyles["A"]
}

// GetReviewTonePrompt returns the prompt for review tone
func GetReviewTonePrompt(tone string) string {
	if prompt, ok := reviewTones[tone]; ok {
		return prompt
	}
	return reviewTones["Analytical"]
}

// GetFAQStylePrompt returns the prompt for FAQ style
func GetFAQStylePrompt(style string) string {
	if prompt, ok := faqStyles[style]; ok {
		return prompt
	}
	return faqStyles["Formal"]
}

// ============================================================================
// Content Variation (deterministic by videoID)
// ============================================================================

// ContentVariation defines the style for content generation
type ContentVariation struct {
	OpeningStyle string // A, B, C, D
	ReviewTone   string // Analytical, Conversational, Comparative
	FAQStyle     string // Formal, Casual
}

// GetContentVariation returns variation based on videoID (deterministic)
func GetContentVariation(videoID string) ContentVariation {
	hash := hashString(videoID)

	// Opening Style: 4 options (A, B, C, D)
	openingStyleKeys := []string{"A", "B", "C", "D"}
	openingIdx := hash % uint32(len(openingStyleKeys))

	// Review Tone: 3 options
	reviewToneKeys := []string{"Analytical", "Conversational", "Comparative"}
	reviewIdx := (hash / 4) % uint32(len(reviewToneKeys))

	// FAQ Style: 2 options
	faqStyleKeys := []string{"Formal", "Casual"}
	faqIdx := (hash / 12) % uint32(len(faqStyleKeys))

	return ContentVariation{
		OpeningStyle: openingStyleKeys[openingIdx],
		ReviewTone:   reviewToneKeys[reviewIdx],
		FAQStyle:     faqStyleKeys[faqIdx],
	}
}

// TitleStyleIndices holds indices for title styles (for Chunk 6)
type TitleStyleIndices struct {
	AggressiveIdx      int
	BalancedIdx        int
	IntentSetIdx       int
	IntentOrderSwapped bool
}

// GetTitleStyleIndices returns style indices for Chunk 6 (deterministic)
func GetTitleStyleIndices(videoID string) TitleStyleIndices {
	h := hashString(videoID)

	return TitleStyleIndices{
		AggressiveIdx:      int(h % 4),
		BalancedIdx:        int((h / 10) % 4),
		IntentSetIdx:       int((h / 100) % 6),
		IntentOrderSwapped: (h/1000)%2 == 1,
	}
}

// hashString generates deterministic hash from string
func hashString(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// TruncateSRT limits SRT content to maxLen characters
func TruncateSRT(srt string, maxLen int) string {
	if len(srt) <= maxLen {
		return srt
	}
	return srt[:maxLen] + "..."
}

// TruncateToWords limits text to approximately maxWords
func TruncateToWords(text string, maxWords int) string {
	words := 0
	lastSpace := 0
	for i, r := range text {
		if r == ' ' || r == '\n' {
			words++
			lastSpace = i
			if words >= maxWords {
				return text[:lastSpace] + "..."
			}
		}
	}
	return text
}

// ============================================================================
// Chunk 1: Search Intent Answer
// ============================================================================

func BuildChunk1Prompt(params *PromptParams) string {
	castNamesStr := strings.Join(params.CastNames, ", ")

	variation := GetContentVariation(params.VideoID)
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

⚠️ ใช้ Opening Style ที่กำหนดให้ข้างบนเท่านั้น!
`,
		params.VideoCode,
		params.VideoCode,
		params.VideoCode,
		params.Duration/60,
		castNamesStr,
		TruncateSRT(params.SRTContent, 2000),
		castNamesStr,
		params.VideoCode,
		castNamesStr,
		params.VideoCode,
		variation.OpeningStyle,
		openingStylePrompt,
	)
}

// ============================================================================
// Chunk 2: Structured Facts
// ============================================================================

func BuildChunk2Prompt(params *PromptParams) string {
	castNamesStr := strings.Join(params.CastNames, ", ")

	var tagsStr strings.Builder
	for _, tag := range params.Tags {
		tagsStr.WriteString(fmt.Sprintf("- %s\n", tag))
	}

	makerName := params.MakerName
	if makerName == "" {
		makerName = "ไม่ระบุ"
	}

	return fmt.Sprintf(`[ROLE]
คุณคือ Data Extractor

[GOAL]
ดึงข้อมูล facts จาก metadata
ใช้สำหรับ Facts Table และ JSON-LD Schema

[INPUT]
- Code: %s
- Studio/Maker: %s
- Cast: %s
- Duration: %d seconds
- Release Date: %s
- Tags:
%s

---

[OUTPUT]

1. **code**: "%s"

2. **studio**: "%s"

3. **cast**: ["%s"] (ใส่ทุกคน)

4. **duration**: "%d นาที"

5. **genre**: ดึงจาก tags 2-4 แนวที่เหมาะสม
   - เลือกแนวที่คนค้นหา เช่น Drama, Romance, Office, Medical

6. **releaseYear**: ดึงจาก release date

7. **subtitleAvailable**: true (เว็บเรามีซับไทย)

---

[RULES]
- ✅ ข้อมูลต้องถูกต้องตาม metadata
- ✅ genre เลือกจาก tags ที่ให้มา
- ✅ ใช้ภาษาไทยสำหรับ duration
`,
		params.VideoCode,
		makerName,
		castNamesStr,
		params.Duration,
		params.ReleaseDate,
		tagsStr.String(),
		params.VideoCode,
		makerName,
		strings.Join(params.CastNames, `", "`),
		params.Duration/60,
	)
}

// ============================================================================
// Chunk 3: Story Recap
// ============================================================================

func BuildChunk3Prompt(params *PromptParams, chunk1 *Chunk1Context) string {
	castNamesStr := strings.Join(params.CastNames, ", ")

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
		params.VideoCode,
		castNamesStr,
		params.Duration/60,
		TruncateSRT(params.SRTContent, 3000),
		castNamesStr,
	)
}

// ============================================================================
// Chunk 4: Review Section
// ============================================================================

func BuildChunk4Prompt(params *PromptParams, chunk1 *Chunk1Context, chunk3 *Chunk3Context) string {
	castNamesStr := strings.Join(params.CastNames, ", ")

	variation := GetContentVariation(params.VideoID)
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

3. **weaknesses** (2-3 ข้อ)
   จุดอ่อน แต่ละข้อ 10-20 คำ
   ⚠️ ต้องมีอย่างน้อย 1 ข้อ (ไม่มีเรื่องไหนสมบูรณ์แบบ)

4. **whoShouldWatch** (50-80 คำ)
   บอกชัดๆ ว่า:
   - เหมาะกับ: [กลุ่มคน]
   - ไม่เหมาะกับ: [กลุ่มคน]

5. **verdictReason** (30-50 คำ)
   เหตุผลสรุป 1-2 ประโยค

---

[RULES]
- ✅ ตรงๆ ไม่อ้อม
- ✅ verdict ต้องชัด
- ✅ ใช้ชื่อ: %s
- ❌ ห้ามใช้คำหยาบ
- ❌ ห้ามเขียนแบบ essay
- ⚠️ ใช้ Review Tone ที่กำหนดให้ข้างบนเท่านั้น!
`,
		variation.ReviewTone,
		reviewTonePrompt,
		chunk1.QuickAnswer,
		chunk1.Verdict,
		TruncateToWords(chunk3.Synopsis, 100),
		chunk3.Tone,
		params.VideoCode,
		castNamesStr,
		castNamesStr,
	)
}

// ============================================================================
// Chunk 5: FAQ Intent Block
// ============================================================================

func BuildChunk5Prompt(params *PromptParams, chunk1 *Chunk1Context, chunk3 *Chunk3Context, chunk4 *Chunk4Context) string {
	castNamesStr := strings.Join(params.CastNames, ", ")
	firstCast := ""
	if len(params.CastNames) > 0 {
		firstCast = params.CastNames[0]
	}

	strengthsStr := strings.Join(chunk4.Strengths, ", ")

	variation := GetContentVariation(params.VideoID)
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
		variation.FAQStyle,
		faqStylePrompt,
		chunk1.QuickAnswer,
		chunk1.Verdict,
		TruncateToWords(chunk3.Synopsis, 80),
		strengthsStr,
		chunk4.WhoShouldWatch,
		params.VideoCode,
		castNamesStr,
		params.VideoCode,
		params.VideoCode,
		params.VideoCode,
		firstCast,
		params.VideoCode,
		params.VideoCode,
		castNamesStr,
	)
}

// ============================================================================
// Chunk 6: SEO Output
// ============================================================================

var titleAggressiveStyles = []string{
	`ใช้รูปแบบ: "[CODE] ซับไทย [คำถาม]?"`,
	`ใช้รูปแบบ: "[CODE] ซับไทย [ตัวเลข] [สิ่งที่น่าสนใจ]"`,
	`ใช้รูปแบบ: "[CODE] ซับไทย [Emotion/Reaction]"`,
	`ใช้รูปแบบ: "[CODE] ซับไทย [จุดเด่นที่ต่าง]"`,
}

var titleBalancedStyles = []string{
	`ใช้รูปแบบ: "รีวิว [CODE] ซับไทย [ข้อมูลเพิ่ม]"`,
	`ใช้รูปแบบ: "[CODE] ซับไทย [Guide]"`,
	`ใช้รูปแบบ: "[CODE] ซับไทย สรุป [อะไร]"`,
	`ใช้รูปแบบ: "[CODE] ซับไทย [ครบจบ]"`,
}

var searchIntentsTemplateSets = [][]string{
	{"%s ซับไทย", "%s รีวิว"},
	{"ดู %s ซับไทย", "รีวิวจริง %s"},
	{"%s ดูที่ไหน", "%s เรื่องย่อ"},
	{"%s ซับไทย ดีไหม", "รีวิว %s ซับไทย"},
	{"ดู %s ได้ที่ไหน", "%s สรุป"},
	{"%s แนะนำ", "%s ความเห็น"},
}

func BuildChunk6Prompt(params *PromptParams, chunk1 *Chunk1Context, chunk3 *Chunk3Context, chunk4 *Chunk4Context) string {
	castNamesStr := strings.Join(params.CastNames, ", ")
	firstCast := ""
	if len(params.CastNames) > 0 {
		firstCast = params.CastNames[0]
	}

	var tagsStr strings.Builder
	for _, tag := range params.Tags {
		tagsStr.WriteString(fmt.Sprintf("- %s\n", tag))
	}

	// Get deterministic style indices
	tsi := GetTitleStyleIndices(params.VideoID)

	code := params.VideoCode
	aggressiveStyle := titleAggressiveStyles[tsi.AggressiveIdx]
	balancedStyle := titleBalancedStyles[tsi.BalancedIdx]

	// Build searchIntents template instruction
	templates := searchIntentsTemplateSets[tsi.IntentSetIdx]
	if tsi.IntentOrderSwapped && len(templates) >= 2 {
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
   🎯 STYLE: %s

2. **titleBalanced** (50-60 ตัวอักษร)
   Google friendly - สำหรับ H1
   🎯 STYLE: %s

3. **metaDescription** (150-160 ตัวอักษร)
   ดึงดูด + มี keyword

4. **slug**
   รูปแบบ: "%s" (lowercase)

5. **keywords** (5-8 คำ)
   ต้องมี:
   - [CODE] ซับไทย
   - [CODE] รีวิว
   - [CODE] เรื่องย่อ
   - [ชื่อนักแสดง]
   - [แนวเรื่อง]

6. **searchIntents** (4-5 คำ)
   สร้าง 2 ข้อจาก template:
%s
   สร้าง 2-3 ข้อจาก context (unique!):
   - "%s แนว%sไหม"
   - "%s เล่นดีไหมใน %s"

7. **rating** (1-5)
   คะแนนรีวิว (ทศนิยม 1 ตำแหน่ง)

---

[RULES]
- ✅ titleAggressive ต้องดึงดูด
- ✅ titleBalanced ต้อง professional
- ✅ metaDescription ต้องมี CTA
- ✅ rating ต้อง realistic
- ❌ ห้ามใช้คำหยาบ
- ❌ ห้าม keywords: "หนังโป๊", "xxx", "av"
`,
		chunk1.QuickAnswer,
		chunk1.Verdict,
		chunk3.Tone,
		strings.Join(chunk4.Strengths, ", "),
		code,
		castNamesStr,
		tagsStr.String(),
		aggressiveStyle,
		balancedStyle,
		strings.ToLower(code),
		intentTemplateStr.String(),
		code,
		chunk3.Tone,
		firstCast,
		code,
	)
}
