# Plan: Subtitle Multi-Language Translation Architecture

## สถานะปัจจุบัน — ปัญหา

ระบบ translate **hardcode ภาษาไทยทั่วทั้ง code** — เมื่อแปลเป็น EN จะตกไป generic prompt ที่อ่อนมาก:

| Feature | JP->TH | JP->EN (ปัจจุบัน) |
|---------|--------|-------------------|
| Prompt | AV-specific (กฎเหล็ก 15+ ข้อ) | Generic 3 บรรทัด |
| Moaning pre-translate | dict 50+ คำ (TH) | ข้ามไป |
| AV slang normalize | ทำ | ทำ (source-side) |
| Post-process | formal->casual TH | ข้ามไป |
| Silent failure detect | เช็ค JP ค้างใน TH | ข้ามไป |
| Soften for blocked LLM | ทำ | ทำ |

**ผลลัพธ์**: EN subtitle มี JP ปน, ไม่มี pre-translate moaning, ไม่มีการเช็คคุณภาพ

---

## Architecture ใหม่ — LanguageProfile (Port/Adapter)

### แนวคิด

แต่ละ language pair (JP->TH, JP->EN, JP->ZH...) มี **Profile** เป็นของตัวเอง ที่กำหนด:
- Prompt template
- Pre-translate dictionary (moaning, fillers)
- Post-process rules
- Silent failure detection patterns
- Source text normalization

เพิ่มภาษาใหม่ = สร้าง Profile + Dict file ใหม่ **ไม่ต้องแก้ core logic**

### File Structure

```
subtitle_translate/
    handler.py              # (MODIFY) ลบ hardcode lang checks, ใช้ profile
    prompts.py              # (MODIFY) Core engine only, ไม่มี hardcode dicts
    profiles/               # (NEW)
        __init__.py         # Registry + factory (get_profile)
        base.py             # LanguageProfile ABC
        ja_th.py            # JP->TH (AV-specific, ย้ายจาก prompts.py)
        ja_en.py            # JP->EN (AV English prompt)
        ja_zh.py            # JP->ZH (อนาคต)
        generic.py          # Fallback สำหรับ pair ที่ไม่มี profile
    dicts/                  # (NEW)
        ja_th.json          # Moaning/fillers/formal->casual (TH)
        ja_en.json          # Moaning/fillers (EN)
        ja_zh.json          # (อนาคต)
```

### LanguageProfile Interface

```python
class LanguageProfile(ABC):
    source_code: str  # "ja"
    target_code: str  # "th"

    def pre_translate(self, text: str) -> Optional[str]:
        """Pre-translate moaning/fillers จาก dict. Return None ถ้าไม่ match"""

    def normalize_source(self, text: str) -> str:
        """Normalize source text ก่อนส่ง LLM (เช่น AV slang -> ปกติ)"""

    def soften_source(self, text: str) -> str:
        """Soften explicit text สำหรับ LLM ที่ block content"""

    def build_prompt(self, lines, previous_summary, context, is_scene_change) -> str:
        """Build LLM prompt สำหรับ language pair นี้"""

    def post_process(self, text: str) -> str:
        """Post-process ผลแปล (เช่น formal->casual)"""

    def is_silent_failure(self, translated_text: str) -> bool:
        """เช็คว่า output ยังเป็นภาษาต้นทางอยู่ไหม"""
```

### Profile ตัวอย่าง

```python
# profiles/ja_th.py — ย้ายจาก prompts.py ที่ hardcode อยู่
class JaThProfile(JaSourceMixin, LanguageProfile):
    source_code = "ja"
    target_code = "th"

    def __init__(self):
        self._dict = load_dict("ja_th.json")
        self._formal_to_casual = { ... }  # ย้ายจาก FORMAL_TO_CASUAL

    def pre_translate(self, text):
        return self._dict.get("moaning", {}).get(text)

    def build_prompt(self, ...):
        # ย้ายจาก _build_av_cluster_prompt() — กฎเหล็กภาษาไทย
        ...

    def post_process(self, text):
        # ย้ายจาก post_process_translation() — formal->casual
        ...

    def is_silent_failure(self, text):
        jp_chars = len(JP_PATTERN.findall(text))
        th_chars = len(TH_PATTERN.findall(text))
        return jp_chars > 0 and th_chars == 0


# profiles/ja_en.py — ใหม่
class JaEnProfile(JaSourceMixin, LanguageProfile):
    source_code = "ja"
    target_code = "en"

    def __init__(self):
        self._dict = load_dict("ja_en.json")

    def pre_translate(self, text):
        return self._dict.get("moaning", {}).get(text)

    def build_prompt(self, ...):
        # AV-specific English prompt
        # "Translate Japanese AV dialogue to natural English..."
        ...

    def is_silent_failure(self, text):
        jp_chars = len(JP_PATTERN.findall(text))
        en_chars = len(re.findall(r'[a-zA-Z]', text))
        return jp_chars > 0 and en_chars == 0
```

### JaSourceMixin — Shared JP preprocessing

