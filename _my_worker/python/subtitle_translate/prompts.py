"""
LLM Prompts for subtitle translation.

Refactored to use LanguageProfile — ไม่มี hardcode ภาษาใดภาษาหนึ่ง
แต่ละ language pair มี profile ของตัวเองใน profiles/
"""

import logging
import re
from typing import List, Dict, Tuple, Optional

from shared.entities import SubtitleLine, LanguageCode
from shared.ports.llm_port import LLMPort, LLMResponse
from subtitle_translate.profiles import get_profile

logger = logging.getLogger(__name__)


# =============================================================================
# Backward Compat — legacy functions (ใช้โดย handler.py เดิม)
# จะถูกลบเมื่อ handler.py refactor เสร็จ
# =============================================================================

# Keep old imports working
from subtitle_translate.profiles.base import load_dict as _load_dict


def _load_translation_dict() -> dict:
    """Legacy: โหลด shared translation_dict.json (backward compat)"""
    import json
    from pathlib import Path
    path = Path(__file__).parent.parent / "shared" / "translation_dict.json"
    if path.exists():
        data = json.loads(path.read_text(encoding="utf-8"))
        merged = {}
        for section in ["moaning", "fillers", "common_phrases"]:
            if section in data and isinstance(data[section], dict):
                merged.update(data[section])
        return merged
    return {}


def reload_translation_dict():
    """Legacy: Force reload (ไม่จำเป็นแล้ว — profile จัดการเอง)"""
    pass


# Legacy: keep for handler.py imports that haven't been refactored yet
def normalize_av_text(text: str) -> str:
    """Legacy: delegate to JaSourceMixin"""
    from subtitle_translate.profiles.base import JaSourceMixin
    mixin = JaSourceMixin()
    return mixin.normalize_source(text)


def soften_text(text: str) -> str:
    """Legacy: delegate to JaSourceMixin"""
    from subtitle_translate.profiles.base import JaSourceMixin
    mixin = JaSourceMixin()
    return mixin.soften_source(text)


def post_process_translation(text: str) -> str:
    """Legacy: delegate to JaThProfile"""
    from subtitle_translate.profiles.ja_th import JaThProfile
    return JaThProfile().post_process(text)


def is_moaning_sound(text: str) -> bool:
    """Legacy: delegate to JaSourceMixin"""
    from subtitle_translate.profiles.base import JaSourceMixin
    return JaSourceMixin().is_moaning_text(text)


def pre_translate_moaning(text: str) -> Optional[str]:
    """Legacy: delegate to JaThProfile"""
    from subtitle_translate.profiles.ja_th import JaThProfile
    return JaThProfile().pre_translate(text)


# =============================================================================
# Parse Functions (language-agnostic — ใช้ร่วมกันทุก profile)
# =============================================================================

def _clean_translated_text(text: str) -> str:
    """ลบ speaker/gender tags ที่ LLM เอามาติดใน output"""
    text = re.sub(r'\|(unknown|female|male|SPEAKER_\d+)\|?\s*$', '', text)
    text = text.rstrip('|').strip()
    return text


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
                    text = _clean_translated_text(parts[1].strip())
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

        if "Summary:" in line or "สรุป:" in line or in_summary:
            in_summary = True
            if "Summary:" in line:
                summary = line.replace("Summary:", "").strip()
            elif "สรุป:" in line:
                summary = line.replace("สรุป:", "").strip()
            else:
                summary += " " + line.strip()
            continue

        if "|" in line:
            parts = line.split("|", 1)
            if len(parts) == 2:
                try:
                    idx = int(parts[0].strip())
                    text = _clean_translated_text(parts[1].strip())
                    results[idx] = text
                except ValueError:
                    continue

    return results, summary.strip()[:100]


def find_missing_lines(
    results: Dict[int, str],
    source_lines: List[SubtitleLine],
) -> List[int]:
    """หาบรรทัดที่ LLM ข้ามไป"""
    return [line.index for line in source_lines if line.index not in results]


def _retry_failed_lines(
    llm: LLMPort,
    lines: List[SubtitleLine],
    source_lang: LanguageCode,
    target_lang: LanguageCode,
) -> Dict[int, str]:
    """Retry แปลบรรทัดที่ silent failure"""
    lines_text = "\n".join([f"{l.index}|{l.text}" for l in lines])
    prompt = f"""Translate these {source_lang.name} lines to {target_lang.name}.
You MUST translate every line. Do NOT return the original text.
Output format: index|translated_text

{lines_text}"""
    response = llm.generate_unsafe(prompt)
    if response.success:
        return parse_translate_response(response.text, lines)
    return {}


def _retry_missing_lines(
    llm: LLMPort,
    lines: List[SubtitleLine],
    source_lang: LanguageCode,
    target_lang: LanguageCode,
) -> Dict[int, str]:
    """Retry แปลบรรทัดที่หายไป"""
    lines_text = "\n".join([f"{l.index}|{l.text}" for l in lines])
    prompt = f"""Translate ALL these {source_lang.name} lines to {target_lang.name}.
You MUST include every index number. Do NOT skip any line.
Output format: index|translated_text

{lines_text}"""
    response = llm.generate_unsafe(prompt)
    if response.success:
        return parse_translate_response(response.text, lines)
    return {}


