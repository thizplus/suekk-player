"""
LLM Prompts for subtitle translation.

Prompt logic อยู่ที่นี่ เพื่อให้เปลี่ยน LLM provider ได้โดยไม่ต้องแก้ prompt
รองรับ language-specific prompts (JP->TH, EN->TH, etc.)
"""

import logging
from typing import List, Dict, Tuple, Optional

from shared.entities import SubtitleLine, LanguageCode
from shared.ports.llm_port import LLMPort, LLMResponse

logger = logging.getLogger(__name__)


# =============================================================================
# Translation Dictionary — โหลดจาก JSON file (แก้ไขง่าย ไม่ต้องแก้ code)
# ไฟล์: shared/translation_dict.json
# =============================================================================

import json
from pathlib import Path

_DICT_PATH = Path(__file__).parent.parent / "shared" / "translation_dict.json"
_DICT_CACHE = None


def _load_translation_dict() -> dict:
    """โหลด dict จาก JSON — cache ไว้ใช้ซ้ำ"""
    global _DICT_CACHE
    if _DICT_CACHE is not None:
        return _DICT_CACHE

    if _DICT_PATH.exists():
        data = json.loads(_DICT_PATH.read_text(encoding="utf-8"))
        merged = {}
        for section in ["moaning", "fillers", "common_phrases"]:
            if section in data and isinstance(data[section], dict):
                merged.update(data[section])
        _DICT_CACHE = merged
        logger.info(f"Loaded translation dict: {len(merged)} entries from {_DICT_PATH.name}")
    else:
        logger.warning(f"Translation dict not found: {_DICT_PATH}")
        _DICT_CACHE = {}

    return _DICT_CACHE


def reload_translation_dict():
    """Force reload dict (เมื่อเพิ่มคำใหม่)"""
    global _DICT_CACHE
    _DICT_CACHE = None
    return _load_translation_dict()


