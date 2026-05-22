package ai

// ============================================================================
// Global Rules V2 for English: Shared across all EN Chunks
// ============================================================================

// GlobalConstraintsEN - Global constraints for English content generation
const GlobalConstraintsEN = `
## GLOBAL CONSTRAINTS (Mandatory for all Chunks)

### 1. SOURCE MATERIAL CONTEXT
You are provided with a high-quality THAI transcript of a Japanese video.
Your mission is to CREATE a NEW article in English - DO NOT TRANSLATE.
Analyze the plot, character motives, and emotional turns from the Thai text,
then write original English content using Film Critic vocabulary.

### 2. VOCABULARY LEVEL: FILM CRITIC
Use sophisticated vocabulary appropriate for professional film criticism:
- Instead of "good acting" -> "captivating performance", "nuanced portrayal"
- Instead of "nice story" -> "compelling narrative", "intricate plotline"
- Instead of "sad scene" -> "emotionally resonant moment", "poignant sequence"
- Instead of "beautiful" -> "visually striking", "aesthetically refined"
- Instead of "interesting" -> "thought-provoking", "engaging", "riveting"

### 3. PRONOUN RULES (Critical)
- First mention: Full romanized name (e.g., "Tachibana Mary")
- 2nd-3rd mention: First name only (e.g., "Mary") or "she/her"
- 4th+ mention: Use "she/her" or role ("the wife", "the nurse")
- For group scenes (3P/4P): Use "they/them" for clarity
- NEVER use Thai-style pronouns like "Ter" or mixed language

### 4. NAME FORMAT
- Always use FULL ROMANIZED names: "Tachibana Mary" not "Mary" alone first time
- Maintain consistency: same spelling throughout
- For Japanese names: Given name Last name order (Western style)

### 5. CULTURAL ADAPTATION
Explain cultural concepts that Western readers may not understand:
- Japanese honorifics: explain when relevant
- Asian family dynamics: provide context
- Cultural practices: brief explanations in parentheses or natural integration

### 6. SEO REQUIREMENTS
- Meta Title: MUST include "[Eng Sub]" or "[English Subtitles]"
- Keywords: Use search terms Western audiences use:
  - "Plot Analysis", "Character Breakdown", "Detailed Review"
  - "Japanese Adult Film", "With English Subtitles"
  - NOT translated Thai terms

### 7. ANTI-REPETITION RULES
- Do NOT start consecutive sentences with the same word/name
- Vary sentence structures throughout
- In lists: alternate starting patterns (situation/action/emotion/technique)
`

// GlobalConstraintsForArraysEN - Additional rules for Array fields in English
const GlobalConstraintsForArraysEN = `
### ARRAY FIELDS - Required Diversity
For highlights, bestMoments, faqItems, atmosphereNotes, genreInsights:
- Item 1: Start with situation/scene description
- Item 2: Start with action verb
- Item 3: Start with "When..." or "During..."
- Item 4: Start with "The use of..." or "A technique..."
- Item 5+: Start with outcome/emotional result
`

// ContextReinforcementEN - Template for reinforcing context in each chunk
const ContextReinforcementEN = `
[SOURCE MATERIAL]
You are analyzing a THAI transcript of a Japanese video.

[YOUR MISSION]
1. Deeply analyze the plot, character motives, and emotional turns from the Thai text.
2. Create a NEW article in English. DO NOT translate word-by-word.
3. Use the vocabulary of a professional movie critic.
4. Ensure the tone is sophisticated yet engaging for a global audience.
5. Explain any cultural elements that Western readers might not understand.

[OUTPUT REQUIREMENTS]
- Write in fluent, natural English
- Target audience: English-speaking film enthusiasts
- Tone: Professional film critic / review style
- SEO optimized for Western search engines
`

// MetaDescriptionRulesEN - Rules for metaDescription length
const MetaDescriptionRulesEN = `
### META DESCRIPTION (English)
- Length: 150-160 characters (strict limit)
- Include: Video code, main cast name, key plot element
- Must end with call-to-action
- Example: "[JUR-177] Mary's secret affair with her husband's friend unfolds in this emotional drama. Watch with English subtitles now."
`

// ReadingTimeAdjustmentEN - Note about EN content length
const ReadingTimeAdjustmentEN = `
### READING TIME ADJUSTMENT
- English content is approximately 20-30% longer than Thai equivalent
- Adjust word counts accordingly:
  - Summary: 500-650 words (vs 400-500 TH)
  - DetailedReview: 750-1000 words (vs 600-800 TH)
- Reading time calculation: Total words / 200 WPM
`
