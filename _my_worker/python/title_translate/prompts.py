"""
LLM Prompts for title translation service.

ทุก prompt อยู่ที่นี่ — เปลี่ยน LLM provider ได้โดยไม่แก้ prompts
Business logic เรียกผ่าน LLMPort.generate(prompt)
"""

import json
import re
import logging
from typing import List, Dict, Optional, Tuple

from shared.ports.llm_port import LLMPort, LLMResponse
from title_translate.entities import TranslationResult

logger = logging.getLogger(__name__)


# =============================================================================
# JAV — กลอน 8 สไตล์หนังแผ่นไทย
# =============================================================================

KLON8_PROMPT = """คุณคือผู้เชี่ยวชาญการตั้งชื่อหนังแผ่นสไตล์ไทย
งานของคุณคือสรุป Metadata เป็นชื่อภาษาไทยที่ **คล้องจอง ติดหู สนุก** สไตล์หนังแผ่นไทยยุคเก่า

**กฎสำคัญ:**
1. ต้องคล้องจอง มีสัมผัสสระ ฟังแล้วติดหู
2. ความยาว 8-16 พยางค์ (ไม่ต้องเป๊ะ ขอให้ลื่นไหล)
3. ใช้คำเล่น คำสแลง คำตลก ได้เต็มที่
4. ห้ามใส่ชื่อดาราลงในชื่อ
5. **ห้ามใช้คำซ้ำๆ** เช่น "ขาดรอน", "สะดุ้ง", "ทลาย" - ต้องคิดคำใหม่ทุกครั้ง

**คำศัพท์แนะนำ:**
- Vibrator/Toy → ไข่สั่น, ของเล่น, เครื่องรัว
- Creampie → แตกใน, ปล่อยใน, ยิงใน
- Squirt → น้ำพุ่ง, ฉีด, แฉะ
- Threesome → รุมสาม, สามคน, จับสาม
- Gangbang → รุมโทรม, รุมกิน, ยกแก๊ง
- Blowjob → อมให้, ดูด, เลีย
- NTR/Cuckold → เมียมีชู้, แอบแซ่บ, ขโมยผัว
- Massage → นวด, คลึง, จับเส้น

**ตัวอย่างชื่อที่ดี (เป็นแนวทาง ห้ามลอก):**
{"ABF-295": "คันตรงหูเอ็นดูที่หรรม"}
{"SNOS-069": "รุ่นน้องชวนโยกเปิดโลกรุ่นพี่"}
{"JUR-570": "จืดซอยสิบตลบเรียนจบรอบที่ร้อย"}

**สิ่งที่ต้องทำ:** อ่าน tags และ title แล้วคิดชื่อใหม่ที่คล้องจอง สื่อความหมาย และไม่ซ้ำกับตัวอย่าง
"""


# =============================================================================
# CN — Chinese AV (แปลตรง 18+)
# =============================================================================

CN_TRANSLATE_PROMPT = """คุณคือผู้เชี่ยวชาญแปลชื่อหนังจีน 18+ เป็นภาษาไทย

**หน้าที่ของคุณ:**
แปล title ภาษาอังกฤษเป็นภาษาไทย โดย:
1. แปลตรงตามความหมาย
2. ใช้ภาษาพูดแบบไทยๆ 18+
3. ทำให้ฟังแล้วน่าสนใจ ยั่วยวน เซ็กซี่

**กฎสำคัญ:**
- ห้ามใช้ภาษาทางการ ต้องเป็นภาษาพูด
- ใช้คำแสลง คำสแลง 18+ ได้เต็มที่
- ห้ามใส่ชื่อดาราลงในชื่อ
- **ห้ามใส่ video code ลงในคำแปล**
- ถ้า title มีหลายประโยค ให้รวมเป็นประโยคเดียวที่กระชับ
"""


# =============================================================================
# EN — Western/Pornhub (แปลตรง 18+)
# =============================================================================

EN_TRANSLATE_PROMPT = """คุณคือผู้เชี่ยวชาญแปลชื่อหนังฝรั่ง 18+ เป็นภาษาไทย

**หน้าที่ของคุณ:**
แปล title ภาษาอังกฤษเป็นภาษาไทย โดย:
1. แปลตรงตามความหมาย
2. ใช้ภาษาพูดแบบไทยๆ 18+
3. ทำให้ฟังแล้วน่าสนใจ เซ็กซี่

**กฎสำคัญ:**
- ห้ามใช้ภาษาทางการ ต้องเป็นภาษาพูด
- ใช้คำแสลง 18+ ได้เต็มที่
- รักษาความหมายเดิม แต่เพิ่มอารมณ์เซ็กซี่
- ความยาวไม่เกิน 80 ตัวอักษร
"""


# =============================================================================
# Helper: parse JSON response จาก LLM
# =============================================================================

