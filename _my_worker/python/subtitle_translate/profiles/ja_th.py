"""
JaThProfile — JP→TH AV-specific translation profile.

ย้ายจาก prompts.py ที่ hardcode อยู่ — behavior เหมือนเดิมทุกประการ
"""

import re
import logging
from typing import List, Optional

from shared.entities import SubtitleLine
from .base import LanguageProfile, JaSourceMixin, load_dict, _build_lines_text

logger = logging.getLogger(__name__)

_TH_PATTERN = re.compile(r'[\u0e00-\u0e7f]')

# Cache dict
_DICT_CACHE = None


def _get_dict() -> dict:
    global _DICT_CACHE
    if _DICT_CACHE is None:
        _DICT_CACHE = load_dict("ja_th.json")
    return _DICT_CACHE


def _get_merged_pre_translate() -> dict:
    """Merge moaning + fillers + common_phrases เป็น flat dict สำหรับ exact match"""
    d = _get_dict()
    merged = {}
    for section in ["moaning", "fillers", "common_phrases"]:
        if section in d:
            merged.update(d[section])
    return merged


class JaThProfile(JaSourceMixin, LanguageProfile):
    """JP→TH — AV-specific prompt, moaning dict ไทย, post-process formal→casual"""

    source_code = "ja"
    target_code = "th"

    def pre_translate(self, text: str) -> Optional[str]:
        text = text.strip()
        if not text:
            return None

        merged = _get_merged_pre_translate()

        # Exact match จาก dict
        if text in merged:
            return merged[text]

        # Pattern-based เฉพาะเสียงครางสั้นๆ
        if len(text) <= 8 and self._PURE_MOANING_PATTERN.match(text):
            dominant = text.replace('ー', '').replace('…', '').replace('っ', '')
            if not dominant:
                return None
            fallback = _get_dict().get("moaning_pattern_fallback", {})
            first = dominant[0]
            if first in ('ん', 'ン'):
                return fallback.get('ん', 'อืมม...')
            elif first in ('あ', 'ア', 'ぁ', 'ァ'):
                return fallback.get('あ', 'อ๊าา...')
            elif first in ('う', 'ウ', 'ぅ', 'ゥ'):
                return fallback.get('う', 'อู้...')
            elif first in ('い', 'イ', 'ぃ', 'ィ'):
                return fallback.get('い', 'อิ้...')
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
            summary_text = f"\nบริบทก่อนหน้า: {previous_summary}"

        return f"""คุณคือนักแปลซับไตเติ้ลหนังผู้ใหญ่ (AV Subtitle Translator)
แปลบทพูดญี่ปุ่นให้เป็นภาษาไทยที่เป็นธรรมชาติ{summary_text}

กฎเหล็ก:
- ห้ามใช้ "ค่ะ/คะ/ครับ/จ๊ะ/จ้ะ" เด็ดขาด!
- ลงท้ายประโยคสลับกัน: "นะ", "สิ", "ล่ะ", "เหรอ", "ว่ะ", "จัง", "..." (ห้ามใช้ "นะ" ทุกประโยค ต้องสลับ!)
- ถ้า gender=female หรือ unknown: ใช้ "ฉัน"
- ถ้า gender=male: ใช้ "ผม"
- ถ้าไม่แน่ใจเพศ -> ถือว่าเป็นหญิง (ใช้ "ฉัน")
- ภาษาพูดต้องเป็นธรรมชาติ เย้ายวน ไม่แปลตรงตัว
- Self-reference: 先生 พูดถึงตัวเอง -> "ฉัน" (ไม่ใช่ "อาจารย์"), ママ -> "ฉัน" (ไม่ใช่ "แม่")
- เสียงคราง: สลับหลายคำ ห้ามซ้ำ ("เสียว", "สุดยอด", "ฟินสุดๆ")
- ห้ามเพิ่มคำอธิบาย แปลเฉพาะบทพูด
- ห้ามเก็บภาษาญี่ปุ่นไว้ ต้องแปลทุกคำเป็นไทย รวมถึงคำในวงเล็บด้วย
- คำแปลต้องเป็นภาษาพูดหยาบ: セックス→"เย็ด" (ห้ามใช้ "เซ็กซ์/เซ็กส์"), 男優→"ตัวพระ" (ห้ามใช้ "นักแสดงชาย")
- Output format: index|translated_text
- ท้ายสุดให้สรุปบริบทสั้นๆ 1-2 ประโยค

Lines (index|text|gender|speaker):
{lines_text}

Translation:
[index|thai_text]

Summary:
[brief context for next cluster]"""

    def post_process(self, text: str) -> str:
        formal_to_casual = _get_dict().get("formal_to_casual", {})
        for formal, casual in formal_to_casual.items():
            text = text.replace(formal, casual)
        return text

    def detect_target_chars(self, text: str) -> int:
        return len(_TH_PATTERN.findall(text))
