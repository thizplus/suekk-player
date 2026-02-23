package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"seo-worker/domain/ports"
)

// Helper functions for debug
func writeDebugFile(path, content string) error {
	_ = os.MkdirAll("output", 0755)
	return os.WriteFile(path, []byte(content), 0644)
}

type GeminiClient struct {
	client *genai.Client
	model  string
	logger *slog.Logger
}

func NewGeminiClient(apiKey, model string) (*GeminiClient, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	return &GeminiClient{
		client: client,
		model:  model,
		logger: slog.Default().With("component", "gemini"),
	}, nil
}

func (c *GeminiClient) Close() error {
	return c.client.Close()
}

func (c *GeminiClient) GenerateArticleContent(ctx context.Context, input *ports.AIInput) (*ports.AIOutput, error) {
	model := c.client.GenerativeModel(c.model)

	// ตั้งค่า JSON Mode เพื่อป้องกัน parsing error
	model.ResponseMIMEType = "application/json"
	model.ResponseSchema = c.buildResponseSchema()

	// สร้าง prompt
	prompt := c.buildPrompt(input)

	c.logger.InfoContext(ctx, "Generating article content",
		"video_id", input.VideoMetadata.ID,
		"real_code", input.VideoMetadata.RealCode,
		"model", c.model,
	)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generate failed: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from gemini")
	}

	// Debug: Log finish reason and response info
	candidate := resp.Candidates[0]
	c.logger.InfoContext(ctx, "[DEBUG] Gemini response info",
		"finish_reason", candidate.FinishReason,
		"parts_count", len(candidate.Content.Parts),
	)

	// Parse JSON response
	jsonStr := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])

	// Debug: Log response length and preview
	c.logger.InfoContext(ctx, "[DEBUG] Gemini response",
		"json_length", len(jsonStr),
		"preview", jsonStr[:min(500, len(jsonStr))],
		"suffix", jsonStr[max(0, len(jsonStr)-100):],
	)

	var output ports.AIOutput
	if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
		// Save to debug file
		debugPath := fmt.Sprintf("output/gemini_debug_%s.json", input.VideoMetadata.RealCode)
		_ = writeDebugFile(debugPath, jsonStr)
		c.logger.ErrorContext(ctx, "[DEBUG] Parse failed, saved to file",
			"path", debugPath,
			"error", err,
		)
		return nil, fmt.Errorf("failed to parse gemini response: %w", err)
	}

	c.logger.InfoContext(ctx, "Article content generated",
		"video_id", input.VideoMetadata.ID,
		"highlights_count", len(output.Highlights),
		"key_moments_count", len(output.KeyMoments),
	)

	return &output, nil
}