```python
class JaSourceMixin:
    """JP source operations ที่ใช้ร่วมกันทุก JP->X profile"""

    def normalize_source(self, text):
        # AV slang normalize (JP->JP) — เหมือนกันทุกภาษาปลายทาง
        for old, new in AV_JP_NORMALIZE.items():
            text = text.replace(old, new)
        return text

    def soften_source(self, text):
        # Soften explicit JP text — เหมือนกันทุกภาษาปลายทาง
        for old, new in SOFT_REPLACEMENTS.items():
            text = text.replace(old, new)
        return text
```

### Registry Pattern

```python
# profiles/__init__.py
_REGISTRY = {}

def register(source, target, cls):
    _REGISTRY[(source, target)] = cls

def get_profile(source_lang, target_lang) -> LanguageProfile:
    key = (source_lang.code, target_lang.code)
    cls = _REGISTRY.get(key)
    return cls() if cls else GenericProfile(source_lang, target_lang)

# Auto-register
register("ja", "th", JaThProfile)
register("ja", "en", JaEnProfile)
# เพิ่มภาษาใหม่แค่บรรทัดเดียว:
# register("ja", "zh", JaZhProfile)
```

### Refactored translate_cluster() — Zero hardcode

```python
def translate_cluster(llm, lines, previous_summary, source_lang, target_lang, ...):
    profile = get_profile(source_lang, target_lang)

    # Step 1: Pre-translate — profile ตัดสินใจ
    moaning_results = {}
    lines_for_llm = []
    for line in lines:
        translated = profile.pre_translate(line.text)
        if translated:
            moaning_results[line.index] = translated
        else:
            lines_for_llm.append(line)

    # Step 2: Normalize source — profile ตัดสินใจ
    lines_for_llm = [l.with_text(profile.normalize_source(l.text)) for l in lines_for_llm]

    # Step 3: Build prompt — profile ตัดสินใจ
    prompt = profile.build_prompt(lines_for_llm, previous_summary, context, is_scene_change)
    response = llm.generate_unsafe(prompt)

    # Step 4: Soften retry — profile ตัดสินใจ
    if not response.success:
        softened = [l.with_text(profile.soften_source(l.text)) for l in lines_for_llm]
        prompt = profile.build_prompt(softened, ...)
        response = llm.generate_unsafe(prompt)

    # Step 5: Parse (language-agnostic)
    results, summary = parse_cluster_response(response.text, lines_for_llm)

    # Step 6: Silent failure — profile ตัดสินใจ
    failed = [idx for idx, text in results.items() if profile.is_silent_failure(text)]

    # Step 7: Post-process — profile ตัดสินใจ
    for line in lines:
        text = results.get(line.index, line.text)
        text = profile.post_process(text)
        # + truncate > 150 chars (global safety)
```

**ไม่มี `if target_lang.code == "th"` เหลืออยู่เลย**

---

## Dict Structure (per language pair)

### dicts/ja_th.json
```json
{
  "moaning": { "あ": "อ๊า...", "ん": "อืม...", ... },
  "fillers": { "えっと": "เอ่อ...", ... },
  "common_phrases": { "ごめん": "ขอโทษนะ", ... },
  "formal_to_casual": { "เซ็กซ์": "เย็ด", "ค่ะ": "จ้ะ", ... }
}
```

### dicts/ja_en.json
```json
{
  "moaning": { "あ": "Ah...", "ん": "Mmm...", "いい": "So good...", ... },
  "fillers": { "えっと": "Um...", "あの": "Well...", ... },
  "common_phrases": { "ごめん": "Sorry", ... },
  "formal_to_casual": {}
}
```

---

## Silent Failure Detection — Multi-Language

| Language Pair | Source Pattern | Target Pattern |
|--------------|---------------|----------------|
| JP->TH | Kana + Kanji | Thai chars |
| JP->EN | Kana + Kanji | Latin chars |
| JP->ZH | Kana only (Kanji shared!) | Hanzi |
| JP->KO | Kana + Kanji | Hangul |

---

## Implementation Plan

| Phase | Task | Effort | Risk |
|-------|------|--------|------|
| **1** | สร้าง `profiles/base.py`, `generic.py`, `__init__.py` | Low (1 hr) | None |
| **2** | สร้าง `profiles/ja_th.py` + `dicts/ja_th.json` — extract จาก prompts.py | Medium (2 hr) | Medium — ต้องไม่ทำ TH พัง |
| **3** | Refactor `prompts.py` — ลบ hardcode, ใช้ profile | Medium (1.5 hr) | Medium — core logic change |
| **4** | Update `handler.py` — ลบ TH-specific report | Low (15 min) | Low |
| **5** | สร้าง `profiles/ja_en.py` + `dicts/ja_en.json` | Medium (1 hr) | Low |
| **6** | Test ทั้ง JP->TH และ JP->EN | Low (30 min) | — |

**รวม ~6-7 ชม.**

---

## เพิ่มภาษาใหม่ในอนาคต (เช่น JP->ZH)

แค่ 2 ไฟล์:
1. `profiles/ja_zh.py` — prompt + detection patterns
2. `dicts/ja_zh.json` — moaning/fillers dict

แล้ว register:
```python
register("ja", "zh", JaZhProfile)
```

**ไม่ต้องแก้ prompts.py, handler.py, หรือ core logic ใดๆ**
