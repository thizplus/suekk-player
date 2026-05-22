package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/models"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 7 EN: Deep Analysis (English Version)
// Focus: In-depth analysis (Cinematography, Character Journey)
// Persona: Film Critic / Cultural Analyst
// ============================================================================

// buildChunk7SchemaEN creates JSON Schema for Chunk 7 English
func (c *GeminiClient) buildChunk7SchemaEN() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			// Section 1: Cinematography & Atmosphere
			"cinematographyAnalysis": {
				Type:        genai.TypeString,
				Description: "Cinematography analysis 300-450 words, 3-4 paragraphs (separate with [PARA])",
			},
			"visualStyle": {
				Type:        genai.TypeString,
				Description: "Overall visual style 60-100 words",
			},
			"atmosphereNotes": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "Atmosphere observations 3-5 items",
			},

			// Section 2: Character Emotional Journey
			"characterJourney": {
				Type:        genai.TypeString,
				Description: "Emotional development 350-500 words, 3-5 paragraphs (separate with [PARA])",
			},
			"emotionalArc": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"phase":       {Type: genai.TypeString, Description: "Phase e.g., 'Opening', 'Rising Action', 'Climax'"},
						"emotion":     {Type: genai.TypeString, Description: "Primary emotion"},
						"description": {Type: genai.TypeString, Description: "Description 40-60 words"},
					},
					Required: []string{"phase", "emotion", "description"},
				},
				Description: "3-4 key emotional arc points",
			},

			// Section 3: Educational Context
			"thematicExplanation": {
				Type:        genai.TypeString,
				Description: "Theme explanation 250-400 words, 2-3 paragraphs (separate with [PARA])",
			},
			"culturalContext": {
				Type:        genai.TypeString,
				Description: "Japanese cultural context for Western readers 120-180 words",
			},
			"genreInsights": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "Genre insights 3-5 items",
			},

			// Section 4: Comparative Analysis
			"studioComparison": {
				Type:        genai.TypeString,
				Description: "Comparison with studio's other works 180-250 words",
			},
			"actorEvolution": {
				Type:        genai.TypeString,
				Description: "Actor's career evolution 180-250 words",
			},
			"genreRanking": {
				Type:        genai.TypeString,
				Description: "Position within genre 60-100 words",
			},

			// Section 5: Viewing Experience
			"viewingTips": {
				Type:        genai.TypeString,
				Description: "Viewing recommendations 180-250 words",
			},
			"bestMoments": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "Best moments 3-5 items, MUST have descriptions, not just actor names!",
			},
			"audienceMatch": {
				Type:        genai.TypeString,
				Description: "Ideal audience match 100-130 words",
			},
			"replayValue": {
				Type:        genai.TypeString,
				Description: "Replay value assessment 60-100 words",
			},
		},
		Required: []string{
			// Section 1
			"cinematographyAnalysis", "visualStyle", "atmosphereNotes",
			// Section 2
			"characterJourney", "emotionalArc",
			// Section 3
			"thematicExplanation", "culturalContext", "genreInsights",
			// Section 4
			"studioComparison", "actorEvolution", "genreRanking",
			// Section 5
			"viewingTips", "bestMoments", "audienceMatch", "replayValue",
		},
	}
}

