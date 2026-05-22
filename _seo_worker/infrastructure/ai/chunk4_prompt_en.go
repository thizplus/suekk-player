package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 4 EN: Authority (Entity Bios) - English Version
// Focus: Create authoritative content (Cast bios, Tags, Detailed Review)
// Persona: Biography Writer / Encyclopedia Writer
// ============================================================================

// buildChunk4SchemaEN creates JSON Schema for Chunk 4 English
func (c *GeminiClient) buildChunk4SchemaEN() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"detailedReview": {
				Type:        genai.TypeString,
				Description: "In-depth review 650-850 words, 5-7 paragraphs (separate with [PARA]), analyze performances, techniques, narrative structure",
			},
			"castBios": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"castId": {Type: genai.TypeString},
						"bio":    {Type: genai.TypeString, Description: "Bio 100-150 words, professional Film Critic style"},
					},
					Required: []string{"castId", "bio"},
				},
				Description: "Biography for each cast member",
			},
			"tagDescriptions": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"id":          {Type: genai.TypeString},
						"name":        {Type: genai.TypeString},
						"description": {Type: genai.TypeString, Description: "Tag description 40-60 words, tasteful language only"},
					},
					Required: []string{"id", "name", "description"},
				},
				Description: "English descriptions for each tag",
			},
			"expertAnalysis": {
				Type:        genai.TypeString,
				Description: "Expert analysis 180-250 words - MUST NOT be empty!",
			},
		},
		Required: []string{"detailedReview", "castBios", "tagDescriptions", "expertAnalysis"},
	}
}

// buildChunk4PromptEN creates prompt for Chunk 4 English version
func (c *GeminiClient) buildChunk4PromptEN(input *ports.AIInput, coreCtx *CoreContext) string {
	// Build cast info
	var castsInfo strings.Builder
	for _, cast := range input.Casts {
		castsInfo.WriteString(fmt.Sprintf("- ID: %s, Name: %s\n", cast.ID, cast.Name))
	}

	// Build tags info
	var tagsInfo strings.Builder
	for _, tag := range input.Tags {
		tagsInfo.WriteString(fmt.Sprintf("- ID: %s, Name: %s\n", tag.ID, tag.Name))
	}

	// Previous works
	var prevWorks strings.Builder
	for _, work := range input.PreviousWorks {
		prevWorks.WriteString(fmt.Sprintf("- %s (%s)\n", work.Title, work.VideoCode))
	}

	// Entities
	entitiesJSON, _ := json.Marshal(coreCtx.Entities)

	return fmt.Sprintf(`[PERSONA]
You are a "Professional Biography Writer and Film Encyclopedia Contributor"
- Expert in writing comprehensive, authoritative bios
- Creates encyclopedia-style content with depth and credibility
- Uses sophisticated Film Critic vocabulary

[SOURCE MATERIAL]
You are analyzing a THAI transcript of a Japanese video.
Your task is to CREATE NEW content in English - DO NOT TRANSLATE.

[TASK]
Mission: Create authoritative content (DetailedReview, CastBios, TagDescriptions)
Output: High-credibility, well-researched content

---

## Context from Chunk 1

### Title:
%s

### Summary (reference for analysis):
%s

### Theme/Tone:
- Theme: %s
- Tone: %s

### Entities:
%s

---

## Input Data

### Cast (generate bio for each):
%s

### Cast Previous Works:
%s

### Tags (generate description for each):
%s

---

## CRITICAL RULES

### 1. detailedReview MUST have paragraphs with [PARA] + avoid name spam
- USE [PARA] between paragraphs (not \n)
- MUST have 5-7 paragraphs separated by [PARA] (650-850 words total)
- DO NOT write "[Actor Name] does [verb]" more than 2 times per paragraph!
- DO NOT start consecutive sentences with full name!
- Maximum 5 full name uses per 100 words!

### 2. Negative Prompting (what NOT to do)
REJECT example:
"Megami Jun massages the client. Megami Jun asks the client. Megami Jun smiles. Megami Jun..."

CORRECT example:
"Megami Jun delivers a captivating performance as a massage therapist. She conveys genuine care through subtle gestures and attentive body language. Her technique reflects years of professional training..."

### 3. How to avoid name spam:
- First mention: Full name (fullName)
- Second mention: First name only
- Third+ mentions: Use "she/her", "the protagonist", "the performer"
- Focus on describing emotions and atmosphere rather than naming

### 4. tagDescriptions MUST use tasteful language
- NO explicit terms
- USE euphemisms: "intimate finale", "romantic climax", "passionate encounter"
- Focus on emotional or artistic aspects

### 5. expertAnalysis MUST NOT be empty
- 180-250 words required
- Analyze production quality, performances, cinematography
- Use Film Critic vocabulary

### 6. Pronoun Rules for English
- Single actress: "she/her"
- Group scenes (3P/4P): "they/them" for clarity
- Be specific when context might be confusing

---

## Output Requirements

1. **detailedReview**: 650-850 words, 5-7 paragraphs
   - Use [PARA] between paragraphs (not \n)
   - Maximum 2 full name uses per paragraph
   - Film Critic vocabulary throughout
   - Analyze: narrative structure, performances, emotional impact

2. **castBios**: 100-150 words per person
   - castId: Use ID as provided
   - bio: Written in professional biographical style
   - Mention notable aspects of their performance in this work

3. **tagDescriptions**: 40-60 words per tag
   - MUST NOT be empty!
   - Use tasteful, artistic language
   - Explain what the tag represents in film context

4. **expertAnalysis**: 180-250 words
   - MUST NOT be empty!
   - Analyze production quality, direction, performances
   - Include technical observations (lighting, composition)

---

## DO NOT (REJECT if found)
- detailedReview without [PARA] paragraph separators (REJECT!)
- detailedReview that spams actor names
- tagDescriptions left empty
- expertAnalysis left empty
- Mixed languages in actor names
- Any Thai words in output
- Generic content that could apply to any film
`,
		coreCtx.Title,
		coreCtx.Summary,
		coreCtx.MainTheme,
		coreCtx.MainTone,
		string(entitiesJSON),
		castsInfo.String(),
		prevWorks.String(),
		tagsInfo.String(),
	)
}
