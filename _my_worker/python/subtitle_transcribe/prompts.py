"""
LLM Prompts for subtitle transcription refinement.

Prompt logic ที่เคยอยู่ใน gemini_adapter.py ย้ายมาอยู่ที่นี่
เพื่อให้เปลี่ยน LLM provider ได้โดยไม่ต้องแก้ prompt
"""

from typing import List, Dict

from shared.entities import SubtitleLine, LanguageCode
from shared.ports.llm_port import LLMPort, LLMResponse


def build_refine_prompt(lines: List[SubtitleLine], language: LanguageCode, context: str = "") -> str:
    """Build refinement prompt for fixing Whisper transcription errors"""
    lines_text = "\n".join([f"{l.index}|{l.text}" for l in lines])

    return f"""Fix Whisper speech-to-text errors in these {language.name} subtitles.
Content: Casual {language.name} dialogue (conversational, informal)

Common Whisper errors to fix:
- Wrong kanji (e.g., 私 vs 渡し, 行く vs 逝く)
- Missing particles (は、が、を、に、で、と)
- Merged words that should be separate
- Split words that should be merged
- Hallucinated repetitive phrases (remove if clearly not speech)
- Incorrect honorifics or verb endings

Do NOT:
- Rewrite meaning or rephrase sentences
- Remove natural fillers (えーと, うん, あの, ふーん)
- Add text that wasn't there
- Change correct text

Rules:
- If a line looks correct, return it unchanged
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
    confidence_threshold: float = -0.3,
) -> List[SubtitleLine]:
    """
    Refine subtitles using LLM (any provider).
    เฉพาะ segments ที่ Whisper confidence < threshold เท่านั้นที่ส่ง LLM
    segments ที่ confidence สูงจะข้ามไป (ลด API calls)

    Args:
        llm: LLM provider (Gemini, OpenAI, etc.)
        lines: Subtitles to refine
        language: Language code
        context: Video context
        batch_size: Lines per API call
        confidence_threshold: ข้ามถ้า avg_logprob >= threshold (default -0.3, higher = more confident)
    """
    if not lines:
        return []

    import logging
    logger = logging.getLogger(__name__)

    # แยก segments ที่ต้อง refine vs ข้ามได้
    needs_refine = []
    high_confidence = []
    for line in lines:
        if line.confidence is not None and line.confidence >= confidence_threshold:
            high_confidence.append(line)
        else:
            needs_refine.append(line)

    logger.info(
        f"Confidence filter: {len(needs_refine)} need refine, "
        f"{len(high_confidence)} skipped (>= {confidence_threshold})"
    )

    # เก็บผลลัพธ์ — high confidence ใช้ text เดิม
    results = {}
    for line in high_confidence:
        results[line.index] = line.text

    # ส่งเฉพาะ needs_refine ไป LLM
    if needs_refine:
        total_batches = (len(needs_refine) + batch_size - 1) // batch_size

        for i in range(0, len(needs_refine), batch_size):
            batch = needs_refine[i:i + batch_size]
            batch_num = i // batch_size + 1
            logger.info(f"Refining batch {batch_num}/{total_batches} ({len(batch)} lines)")
            prompt = build_refine_prompt(batch, language, context)

            response = llm.generate_unsafe(prompt)
            if response.success:
                batch_results = parse_refine_response(response.text, batch)
                results.update(batch_results)
                logger.info(f"Batch {batch_num}/{total_batches} done, refined {len(batch_results)} lines")
            else:
                logger.warning(f"Batch {batch_num}/{total_batches} failed: {response.error}, keeping original")
                for line in batch:
                    results[line.index] = line.text

    # รวมผลลัพธ์ตาม index เดิม
    refined = []
    for line in lines:
        new_text = results.get(line.index, line.text)
        refined.append(line.with_text(new_text))

    return refined