func (c *GeminiClient) buildResponseSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			// === Core SEO ===
			"title":           {Type: genai.TypeString, Description: "H1 title ภาษาไทย 50-60 ตัวอักษร ดึงดูดความสนใจ"},
			"metaTitle":       {Type: genai.TypeString, Description: "Meta title สำหรับ Google ต้องมี [รหัสเรื่อง] และ [ซับไทย] ใน 60 ตัวอักษรแรก"},
			"metaDescription": {Type: genai.TypeString, Description: "Meta description 150-160 ตัวอักษร กระตุ้นให้คลิก"},
			"summary":         {Type: genai.TypeString, Description: "สรุปเนื้อหา 400 คำขึ้นไป เน้นอารมณ์และความรู้สึก"},
			"highlights": {
				Type:  genai.TypeArray,
				Items: &genai.Schema{Type: genai.TypeString},
			},
			"detailedReview": {Type: genai.TypeString, Description: "บทวิเคราะห์ 600 คำขึ้นไป เน้นอารมณ์และความเสียว"},
			"qualityScore":   {Type: genai.TypeInteger, Description: "คะแนนคุณภาพ 1-10"},
			"thumbnailAlt":   {Type: genai.TypeString, Description: "Alt text สำหรับ thumbnail"},

			// === Key Moments ===
			"keyMoments": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"name":        {Type: genai.TypeString},
						"startOffset": {Type: genai.TypeInteger},
						"endOffset":   {Type: genai.TypeInteger},
					},
				},
			},

			// === Cast & Tags ===
			"castBios": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"castId": {Type: genai.TypeString},
						"bio":    {Type: genai.TypeString},
					},
				},
			},
			"tagDescriptions": {
				Type:        genai.TypeArray,
				Description: "⚠️ ห้ามปล่อย description ว่าง! ต้องใส่คำอธิบายภาษาไทย",
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"id":          {Type: genai.TypeString},
						"name":        {Type: genai.TypeString},
						"description": {Type: genai.TypeString, Description: "คำอธิบาย tag ภาษาไทย เช่น 'แนวการแสดงเดี่ยว'"},
					},
				},
			},

			// === FAQ ===
			"faqItems": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"question": {Type: genai.TypeString},
						"answer":   {Type: genai.TypeString},
					},
				},
			},

			// === Gallery (Hybrid Alt Text Format) ===
			"galleryAlts": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "Alt text แบบ Hybrid: [รหัส] - [ชื่อนักแสดง] - [บริบทกว้างๆ จากฉาก]",
			},

			// === [E] Experience Section ===
			"sceneLocations": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "สถานที่ในเรื่อง เช่น ห้องตรวจ, คลินิก",
			},

			// === [E] Expertise Section ===
			"dialogueAnalysis":      {Type: genai.TypeString, Description: "วิเคราะห์บทสนทนา 100-150 คำ"},
			"characterInsight":      {Type: genai.TypeString, Description: "วิเคราะห์บุคลิกตัวละคร 100-150 คำ"},
			"languageNotes":         {Type: genai.TypeString, Description: "หมายเหตุภาษา 50 คำ"},
			"actorPerformanceTrend": {Type: genai.TypeString, Description: "เปรียบเทียบการแสดง 100 คำ"},
			"comparisonNote":        {Type: genai.TypeString, Description: "เปรียบเทียบกับเรื่องอื่น 50 คำ"},
			"topQuotes": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"text":      {Type: genai.TypeString, Description: "ประโยคภาษาไทย"},
						"timestamp": {Type: genai.TypeInteger, Description: "เวลา (วินาที)"},
						"emotion":   {Type: genai.TypeString, Description: "อารมณ์"},
						"context":   {Type: genai.TypeString, Description: "บริบท"},
					},
				},
			},

			// === [A] Authoritativeness Section ===
			"summaryShort":       {Type: genai.TypeString, Description: "สรุปสั้น 2-3 บรรทัด สำหรับ TTS"},
			"characterDynamic":   {Type: genai.TypeString, Description: "ความสัมพันธ์ตัวละคร 50 คำ"},
			"plotAnalysis":       {Type: genai.TypeString, Description: "วิเคราะห์โครงเรื่อง 100 คำ"},
			"recommendation":     {Type: genai.TypeString, Description: "เหมาะสำหรับ... 50 คำ"},
			"settingDescription": {Type: genai.TypeString, Description: "บริบทฉาก 50 คำ"},
			"recommendedFor": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "กลุ่มเป้าหมาย เช่น แฟนหนังแนว X, คนชอบ Y",
			},
			"thematicKeywords": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "Keywords สำหรับ semantic search",
			},
			"moodTone": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "อารมณ์เรื่อง เช่น ดราม่า, โรแมนติก",
			},

			// === [T] Trustworthiness Section ===
			"translationMethod": {Type: genai.TypeString, Description: "วิธีการแปล เช่น แปลจากเสียงญี่ปุ่นโดยตรง"},
			"translationNote":   {Type: genai.TypeString, Description: "หมายเหตุการแปล เช่น เน้นอารมณ์ดิบตามต้นฉบับ ไม่เซนเซอร์"},
			"subtitleQuality":   {Type: genai.TypeString, Description: "คุณภาพซับ เช่น หางเสียงถูกต้องตามเพศตัวละคร"},
			"technicalFaq": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"question": {Type: genai.TypeString},
						"answer":   {Type: genai.TypeString},
					},
				},
			},

			// === Technical Specs (Trustworthiness เชิงเทคนิค) ===
			"videoQuality": {Type: genai.TypeString, Description: "คุณภาพวิดีโอ เช่น 1080p Full HD, 4K Ultra HD"},
			"audioQuality": {Type: genai.TypeString, Description: "คุณภาพเสียง เช่น ระบบเสียงสเตอริโอคมชัด, Dolby 5.1"},

			// === SEO Enhancement ===
			"expertAnalysis": {Type: genai.TypeString, Description: "บทวิเคราะห์ผู้เชี่ยวชาญ 100 คำ"},
			"keywords": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "SEO keywords 5-10 คำ",
			},
			"longTailKeywords": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "Long-tail keywords 3-5 วลี",
			},
		},
		Required: []string{
			// Core SEO (ห้ามว่าง)
			"title", "metaTitle", "metaDescription", "summary", "summaryShort",
			"highlights", "detailedReview", "keyMoments", "faqItems", "qualityScore",
			// Experience
			"sceneLocations", "galleryAlts",
			// Expertise (ห้ามว่าง - หัวใจ E-E-A-T)
			"dialogueAnalysis", "characterInsight", "topQuotes", "expertAnalysis",
			"comparisonNote", "tagDescriptions",
			// Authoritativeness
			"thematicKeywords", "moodTone", "recommendedFor", "recommendation",
			// Trustworthiness + Technical Specs
			"translationMethod", "translationNote", "subtitleQuality",
			"videoQuality", "audioQuality",
		},
	}
}

