package ai

import (
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 3: Technical + FAQ
// Focus: Trustworthiness & SEO keywords
// ============================================================================

// buildChunk3Schema สร้าง JSON Schema สำหรับ Chunk 3
func (c *GeminiClient) buildChunk3Schema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			// === [T] Trustworthiness Section ===
			"translationMethod": {
				Type:        genai.TypeString,
				Description: "วิธีการแปล เช่น แปลจากเสียงญี่ปุ่นโดยตรง",
			},
			"translationNote": {
				Type:        genai.TypeString,
				Description: "หมายเหตุการแปล เช่น ซับไทยเน้นอารมณ์ดิบตามต้นฉบับ",
			},
			"subtitleQuality": {
				Type:        genai.TypeString,
				Description: "คุณภาพซับ เช่น หางเสียงถูกต้องตามเพศตัวละคร",
			},
			"videoQuality": {
				Type:        genai.TypeString,
				Description: "คุณภาพวิดีโอ เช่น 1080p Full HD, 4K Ultra HD",
			},
			"audioQuality": {
				Type:        genai.TypeString,
				Description: "คุณภาพเสียง เช่น ระบบเสียงสเตอริโอคมชัด 320kbps",
			},
			"technicalFaq": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"question": {Type: genai.TypeString},
						"answer":   {Type: genai.TypeString, Description: "คำตอบยาว 40-60 คำ"},
					},
					Required: []string{"question", "answer"},
				},
				Description: "FAQ เทคนิค 3-4 ข้อ (ซับไทย, คุณภาพวิดีโอ, เสียง)",
			},

			// === FAQ ===
			"faqItems": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"question": {Type: genai.TypeString},
						"answer":   {Type: genai.TypeString, Description: "คำตอบยาว 50-80 คำ ⚠️ ห้ามใช้คำว่า: หลั่งใน, แตกใน, Creampie, เซ็กส์, ร่วมเพศ"},
					},
					Required: []string{"question", "answer"},
				},
				Description: "FAQ 8-10 ข้อ (สำคัญมาก! ช่วย SEO) ⚠️ ใช้ภาษาอ้อมค้อม เช่น 'ฉากโรแมนติก' แทน 'ฉากเซ็กส์'",
			},

			// === SEO Keywords ===
			"keywords": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "SEO keywords 5-10 คำ ⚠️ ห้ามใช้คำว่า 'หนังโป๊', 'xxx', 'av' - ใช้คำทางเลือกเช่น 'หนังรัก', 'หนังญี่ปุ่น'",
			},
			"longTailKeywords": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "Long-tail keywords 3-5 วลี ⚠️ ห้ามใช้คำโป๊โดยตรง",
			},
		},
		Required: []string{
			"translationMethod", "translationNote", "subtitleQuality",
			"videoQuality", "audioQuality", "technicalFaq",
			"faqItems", "keywords", "longTailKeywords",
		},
	}
}