// buildChunk7PromptEN creates prompt for Chunk 7 English version
func (c *GeminiClient) buildChunk7PromptEN(input *ports.AIInput, extCtx *ExtendedContext) string {
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

	// Top highlights
	highlightsStr := strings.Join(extCtx.TopHighlights, "\n- ")

	// Entities
	entitiesJSON, _ := json.Marshal(extCtx.Entities)

	// Duration
	durationStr := formatDurationEnglish(input.VideoMetadata.Duration)

	return fmt.Sprintf(`[PERSONA]
You are a "Premium Film Critic / Cultural Analyst"
- Expert in Cinematography and Visual Aesthetics
- Analyzes Character Arc and Emotional Journey
- Understands Japanese culture and can explain it to Western audiences
- Writes valuable viewing recommendations

[SOURCE MATERIAL]
You are analyzing a THAI transcript of a Japanese video.
Your task is to CREATE NEW content in English - DO NOT TRANSLATE.

[TASK]
Mission: In-depth analysis to boost SEO text content
Output: Cinematography, CharacterJourney, ThematicExplanation, ViewingTips

---

## Extended Context from Previous Chunks

### Title:
%s

### Summary:
%s

### Top Highlights (analyze these scenes):
- %s

### Key Scenes:
%s

### Expert Summary:
%s

### Main Insight:
%s

### Entities:
%s

---

## Video Information

- Code: %s
- Duration: %s
- Cast: %s
- Studio/Maker: %s
- Tags:
%s

---
%s
---

## CRITICAL RULES

### 1. Long content MUST have paragraphs with [PARA]
- cinematographyAnalysis: 3-4 paragraphs (use [PARA])
- characterJourney: 3-5 paragraphs (use [PARA])
- thematicExplanation: 2-3 paragraphs (use [PARA])

### 2. bestMoments MUST NOT start with actor names + reduce repetition!
- WRONG: "Megami Jun displays..." (starts with name)
- WRONG: "Megami Jun uses technique..." (starts with name)
- CORRECT: "The scene where determination is displayed through nuanced body language"
- CORRECT: "A moment where the young woman creates a relaxing atmosphere"
- CORRECT: "The use of close-up shots emphasizing tender care"
- Focus on "what happens/what technique" > "who does it"
- If you must reference an actor, place name in middle or end

### 2.1 Reduce name repetition with Pronoun/Role Substitution
- Maximum 2 full name uses in 5 bestMoments!
- SUBSTITUTE with pronouns: "she", "the young woman", "the protagonist"
- SUBSTITUTE with roles: "the patient", "the therapist", "the lead performer"
- WRONG: "The moment when Zemba Mami..." repeated every item (Spammy!)
- CORRECT: "The moment when she faces...", "As the young woman attempts..."

- Each bestMoment must be 20-40 words

### 3. Use Technical Vocabulary (SEO boost!)
- Cinematography: Lighting, Close-up, POV, Color Grading, Mise-en-scène
- Psychology: Anxiety, Trust, Catharsis, Emotional Arc, Vulnerability
- Film Terms: Pacing, Narrative Structure, Character Development

### 4. Cultural Adaptation for Western Readers
- Explain Japanese concepts that may be unfamiliar
- Example: "the dynamic between sempai and kohai (senior and junior colleagues)"
- Provide context for cultural practices

### 5. Entity-Consistency
- Use names from entities.actors ONLY
- First mention: fullName, subsequent: firstName or "she/her"

### 6. Pronoun Rules for English
- Single actress: "she/her"
- Group scenes (3P/4P): "they/them" for clarity
- Be specific when context might be confusing

### 7. NO explicit language
- USE: "intimate moments", "passionate encounter", "romantic culmination"
- NOT: explicit terms

---

## Output Requirements

### Section 1: Cinematography (Visual Analysis)
1. **cinematographyAnalysis**: 300-450 words, 3-4 paragraphs (use [PARA])
   - Lighting (natural, soft, clinical)
   - Camera Angles (close-up, wide shot, POV)
   - Color Grading (warm tones, cool tones)
   - Composition and visual storytelling

2. **visualStyle**: 60-100 words

3. **atmosphereNotes**: 3-5 observations

### Section 2: Character Journey (Emotional Development)
4. **characterJourney**: 350-500 words, 3-5 paragraphs (use [PARA])
   - Opening: Initial state
   - Development: How relationships evolve
   - Climax: Peak emotional moment
   - Resolution: How characters transform

5. **emotionalArc**: 3-4 key points
   - phase: "Opening", "Rising Action", "Climax", "Resolution"
   - emotion: Primary emotion at each phase
   - description: 40-60 words

### Section 3: Educational (Thematic Depth)
6. **thematicExplanation**: 250-400 words, 2-3 paragraphs (use [PARA])
   - If Medical theme: Explain healthcare context, patient-doctor dynamics
   - If Office theme: Explain workplace power dynamics, Japanese corporate culture
   - Educational tone to attract informational search traffic

7. **culturalContext**: 120-180 words
   - Japanese cultural elements explained for Western audiences

8. **genreInsights**: 3-5 insights

### Section 4: Comparative (Analysis)
9. **studioComparison**: 180-250 words
   - How this compares to other studio productions

10. **actorEvolution**: 180-250 words
    - Career progression and growth

11. **genreRanking**: 60-100 words
    - Position within the genre

### Section 5: Viewing (Recommendations)
12. **viewingTips**: 180-250 words
    - What to focus on
    - Key moments not to miss
    - User-centric recommendations (Google Helpful Content)

13. **bestMoments**: 3-5 moments (20-40 words each)
    - Emotional descriptions, not just actor names

14. **audienceMatch**: 100-130 words
    - Ideal audience + who should avoid

15. **replayValue**: 60-100 words

---

## DO NOT (REJECT if found)
- Long content without [PARA] paragraph separators (REJECT!)
- bestMoments that are just actor names
- Copy directly from Summary
- Explicit language
- Generic content that could apply to any film
- Mixed languages in actor names
- Any Thai words in output
`,
		extCtx.Title,
		extCtx.Summary,
		highlightsStr,
		strings.Join(extCtx.KeyScenes, ", "),
		extCtx.ExpertSummary,
		extCtx.MainInsight,
		string(entitiesJSON),
		input.VideoMetadata.RealCode,
		durationStr,
		castNamesStr,
		getMakerNameEN(input.VideoMetadata.Maker),
		tagsStr.String(),
		GlobalConstraintsEN+GlobalConstraintsForArraysEN, // Global Rules EN
	)
}

// getMakerNameEN safely extracts maker name for English content
func getMakerNameEN(maker *models.MakerMetadata) string {
	if maker == nil {
		return "Unknown Studio"
	}
	return maker.Name
}
