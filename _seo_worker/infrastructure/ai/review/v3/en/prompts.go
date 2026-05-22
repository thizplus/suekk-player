package en

import (
	"fmt"
	"hash/fnv"
	"strings"
)

// ============================================================================
// English Review Prompts (V3 Intent-Driven)
// Native English content - NOT translation from Thai
// ============================================================================

// PromptParams contains all data needed for prompt generation
type PromptParams struct {
	VideoCode    string
	VideoID      string
	Duration     int // seconds
	ReleaseDate  string
	CastNames    []string
	Tags         []string
	MakerName    string
	SRTContent   string
	GalleryCount int
}

// Chunk1Context is the output from Chunk 1
type Chunk1Context struct {
	QuickAnswer string
	Verdict     string
}

// Chunk3Context is the output from Chunk 3
type Chunk3Context struct {
	Synopsis  string
	StoryFlow string
	Tone      string
	KeyScenes []string
}

// Chunk4Context is the output from Chunk 4
type Chunk4Context struct {
	ReviewSummary  string
	Strengths      []string
	Weaknesses     []string
	WhoShouldWatch string
	VerdictReason  string
}

// ============================================================================
// Opening Styles (Anti-Similarity) - English Native
// ============================================================================

var openingStyles = map[string]string{
	"A": `Style A - Direct Statement: Start with factual information
Example: "This release follows [character] as they navigate [situation]..."`,

	"B": `Style B - Premise Hook: Start with the core premise
Example: "What happens when [situation]? This title explores that exact scenario..."`,

	"C": `Style C - Genre Appeal: Start with genre positioning
Example: "Fans of [genre] will find exactly what they're looking for in this release..."`,

	"D": `Style D - Studio Context: Start with studio/production context
Example: "From [studio], known for their [style] productions, comes another compelling entry..."`,
}

var reviewTones = map[string]string{
	"Analytical": `Analytical: In-depth analysis with clear reasoning
- Use formal language
- Provide specific examples
- Focus on technical merits`,

	"Conversational": `Conversational: Engaging, accessible review style
- Use natural, flowing language
- Share genuine reactions
- Balance objectivity with personality`,

	"Comparative": `Comparative: Context within the genre/studio catalog
- Reference similar works
- Highlight distinguishing factors
- Provide broader perspective`,
}

var faqStyles = map[string]string{
	"Formal": `Formal: Professional question structure
Example: "What is [CODE] about?"`,

	"Casual": `Casual: Natural question phrasing
Example: "Is [CODE] worth watching?"`,
}

// GetOpeningStylePrompt returns the prompt for opening style
func GetOpeningStylePrompt(style string) string {
	if prompt, ok := openingStyles[style]; ok {
		return prompt
	}
	return openingStyles["A"]
}

// GetReviewTonePrompt returns the prompt for review tone
func GetReviewTonePrompt(tone string) string {
	if prompt, ok := reviewTones[tone]; ok {
		return prompt
	}
	return reviewTones["Analytical"]
}

// GetFAQStylePrompt returns the prompt for FAQ style
func GetFAQStylePrompt(style string) string {
	if prompt, ok := faqStyles[style]; ok {
		return prompt
	}
	return faqStyles["Formal"]
}

// ============================================================================
// Content Variation (deterministic by videoID)
// ============================================================================

// ContentVariation defines the style for content generation
type ContentVariation struct {
	OpeningStyle string // A, B, C, D
	ReviewTone   string // Analytical, Conversational, Comparative
	FAQStyle     string // Formal, Casual
}

// GetContentVariation returns variation based on videoID (deterministic)
func GetContentVariation(videoID string) ContentVariation {
	hash := hashString(videoID)

	openingStyleKeys := []string{"A", "B", "C", "D"}
	openingIdx := hash % uint32(len(openingStyleKeys))

	reviewToneKeys := []string{"Analytical", "Conversational", "Comparative"}
	reviewIdx := (hash / 4) % uint32(len(reviewToneKeys))

	faqStyleKeys := []string{"Formal", "Casual"}
	faqIdx := (hash / 12) % uint32(len(faqStyleKeys))

	return ContentVariation{
		OpeningStyle: openingStyleKeys[openingIdx],
		ReviewTone:   reviewToneKeys[reviewIdx],
		FAQStyle:     faqStyleKeys[faqIdx],
	}
}

// TitleStyleIndices holds indices for title styles (for Chunk 6)
type TitleStyleIndices struct {
	AggressiveIdx      int
	BalancedIdx        int
	IntentSetIdx       int
	IntentOrderSwapped bool
}

