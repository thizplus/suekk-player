package en

import (
	"github.com/google/generative-ai-go/genai"
)

// ============================================================================
// English Review Schemas (V3 Intent-Driven)
// Genai schema definitions for structured JSON output
// ============================================================================

// BuildChunk1Schema returns the schema for Chunk 1: Search Intent Answer
func BuildChunk1Schema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"quickAnswer": {
				Type:        genai.TypeString,
				Description: "2-3 declarative sentences. NO questions in first sentence. Start with '[CODE] is...' or 'This release...'",
			},
			"mainHook": {
				Type:        genai.TypeString,
				Description: "1 sentence that creates curiosity without clickbait",
			},
			"verdict": {
				Type:        genai.TypeString,
				Description: "1 soft recommendation sentence stating who it's for (no clickbait like 'must watch!')",
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
				Description: "Video code e.g. DASS-541",
			},
			"studio": {
				Type:        genai.TypeString,
				Description: "Studio/Production company name",
			},
			"cast": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "List of performers (romanized names)",
			},
			"duration": {
				Type:        genai.TypeString,
				Description: "Duration e.g. '120 minutes'",
			},
			"durationMinutes": {
				Type:        genai.TypeInteger,
				Description: "Duration in minutes (integer) for frontend schema",
			},
			"genre": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "2-4 genre tags in English",
			},
			"releaseYear": {
				Type:        genai.TypeString,
				Description: "Release year e.g. '2024'",
			},
			"subtitleAvailable": {
				Type:        genai.TypeBoolean,
				Description: "English subtitles available (true)",
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
				Description: "Plot summary 150-250 words, 2-3 paragraphs separated by [PARA]",
			},
			"storyFlow": {
				Type:        genai.TypeString,
				Description: "Timeline: Opening → Development → Climax (80-100 words)",
			},
			"keyScenes": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "3-5 key scenes, each 15-25 words",
			},
			"featuredScene": {
				Type:        genai.TypeString,
				Description: "Most compelling scene 120-150 words with: atmosphere, context, action/dialogue, impact",
			},
			"tone": {
				Type:        genai.TypeString,
				Description: "Mood/tone 2-3 words e.g. 'Sensual and tender'",
			},
			"relationshipDynamic": {
				Type:        genai.TypeString,
				Description: "Character relationship dynamics 50-80 words",
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
				Description: "Review 200-300 words, 3 paragraphs separated by [PARA]. Film critic style, not essay.",
			},
			"strengths": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "3-5 strengths, each 10-20 words",
			},
			"weaknesses": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "2-3 weaknesses, each 10-20 words (minimum 1 required)",
			},
			"whoShouldWatch": {
				Type:        genai.TypeString,
				Description: "Target audience description 50-80 words",
			},
			"verdictReason": {
				Type:        genai.TypeString,
				Description: "Summary reasoning 30-50 words",
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
						"question": {Type: genai.TypeString, Description: "Complete question in English"},
						"answer":   {Type: genai.TypeString, Description: "Answer 40-60 words"},
					},
					Required: []string{"question", "answer"},
				},
				Description: "5 FAQs matching English search intent",
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
				Description: "CTR-focused title 50-60 characters",
			},
			"titleBalanced": {
				Type:        genai.TypeString,
				Description: "Google-friendly H1 title 50-60 characters",
			},
			"metaDescription": {
				Type:        genai.TypeString,
				Description: "Meta description 150-160 characters",
			},
			"slug": {
				Type:        genai.TypeString,
				Description: "URL slug e.g. 'dass-541-eng-sub-review'",
			},
			"keywords": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "5-8 SEO keywords in English",
			},
			"searchIntents": {
				Type:        genai.TypeArray,
				Items:       &genai.Schema{Type: genai.TypeString},
				Description: "4-5 search intent phrases for internal linking",
			},
			"rating": {
				Type:        genai.TypeNumber,
				Description: "Rating 1-5 (1 decimal place) for Review JSON-LD",
			},
		},
		Required: []string{"titleAggressive", "titleBalanced", "metaDescription", "slug", "keywords", "searchIntents", "rating"},
	}
}
