package ai

import (
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 2 V3: Structured Facts
// Focus: ข้อมูล facts สำหรับ table และ JSON-LD
// ============================================================================

func (c *GeminiClient) buildChunk2SchemaV3() *genai.Schema {
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
		Required: []string{"code", "studio", "cast", "duration", "genre", "releaseYear", "subtitleAvailable"},
	}
}

func (c *GeminiClient) buildChunk2PromptV3(input *ports.AIInput) string {
	// Cast names
	castNames := make([]string, len(input.Casts))
	for i, cast := range input.Casts {
		castNames[i] = cast.Name
	}
	castNamesStr := strings.Join(castNames, ", ")

	// Tags for genre
	var tagsStr strings.Builder
	for _, tag := range input.Tags {
		tagsStr.WriteString(fmt.Sprintf("- %s\n", tag.Name))
	}

	// Maker
	makerName := "ไม่ระบุ"
	if input.VideoMetadata.Maker != nil {
		makerName = input.VideoMetadata.Maker.Name
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
		input.VideoMetadata.RealCode,
		makerName,
		castNamesStr,
		input.VideoMetadata.Duration,
		input.VideoMetadata.ReleaseDate,
		tagsStr.String(),
		input.VideoMetadata.RealCode,
		makerName,
		strings.Join(castNames, `", "`),
		input.VideoMetadata.Duration/60,
	)
}