// GetTitleStyleIndices returns style indices for Chunk 6 (deterministic)
func GetTitleStyleIndices(videoID string) TitleStyleIndices {
	h := hashString(videoID)

	return TitleStyleIndices{
		AggressiveIdx:      int(h % 4),
		BalancedIdx:        int((h / 10) % 4),
		IntentSetIdx:       int((h / 100) % 6),
		IntentOrderSwapped: (h/1000)%2 == 1,
	}
}

func hashString(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// TruncateSRT limits SRT content to maxLen characters
func TruncateSRT(srt string, maxLen int) string {
	if len(srt) <= maxLen {
		return srt
	}
	return srt[:maxLen] + "..."
}

// TruncateToWords limits text to approximately maxWords
func TruncateToWords(text string, maxWords int) string {
	words := 0
	lastSpace := 0
	for i, r := range text {
		if r == ' ' || r == '\n' {
			words++
			lastSpace = i
			if words >= maxWords {
				return text[:lastSpace] + "..."
			}
		}
	}
	return text
}

// ============================================================================
// Chunk 1: Search Intent Answer (English Native)
// ============================================================================

func BuildChunk1Prompt(params *PromptParams) string {
	castNamesStr := strings.Join(params.CastNames, ", ")

	variation := GetContentVariation(params.VideoID)
	openingStylePrompt := GetOpeningStylePrompt(variation.OpeningStyle)

	return fmt.Sprintf(`[ROLE]
You are a Google Featured Snippet Writer specializing in adult film reviews.

[GOAL]
Answer search queries for "%s review" and "%s English sub" instantly.
Write as if someone asked "Is this worth watching?" and you're giving a direct answer.

[IMPORTANT - NATIVE ENGLISH CONTENT]
- Write original English content, NOT a translation
- Use natural English phrasing and idioms
- Target English-speaking audience searching for subtitled content
- Assume reader may be unfamiliar with Japanese adult film conventions

[INPUT]
- Code: %s
- Duration: %d minutes
- Cast: %s

SRT Transcript (for understanding the plot):
%s

---

[OUTPUT]

1. **quickAnswer** (2-3 sentences) - DECLARATIVE STATEMENT only!
   ✅ Must start with a factual statement:
   - "[CODE] is a drama about..."
   - "This release follows [character] as..."
   - "Featuring [actress], this title explores..."

   🚫 FORBIDDEN - No questions in the first sentence:
   - "Looking for...?" ❌
   - "Ever wondered...?" ❌
   - "Curious about...?" ❌

   Google doesn't rank question hooks well!

2. **mainHook** (1 sentence)
   Create curiosity without clickbait:
   - Tease the unique angle
   - Highlight what makes it stand out

3. **verdict** (1 sentence) - SOFT RECOMMENDATION
   ✅ State who it's for:
   - "Ideal for fans of slow-burn drama and character development"
   - "Best suited for viewers who appreciate..."

   ❌ NO aggressive clickbait:
   - "You'll regret missing this!"
   - "A must-watch!"

---

[RULES]
- ❌ No lengthy introductions
- ❌ No essay-style writing
- ❌ No clickbait tone
- ✅ Direct, declarative statements
- ✅ Use cast names: %s
- ✅ Write for English-speaking audience

🆕 [FIRST 150 WORDS RULE - Critical!]
quickAnswer must naturally contain:
- CODE: %s
- "English sub" or "subtitled"
- "review"
- Cast names: %s

⚠️ [ANTI-STUFFING RULE]
- ❌ Don't use CODE (%s) more than 3 times in first 200 words
- ✅ Use synonyms: "this title", "the film", "this release"
- ✅ Maintain natural keyword density
- ❌ No keyword stuffing

🚨 [OPENING STYLE - Use Style %s] 🚨
%s

🚨 [ANTI-SIMILARITY RULE - Critical!] 🚨
❌ Don't use the same opening pattern as other articles
❌ Don't always start with "[CODE] is..." (except Style A)
❌ Vary sentence structure across articles

✅ Must vary:
- Subject position (character vs studio vs genre)
- Sentence length (short vs long)
- Opening hook type

⚠️ Use ONLY the Opening Style specified above!
`,
		params.VideoCode,
		params.VideoCode,
		params.VideoCode,
		params.Duration/60,
		castNamesStr,
		TruncateSRT(params.SRTContent, 2000),
		castNamesStr,
		params.VideoCode,
		castNamesStr,
		params.VideoCode,
		variation.OpeningStyle,
		openingStylePrompt,
	)
}

// ============================================================================
// Chunk 2: Structured Facts
// ============================================================================

func BuildChunk2Prompt(params *PromptParams) string {
	castNamesStr := strings.Join(params.CastNames, ", ")

	var tagsStr strings.Builder
	for _, tag := range params.Tags {
		tagsStr.WriteString(fmt.Sprintf("- %s\n", tag))
	}

	makerName := params.MakerName
	if makerName == "" {
		makerName = "Unknown"
	}

	return fmt.Sprintf(`[ROLE]
You are a Data Extractor for video metadata.

[GOAL]
Extract structured facts from metadata.
This data will be used for Facts Table and JSON-LD Schema.

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

3. **cast**: ["%s"] (include all performers)

4. **duration**: "%d minutes"

5. **genre**: Extract 2-4 relevant genres from tags
   - Choose searchable genres: Drama, Romance, Office, Medical, etc.

6. **releaseYear**: Extract from release date

7. **subtitleAvailable**: true (our site has English subtitles)

---

[RULES]
- ✅ Data must match provided metadata
- ✅ Select genres from provided tags
- ✅ Use English for all fields
`,
		params.VideoCode,
		makerName,
		castNamesStr,
		params.Duration,
		params.ReleaseDate,
		tagsStr.String(),
		params.VideoCode,
		makerName,
		strings.Join(params.CastNames, `", "`),
		params.Duration/60,
	)
}

// ============================================================================
// Chunk 3: Story Recap (English Native)
// ============================================================================

func BuildChunk3Prompt(params *PromptParams, chunk1 *Chunk1Context) string {
	castNamesStr := strings.Join(params.CastNames, ", ")

	return fmt.Sprintf(`[ROLE]
You are a Film Synopsis Writer for an English-speaking audience.

[GOAL]
Create an original story summary from the SRT transcript.
This is unique content that competitors don't have.
Write for readers unfamiliar with Japanese adult film conventions.

[CONTEXT from Chunk 1]
Quick Answer: %s
Verdict: %s

[INPUT]
- Code: %s
- Cast: %s
- Duration: %d minutes

SRT Transcript:
%s

---

[OUTPUT]

1. **synopsis** (150-250 words, 2-3 paragraphs)
   - Use [PARA] to separate paragraphs
   - Tell the story chronologically
   - Focus on character motivations from dialogue
   - Explain any cultural context Western viewers might need

   Structure example:
   "The story begins when [character] [situation]... [PARA] As events unfold, [development]... [PARA] Ultimately, [resolution]..."

2. **storyFlow** (80-100 words)
   Timeline breakdown:
   - Opening: What sets the scene
   - Development: How relationships evolve
   - Climax: The pivotal moment

3. **keyScenes** (3-5 scenes)
   - Each scene 15-25 words
   - ❌ Don't start with performer names
   - ✅ Start with "The scene where...", "A pivotal moment...", "The sequence featuring..."

4. **featuredScene** (120-150 words) 🔥 Scroll Depth Trigger!
   Select the most compelling scene and describe it in detail.

   ✅ Must include all 4 elements:
   a) **Atmosphere** - Setting, lighting, mood
   b) **Context** - What led to this moment, why it matters
   c) **Action/Dialogue** - What happens, brief quote if relevant
   d) **Impact** - How this changes the story

   ❌ Don't just quote dialogue
   ❌ No generic descriptions like "this scene is exciting"
   ✅ Be specific to THIS title

   Structure example:
   "In a softly lit room [atmosphere]... Following [context]...
   [character] says '...' [dialogue]... This moment shifts everything because... [impact]"

5. **tone** (2-3 words)
   Examples: "Sensual and tender", "Intense drama", "Playful romantic"

6. **relationshipDynamic** (50-80 words)
   - Character relationship dynamics
   - Power balance
   - Emotional undertones

---

[RULES]
- ✅ Use only information from the SRT
- ✅ Use performer names: %s
- ✅ Write for English-speaking audience
- ✅ Explain Japanese cultural elements if needed
- ❌ Don't invent plot points
- ❌ No explicit descriptions
`,
		chunk1.QuickAnswer,
		chunk1.Verdict,
		params.VideoCode,
		castNamesStr,
		params.Duration/60,
		TruncateSRT(params.SRTContent, 3000),
		castNamesStr,
	)
}

// ============================================================================
// Chunk 4: Review Section (English Native)
// ============================================================================

func BuildChunk4Prompt(params *PromptParams, chunk1 *Chunk1Context, chunk3 *Chunk3Context) string {
	castNamesStr := strings.Join(params.CastNames, ", ")

	variation := GetContentVariation(params.VideoID)
	reviewTonePrompt := GetReviewTonePrompt(variation.ReviewTone)

	return fmt.Sprintf(`[ROLE]
You are a Film Critic reviewing for an English-speaking audience.
Write like a professional reviewer, not an academic essay.

[GOAL]
Create a verdict-style review.
Be clear about strengths, weaknesses, and target audience.

🚨 [REVIEW TONE - Use %s Style] 🚨
%s

[CONTEXT]
Quick Answer: %s
Verdict: %s
Synopsis: %s
Tone: %s

[INPUT]
- Code: %s
- Cast: %s

---

[OUTPUT]

1. **reviewSummary** (200-300 words, 3 paragraphs)
   Use [PARA] to separate paragraphs.

   Structure:
   - Paragraph 1: Overall impression + main highlight
   - Paragraph 2: Performance quality + technical aspects
   - Paragraph 3: Final thoughts + target audience

   ❌ No academic essay style
   ✅ Write like a professional film critic

2. **strengths** (3-5 points)
   Key strengths, each 10-20 words

3. **weaknesses** (2-3 points)
   Honest critiques, each 10-20 words
   ⚠️ Must include at least 1 (nothing is perfect)

4. **whoShouldWatch** (50-80 words)
   Be specific about:
   - Ideal viewer: [type of viewer]
   - Not recommended for: [type of viewer]

5. **verdictReason** (30-50 words)
   Summary reasoning in 1-2 sentences

---

[RULES]
- ✅ Direct and honest
- ✅ Clear verdict
- ✅ Use names: %s
- ✅ Professional critic tone
- ❌ No vulgar language
- ❌ No essay-style writing
- ⚠️ Use ONLY the Review Tone specified above!
`,
		variation.ReviewTone,
		reviewTonePrompt,
		chunk1.QuickAnswer,
		chunk1.Verdict,
		TruncateToWords(chunk3.Synopsis, 100),
		chunk3.Tone,
		params.VideoCode,
		castNamesStr,
		castNamesStr,
	)
}

// ============================================================================
// Chunk 5: FAQ Intent Block (English Native)
// ============================================================================

func BuildChunk5Prompt(params *PromptParams, chunk1 *Chunk1Context, chunk3 *Chunk3Context, chunk4 *Chunk4Context) string {
	castNamesStr := strings.Join(params.CastNames, ", ")
	firstCast := ""
	if len(params.CastNames) > 0 {
		firstCast = params.CastNames[0]
	}

	strengthsStr := strings.Join(chunk4.Strengths, ", ")

	variation := GetContentVariation(params.VideoID)
	faqStylePrompt := GetFAQStylePrompt(variation.FAQStyle)

	return fmt.Sprintf(`[ROLE]
You are an FAQ Writer targeting English-speaking search intent + conversion.

[GOAL]
Create 5 FAQs that match what English speakers actually search for.
FAQ #5 must be a Conversion FAQ (drive membership signup).

🚨 [FAQ STYLE - Use %s Style] 🚨
%s

[CONTEXT]
Quick Answer: %s
Verdict: %s
Synopsis: %s
Strengths: %s
Who Should Watch: %s

[INPUT]
- Code: %s
- Cast: %s

---

[OUTPUT]

Create **faqItems** with exactly 5 items:

### FAQ 1: Plot Overview
Question: "What is %s about?"
Answer: [Summarize plot in 2-3 sentences from synopsis]

### FAQ 2: Subtitles
Question: "Does %s have English subtitles?"
Answer: "Yes, %s features high-quality English subtitles with complete dialogue translation, allowing you to fully enjoy the story and performances."

### FAQ 3: Worth Watching
Question: "Is %s worth watching?"
Answer: [Answer based on verdict + reasoning, 2-3 sentences]

### FAQ 4: Standout Performance
Question: "How is %s's performance in %s?"
Answer: "[Performer] delivers a [quality] performance as [character], [specific praise 1-2 sentences]"

### FAQ 5: Where to Watch (CONVERSION FAQ!)
Question: "Where can I watch %s with English subtitles?"
Answer: "Watch the full version of %s with English subtitles through our membership. Subscribe for access to high-quality content with professional subtitles across our entire library."

---

[RULES]
- ✅ Complete question format (end with ?)
- ✅ Answers should be 40-60 words
- ✅ FAQ #5 must include conversion copy
- ✅ Use names: %s
- ✅ Natural English phrasing
- ❌ No vulgar language
- ⚠️ Use ONLY the FAQ Style specified above!
`,
		variation.FAQStyle,
		faqStylePrompt,
		chunk1.QuickAnswer,
		chunk1.Verdict,
		TruncateToWords(chunk3.Synopsis, 80),
		strengthsStr,
		chunk4.WhoShouldWatch,
		params.VideoCode,
		castNamesStr,
		params.VideoCode,
		params.VideoCode,
		params.VideoCode,
		params.VideoCode,
		firstCast,
		params.VideoCode,
		params.VideoCode,
		params.VideoCode,
		castNamesStr,
	)
}

// ============================================================================
// Chunk 6: SEO Output (English Native)
// ============================================================================

var titleAggressiveStyles = []string{
	`Format: "[CODE] English Sub - [Intriguing Question]?"`,
	`Format: "[CODE] Review: [Number] Reasons to Watch"`,
	`Format: "[CODE] English Sub - [Emotional Hook]"`,
	`Format: "[CODE] - What Makes This [Unique Angle]"`,
}

var titleBalancedStyles = []string{
	`Format: "[CODE] Review [Eng Sub] - Complete Analysis"`,
	`Format: "[CODE] English Subtitles - Your Guide"`,
	`Format: "[CODE] [Eng Sub] - Plot & Review"`,
	`Format: "[CODE] Full Review with English Subtitles"`,
}

var searchIntentsTemplateSets = [][]string{
	{"%s English sub", "%s review"},
	{"watch %s subtitled", "%s plot summary"},
	{"%s streaming", "%s synopsis"},
	{"%s English subtitles", "%s worth watching"},
	{"where to watch %s", "%s full review"},
	{"%s analysis", "%s recommendation"},
}

func BuildChunk6Prompt(params *PromptParams, chunk1 *Chunk1Context, chunk3 *Chunk3Context, chunk4 *Chunk4Context) string {
	castNamesStr := strings.Join(params.CastNames, ", ")
	firstCast := ""
	if len(params.CastNames) > 0 {
		firstCast = params.CastNames[0]
	}

	var tagsStr strings.Builder
	for _, tag := range params.Tags {
		tagsStr.WriteString(fmt.Sprintf("- %s\n", tag))
	}

	tsi := GetTitleStyleIndices(params.VideoID)

	code := params.VideoCode
	aggressiveStyle := titleAggressiveStyles[tsi.AggressiveIdx]
	balancedStyle := titleBalancedStyles[tsi.BalancedIdx]

	templates := searchIntentsTemplateSets[tsi.IntentSetIdx]
	if tsi.IntentOrderSwapped && len(templates) >= 2 {
		templates = []string{templates[1], templates[0]}
	}
	var intentTemplateStr strings.Builder
	for _, tmpl := range templates {
		intentTemplateStr.WriteString(fmt.Sprintf("   - \"%s\"\n", fmt.Sprintf(tmpl, code)))
	}

	return fmt.Sprintf(`[ROLE]
You are an SEO Title Specialist for English-speaking markets.

[GOAL]
Create 2 title variants:
- Aggressive: High CTR focus
- Balanced: Google-friendly for ranking

[CONTEXT]
Quick Answer: %s
Verdict: %s
Tone: %s
Strengths: %s

[INPUT]
- Code: %s
- Cast: %s
- Tags:
%s

---

[OUTPUT]

1. **titleAggressive** (50-60 characters)
   CTR-focused - entice clicks
   🎯 STYLE: %s

2. **titleBalanced** (50-60 characters)
   Google-friendly - for H1/ranking
   🎯 STYLE: %s

3. **metaDescription** (150-160 characters)
   Compelling + keyword-rich

4. **slug**
   Format: "%s-eng-sub" or "%s-review" (lowercase, hyphens)

5. **keywords** (5-8 terms)
   Must include:
   - [CODE] English sub
   - [CODE] review
   - [CODE] plot
   - [Performer name]
   - [Genre terms]

6. **searchIntents** (4-5 phrases)
   Create 2 from template:
%s
   Create 2-3 unique from context:
   - "is %s [genre] themed"
   - "%s performance in %s"

7. **rating** (1-5)
   Review score (1 decimal place)

---

[RULES]
- ✅ titleAggressive must be compelling
- ✅ titleBalanced must be professional
- ✅ metaDescription needs CTA
- ✅ rating must be realistic (not everything is 5/5)
- ✅ All content in English
- ❌ No vulgar keywords
- ❌ No "porn", "xxx", "jav" in keywords
`,
		chunk1.QuickAnswer,
		chunk1.Verdict,
		chunk3.Tone,
		strings.Join(chunk4.Strengths, ", "),
		code,
		castNamesStr,
		tagsStr.String(),
		aggressiveStyle,
		balancedStyle,
		strings.ToLower(code),
		strings.ToLower(code),
		intentTemplateStr.String(),
		code,
		firstCast,
		code,
	)
}