# Backward compat
MOANING_SOUNDS = {
    'あ': 'อ๊า...',
    'ああ': 'อ๊าา...',
    'あぁ': 'อ๊าา...',
    'あああ': 'อ๊าาา...',
    'あぁぁ': 'อ๊าาา...',
    'あぁあぁ': 'อ๊าาา...',
    'ん': 'อืม...',
    'んん': 'อืมม...',
    'んんん': 'อืมมม...',
    'んーー': 'อืมมม...',
    'んーーー': 'อืมมม...',
    'うん': 'อืม',
    'う': 'อู...',
    'うう': 'อู้...',
    'ううう': 'อูู้...',
    'はぁ': 'ฮ่า...',
    'はあ': 'ฮ่าา...',
    'はぁはぁ': 'หอบ...หอบ...',
    'ふぅ': 'ฟู่...',
    'ふー': 'ฟู่...',
    'ひぃ': 'ฮี้...',
    'いく': 'อิคุ...',
    'イク': 'อิคุ...',
    'いっちゃう': 'จะไปแล้ว...',
    'だめ': 'ไม่ได้...',
    'ダメ': 'ไม่ได้...',
    'やだ': 'ไม่เอา...',
    'すごい': 'สุดยอด...',
    'きもちいい': 'เสียวจัง...',
    '気持ちいい': 'เสียวจัง...',
    'もっと': 'อีก...',
    # === Japanese Fillers (คำอุทาน/คำเติม) ===
    'えっと': 'เอ่อ...',
    'えーと': 'เอ่อ...',
    'ええ': 'เอ้อ',
    'え?': 'ห๊ะ?',
    'え': 'เอ๊ะ',
    'えー': 'เอ๊ะ...',
    'えっ': 'เอ๊ะ!',
    'えええ': 'เอ๊ะะ!',
    'あの': 'นั่น...',
    'あのね': 'นี่นะ...',
    'なんか': 'ยังไงดี...',
    'ちなみに': 'ว่าแต่',
    'そうですね': 'ก็จริงนะ',
    'そう': 'อืม',
    'ちょっと': 'แป๊บนึง',
    'こんなところで': 'ตรงนี้เลยเหรอ',
    'じゃあ': 'งั้น...',
    'じゃあちょっと': 'งั้นเดี๋ยว...',
    'ねえ': 'เฮ้',
    'ほら': 'นี่ไง',
    'まあ': 'ก็...',
    'やっぱり': 'ก็อย่างที่คิดนะ',
    'よし': 'โอเค',
    'うーん': 'อืม...',
    'ふーん': 'หืม...',
    # === Common JP Phrases (ไม่ใช่ filler แต่สั้นมาก) ===
    'ごめん': 'ขอโทษนะ',
    'ごめんね': 'ขอโทษนะ',
    'ごめんなさい': 'ขอโทษนะ',
    'ごちそう': 'เลี้ยงหน่อยสิ',
    'ごちそうさま': 'อิ่มแล้ว',
    'その': 'นั่น...',
    'そうなんですね': 'อ้อ งั้นเหรอ',
    'そうですね': 'ก็จริงสิ',
    'なるほど': 'อ้อ เข้าใจแล้ว',
    'わあ': 'ว้าว',
    'わー': 'ว้าว',
    'すごいですね': 'เจ๋งมากเลย',
    'まあいいですね': 'ก็ดีนะ',
    'お': 'โอ้',
    'おお': 'โอ้โห',
    'おい': 'เฮ้',
    'おいで': 'มานี่สิ',
    'どうぞ': 'เชิญเลย',
    'どっちがいい': 'เอาแบบไหนดี',
    'どっちがいい?': 'เอาแบบไหนดี?',
    'どっちも': 'ทั้งสองอย่างเลย',
    'どういうこと': 'หมายความว่าไง',
    'どういうこと?': 'หมายความว่าไง?',
    'どうなっちゃいます?': 'จะเป็นยังไงล่ะ?',
    'どうします?': 'จะทำยังไงดี?',
    'そうです': 'ใช่แล้ว',
    'ないです': 'ไม่มี',
    'はい': 'ค่ะ',
    'よいしょ': 'เฮ้โย',
    'やったー': 'เย้!',
    'ぜひ': 'ได้เลย',
    'こっちに': 'มาทางนี้',
    'そして': 'แล้วก็...',
    'ふふ': 'ฮิฮิ',
    'もっといっぱい': 'อีกเยอะๆ',
    'ながい': 'ยาวจัง',
    'こんにちは': 'สวัสดี',
}

# Pattern: เสียงคราง = ตัวอักษรครางซ้ำสั้นๆ เท่านั้น (ไม่รวมประโยคทั่วไป)
import re
MOANING_CHARS = 'あぁいぃうぅえぇおぉんッっーアァイィウゥエェオォン'
PURE_MOANING_PATTERN = re.compile(
    rf'^[{re.escape(MOANING_CHARS)}…]+$'
)


def is_moaning_sound(text: str) -> bool:
    """ตรวจสอบว่า text เป็นเสียงครางหรือไม่ (ไม่ใช่ประโยค)"""
    text = text.strip()
    if not text:
        return False
    if text in MOANING_SOUNDS:
        return True
    # Pattern: เฉพาะตัวอักษรครางสั้นๆ (ไม่เกิน 8 chars)
    if len(text) <= 8 and PURE_MOANING_PATTERN.match(text):
        return True
    return False


def pre_translate_moaning(text: str) -> Optional[str]:
    """แปลเสียงคราง/filler จาก dict — exact match only ไม่ใช้ pattern กับประโยค"""
    text = text.strip()
    # ลองจาก JSON dict ก่อน (มีคำมากกว่า + แก้ไขง่าย)
    translation_dict = _load_translation_dict()
    if text in translation_dict:
        return translation_dict[text]
    # Fallback: hardcoded dict
    if text in MOANING_SOUNDS:
        return MOANING_SOUNDS[text]
    # Pattern-based เฉพาะเสียงครางสั้นๆ (ไม่ใช่ประโยค)
    if len(text) <= 8 and PURE_MOANING_PATTERN.match(text):
        dominant = text.replace('ー', '').replace('…', '').replace('っ', '')
        if not dominant:
            return None
        first = dominant[0]
        if first in ('ん', 'ン'):
            return 'อืมม...'
        elif first in ('あ', 'ア', 'ぁ', 'ァ'):
            return 'อ๊าา...'
        elif first in ('う', 'ウ', 'ぅ', 'ゥ'):
            return 'อู้...'
        elif first in ('い', 'イ', 'ぃ', 'ィ'):
            return 'อิ้...'
        return None
    return None