# =============================================================================
# translate_cluster — main entry point (uses LanguageProfile)
# =============================================================================

def translate_cluster(
    llm: LLMPort,
    lines: List[SubtitleLine],
    previous_summary: Optional[str],
    source_lang: LanguageCode,
    target_lang: LanguageCode,
    context: str = "",
    is_scene_change: bool = False,
    use_softening: bool = True,
) -> Tuple[List[SubtitleLine], str]:
    """
    Translate speech cluster using LLM + LanguageProfile.
    Profile ตัดสินใจทุกอย่าง — ไม่มี hardcode ภาษา
    """
    if not lines:
        return [], previous_summary or ""

    profile = get_profile(source_lang, target_lang)

    # Step 1: Pre-translate (moaning/fillers) — profile ตัดสินใจ
    moaning_results = {}
    lines_for_llm = []
    for line in lines:
        translated = profile.pre_translate(line.text)
        if translated:
            moaning_results[line.index] = translated
        else:
            lines_for_llm.append(line)

    if moaning_results:
        logger.info(f"Pre-translated {len(moaning_results)} moaning sounds")

    # Step 2: Normalize source text — profile ตัดสินใจ
    normalized_lines = []
    for line in lines_for_llm:
        normalized_text = profile.normalize_source(line.text)
        if normalized_text != line.text:
            logger.debug(f"Normalize: {line.text} → {normalized_text}")
            normalized_lines.append(line.with_text(normalized_text))
        else:
            normalized_lines.append(line)
    lines_for_llm = normalized_lines

    # Step 3: ถ้าทุกบรรทัดเป็น pre-translate → ไม่ต้องเรียก LLM
    if not lines_for_llm:
        translated = []
        for line in lines:
            new_text = moaning_results.get(line.index, line.text)
            translated.append(line.with_text(new_text))
        return translated, previous_summary or ""

    # Step 4: Build prompt + call LLM — profile ตัดสินใจ prompt
    prompt = profile.build_prompt(
        lines_for_llm, previous_summary, context, is_scene_change
    )
    response = llm.generate_unsafe(prompt)

    # Step 5: ถ้า block → soften แล้ว retry — profile ตัดสินใจ soften
    if not response.success and use_softening:
        logger.warning("LLM blocked, retrying with softened text...")
        softened_lines = [
            line.with_text(profile.soften_source(line.text))
            for line in lines_for_llm
        ]
        prompt_soft = profile.build_prompt(
            softened_lines, previous_summary, context, is_scene_change
        )
        response = llm.generate_unsafe(prompt_soft)

    if not response.success:
        logger.warning(f"Translation failed even after softening: {response.error}")
        return lines, previous_summary or ""

    # Step 6: Parse results (language-agnostic)
    results, new_summary = parse_cluster_response(response.text, lines_for_llm)

    # Step 7: Silent Failure Detection — profile ตัดสินใจ
    if results:
        failed_indices = [
            idx for idx, text in results.items()
            if profile.is_silent_failure(text)
        ]
        if failed_indices:
            logger.warning(f"Silent failure: {len(failed_indices)} lines still in source language")
            retry_lines = [l for l in lines_for_llm if l.index in failed_indices]
            if retry_lines:
                retry_results = _retry_failed_lines(llm, retry_lines, source_lang, target_lang)
                results.update(retry_results)

    # Step 8: Missing Lines Recovery
    missing_indices = find_missing_lines(results, lines_for_llm)
    if missing_indices:
        logger.warning(f"Missing {len(missing_indices)} lines, retrying...")
        missing_lines = [l for l in lines_for_llm if l.index in missing_indices]
        if missing_lines:
            retry_results = _retry_missing_lines(llm, missing_lines, source_lang, target_lang)
            results.update(retry_results)

    # Step 9: Combine moaning + LLM results + post-process — profile ตัดสินใจ
    translated = []
    for line in lines:
        if line.index in moaning_results:
            translated.append(line.with_text(moaning_results[line.index]))
        else:
            new_text = results.get(line.index, line.text)
            new_text = profile.post_process(new_text)
            # Global safety: ป้องกัน LLM สร้าง text ซ้ำยาวเกินไป
            if len(new_text) > 150:
                new_text = new_text[:100].rstrip('. ')
                if not new_text.endswith('...'):
                    new_text += '...'
            translated.append(line.with_text(new_text))

    return translated, new_summary


# =============================================================================
# Legacy exports — keep for backward compat (handler.py imports)
# =============================================================================

# Legacy: build_cluster_prompt (handler.py อาจ import โดยตรง)
def build_cluster_prompt(lines, previous_summary, source_lang, target_lang, context="", is_scene_change=False):
    """Legacy: delegate to profile"""
    profile = get_profile(source_lang, target_lang)
    return profile.build_prompt(lines, previous_summary, context, is_scene_change)


# Legacy: detect_silent_failures
def detect_silent_failures(results, source_lines, source_lang, target_lang):
    """Legacy: delegate to profile"""
    profile = get_profile(source_lang, target_lang)
    return [
        line.index for line in source_lines
        if line.index in results and profile.is_silent_failure(results[line.index])
    ]