def _clean_llm_json(text: str) -> str:
    """Clean markdown code blocks from LLM response"""
    if "```" in text:
        match = re.search(r'```(?:json)?\s*([\s\S]*?)\s*```', text)
        if match:
            text = match.group(1).strip()
    if not text.startswith("{") and not text.startswith("["):
        match = re.search(r'[\{\[][\s\S]*[\}\]]', text)
        if match:
            text = match.group(0)
    return text.strip()


def _clean_title(text: str) -> str:
    """Clean up LLM generated title"""
    text = text.replace('**', '').replace('  ', ' ').strip()
    text = text.strip('"\'')
    if '\n' in text:
        text = text.split('\n')[0].strip()
    return text


def extract_code(title: str) -> str:
    """Extract video code from title (e.g. ABF-144, MD-0312-AD)"""
    if not title:
        return ""
    match = re.match(r'^([A-Z]+-\d+(?:-[A-Z0-9]+)*)', title)
    return match.group(1) if match else ""


# =============================================================================
# Title Translation Functions (ใช้ LLMPort)
# =============================================================================

def translate_title_klon8(
    llm: LLMPort,
    code: str,
    title_en: str,
    tags: List[str],
    cast_name: str = "",
) -> Optional[str]:
    """Translate JAV title to กลอน 8 format"""
    video_data = {
        "code": code,
        "title": title_en[:200],
        "tags": tags[:8],
    }
    prompt = f"""{KLON8_PROMPT}

**ข้อมูล video**:
{json.dumps(video_data, ensure_ascii=False, indent=2)}

**ตอบเป็น JSON object โดยใช้ code "{code}" เป็น key**
ตัวอย่าง: {{"{code}": "กลอน8พยางค์"}}"""

    response = llm.generate_unsafe(prompt)
    if not response.success:
        logger.error(f"Klon8 translation failed: {response.error}")
        return None

    try:
        text = _clean_llm_json(response.text)
        result = json.loads(text)
        klon8 = result.get(code)
        if not klon8:
            return None

        klon8 = klon8.replace('-', '').replace('|', '').replace('**', '').replace('  ', ' ').strip()

        if cast_name:
            return f"{code} ซับไทย {klon8} {cast_name}".strip()
        return f"{code} ซับไทย {klon8}".strip()

    except Exception as e:
        logger.error(f"Klon8 parse failed: {e}")
        return None


def translate_title_cn(
    llm: LLMPort,
    code: str,
    title_en: str,
    tags: List[str],
    cast_name: str = "",
) -> Optional[str]:
    """Translate Chinese AV title to Thai"""
    video_data = {
        "code": code,
        "title": title_en[:300],
        "tags": tags[:8],
    }
    prompt = f"""{CN_TRANSLATE_PROMPT}

**ข้อมูล video**:
{json.dumps(video_data, ensure_ascii=False, indent=2)}

**ตอบเป็น JSON object โดยใช้ code "{code}" เป็น key**
ตัวอย่าง: {{"{code}": "ชื่อแปลภาษาไทย18+"}}"""

    response = llm.generate_unsafe(prompt)
    if not response.success:
        return None

    try:
        text = _clean_llm_json(response.text)
        result = json.loads(text)
        title_th = result.get(code)
        if not title_th:
            return None

        title_th = _clean_title(title_th)

        if cast_name:
            return f"{code} ซับไทย {title_th} {cast_name}".strip()
        return f"{code} ซับไทย {title_th}".strip()

    except Exception as e:
        logger.error(f"CN parse failed: {e}")
        return None


def translate_title_en(
    llm: LLMPort,
    title_en: str,
    cast_name: str = "",
    cast_list: List[str] = None,
) -> Optional[str]:
    """Translate Western/Pornhub title to Thai"""
    cast_en = None
    if cast_list and len(cast_list) > 0:
        cast_en = cast_list[0]
    elif cast_name:
        cast_en = cast_name

    # ตัดชื่อ cast ออกจาก title ก่อนแปล
    title_to_translate = title_en
    if cast_en:
        patterns = [
            rf'\s*[-|]\s*{re.escape(cast_en)}\s*$',
            rf'\s*{re.escape(cast_en)}\s*[-|]\s*',
            rf'^{re.escape(cast_en)}\s*[-|:]\s*',
        ]
        for pattern in patterns:
            title_to_translate = re.sub(pattern, '', title_to_translate, flags=re.IGNORECASE).strip()

    # Step 1: แปล title
    prompt_title = f"""{EN_TRANSLATE_PROMPT}

**Title ที่ต้องแปล:**
{title_to_translate[:300]}

**ตอบแค่คำแปลภาษาไทยเท่านั้น ไม่ต้องใส่อะไรเพิ่ม**"""

    response = llm.generate_unsafe(prompt_title)
    if not response.success:
        return None

    title_th = _clean_title(response.text)
    if not title_th or len(title_th) < 5:
        return None

    # Step 2: ทับศัพท์ชื่อ cast
    name_th = None
    if cast_en:
        name_th = transliterate_name(llm, cast_en)

    # Step 3: รวม format
    if cast_en and name_th:
        return f"{cast_en} ({name_th}) {title_th} ซับไทย".strip()
    elif cast_en:
        return f"{cast_en} {title_th} ซับไทย".strip()
    return f"{title_th} ซับไทย".strip()