# =============================================================================
# AV Vocabulary Normalizer — แปลง JP แสลง → JP ปกติ ก่อนส่ง LLM
# LLM ไม่รู้จักศัพท์ AV บางคำ ต้อง normalize ก่อน
# =============================================================================

AV_JP_NORMALIZE = {
    'ワンコ': 'まんこ',           # AV slang for vagina (ไม่ใช่ "หมา")
    'わんこ': 'まんこ',
    '仲出し': '中出し',            # Whisper มักแกะผิด 中→仲
    'なかだし': '中出し',
    'ハメ撮り': 'セックス撮影',     # POV filming
    'おちんちん': 'ちんこ',
    'オチンチン': 'チンコ',
}


def normalize_av_text(text: str) -> str:
    """แปลง AV slang → คำปกติ ก่อนส่ง LLM (JP→JP)"""
    for slang, normal in AV_JP_NORMALIZE.items():
        text = text.replace(slang, normal)
    return text


# =============================================================================
# Post-process: แทนคำทางการด้วยคำหยาบ (หลัง LLM translate, TH→TH)
# =============================================================================

FORMAL_TO_CASUAL = {
    # คำที่ Gemini มักใช้ → คำที่ธรรมชาติกว่า
    'เซ็กซ์': 'เย็ด',
    'เซ็กส์': 'เย็ด',
    'เพศสัมพันธ์': 'เย็ด',
    'ร่วมเพศ': 'เย็ด',
    'มีเพศสัมพันธ์': 'เย็ดกัน',
    'นักแสดงชาย': 'พี่',
    'นักแสดงหญิง': 'น้อง',
    'พระเอก': 'พี่',
    'นางเอก': 'น้อง',
    'ตัวพระ': 'พี่',
    'ตัวนาง': 'น้อง',
    'อวัยวะเพศ': 'จู๋',
    'สำเร็จความใคร่': 'ชักว่าว',
    'น้องหมา': 'หี',              # ワンコ mistranslation
    'ไอ้จ้อน': 'ควย',             # チンコ mistranslation
    'องคชาต': 'ควย',
    'ช่องคลอด': 'หี',
    'ทวารหนัก': 'ตูด',
    'หลั่งน้ำอสุจิ': 'แตกใน',
    'น้ำอสุจิ': 'น้ำ',
    'ถุงยางอนามัย': 'ถุงยาง',
    'ค่ะ': 'จ้ะ',
    'คะ': 'จ๊ะ',
    'ครับ': 'นะ',
}


def post_process_translation(text: str) -> str:
    """แทนคำทางการด้วยคำภาษาพูด หลัง LLM translate"""
    for formal, casual in FORMAL_TO_CASUAL.items():
        text = text.replace(formal, casual)
    return text


# =============================================================================
# Softening — แทนคำโป๊ด้วยคำอ้อมเมื่อ Gemini block
# =============================================================================

SOFT_REPLACEMENTS = {
    'ちんちん': 'あそこ',
    'チンチン': 'あそこ',
    'おちんちん': 'あそこ',
    'オチンチン': 'あそこ',
    'チンポ': 'あそこ',
    'ちんぽ': 'あそこ',
    'オマンコ': 'あそこ',
    'おまんこ': 'あそこ',
    'マンコ': 'あそこ',
    'まんこ': 'あそこ',
    'セックス': '行為',
    'せっくす': '行為',
    'フェラ': '口で',
    'ふぇら': '口で',
    '中出し': '中に',
    'なかだし': '中に',
    '射精': '出す',
    'オナニー': '一人で',
    'おなにー': '一人で',
    '潮吹き': '感じる',
    'レイプ': '無理やり',
    '強姦': '無理やり',
}