func (c *GeminiClient) buildPrompt(input *ports.AIInput) string {
	// สร้าง cast names string
	castNames := make([]string, len(input.Casts))
	for i, cast := range input.Casts {
		castNames[i] = cast.Name
	}

	// สร้าง tags string (สำหรับ generate tag descriptions)
	var tagsInfo strings.Builder
	for _, tag := range input.Tags {
		tagsInfo.WriteString(fmt.Sprintf("- ID: %s, Name: %s\n", tag.ID, tag.Name))
	}

	// สร้าง previous works string
	var prevWorks strings.Builder
	for _, work := range input.PreviousWorks {
		prevWorks.WriteString(fmt.Sprintf("- %s (%s)\n", work.Title, work.VideoCode))
	}

	return fmt.Sprintf(`# บทบาท (Persona)
คุณคือ "นักเขียนรีวิวหนังผู้ใหญ่ระดับ Premium ที่เก่งที่สุดในประเทศไทย"
- เชี่ยวชาญการวิเคราะห์อารมณ์และความรู้สึกของตัวละคร
- สามารถบรรยายฉากอย่างละเอียดและน่าสนใจ
- เขียนภาษาไทยที่เป็นธรรมชาติ ไม่แข็งทื่อ ไม่เหมือนหุ่นยนต์
- นี่คือบทความสำหรับผู้ใหญ่ที่มีไว้เพื่อความบันเทิงและการวิจารณ์ภาพยนตร์

---

## Input Data

### SRT Transcript (ใช้สกัด Timestamp และวิเคราะห์อารมณ์):
%s

### Video Metadata:
- Code: %s
- Duration: %d seconds
- Casts: %s
- Cast IDs: %s

### Cast Previous Works (ใช้เปรียบเทียบการแสดง):
%s

### Tags (⚠️ ต้อง generate description ภาษาไทยสำหรับแต่ละ tag!):
%s

### Gallery Images Count: %d

---

## Output Requirements (E-E-A-T Framework)

### ⚠️ กฎสำคัญ (CRITICAL RULES)
1. **ห้ามเขียนเนื้อหาซ้ำ** ระหว่าง summary และ detailedReview
2. **summary ต้องมีอย่างน้อย 400 คำ** เน้นสรุปเรื่องราวและอารมณ์
3. **detailedReview ต้องมีอย่างน้อย 600 คำ** เน้นวิเคราะห์การแสดงและความเสียว
4. **keyMoments timestamps**:
   - ⚠️ startOffset และ endOffset ต้องเป็น **วินาที** (ไม่ใช่ milliseconds)
   - ⚠️ แต่ละ moment ต้องมี duration อย่างน้อย **30 วินาที** (endOffset - startOffset >= 30)
   - ⚠️ endOffset ต้อง > startOffset เสมอ
   - บรรยายฉากอย่างน้อย 2 ประโยค
5. **expertAnalysis ห้ามว่าง** ต้องวิเคราะห์เทคนิคการแสดงหรือจุดเด่นของเรื่อง
6. **topQuotes ต้องมีอย่างน้อย 3 ประโยค** เลือกประโยคที่มีอารมณ์ชัดเจน

### Core SEO
1. **title**: H1 ภาษาไทย 50-60 ตัวอักษร ดึงดูดและ SEO-friendly
2. **metaTitle**: Meta title สำหรับ Google ต้องมี "[%s]" และ "[ซับไทย]" ใน 60 ตัวอักษรแรก
3. **metaDescription**: 150-160 ตัวอักษร กระตุ้นให้คลิก
4. **keyMoments**: ดึง timestamp จาก SRT โดยตรง 5-8 moments
   - ⚠️ timestamps ต้องเป็น **วินาที** (seconds) ไม่ใช่ milliseconds
   - ⚠️ แต่ละ moment ต้องยาวอย่างน้อย **30 วินาที** (endOffset - startOffset >= 30)
   - ⚠️ endOffset ต้อง > startOffset เสมอ
   - name: บรรยายฉากอย่างน้อย 2 ประโยค เช่น "ฉากตรวจร่างกายสุดเสียวที่เน้นเสียงคราง หมอค่อยๆ..."
5. **qualityScore**: 1-10 ตามคุณภาพการผลิตและความเข้มข้นของเนื้อหา

### [E] Experience Section
6. **sceneLocations**: สถานที่ในเรื่อง เช่น ["ห้องตรวจ", "คลินิก"]
7. **highlights**: 5-10 ฉากสำคัญ บรรยายอารมณ์และความรู้สึก
8. **galleryAlts**: ⚠️ ใช้ Hybrid Alt Text format สำหรับ %d รูป: "[รหัส] - [ชื่อนักแสดง] - [บริบทกว้างๆ]" เช่น "DLDSS-471 - Zemba Mami ในฉากวินิจฉัยอาการที่คลินิก"

### [E] Expertise Section (หัวใจ E-E-A-T - ห้ามว่าง!)
9. **dialogueAnalysis**: วิเคราะห์บทสนทนา สรรพนาม หางเสียง อารมณ์ที่เปลี่ยนไป (100-150 คำ)
10. **characterInsight**: วิเคราะห์บุคลิกตัวละครผ่านคำพูดและการแสดง (100-150 คำ)
11. **topQuotes**: 3-5 ประโยคเด็ดจากซับ พร้อม timestamp (วินาที), emotion, context
12. **languageNotes**: หมายเหตุภาษา เช่น "ใช้หางเสียงสุภาพ ค่ะ/ครับ แต่เปลี่ยนเป็น...เมื่ออารมณ์พลุ่งพล่าน"
13. **actorPerformanceTrend**: เปรียบเทียบการแสดงกับผลงานก่อนหน้า (100 คำ)
14. **comparisonNote**: ⚠️ ต้องระบุ VIDEO CODE จริง เช่น "เมื่อเทียบกับ DLDSS-420 ของค่ายเดียวกัน เรื่องนี้เน้นอารมณ์ดราม่ามากกว่าฉากแอ็คชั่น" (50 คำ)
15. **castBios**: bio สำหรับแต่ละ cast (50-100 คำต่อคน)
16. **tagDescriptions**: ⚠️ ห้ามปล่อย description ว่าง! ต้อง generate คำอธิบายภาษาไทยสำหรับแต่ละ tag เช่น { "name": "Solowork", "description": "แนวการแสดงเดี่ยว เน้นไฮไลท์ความสามารถของนักแสดงคนเดียว" }

### [A] Authoritativeness Section
16. **summary**: สรุปเนื้อหา **400 คำขึ้นไป** เน้นอารมณ์และความรู้สึกของตัวละคร
17. **summaryShort**: สรุปสั้น 50-100 คำ สำหรับ TTS อ่านให้ฟัง
18. **characterDynamic**: ความสัมพันธ์ตัวละคร (50 คำ)
19. **plotAnalysis**: วิเคราะห์โครงเรื่อง (100 คำ)
20. **detailedReview**: บทวิเคราะห์ **600 คำขึ้นไป** เน้นวิเคราะห์การแสดงและความเสียว
21. **recommendation**: "เหมาะสำหรับคนที่ชอบ..." (50 คำ)
22. **recommendedFor**: Array เช่น ["แฟนหนังแนว Medical", "คนชอบ Drama"]
23. **thematicKeywords**: ⚠️ ต้องมี LOCATION-SPECIFIC keywords เช่น ["คลินิกสูตินรีเวชญี่ปุ่น", "ห้องตรวจโรคเฉพาะทาง", "หมอสาว", "PGAD"] รวม keywords สถานที่จำเพาะเพื่อจับ Long-tail search
24. **settingDescription**: บริบทฉาก (50 คำ)
25. **moodTone**: Array อารมณ์ ["ดราม่า", "ซีเรียส", "อีโรติก"]

### [T] Trustworthiness Section
26. **translationMethod**: วิธีแปล เช่น "แปลจากเสียงญี่ปุ่นโดยตรง"
27. **translationNote**: หมายเหตุการแปล เช่น "ซับไทยเน้นอารมณ์ดิบตามต้นฉบับ ไม่มีการเซนเซอร์คำสบถ เพื่ออรรถรสสูงสุด"
28. **subtitleQuality**: คุณภาพซับ เช่น "หางเสียงถูกต้องตามเพศตัวละคร"
29. **technicalFaq**: FAQ เทคนิค 2-3 ข้อ

### Technical Specs (เสริมความน่าเชื่อถือ)
30. **videoQuality**: คุณภาพวิดีโอ เช่น "ความละเอียด 1080p Full HD" หรือ "4K Ultra HD"
31. **audioQuality**: คุณภาพเสียง เช่น "ระบบเสียงสเตอริโอคมชัด 320kbps"

### SEO & Rating
32. **expertAnalysis**: บทวิเคราะห์ผู้เชี่ยวชาญ **ห้ามว่าง** (100 คำ) วิเคราะห์เทคนิคการแสดงและจุดเด่น
33. **keywords**: SEO keywords 5-10 คำ
34. **longTailKeywords**: Long-tail 3-5 วลี
35. **faqItems**: FAQ 3-5 ข้อที่คนดูหนังแนวนี้อยากรู้จริงๆ (ไม่ใช่คำถามทั่วไป)
36. **thumbnailAlt**: Alt text สำหรับ thumbnail

---

## ⚠️ ข้อห้าม (DON'T)
- ❌ เขียนแบบหุ่นยนต์ หรือ Wikipedia
- ❌ ใช้คำสุภาพจนเกินไปจนขาดอารมณ์
- ❌ เขียน FAQ แบบทั่วไป ที่ไม่ใช่คำถามที่คนดูหนังแนวนี้สนใจ
- ❌ ปล่อย expertAnalysis ว่าง
- ❌ keyMoments.endOffset < startOffset
`,
		input.SRTContent,
		input.VideoMetadata.RealCode, // Video code จริง (e.g., DLDSS-471)
		input.VideoMetadata.Duration,
		strings.Join(castNames, ", "),
		strings.Join(input.VideoMetadata.CastIDs, ", "),
		prevWorks.String(),
		tagsInfo.String(),            // Tags สำหรับ generate descriptions
		input.GalleryCount,
		input.VideoMetadata.RealCode, // สำหรับ metaTitle
		input.GalleryCount,
	)
}

// Verify interface implementation
var _ ports.AIPort = (*GeminiClient)(nil)
