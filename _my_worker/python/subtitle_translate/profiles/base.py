"""
LanguageProfile — Base class for per-language-pair translation behavior.

แต่ละ source→target pair มี profile ของตัวเอง กำหนด:
- Pre-translate dictionary (moaning, fillers)
- Source text normalization
- LLM prompt template
- Post-processing rules
- Silent failure detection
"""

import json
import logging
import re
from abc import ABC, abstractmethod
from pathlib import Path
from typing import List, Dict, Optional

from shared.entities import SubtitleLine, LanguageCode

logger = logging.getLogger(__name__)

_DICTS_DIR = Path(__file__).parent.parent / "dicts"


def load_dict(filename: str) -> dict:
    """โหลด dict file จาก dicts/ directory"""
    path = _DICTS_DIR / filename
    if path.exists():
        data = json.loads(path.read_text(encoding="utf-8"))
        logger.info(f"Loaded dict: {filename}")
        return data
    logger.warning(f"Dict not found: {path}")
    return {}


def _build_lines_text(lines: List[SubtitleLine]) -> str:
    """Format lines สำหรับส่ง LLM — ใช้ร่วมกันทุก profile"""
    return "\n".join([
        f"{l.index}|{l.text}|{l.gender or 'unknown'}|{l.speaker_id or ''}"
        for l in lines
    ])


class LanguageProfile(ABC):
    """
    Abstract base class — แต่ละ language pair implement ตัวเอง.
    ไม่ต้องแก้ core logic เมื่อเพิ่มภาษาใหม่
    """

    @property
    @abstractmethod
    def source_code(self) -> str:
        """e.g. 'ja'"""

    @property
    @abstractmethod
    def target_code(self) -> str:
        """e.g. 'th'"""

    def pre_translate(self, text: str) -> Optional[str]:
        """Pre-translate known phrases (moaning, fillers). Return None if not matched."""
        return None

    def normalize_source(self, text: str) -> str:
        """Normalize source text before LLM (e.g. AV slang → normal JP)"""
        return text

    def soften_source(self, text: str) -> str:
        """Soften explicit text for content-filtered LLMs"""
        return text

    @abstractmethod
    def build_prompt(
        self,
        lines: List[SubtitleLine],
        previous_summary: Optional[str],
        context: str,
        is_scene_change: bool,
    ) -> str:
        """Build the LLM prompt for this language pair"""

    def post_process(self, text: str) -> str:
        """Post-process translated text (e.g. formal → casual)"""
        return text

    def detect_source_chars(self, text: str) -> int:
        """Count source language characters in text"""
        return 0

    def detect_target_chars(self, text: str) -> int:
        """Count target language characters in text"""
        return 0

    def is_silent_failure(self, translated_text: str) -> bool:
        """Check if translated text is still in source language"""
        if len(translated_text) <= 3:
            return False
        source_count = self.detect_source_chars(translated_text)
        target_count = self.detect_target_chars(translated_text)
        return source_count > 0 and target_count == 0


class JaSourceMixin:
    """Shared JP source-side operations — ใช้ร่วมกันทุก JP→X profile"""

    # AV slang → normal JP (source-side, ไม่เกี่ยวกับ target language)
    _AV_NORMALIZE = {
        'ワンコ': 'まんこ',
        'わんこ': 'まんこ',
        '仲出し': '中出し',
        'なかだし': '中出し',
        'ハメ撮り': 'セックス撮影',
        'おちんちん': 'ちんこ',
        'オチンチン': 'チンコ',
    }

    # Soften explicit JP → mild JP (for content-filtered LLMs)
    _SOFT_REPLACEMENTS = {
        'ちんちん': 'あそこ', 'チンチン': 'あそこ',
        'おちんちん': 'あそこ', 'オチンチン': 'あそこ',
        'チンポ': 'あそこ', 'ちんぽ': 'あそこ',
        'オマンコ': 'あそこ', 'おまんこ': 'あそこ',
        'マンコ': 'あそこ', 'まんこ': 'あそこ',
        'セックス': '行為', 'せっくす': '行為',
        'フェラ': '口で', 'ふぇら': '口で',
        '中出し': '中に', 'なかだし': '中に',
        '射精': '出す',
        'オナニー': '一人で', 'おなにー': '一人で',
        '潮吹き': '感じる',
        'レイプ': '無理やり', '強姦': '無理やり',
    }

    # Moaning char pattern (JP)
    _MOANING_CHARS = 'あぁいぃうぅえぇおぉんッっーアァイィウゥエェオォン'
    _PURE_MOANING_PATTERN = re.compile(
        rf'^[{re.escape(_MOANING_CHARS)}…]+$'
    )

    # JP char detection
    _JP_PATTERN = re.compile(r'[\u3040-\u309f\u30a0-\u30ff\u4e00-\u9fff]')

    def normalize_source(self, text: str) -> str:
        for slang, normal in self._AV_NORMALIZE.items():
            text = text.replace(slang, normal)
        return text

    def soften_source(self, text: str) -> str:
        for explicit, soft in self._SOFT_REPLACEMENTS.items():
            text = text.replace(explicit, soft)
        return text

    def detect_source_chars(self, text: str) -> int:
        return len(self._JP_PATTERN.findall(text))

    def is_moaning_text(self, text: str) -> bool:
        """ตรวจว่า text เป็นเสียงคราง (JP)"""
        text = text.strip()
        if not text:
            return False
        if len(text) <= 8 and self._PURE_MOANING_PATTERN.match(text):
            return True
        return False
