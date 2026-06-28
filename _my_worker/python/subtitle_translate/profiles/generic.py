"""
GenericProfile — Fallback profile for unsupported language pairs.
ใช้ generic prompt, ไม่มี pre/post processing
"""

import logging
from typing import List, Optional

from shared.entities import SubtitleLine, LanguageCode
from .base import LanguageProfile, _build_lines_text

logger = logging.getLogger(__name__)


class GenericProfile(LanguageProfile):
    """Fallback — generic translation prompt สำหรับ pair ที่ไม่มี profile"""

    def __init__(self, source_lang: LanguageCode, target_lang: LanguageCode):
        self._source_lang = source_lang
        self._target_lang = target_lang

    @property
    def source_code(self) -> str:
        return self._source_lang.code

    @property
    def target_code(self) -> str:
        return self._target_lang.code

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

        return f"""Translate these {self._source_lang.name} subtitles to {self._target_lang.name}.

Video context: {context or 'Video subtitle'}{summary_text}

Rules:
- Translate naturally with context awareness
- Consider speaker gender for pronouns
- Do NOT keep any source language text — translate everything
- Output format: index|translated_text
- After translations, provide a 1-2 sentence summary

Lines (index|text|gender|speaker):
{lines_text}

Translation:
[translations here]

Summary:
[brief summary for next cluster]"""
