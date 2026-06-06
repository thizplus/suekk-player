"""
LLM Prompts for subtitle translation.

Prompt logic ที่เคยอยู่ใน gemini_adapter.py ย้ายมาอยู่ที่นี่
เพื่อให้เปลี่ยน LLM provider ได้โดยไม่ต้องแก้ prompt
"""

from typing import List, Dict, Tuple, Optional

from shared.entities import SubtitleLine, LanguageCode
from shared.ports.llm_port import LLMPort, LLMResponse


def build_translate_prompt(
    lines: List[SubtitleLine],
    source_lang: LanguageCode,
    target_lang: LanguageCode,
    context: str = "",
) -> str:
    """Build translation prompt for a batch of subtitles"""
    lines_text = "\n".join([f"{l.index}|{l.text}" for l in lines])

    return f"""Translate the following subtitles from {source_lang.name} to {target_lang.name}.

Context: {context or 'Video subtitle'}

Rules:
- Keep the index number before each line
- Translate naturally, not literally
- Keep short and readable (max 45 chars per line)
- Output format: index|translated_text

Subtitles:
{lines_text}

Translation:"""


def build_cluster_prompt(
    lines: List[SubtitleLine],
    previous_summary: Optional[str],
    source_lang: LanguageCode,
    target_lang: LanguageCode,
    context: str = "",
    is_scene_change: bool = False,
) -> str:
    """Build cluster translation prompt with context"""
    lines_text = "\n".join([
        f"{l.index}|{l.text}|{l.gender or 'unknown'}|{l.speaker_id or ''}"
        for l in lines
    ])

    summary_text = ""
    if previous_summary and not is_scene_change:
        summary_text = f"\nPrevious context: {previous_summary}"

    return f"""Translate these {source_lang.name} subtitles to {target_lang.name}.

Video context: {context or 'Video subtitle'}{summary_text}

Rules:
- Translate naturally with context awareness
- Consider speaker gender for pronouns
- Output format: index|translated_text
- After translations, provide a 1-2 sentence summary

Lines (index|text|gender|speaker):
{lines_text}

Translation:
[translations here]

Summary:
[brief summary for next cluster]"""


def parse_translate_response(response_text: str, lines: List[SubtitleLine]) -> Dict[int, str]:
    """Parse translation response into index->text mapping"""
    results = {}
    for line in response_text.strip().split("\n"):
        line = line.strip()
        if "|" in line:
            parts = line.split("|", 1)
            if len(parts) == 2:
                try:
                    idx = int(parts[0].strip())
                    text = parts[1].strip()
                    results[idx] = text
                except ValueError:
                    continue
    return results


def parse_cluster_response(
    response_text: str,
    lines: List[SubtitleLine],
) -> Tuple[Dict[int, str], str]:
    """Parse cluster response with summary"""
    results = {}
    summary = ""

    in_summary = False
    for line in response_text.strip().split("\n"):
        line = line.strip()

        if "Summary:" in line or in_summary:
            in_summary = True
            if "Summary:" in line:
                summary = line.replace("Summary:", "").strip()
            else:
                summary += " " + line.strip()
            continue

        if "|" in line:
            parts = line.split("|", 1)
            if len(parts) == 2:
                try:
                    idx = int(parts[0].strip())
                    text = parts[1].strip()
                    results[idx] = text
                except ValueError:
                    continue

    return results, summary.strip()[:100]


def translate_cluster(
    llm: LLMPort,
    lines: List[SubtitleLine],
    previous_summary: Optional[str],
    source_lang: LanguageCode,
    target_lang: LanguageCode,
    context: str = "",
    is_scene_change: bool = False,
) -> Tuple[List[SubtitleLine], str]:
    """
    Translate speech cluster using LLM (any provider).

    Args:
        llm: LLM provider (Gemini, OpenAI, etc.)
        lines: Subtitles in this cluster
        previous_summary: Summary from previous cluster
        source_lang: Source language
        target_lang: Target language
        context: Video context
        is_scene_change: True if new scene

    Returns:
        Tuple of (translated lines, new summary)
    """
    if not lines:
        return [], previous_summary or ""

    prompt = build_cluster_prompt(
        lines, previous_summary, source_lang, target_lang, context, is_scene_change
    )

    response = llm.generate_unsafe(prompt)

    if not response.success:
        return lines, previous_summary or ""

    results, new_summary = parse_cluster_response(response.text, lines)

    translated = []
    for line in lines:
        new_text = results.get(line.index, line.text)
        translated.append(line.with_text(new_text))

    return translated, new_summary
