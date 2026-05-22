package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 5 EN: Recommendations & Links (English Version)
// Focus: Create Internal Links and Recommendations
// Persona: Content Strategist / SEO Specialist
// ============================================================================

// buildChunk5SchemaEN creates JSON Schema for Chunk 5 English
func (c *GeminiClient) buildChunk5SchemaEN() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"characterDynamic": {
				Type:        genai.TypeString,
				Description: "Character relationships and dynamics 120-180 words",
			},
			"plotAnalysis": {
				Type:        genai.TypeString,
				Description: "Plot structure analysis 120-180 words",
			},
			"recommendation": {
				Type:        genai.TypeString,
				Description: "Who should watch this 60-100 words",
			},
			"recommendedFor": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "Target audience groups 3-5 items",
			},
			"comparisonNote": {
				Type:        genai.TypeString,
				Description: "Comparison with other works 100-130 words - MUST reference real VIDEO CODEs",
			},
			"contextualLinks": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"text":        {Type: genai.TypeString, Description: "Natural linking sentence"},
						"linkedSlug":  {Type: genai.TypeString, Description: "Slug of recommended article"},
						"linkedTitle": {Type: genai.TypeString, Description: "Display title"},
					},
					Required: []string{"text", "linkedSlug", "linkedTitle"},
				},
				Description: "2-4 contextual links to related articles (SEO Internal Linking)",
			},
			"settingDescription": {
				Type:        genai.TypeString,
				Description: "Scene/setting description 60-100 words",
			},
			"moodTone": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "Mood/tone descriptors 3-5 items",
			},
			"thematicKeywords": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "Keywords for semantic search 5-8 items",
			},
		},
		Required: []string{
			"characterDynamic", "plotAnalysis", "recommendation",
			"recommendedFor", "comparisonNote", "settingDescription",
			"moodTone", "thematicKeywords",
		},
	}
}

// buildChunk5PromptEN creates prompt for Chunk 5 English version
func (c *GeminiClient) buildChunk5PromptEN(
	input *ports.AIInput,
	coreCtx *CoreContext,
	chunk2 *Chunk2OutputV2,
	chunk3 *Chunk3OutputV2,
	chunk4 *Chunk4OutputV2,
) string {
	// Entities
	entitiesJSON, _ := json.Marshal(coreCtx.Entities)

	// Highlights from Chunk 2
	highlightsStr := strings.Join(chunk2.Highlights, "\n- ")

	// Scene locations from Chunk 2
	locationsStr := strings.Join(chunk2.SceneLocations, ", ")

	// Character insight from Chunk 3
	characterInsight := chunk3.CharacterInsight

	// Expert analysis from Chunk 4
	expertAnalysis := chunk4.ExpertAnalysis

	// Related articles
	var relatedArticles strings.Builder
	if len(input.RelatedArticles) == 0 {
		relatedArticles.WriteString("⚠️ No Related Articles - return contextualLinks as empty array []\n")
	} else {
		for _, article := range input.RelatedArticles {
			relatedArticles.WriteString(fmt.Sprintf("- Slug: %s, Code: %s, Title: %s\n  Cast: %s, Tags: %s\n",
				article.Slug, article.RealCode, article.Title,
				strings.Join(article.CastNames, ", "),
				strings.Join(article.Tags, ", "),
			))
		}
	}

	return fmt.Sprintf(`[PERSONA]
You are a "Content Strategist / SEO Specialist"
- Expert in creating valuable Internal Links
- Analyzes target audiences and recommendations
- Understands Semantic Search and keyword optimization

[SOURCE MATERIAL]
You are analyzing a THAI transcript of a Japanese video.
Your task is to CREATE NEW content in English - DO NOT TRANSLATE.

[TASK]
Mission: Create recommendations and Internal Links based on Theme/Mood
Output: CharacterDynamic, PlotAnalysis, Recommendations, ContextualLinks

---

## Context from Previous Chunks

### From Chunk 1:
- Title: %s
- Summary: %s
- Theme: %s
- Tone: %s

### From Chunk 2 (Highlights & Scenes):
- Highlights:
  - %s
- Scene Locations: %s

### From Chunk 3 (Character Insight):
%s

### From Chunk 4 (Expert Analysis):
%s

### Entities:
%s

---

## Related Articles Data (for contextualLinks)

%s

---

## CRITICAL RULES

### 1. Semantic Bridge (VERY IMPORTANT!)
Links must focus on Theme/Mood, NOT just same actor:

WRONG: Recommend just because "same actress"
CORRECT: Recommend because "similar theme/mood"

**Linking Criteria (priority order):**
1. Theme Match: Similar story themes (e.g., forbidden desire, double life)
2. Mood Match: Similar emotional tone (e.g., relaxing, romantic, intense)
3. Setting Match: Similar settings (e.g., office, massage parlor)
4. Actor Match: Same performer (use as last resort)

### 2. Good contextualLinks Examples
CORRECT: "If you enjoyed this exploration of 'secret desires', you might appreciate ABC-123 which presents a similar theme"
CORRECT: "For those seeking the same relaxing atmosphere, XYZ-456 offers a comparable experience"

WRONG: "Watch other works by Megami Jun at..." (focuses only on actor)

### 3. contextualLinks MUST use real slugs
- Use ONLY slugs from Related Articles provided
- DO NOT create fake slugs!
- If no Related Articles → return empty array []

### 4. Keywords for Western Search Intent
- Use terms Western audiences search for:
- "plot summary", "detailed review", "English subtitles"
- "character analysis", "full movie review"

---

## Output Requirements

1. **characterDynamic**: 120-180 words
   - Relationships between characters
   - Power dynamics and emotional connections

2. **plotAnalysis**: 120-180 words
   - Narrative structure
   - Plot development and turning points

3. **recommendation**: 60-100 words
   - Who should watch this film

4. **recommendedFor**: 3-5 audience groups
   - e.g., ["Fans of medical drama", "Those who enjoy slow-burn narratives"]

5. **comparisonNote**: 100-130 words
   - MUST reference real VIDEO CODEs if comparing

6. **contextualLinks**: 2-4 links
   - Use Semantic Bridge approach
   - Use ONLY real slugs from provided data

7. **settingDescription**: 60-100 words

8. **moodTone**: 3-5 descriptors
   - e.g., ["sensual", "dramatic", "intimate"]

9. **thematicKeywords**: 5-8 keywords
   - Include location-specific keywords
   - Use Western search terms

---

## DO NOT (REJECT if found)
- contextualLinks with made-up slugs
- Links based only on same actor
- Mixed languages in actor names
- Any Thai words in output
`,
		coreCtx.Title,
		truncateToWords(coreCtx.Summary, 150),
		coreCtx.MainTheme,
		coreCtx.MainTone,
		highlightsStr,
		locationsStr,
		characterInsight,
		expertAnalysis,
		string(entitiesJSON),
		relatedArticles.String(),
	)
}
