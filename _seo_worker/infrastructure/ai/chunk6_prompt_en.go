package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 6 EN: Technical & FAQ (English Version)
// Focus: Technical info and FAQ
// Persona: Technical Writer / Customer Support
// ============================================================================

// buildChunk6SchemaEN creates JSON Schema for Chunk 6 English
func (c *GeminiClient) buildChunk6SchemaEN() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			// Trustworthiness
			"translationMethod": {
				Type:        genai.TypeString,
				Description: "Translation method 40-60 words",
			},
			"translationNote": {
				Type:        genai.TypeString,
				Description: "Translation notes 40-60 words",
			},
			"subtitleQuality": {
				Type:        genai.TypeString,
				Description: "Subtitle quality assessment 40-60 words",
			},
			"videoQuality": {
				Type:        genai.TypeString,
				Description: "Video quality specs 25-40 words",
			},
			"audioQuality": {
				Type:        genai.TypeString,
				Description: "Audio quality specs 25-40 words",
			},
			"technicalFaq": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"question": {Type: genai.TypeString},
						"answer":   {Type: genai.TypeString, Description: "Answer 50-80 words"},
					},
					Required: []string{"question", "answer"},
				},
				Description: "Technical FAQ 2-3 items",
			},

			// FAQ
			"faqItems": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"question": {Type: genai.TypeString, Description: "MUST be a complete question (What/How/Where/Who/When)"},
						"answer":   {Type: genai.TypeString, Description: "Answer 60-100 words, tasteful language only"},
					},
					Required: []string{"question", "answer"},
				},
				Description: "General FAQ 5-8 items - questions MUST be complete sentences!",
			},

			// SEO
			"keywords": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "SEO keywords 5-10 items - Western search terms",
			},
			"longTailKeywords": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "Long-tail keywords 3-5 phrases",
			},
		},
		Required: []string{
			"translationMethod", "translationNote", "subtitleQuality",
			"videoQuality", "audioQuality", "technicalFaq",
			"faqItems", "keywords", "longTailKeywords",
		},
	}
}

