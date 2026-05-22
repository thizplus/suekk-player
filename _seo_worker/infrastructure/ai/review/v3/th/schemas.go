package th

import (
	"github.com/google/generative-ai-go/genai"
)

// ============================================================================
// Thai Review Schemas (V3 Intent-Driven)
// Genai schema definitions for structured JSON output
// ============================================================================

// BuildChunk1Schema returns the schema for Chunk 1: Search Intent Answer
func BuildChunk1Schema() *genai.Schema {
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

// BuildChunk2Schema returns the schema for Chunk 2: Structured Facts
func BuildChunk2Schema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"code": {
				Type:        genai.TypeString,
				Description: "รหัสเรื่อง เช่น DASS-541",
			},
			"studio": {
				Type:        genai.TypeString,
				Description: "ชื่อค่าย/Studio",
			},
			"cast": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "รายชื่อนักแสดง",
			},
			"duration": {
				Type:        genai.TypeString,
				Description: "ความยาว เช่น '120 นาที'",
			},
			"durationMinutes": {
				Type:        genai.TypeInteger,
				Description: "จำนวนนาที (integer) สำหรับ Frontend schema",
			},
			"genre": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "แนวเรื่อง 2-4 แนว",
			},
			"releaseYear": {
				Type:        genai.TypeString,
				Description: "ปีที่ออก เช่น '2024'",
			},
			"subtitleAvailable": {
				Type:        genai.TypeBoolean,
				Description: "มีซับไทยหรือไม่ (true)",
			},
		},
		Required: []string{"code", "studio", "cast", "duration", "durationMinutes", "genre", "releaseYear", "subtitleAvailable"},
	}
}

// BuildChunk3Schema returns the schema for Chunk 3: Story Recap
func BuildChunk3Schema() *genai.Schema {
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

// BuildChunk4Schema returns the schema for Chunk 4: Review Section
func BuildChunk4Schema() *genai.Schema {
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

// BuildChunk5Schema returns the schema for Chunk 5: FAQ Intent Block
func BuildChunk5Schema() *genai.Schema {
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

// BuildChunk6Schema returns the schema for Chunk 6: SEO Output
func BuildChunk6Schema() *genai.Schema {
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