def soften_text(text: str) -> str:
    """แทนคำโป๊ด้วยคำอ้อม เพื่อ bypass Gemini content filter"""
    result = text
    for explicit, soft in SOFT_REPLACEMENTS.items():
        result = result.replace(explicit, soft)
    return result


# =============================================================================
# AV-Specific Translation Prompts
# =============================================================================

def _build_av_cluster_prompt(
    lines: List[SubtitleLine],
    previous_summary: Optional[str],
    source_lang: LanguageCode,
    target_lang: LanguageCode,
    context: str = "",
    is_scene_change: bool = False,
) -> str:
    """Build AV-specific cluster prompt (JP->TH)"""
    lines_text = "\n".join([
        f"{l.index}|{l.text}|{l.gender or 'unknown'}|{l.speaker_id or ''}"
        for l in lines
    ])

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


def _build_generic_cluster_prompt(
    lines: List[SubtitleLine],
    previous_summary: Optional[str],
    source_lang: LanguageCode,
    target_lang: LanguageCode,
    context: str = "",
    is_scene_change: bool = False,
) -> str:
    """Build generic cluster prompt (non-AV content)"""
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


# =============================================================================
# Public API — build_cluster_prompt (auto-select AV or generic)
# =============================================================================

def build_cluster_prompt(
    lines: List[SubtitleLine],
    previous_summary: Optional[str],
    source_lang: LanguageCode,
    target_lang: LanguageCode,
    context: str = "",
    is_scene_change: bool = False,
) -> str:
    """Build cluster prompt — auto-select AV prompt for JP->TH"""
    # JP->TH ใช้ AV prompt
    if source_lang.code == "ja" and target_lang.code == "th":
        return _build_av_cluster_prompt(
            lines, previous_summary, source_lang, target_lang, context, is_scene_change
        )
    # อื่นๆ ใช้ generic
    return _build_generic_cluster_prompt(
        lines, previous_summary, source_lang, target_lang, context, is_scene_change
    )


# =============================================================================
# Parse Functions
# =============================================================================

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
                    text = parts[1].strip()
                    results[idx] = text
                except ValueError:
                    continue

    return results, summary.strip()[:100]


