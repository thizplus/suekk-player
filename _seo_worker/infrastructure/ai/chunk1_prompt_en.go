package ai

import (
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 1 EN: Core Identity (English Version)
// Focus: Create the "core" of the article for use as Context in other Chunks
// Persona: Professional Film Critic / SEO Expert
// ============================================================================

// buildChunk1SchemaEN creates JSON Schema for Chunk 1 English
func (c *GeminiClient) buildChunk1SchemaEN() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			// === Core SEO ===
			"title": {
				Type:        genai.TypeString,
				Description: "H1 title in English, 50-60 characters, engaging and clickable",
			},
			"metaTitle": {
				Type:        genai.TypeString,
				Description: "Meta title for Google, MUST include [VideoCode] and [Eng Sub] within first 60 characters",
			},
			"metaDescription": {
				Type:        genai.TypeString,
				Description: "Meta description 150-160 characters, compelling call-to-action",
			},

			// === Content Summary ===
			"summary": {
				Type:        genai.TypeString,
				Description: "Content summary 500-650 words, divided into 4-6 paragraphs (separated by \\n\\n), focus on emotions and key scenes, Film Critic vocabulary",
			},
			"summaryShort": {
				Type:        genai.TypeString,
				Description: "TTS Teaser Script 100-150 words - write like a YouTube/TikTok intro, use hooks (What happens next?, Imagine this...), NO spoilers, must create curiosity!",
			},

			// === Thumbnail ===
			"thumbnailAlt": {
				Type:        genai.TypeString,
				Description: "Alt text for thumbnail 20-30 words, descriptive and SEO-friendly",
			},

			// === Quality ===
			"qualityScore": {
				Type:        genai.TypeInteger,
				Description: "Quality score 1-10 based on criteria in prompt - be critical, NOT every video is 9-10",
			},

			// === Theme/Tone (for CoreContext) ===
			"mainTheme": {
				Type:        genai.TypeString,
				Description: "Main theme of the story, e.g., 'forbidden desire', 'secret affair', 'self-discovery'",
			},
			"mainTone": {
				Type:        genai.TypeString,
				Description: "Main tone of the story, e.g., 'sensual', 'dramatic', 'romantic', 'intense'",
			},
		},
		Required: []string{
			"title", "metaTitle", "metaDescription",
			"summary", "summaryShort", "thumbnailAlt",
			"qualityScore", "mainTheme", "mainTone",
		},
	}
}