// buildChunk6PromptEN creates prompt for Chunk 6 English version
func (c *GeminiClient) buildChunk6PromptEN(input *ports.AIInput, extCtx *ExtendedContext) string {
	// Cast names
	castNames := make([]string, len(input.Casts))
	for i, cast := range input.Casts {
		castNames[i] = cast.Name
	}
	castNamesStr := strings.Join(castNames, ", ")

	// Tags
	var tagsStr strings.Builder
	for _, tag := range input.Tags {
		tagsStr.WriteString(fmt.Sprintf("- %s\n", tag.Name))
	}

	// Top highlights from Extended Context
	highlightsStr := strings.Join(extCtx.TopHighlights, "\n- ")

	// Entities
	entitiesJSON, _ := json.Marshal(extCtx.Entities)

	// Duration formatted
	durationStr := formatDurationEnglish(input.VideoMetadata.Duration)

	return fmt.Sprintf(`[PERSONA]
You are a "Technical Writer / Customer Support Specialist"
- Expert in writing FAQ that answers real user questions
- Analyzes technical quality of video and subtitles
- Creates SEO keywords with clear search intent

[SOURCE MATERIAL]
You are analyzing a THAI transcript of a Japanese video.
Your task is to CREATE NEW content in English - DO NOT TRANSLATE.

[TASK]
Mission: Create FAQ and technical information
Output: TranslationInfo, FAQ, Keywords

---

## Extended Context from Previous Chunks

### Title:
%s

### Summary:
%s

### Top Highlights:
- %s

### Key Scenes:
%s

### Expert Summary:
%s

### Entities (verify names):
%s

---

## Video Information

- Code: %s
- Duration: %s
- Cast: %s
- Tags:
%s

### SRT Sample:
%s

---

## CRITICAL RULES

### 1. FAQ questions MUST be complete
- MUST be proper question sentences with: What/How/Where/Who/When/Is/Does
- WRONG: "Megami Jun?" (REJECT!)
- WRONG: "%s?" (REJECT!)
- CORRECT: "What is %s about?"

### 2. FAQ must cover multiple categories
Must include FAQ types:
1. Plot questions (What is it about?)
2. Cast questions (Who stars in this?)
3. Where to watch (Where can I stream this?)
4. Quality questions (How is the subtitle quality?)
5. Audience questions (Who should watch this?)
6. Comparison questions (How does it compare to similar films?)

### 3. FAQ Actor Name Verification
- Actor names in questions MUST match entities.actors
- DO NOT make up actor names!
- Use ONLY names from: %s

### 4. Good FAQ Examples
CORRECT: "What is %s about?"
CORRECT: "Who plays the lead role in this film?"
CORRECT: "Where can I watch %s with English subtitles?"
CORRECT: "Is this film suitable for fans of romantic drama?"
CORRECT: "How is the subtitle quality for %s?"

### 5. Keywords for Western Audiences
- Use search terms Western audiences use:
- "plot summary", "full review", "English subtitles", "watch online"
- "character breakdown", "detailed analysis"
- NO explicit terms

### 6. Pronoun Rules
- Single actress: "she/her"
- Group scenes: "they/them"

---

## Output Requirements

### Technical Info
1. **translationMethod**: 40-60 words
2. **translationNote**: 40-60 words
3. **subtitleQuality**: 40-60 words
4. **videoQuality**: 25-40 words
5. **audioQuality**: 25-40 words
6. **technicalFaq**: 2-3 technical questions/answers

### FAQ
7. **faqItems**: 5-8 items
   - Questions MUST be complete sentences
   - Answers 60-100 words each
   - Tasteful language only

### SEO
8. **keywords**: 5-10 Western-friendly keywords
   - Include: video code, cast names, genre terms
   - Example: ["%s review", "Japanese drama English sub"]

9. **longTailKeywords**: 3-5 phrases
   - Example: ["%s plot summary", "Where to watch %s English subtitles"]

---

## DO NOT (REJECT if found)
- FAQ questions that are just actor name/video code
- FAQ questions without proper question words
- FAQ with made-up actor names
- Keywords with explicit terms
- Mixed languages in actor names
- Any Thai words in output
`,
		extCtx.Title,
		extCtx.Summary,
		highlightsStr,
		strings.Join(extCtx.KeyScenes, ", "),
		extCtx.ExpertSummary,
		string(entitiesJSON),
		input.VideoMetadata.RealCode,
		durationStr,
		castNamesStr,
		tagsStr.String(),
		truncateSRT(input.SRTContent, 500),
		input.VideoMetadata.RealCode,
		input.VideoMetadata.RealCode,
		castNamesStr,
		input.VideoMetadata.RealCode,
		input.VideoMetadata.RealCode,
		input.VideoMetadata.RealCode,
		input.VideoMetadata.RealCode,
		input.VideoMetadata.RealCode,
		input.VideoMetadata.RealCode,
	)
}

// formatDurationEnglish formats seconds to readable English format
// e.g., 7331 → "2 hours 2 minutes"
func formatDurationEnglish(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60

	if hours > 0 && minutes > 0 {
		hourStr := "hour"
		if hours > 1 {
			hourStr = "hours"
		}
		minuteStr := "minute"
		if minutes > 1 {
			minuteStr = "minutes"
		}
		return fmt.Sprintf("%d %s %d %s", hours, hourStr, minutes, minuteStr)
	} else if hours > 0 {
		hourStr := "hour"
		if hours > 1 {
			hourStr = "hours"
		}
		return fmt.Sprintf("%d %s", hours, hourStr)
	} else if minutes > 0 {
		minuteStr := "minute"
		if minutes > 1 {
			minuteStr = "minutes"
		}
		return fmt.Sprintf("%d %s", minutes, minuteStr)
	}
	return fmt.Sprintf("%d seconds", seconds)
}