# =============================================================================
# Cast/Tag/Category Translation (ใช้ LLMPort)
# =============================================================================

def transliterate_name(llm: LLMPort, name_en: str) -> Optional[str]:
    """ทับศัพท์ชื่อเป็นภาษาไทย"""
    if not name_en:
        return None

    prompt = f"""ทับศัพท์ชื่อนี้เป็นภาษาไทย:
{name_en}

ตอบแค่ชื่อทับศัพท์เท่านั้น ไม่ต้องอธิบาย
ตัวอย่าง: Yua Mikami → ยัว มิคามิ"""

    response = llm.generate(prompt)
    if not response.success:
        return None

    name_th = _clean_title(response.text)
    if len(name_th) > 50 or len(name_th) < 2:
        return None
    return name_th


def translate_tags_batch(llm: LLMPort, tags: List[str]) -> List[TranslationResult]:
    """Translate tag names to Thai"""
    return _translate_batch(llm, tags, '''แปลชื่อ tags เป็นภาษาไทยแบบภาษาพูด ไม่เป็นทางการ
ตัวอย่าง: "Big Tits" -> "นมใหญ่", "Massage" -> "นวด", "Nurse" -> "พยาบาล"

Input: {items}

ตอบเป็น JSON array เท่านั้น ไม่ต้องมี markdown หรือคำอธิบาย
ตัวอย่าง: ["นมใหญ่", "นวด", "พยาบาล"]''')


def translate_casts_batch(llm: LLMPort, casts: List[str]) -> List[TranslationResult]:
    """Translate cast names to Thai"""
    return _translate_batch(llm, casts, '''ทับศัพท์ชื่อนักแสดงเป็นภาษาไทย

**ถ้าเป็นชื่อญี่ปุ่น → สลับลำดับ:**
- "Aizawa Minami" -> "มินามิ ไอซาวะ" (สลับ)
- "Mikami Yua" -> "ยัว มิคามิ" (สลับ)
- "AIKA" -> "ไอกะ" (ชื่อเดี่ยว)

**ถ้าเป็นชื่อ Western → ไม่สลับ:**
- "Abella Danger" -> "อาเบลล่า เดนเจอร์"

**ถ้าเป็น username → คงเดิม:**
- "babyjee" -> "babyjee"

Input: {items}

ตอบเป็น JSON array เท่านั้น''')


def translate_categories_batch(llm: LLMPort, categories: List[str]) -> List[TranslationResult]:
    """Translate category names to Thai"""
    return _translate_batch(llm, categories, '''แปลชื่อหมวดหมู่เป็นภาษาไทย ใช้คำที่เข้าใจง่าย
ตัวอย่าง: "Drama" -> "ดราม่า", "Action" -> "แอ็คชั่น"

Input: {items}

ตอบเป็น JSON array เท่านั้น''')


def translate_titles_batch(llm: LLMPort, titles: List[str]) -> List[TranslationResult]:
    """Translate video titles to Thai"""
    return _translate_batch(llm, titles, '''แปล titles เป็นภาษาไทย
- รักษาความหมายเดิม
- ใช้ภาษาที่เป็นธรรมชาติ
- ถ้ามีรหัส (เช่น SSIS-312) ให้คงไว้

Input: {items}

ตอบเป็น JSON array เท่านั้น''')


def translate_query(llm: LLMPort, query: str) -> str:
    """Translate search query from Thai to English"""
    prompt = f'''แปลคำค้นหาจากภาษาไทยเป็นภาษาอังกฤษ สำหรับใช้ค้นหารูปภาพ/วิดีโอ

Input: "{query}"

กฎ:
1. แปลให้สั้น กระชับ เหมาะกับการค้นหา
2. ถ้าเป็นคำอธิบายลักษณะคน ให้แปลเป็น descriptive phrase
3. ตอบแค่คำแปลเท่านั้น ไม่ต้องใส่ quotes'''

    response = llm.generate(prompt)
    if response.success:
        return response.text.strip().strip('"\'')
    return query  # fallback to original


def _translate_batch(
    llm: LLMPort,
    items: List[str],
    prompt_template: str,
) -> List[TranslationResult]:
    """Generic batch translation with LLM"""
    if not items:
        return []

    items_json = json.dumps(items, ensure_ascii=False)
    prompt = prompt_template.format(items=items_json)

    response = llm.generate(prompt)

    if not response.success:
        return [TranslationResult(original=item, translated="", success=False, error=response.error) for item in items]

    try:
        text = _clean_llm_json(response.text)
        translations = json.loads(text)

        results = []
        for original, translated in zip(items, translations):
            results.append(TranslationResult(original=original, translated=translated, success=True))
        return results

    except Exception as e:
        logger.error(f"Batch translation parse error: {e}")
        return [TranslationResult(original=item, translated="", success=False, error=str(e)) for item in items]