// buildChunk1PromptEN creates prompt for Chunk 1 English version
func (c *GeminiClient) buildChunk1PromptEN(input *ports.AIInput) string {
	// Build cast names string
	castNames := make([]string, len(input.Casts))
	for i, cast := range input.Casts {
		castNames[i] = cast.Name
	}
	castNamesStr := strings.Join(castNames, ", ")

	return fmt.Sprintf(`[PERSONA]
You are a "Professional Film Critic and SEO Expert"
- Expert at analyzing narratives, character development, and emotional arcs
- Write sophisticated, engaging English suitable for international audiences
- Use Film Critic vocabulary: "captivating", "nuanced", "compelling", "poignant"

[SOURCE MATERIAL]
You are provided with a THAI transcript of a Japanese video.
Your task is to CREATE a NEW article in English - DO NOT TRANSLATE word-by-word.
Analyze the content deeply, then write original English content.

[TASK - Clear Objective]
Your mission: Create the "Core Identity" of the article
This will be used as Context for other Chunks
Output: Title, Summary, Theme, Tone as the foundation

---

## Input Data

### SRT Transcript (Thai):
%s

### Video Metadata:
- Code: %s
- Duration: %d seconds
- Cast: %s

---

## CRITICAL RULES

### 1. Summary MUST have paragraphs (Very Important!)
- Use [PARA] to separate paragraphs (not \n)
- Must have 4-6 paragraphs separated by [PARA]
- Each paragraph: 100-130 words
- DO NOT write as one solid block (REJECT!)

### 2. Summary MUST be 500-650 words
- DO NOT write short 50-200 word summaries (REJECT!)
- Describe all key scenes with emotional depth
- Use Film Critic vocabulary throughout

### 3. Cast Names - Romanized Full Names Only
- USE: %s (as provided)
- First mention: Full name (e.g., "Tachibana Mary")
- Later mentions: First name or "she/her"
- NEVER mix languages or use Thai transliterations

### 4. Cultural Adaptation for Western Readers
- Explain Japanese/Asian cultural elements when relevant
- Example: "her sempai (senior colleague)" instead of just "sempai"

### 5. mainTheme and mainTone must be clear
- mainTheme: Main theme in 2-4 words (English)
- mainTone: Emotional tone in 2-4 words (English)
- These will be used as Context for other Chunks

---

## Output Requirements

1. **title**: H1 in English, 50-60 characters, Film Critic style
2. **metaTitle**: MUST include "[%s]" and "[Eng Sub]"
   Example: "[%s] Tachibana Mary's Secret Affair [Eng Sub] - Full Review"
3. **metaDescription**: 150-160 characters, compelling call-to-action
4. **summary**: 500-650 words, 4-6 paragraphs (use [PARA] separator)
   - Write like a professional film review
   - Use sophisticated vocabulary
   - Analyze character motivations and emotional arcs
5. **summaryShort**: 100-150 words, TTS Teaser style
   - Write like a YouTube/TikTok intro
   - Use hooks: "What happens when...", "Imagine...", "You won't believe..."
   - Create curiosity without spoilers
   - Exciting, not documentary style
6. **thumbnailAlt**: Alt text 20-30 words
7. **qualityScore**: Rate 1-10 based on criteria below (be critical!)
8. **mainTheme**: Main theme 2-4 words (e.g., "forbidden office romance")
9. **mainTone**: Main tone 2-4 words (e.g., "sensual and dramatic")

---

## Quality Score Criteria (Important!)

### Score Levels:
- **9-10 (Masterpiece)**: Exceptional plot, outstanding performances, beautiful cinematography, deep emotions (rare ~5%%)
- **7-8 (Excellent)**: Interesting story, good performances, clear highlights (~30%%)
- **5-6 (Average)**: Standard story, decent performances, nothing remarkable (~40%%)
- **3-4 (Below Average)**: Weak plot, unconvincing performances (~20%%)
- **1-2 (Poor)**: Not recommended (~5%%)

### Factors to Consider:
1. **Plot Interest (40%%)**: Plot twists? Complexity? Or generic formula?
2. **Performance Quality (30%%)**: Authentic emotions? Chemistry? Or stiff?
3. **Cinematography (20%%)**: Interesting angles? Good lighting? Or plain?
4. **Dialogue Quality (10%%)**: Well-written? Or boring?

### Warnings:
- DO NOT give 9-10 to every video
- Be genuinely critical in your assessment
- Most should be 5-7 range (average)

---

## Example of GOOD summary (using [PARA] separators)

"The narrative unfolds as Tachibana Mary, a seemingly ordinary office worker, finds herself in an unexpected situation that challenges her sense of loyalty and desire. Her encounter with her husband's colleague sets the stage for a compelling exploration of forbidden attraction. [PARA] As the story progresses, we witness Mary's internal conflict beautifully portrayed through subtle glances and hesitant gestures. The chemistry between the characters builds gradually, creating an atmosphere of palpable tension. [PARA] The pivotal scene occurs in the dimly lit hotel room, where all pretenses are stripped away. The cinematography captures the emotional intensity with masterful use of close-ups and ambient lighting. [PARA] What makes this narrative particularly engaging is its refusal to judge its characters. Instead, it presents a nuanced exploration of human desire and the complexities of modern relationships."

---

## Example of GOOD summaryShort (TTS Teaser)

BAD (boring - REJECT!):
"Tachibana Mary visits a massage parlor. She receives a massage from the therapist. The session proceeds normally."

GOOD (exciting - REQUIRED!):
"What happens when a simple massage appointment turns into something neither party expected? Tachibana Mary thought she was just relieving work stress, but the skilled hands of her therapist awakened something she never knew existed. As the session progresses, the line between professional and personal begins to blur. Will she give in to these unexpected feelings? Watch to find out!"

---

## DO NOT (Critical Errors)
- summary without [PARA] paragraph separators (REJECT!)
- summary shorter than 500 words
- summaryShort written like a documentary (must be exciting!)
- Mixed language in cast names
- Empty mainTheme/mainTone
- Using Thai words or transliterations
`,
		input.SRTContent,
		input.VideoMetadata.RealCode,
		input.VideoMetadata.Duration,
		castNamesStr,
		castNamesStr,
		input.VideoMetadata.RealCode,
		input.VideoMetadata.RealCode,
	)
}