# =============================================================================
# translate_cluster — main entry point (uses LLMPort)
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
    Translate speech cluster using LLM (any provider).
    รองรับ: moaning pre-translate, softening retry, AV prompt

    Returns:
        Tuple of (translated lines, new summary)
    """
    if not lines:
        return [], previous_summary or ""

    # Step 1: Pre-translate moaning sounds (ไม่ต้องเรียก LLM)
    moaning_results = {}
    lines_for_llm = []
    for line in lines:
        translated_moaning = pre_translate_moaning(line.text)
        if translated_moaning:
            moaning_results[line.index] = translated_moaning
        else:
            lines_for_llm.append(line)

    if moaning_results:
        logger.info(f"Pre-translated {len(moaning_results)} moaning sounds")

    # Step 2: Normalize AV slang (JP→JP) ก่อนส่ง LLM
    normalized_lines = []
    for line in lines_for_llm:
        normalized_text = normalize_av_text(line.text)
        if normalized_text != line.text:
            logger.debug(f"AV normalize: {line.text} → {normalized_text}")
            normalized_lines.append(line.with_text(normalized_text))
        else:
            normalized_lines.append(line)
    lines_for_llm = normalized_lines

    # Step 3: ถ้าทุกบรรทัดเป็นเสียงคราง → ไม่ต้องเรียก LLM
    if not lines_for_llm:
        translated = []
        for line in lines:
            new_text = moaning_results.get(line.index, line.text)
            translated.append(line.with_text(new_text))
        return translated, previous_summary or ""

    # Step 3: Build prompt + call LLM
    prompt = build_cluster_prompt(
        lines_for_llm, previous_summary, source_lang, target_lang, context, is_scene_change
    )

    response = llm.generate_unsafe(prompt)

    # Step 4: ถ้า block → soften แล้ว retry
    if not response.success and use_softening:
        logger.warning("LLM blocked, retrying with softened text...")
        softened_lines = []
        for line in lines_for_llm:
            softened_lines.append(line.with_text(soften_text(line.text)))

        prompt_soft = build_cluster_prompt(
            softened_lines, previous_summary, source_lang, target_lang, context, is_scene_change
        )
        response = llm.generate_unsafe(prompt_soft)

    if not response.success:
        logger.warning(f"Translation failed even after softening: {response.error}")
        return lines, previous_summary or ""

    # Step 5: Parse results
    results, new_summary = parse_cluster_response(response.text, lines_for_llm)

    # Step 6: Silent Failure Detection — ตรวจว่าแปลจริงหรือยัง
    if results and target_lang.code == "th":
        failed_indices = detect_silent_failures(results, lines_for_llm, source_lang, target_lang)
        if failed_indices:
            logger.warning(f"Silent failure detected: {len(failed_indices)} lines still in source language")
            # Retry เฉพาะบรรทัดที่ไม่แปล
            retry_lines = [l for l in lines_for_llm if l.index in failed_indices]
            if retry_lines:
                retry_results = _retry_failed_lines(llm, retry_lines, source_lang, target_lang)
                results.update(retry_results)

    # Step 7: Missing Lines Recovery — ตรวจบรรทัดที่หาย
    missing_indices = find_missing_lines(results, lines_for_llm)
    if missing_indices:
        logger.warning(f"Missing {len(missing_indices)} lines, retrying...")
        missing_lines = [l for l in lines_for_llm if l.index in missing_indices]
        if missing_lines:
            retry_results = _retry_missing_lines(llm, missing_lines, source_lang, target_lang)
            results.update(retry_results)

    # Step 8: Combine moaning + LLM results + post-process คำทางการ
    translated = []
    for line in lines:
        if line.index in moaning_results:
            translated.append(line.with_text(moaning_results[line.index]))
        else:
            new_text = results.get(line.index, line.text)
            new_text = post_process_translation(new_text)
            translated.append(line.with_text(new_text))

    return translated, new_summary


# =============================================================================
# Silent Failure Detection — ตรวจว่า output ยังเป็นภาษาต้นทางอยู่ไหม
# =============================================================================

# Pattern ตรวจภาษา
_JP_PATTERN = re.compile(r'[\u3040-\u309f\u30a0-\u30ff\u4e00-\u9fff]')
_TH_PATTERN = re.compile(r'[\u0e00-\u0e7f]')
_EN_PATTERN = re.compile(r'[a-zA-Z]')


def detect_silent_failures(
    results: Dict[int, str],
    source_lines: List[SubtitleLine],
    source_lang: LanguageCode,
    target_lang: LanguageCode,
) -> List[int]:
    """ตรวจจับบรรทัดที่ LLM ไม่ได้แปลจริง (ยังเป็นภาษาต้นทาง)"""
    failed = []
    for line in source_lines:
        translated_text = results.get(line.index, "")
        if not translated_text:
            continue

        # เช็คว่า output ยังเป็นภาษาต้นทางอยู่ไหม
        if source_lang.code == "ja" and target_lang.code == "th":
            jp_chars = len(_JP_PATTERN.findall(translated_text))
            th_chars = len(_TH_PATTERN.findall(translated_text))
            total_chars = len(translated_text)

            if total_chars > 3 and jp_chars > 0 and th_chars == 0:
                # ยังเป็น JP ทั้งหมด ไม่มี TH เลย → ไม่ได้แปล
                failed.append(line.index)

    return failed


def find_missing_lines(
    results: Dict[int, str],
    source_lines: List[SubtitleLine],
) -> List[int]:
    """หาบรรทัดที่ LLM ข้ามไป (ไม่มีใน results)"""
    missing = []
    for line in source_lines:
        if line.index not in results:
            missing.append(line.index)
    return missing


def _retry_failed_lines(
    llm: LLMPort,
    lines: List[SubtitleLine],
    source_lang: LanguageCode,
    target_lang: LanguageCode,
) -> Dict[int, str]:
    """Retry แปลบรรทัดที่ silent failure (ใช้ prompt ง่ายกว่า)"""
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
