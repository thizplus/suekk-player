package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"seo-worker/domain/ports"
)

// ============================================================================
// Chunk 2 EN: Scene & Moments (English Version)
// Focus: Analyze scenes and key moments
// Persona: Film Director / Scene Analyst
// ============================================================================

// buildChunk2SchemaEN creates JSON Schema for Chunk 2 English
func (c *GeminiClient) buildChunk2SchemaEN() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"highlights": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "5-8 key scenes, 20-40 words each, focus on what happens, not just actor names",
			},
			"keyMoments": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"name":        {Type: genai.TypeString, Description: "Scene name (tasteful language)"},
						"startOffset": {Type: genai.TypeInteger, Description: "Start time (seconds)"},
						"endOffset":   {Type: genai.TypeInteger, Description: "End time (seconds), must be > startOffset"},
					},
					Required: []string{"name", "startOffset", "endOffset"},
				},
				Description: "3-5 key moments distributed throughout the video",
			},
			"sceneLocations": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "3-5 locations in the story, e.g., ['examination room', 'clinic']",
			},
			"galleryAlts": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "Alt text for images: [VideoCode] - [Actor Name] - [Context]",
			},
		},
		Required: []string{"highlights", "keyMoments", "sceneLocations", "galleryAlts"},
	}
}

// buildChunk2PromptEN creates prompt for Chunk 2 English version
func (c *GeminiClient) buildChunk2PromptEN(input *ports.AIInput, coreCtx *CoreContext) string {
	// Build cast names string
	castNames := make([]string, len(input.Casts))
	for i, cast := range input.Casts {
		castNames[i] = cast.Name
	}
	castNamesStr := strings.Join(castNames, ", ")

	// Serialize entities
	entitiesJSON, _ := json.Marshal(coreCtx.Entities)

	return fmt.Sprintf(`[PERSONA]
You are a "Professional Film Director / Scene Analyst"
- Expert in analyzing scenes and cinematic timing
- Observes details and atmosphere with precision
- Uses Film Critic vocabulary

[SOURCE MATERIAL]
You are analyzing a THAI transcript of a Japanese video.
Your task is to CREATE NEW content in English - DO NOT TRANSLATE.

[TASK]
Mission: Analyze key scenes and notable moments
Output: Highlights, KeyMoments, SceneLocations, GalleryAlts

---

## Context from Chunk 1 (DO NOT re-invent the story!)

### Title:
%s

### Summary (reference scenes from here):
%s

### Theme/Tone:
- Theme: %s
- Tone: %s

### Entities (USE these names ONLY):
%s

---

## Input Data

### SRT Transcript (extract timestamps from here):
%s

### Video Metadata:
- Code: %s
- Duration: %d seconds
- Cast: %s
- Gallery Count: %d

---
%s
---

## CRITICAL RULES

### 1. Highlights MUST NOT start with actor names + reduce repetition!
- WRONG: "Tachibana Mary, determined to..." (starts with name)
- WRONG: "Tachibana Mary uses technique..." (starts with name)
- CORRECT: "The massage scene creates a naturally relaxing atmosphere"
- CORRECT: "A moment where the young woman shows genuine care for her client"
- CORRECT: "The use of varied massage techniques to relieve fatigue"
- Focus on "what happens" > "who does it"
- If you must reference an actor, place the name in the middle or end of the sentence

### 1.1 Reduce name repetition with Pronoun/Role Substitution
- Use full name maximum 3 times in 8 highlights!
- SUBSTITUTE with pronouns: "she", "the young woman"
- SUBSTITUTE with roles: "the patient", "the massage therapist", "the protagonist"
- WRONG: "When Zemba Mami tries..." repeated every item
- CORRECT: "When the young woman tries...", "As she faces..."

- Each highlight must be 20-40 words

### 2. keyMoments must be distributed throughout the video
- Extract timestamps directly from SRT
- Distribute 3-5 moments across the entire video
- Use tasteful scene names (no explicit language)

### 3. Entity-Consistency
- Use actor names ONLY from entities.actors
- DO NOT create new names or mix languages
- First mention: Full name (e.g., "Tachibana Mary")
- Later mentions: First name or "she/her"

---

## Output Requirements

1. **highlights**: 5-8 key scenes, 20-40 words each
2. **keyMoments**: 3-5 timestamps with tasteful names
3. **sceneLocations**: 3-5 locations in the story
4. **galleryAlts**: %d alt texts in format "[%s] - [Actor Name] - [Context]"

---

## DO NOT (REJECT if found)
- Highlights that are just actor names (filtered out!)
- keyMoments with duration < 30 seconds
- Explicit language in keyMoments names
- Mixed languages in actor names
- Thai words in English content
`,
		coreCtx.Title,
		coreCtx.Summary,
		coreCtx.MainTheme,
		coreCtx.MainTone,
		string(entitiesJSON),
		truncateSRT(input.SRTContent, 2000),
		input.VideoMetadata.RealCode,
		input.VideoMetadata.Duration,
		castNamesStr,
		input.GalleryCount,
		GlobalConstraintsEN+GlobalConstraintsForArraysEN, // Global Rules EN
		input.GalleryCount,
		input.VideoMetadata.RealCode,
	)
}