// buildChunk3Prompt สร้าง prompt สำหรับ Chunk 3
// ⚠️ CRITICAL: รับ context จาก Chunk 1 เพื่อให้ FAQ ตรงกับเนื้อเรื่อง
func (c *GeminiClient) buildChunk3Prompt(input *ports.AIInput, chunk1 *Chunk1Output) string {
	// สร้าง cast names string
	castNames := make([]string, len(input.Casts))
	for i, cast := range input.Casts {
		castNames[i] = cast.Name
	}

	// สร้าง tags string
	var tagsStr strings.Builder
	for _, tag := range input.Tags {
		tagsStr.WriteString(fmt.Sprintf("- %s\n", tag.Name))
	}

	// สร้าง highlights string
	highlightsStr := ""
	for i, h := range chunk1.Highlights {
		highlightsStr += fmt.Sprintf("%d. %s\n", i+1, h)
	}

	// Format duration to readable Thai format
	durationStr := formatDurationThai(input.VideoMetadata.Duration)

	return fmt.Sprintf(`# บทบาท (Persona)
คุณคือ "ผู้เชี่ยวชาญด้าน SEO และ Technical Content สำหรับเว็บไซต์หนังผู้ใหญ่"
- เชี่ยวชาญการเขียน FAQ ที่ตอบคำถามที่คนค้นหาจริง
- สามารถวิเคราะห์คุณภาพเทคนิคของวิดีโอและซับไตเติ้ล
- เขียน SEO keywords ที่มี search intent ชัดเจน

---

## ⚠️ Context จาก Chunk 1 (ใช้เป็นฐานในการเขียน FAQ)

### Summary (อ้างอิงเนื้อเรื่องจากที่นี่):
%s

### Highlights (ฉากสำคัญ - ใช้ตอบ FAQ):
%s

---

## ข้อมูล Video

- Code: %s
- Duration: %s
- Casts: %s
- Tags:
%s

### ตัวอย่างจาก SRT (ใช้ดูคุณภาพซับ):
%s

---

## Output Requirements (Chunk 3: Technical + FAQ)

### [T] Trustworthiness Section
1. **translationMethod**: วิธีการแปล (เช่น "แปลจากเสียงญี่ปุ่นโดยตรงโดยผู้เชี่ยวชาญ")
2. **translationNote**: หมายเหตุการแปล (เช่น "ซับไทยเน้นอารมณ์ดิบตามต้นฉบับ ไม่มีการเซนเซอร์")
3. **subtitleQuality**: คุณภาพซับ (เช่น "หางเสียงถูกต้องตามเพศตัวละคร ไทม์มิ่งแม่นยำ")
4. **videoQuality**: คุณภาพวิดีโอ (เช่น "ความละเอียด 1080p Full HD")
5. **audioQuality**: คุณภาพเสียง (เช่น "ระบบเสียงสเตอริโอคมชัด 320kbps")
6. **technicalFaq**: FAQ เทคนิค 2-3 ข้อ เช่น
   - "ซับไทยตรงกับต้นฉบับไหม?" → "ซับไทยแปลจากเสียงต้นฉบับโดยตรง..."

### FAQ Section (สำคัญมาก! ช่วย SEO และ People Also Ask)
7. **faqItems**: ⚠️ FAQ **8-10 ข้อ** ที่คนดูหนังแนวนี้อยากรู้จริงๆ
   - ⚠️ **ต้องมี 8-10 ข้อ!** (ไม่ใช่ 3-5 ข้อ) - FAQ เยอะ = Text เยอะ = SEO ดี
   - ⚠️ **คำตอบต้องยาว 50-80 คำ** ไม่ใช่ตอบสั้นๆ
   - ⚠️ ห้ามเขียน FAQ แบบทั่วไป ที่ไม่ใช่คำถามที่คนดูหนังแนวนี้สนใจ
   - ⚠️ ห้ามใช้ placeholder เช่น "XXXXX" หรือ "เว็บไซต์นี้" ให้ใช้ชื่อ "SubTH" แทน
   - ⚠️ **ห้ามใช้คำเหล่านี้ในคำตอบ:** หลั่งใน, แตกใน, Creampie, เซ็กส์, ร่วมเพศ
   - ✅ **ใช้คำอ้อมค้อมแทน:**
     * "ฉากหลั่งใน" → "ฉากโรแมนติกแบบใกล้ชิด"
     * "ฉากเซ็กส์" → "ฉากรักใคร่"
     * "Creampie" → "ฉากจบแบบพิเศษ"
   - ✅ **ตัวอย่าง FAQ ที่ดี (ต้องมีครบทุกแนว):**
     1. "[รหัส] เกี่ยวกับอะไร?" → สรุปเนื้อเรื่อง
     2. "[นักแสดง] เล่นเป็นใคร?" → อธิบายบทบาท
     3. "เรื่องนี้มีฉากอะไรบ้าง?" → บรรยายฉากสำคัญ
     4. "ความยาวเรื่องนี้เท่าไหร่?" → บอกความยาว + เหมาะกับใคร
     5. "[รหัส] ซับไทยดีไหม?" → ชมคุณภาพซับ
     6. "หา [รหัส] ดูได้ที่ไหน?" → แนะนำ SubTH
     7. "[นักแสดง] มีผลงานอื่นอีกไหม?" → แนะนำเรื่องอื่น
     8. "เรื่องนี้เหมาะกับใคร?" → บอกกลุ่มเป้าหมาย
     9. "[รหัส] คุณภาพวิดีโอเป็นอย่างไร?" → บอก specs
     10. "ข้อดีข้อเสียของ [รหัส]?" → วิเคราะห์

### SEO Keywords
8. **keywords**: SEO keywords 5-10 คำ
   - รวม: รหัสเรื่อง, ชื่อนักแสดง, ชื่อค่าย, แนวเรื่อง
   - ตัวอย่าง: ["%s", "%s ซับไทย", "หนังแนว Medical"]
   - ⚠️ **ห้ามใช้คำเหล่านี้:** "หนังโป๊", "xxx", "av", "หนังผู้ใหญ่", "หนังเอ็กซ์"
   - ✅ ใช้คำทางเลือก: "หนังญี่ปุ่น", "หนังรัก", "ซีรีส์ญี่ปุ่น"

9. **longTailKeywords**: Long-tail keywords 3-5 วลี
   - ตัวอย่าง: ["%s รีวิว ซับไทย", "%s เรื่องย่อ", "หนัง %s ดูฟรี"]
   - ⚠️ ห้ามใช้คำโป๊โดยตรง

---

## ⚠️ ข้อห้าม (DON'T)
- ❌ เขียน FAQ แบบทั่วไป (เช่น "หนังเรื่องนี้ดีไหม?")
- ❌ keywords ที่ไม่มี search volume
- ❌ **keywords ที่มีคำว่า "หนังโป๊", "xxx", "av"**
- ❌ ปล่อย field ใดว่าง
- ❌ **ผสมภาษาในชื่อนักแสดง** เช่น "เซ็นมะ Mami", "Zemba มามิ" ← ผิด! REJECT ทันที!
- ✅ **ใช้ชื่อ ENGLISH ตามที่ให้มาเท่านั้น** - copy จาก Casts info โดยตรง ห้ามแปลง!
`,
		chunk1.Summary,                     // Context from Chunk 1
		highlightsStr,                      // Highlights from Chunk 1
		input.VideoMetadata.RealCode,
		durationStr,                        // Formatted duration (e.g., "2 ชั่วโมง 2 นาที")
		strings.Join(castNames, ", "),
		tagsStr.String(),
		truncateSRT(input.SRTContent, 500), // แค่ตัวอย่าง ไม่ต้องส่งทั้งหมด
		input.VideoMetadata.RealCode,
		input.VideoMetadata.RealCode,
		input.VideoMetadata.RealCode,
		input.VideoMetadata.RealCode,
		castNames[0], // ใช้ชื่อนักแสดงคนแรก
	)
}

// truncateSRT ตัด SRT ให้สั้นลงสำหรับ Chunk 3 (ไม่ต้องใช้ทั้งหมด)
func truncateSRT(srt string, maxLen int) string {
	if len(srt) <= maxLen {
		return srt
	}
	return srt[:maxLen] + "\n... [truncated for brevity]"
}

// formatDurationThai แปลง seconds เป็น format ภาษาไทยที่อ่านง่าย
// เช่น 7331 → "2 ชั่วโมง 2 นาที"
func formatDurationThai(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60

	if hours > 0 && minutes > 0 {
		return fmt.Sprintf("%d ชั่วโมง %d นาที", hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%d ชั่วโมง", hours)
	} else if minutes > 0 {
		return fmt.Sprintf("%d นาที", minutes)
	}
	return fmt.Sprintf("%d วินาที", seconds)
}
