"""
JaEnProfile — JP→EN AV-specific translation profile.
"""

import re
import logging
from typing import List, Optional

from shared.entities import SubtitleLine
from .base import LanguageProfile, JaSourceMixin, load_dict, _build_lines_text

logger = logging.getLogger(__name__)

_EN_PATTERN = re.compile(r'[a-zA-Z]')

_DICT_CACHE = None


def _get_dict() -> dict:
    global _DICT_CACHE
    if _DICT_CACHE is None:
        _DICT_CACHE = load_dict("ja_en.json")
    return _DICT_CACHE


def _get_merged_pre_translate() -> dict:
    d = _get_dict()
    merged = {}
    for section in ["moaning", "fillers", "common_phrases"]:
        if section in d:
            merged.update(d[section])
    return merged


class JaEnProfile(JaSourceMixin, LanguageProfile):
    """JP→EN — AV-specific English prompt, moaning dict EN, silent failure detection"""

    source_code = "ja"
    target_code = "en"

    def pre_translate(self, text: str) -> Optional[str]:
        text = text.strip()
        if not text:
            return None

        merged = _get_merged_pre_translate()

        if text in merged:
            return merged[text]

        # Pattern-based moaning
        if len(text) <= 8 and self._PURE_MOANING_PATTERN.match(text):
            dominant = text.replace('ー', '').replace('…', '').replace('っ', '')
            if not dominant:
                return None
            fallback = _get_dict().get("moaning_pattern_fallback", {})
            first = dominant[0]
            if first in ('ん', 'ン'):
                return fallback.get('ん', 'Mmm...')
            elif first in ('あ', 'ア', 'ぁ', 'ァ'):
                return fallback.get('あ', 'Ahh...')
            elif first in ('う', 'ウ', 'ぅ', 'ゥ'):
                return fallback.get('う', 'Ooh...')
            elif first in ('い', 'イ', 'ぃ', 'ィ'):
                return fallback.get('い', 'Eeh...')
            return None

        return None

    def build_prompt(
        self,
        lines: List[SubtitleLine],
        previous_summary: Optional[str],
        context: str,
        is_scene_change: bool,
    ) -> str:
        lines_text = _build_lines_text(lines)

        summary_text = ""
        if previous_summary and not is_scene_change:
            summary_text = f"\nPrevious context: {previous_summary}"

        return f"""You are an adult video (AV/JAV) subtitle translator.
Translate Japanese dialogue into natural, casual English.{summary_text}

Rules:
- Use natural spoken English — casual, conversational tone
- If gender=female or unknown: use "I" with feminine tone
- If gender=male: use "I" with masculine tone
- Moaning sounds: vary expressions ("Feels so good", "Oh god", "Yes", "Right there") — NEVER repeat the same phrase
- Do NOT keep any Japanese text — translate EVERYTHING into English
- Do NOT add descriptions or stage directions — translate dialogue only
- Use explicit language naturally: セックス→"fuck", ちんこ→"dick", まんこ→"pussy"
- Do NOT use overly formal or clinical terms (no "intercourse", "penis", "vagina")
- Output format: index|translated_text
- End with a brief 1-2 sentence summary

Lines (index|text|gender|speaker):
{lines_text}

Translation:
[index|english_text]

Summary:
[brief context for next cluster]"""

    def detect_target_chars(self, text: str) -> int:
        return len(_EN_PATTERN.findall(text))
