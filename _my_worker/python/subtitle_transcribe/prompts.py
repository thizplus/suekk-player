"""
LLM Prompts for subtitle transcription refinement.

Prompt logic ที่เคยอยู่ใน gemini_adapter.py ย้ายมาอยู่ที่นี่
เพื่อให้เปลี่ยน LLM provider ได้โดยไม่ต้องแก้ prompt
"""

from typing import List, Dict

from shared.entities import SubtitleLine, LanguageCode
from shared.ports.llm_port import LLMPort, LLMResponse


def build_refine_prompt(lines: List[SubtitleLine], language: LanguageCode, context: str = "") -> str:
    """Build refinement prompt for fixing transcription errors"""
    lines_text = "\n".join([f"{l.index}|{l.text}" for l in lines])

    return f"""Fix transcription errors in these {language.name} subtitles.

Context: {context or 'Video subtitle'}

Rules:
- Fix obvious typos and errors
- Keep natural speech patterns
- Keep the index number before each line
- Output format: index|fixed_text

Subtitles:
{lines_text}

Fixed:"""


def parse_refine_response(response_text: str, lines: List[SubtitleLine]) -> Dict[int, str]:
    """Parse LLM response into index->text mapping"""
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


def refine_subtitles(
    llm: LLMPort,
    lines: List[SubtitleLine],
    language: LanguageCode,
    context: str = "",
    batch_size: int = 30,
) -> List[SubtitleLine]:
    """
    Refine subtitles using LLM (any provider).

    Args:
        llm: LLM provider (Gemini, OpenAI, etc.)
        lines: Subtitles to refine
        language: Language code
        context: Video context
        batch_size: Lines per API call
    """
    if not lines:
        return []

    results = {}

    for i in range(0, len(lines), batch_size):
        batch = lines[i:i + batch_size]
        prompt = build_refine_prompt(batch, language, context)

        response = llm.generate_unsafe(prompt)
        if response.success:
            batch_results = parse_refine_response(response.text, batch)
            results.update(batch_results)
        else:
            # Keep original text on error
            for line in batch:
                results[line.index] = line.text

    refined = []
    for line in lines:
        new_text = results.get(line.index, line.text)
        refined.append(line.with_text(new_text))

    return refined
