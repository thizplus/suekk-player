package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 3 EN: Expertise (Linguistic Analysis) - English Version
// Focus: Analyze dialogue and performance
// Persona: Film Critic / Performance Analyst
// ============================================================================

// buildChunk3SchemaEN creates JSON Schema for Chunk 3 English
func (c *GeminiClient) buildChunk3SchemaEN() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"dialogueAnalysis": {
				Type:        genai.TypeString,
				Description: "Dialogue analysis: communication styles, emotional shifts, character dynamics (120-180 words)",
			},
			"characterInsight": {
				Type:        genai.TypeString,
				Description: "Character personality analysis based on speech patterns and reactions (120-180 words)",
			},
			"topQuotes": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"text":      {Type: genai.TypeString, Description: "Quote in English (translated from Thai sub, not explicit)"},
						"timestamp": {Type: genai.TypeInteger, Description: "Time (seconds) - MUST be within first 600 seconds"},
						"emotion":   {Type: genai.TypeString, Description: "Emotion conveyed"},
						"context":   {Type: genai.TypeString, Description: "Scene context"},
					},
					Required: []string{"text", "timestamp", "emotion", "context"},
				},
				Description: "4-5 memorable quotes from first 10 minutes only",
			},
			"languageNotes": {
				Type:        genai.TypeString,
				Description: "Notes on language usage: formal/informal speech, honorifics, cultural nuances (60-100 words)",
			},
			"actorPerformanceTrend": {
				Type:        genai.TypeString,
				Description: "Performance analysis compared to previous works (100-130 words)",
			},
		},
		Required: []string{
			"dialogueAnalysis", "characterInsight", "topQuotes",
			"languageNotes", "actorPerformanceTrend",
		},
	}
}

// buildChunk3PromptEN creates prompt for Chunk 3 English version
func (c *GeminiClient) buildChunk3PromptEN(input *ports.AIInput, coreCtx *CoreContext) string {
	// Build cast info
	var castsInfo strings.Builder
	for _, cast := range input.Casts {
		castsInfo.WriteString(fmt.Sprintf("- %s\n", cast.Name))
	}

	// Previous works
	var prevWorks strings.Builder
	for _, work := range input.PreviousWorks {
		prevWorks.WriteString(fmt.Sprintf("- %s (%s)\n", work.Title, work.VideoCode))
	}
	if prevWorks.Len() == 0 {
		prevWorks.WriteString("(No previous works data available)")
	}

	// Entities
	entitiesJSON, _ := json.Marshal(coreCtx.Entities)

	return fmt.Sprintf(`[PERSONA]
You are a "Professional Film Critic and Performance Analyst"
- Expert in analyzing dialogue, body language, and character portrayal
- Uses sophisticated Film Critic vocabulary
- Understands Japanese cultural nuances and can explain them to Western audiences

[SOURCE MATERIAL]
You are analyzing a THAI transcript of a Japanese video.
Your task is to CREATE NEW content in English - DO NOT TRANSLATE word-by-word.

[TASK]
Mission: Analyze dialogue, performances, and character dynamics
Output: DialogueAnalysis, CharacterInsight, TopQuotes, LanguageNotes

---

## Context from Chunk 1

### Title:
%s

### Summary:
%s

### Theme/Tone:
- Theme: %s
- Tone: %s

### Entities (USE these names ONLY):
%s

---

## Input Data

### SRT Transcript (analyze dialogue from here):
%s

### Cast:
%s

### Previous Works (for performance comparison):
%s

---

## CRITICAL RULES

### 1. topQuotes MUST be from first 600 seconds only
- Extract timestamps from SRT
- Select ONLY quotes with timestamp ≤ 600 seconds
- NO explicit quotes - choose meaningful dialogue
- Translate the essence, not word-for-word

### 2. Use Film Critic vocabulary
- "Nuanced portrayal", "compelling delivery", "emotional resonance"
- "Subtle gestures", "captivating chemistry", "understated performance"

### 3. Cultural Adaptation for Western Readers
- Explain Japanese honorifics when relevant
- Provide context for cultural practices
- Example: "her use of formal speech (keigo) emphasizes the power dynamic"

### 4. Entity-Consistency
- Use actor names ONLY from entities.actors
- First mention: fullName, subsequent: firstName or "she/her"

### 5. Pronoun Rules
- For single actress: "she/her"
- For group scenes (3P/4P): use "they/them" for clarity
- Specify names when context might be ambiguous

---

## Output Requirements

1. **dialogueAnalysis**: 120-180 words
   - Communication patterns
   - Emotional shifts in dialogue
   - Power dynamics through speech

2. **characterInsight**: 120-180 words
   - Personality traits revealed through behavior
   - Reactions and responses
   - Character development

3. **topQuotes**: 4-5 memorable quotes
   - text: Quote translated to English (capture essence)
   - timestamp: ≤ 600 seconds only
   - emotion: Primary emotion
   - context: Scene context

4. **languageNotes**: 60-100 words
   - Formal vs informal speech patterns
   - Cultural significance of language choices

5. **actorPerformanceTrend**: 100-130 words
   - Compare to previous works (if available)
   - Note improvements or stylistic changes

---

## DO NOT (REJECT if found)
- topQuotes with timestamp > 600 seconds (filtered!)
- Explicit quotes in topQuotes
- Mixed languages in actor names
- Shallow or generic analysis
- Any Thai words in output
`,
		coreCtx.Title,
		coreCtx.Summary,
		coreCtx.MainTheme,
		coreCtx.MainTone,
		string(entitiesJSON),
		truncateSRT(input.SRTContent, 3000),
		castsInfo.String(),
		prevWorks.String(),
	)
}
