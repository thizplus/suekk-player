# Worker Architecture Plan — Unified Python Workers

> เป้าหมาย: ทุก service ใช้โครงสร้างเดียวกัน + Port/Adapter สำหรับ LLM
> Gemini เริ่ม ban → ต้องเปลี่ยน LLM ได้ง่าย โดยไม่แก้ business logic
> แต่ละ service เลือกใช้ LLM คนละตัวได้

---

## ปัญหาปัจจุบัน

### `GeminiAdapter` ผสม 2 หน้าที่ไว้ในตัวเดียว

```python
# shared/adapters/gemini_adapter.py (ปัจจุบัน)
class GeminiAdapter:
    def _get_client(self):        # ← LLM client (generic)
    def translate_batch(self):     # ← subtitle-specific logic (prompt + parse)
    def refine_batch(self):        # ← subtitle-specific logic
    def translate_cluster(self):   # ← subtitle-specific logic
```

**ปัญหา**:
- `title_translate` ใช้ `GeminiAdapter` นี้ไม่ได้ เพราะ methods เป็น subtitle-specific
- ถ้าเพิ่ม `translate_titles()` เข้าไป → adapter บวมขึ้นเรื่อยๆ ทุก service
- เปลี่ยน LLM ต้องแก้ทุก method ใน adapter

---

## แนวทางที่ถูกต้อง: แยก LLM Client ออกจาก Business Logic

```
Before (ปัจจุบัน):
  subtitle_translate → GeminiAdapter (LLM + subtitle prompts รวมกัน)

After (ใหม่):
  subtitle_translate → SubtitleLLMService → LLMPort → GeminiAdapter (แค่ send prompt)
  title_translate    → TitleLLMService    → LLMPort → GeminiAdapter (แค่ send prompt)
  title_translate    → TitleLLMService    → LLMPort → OpenAIAdapter  (เปลี่ยนได้)
```

---

## โครงสร้างใหม่

```
_my_worker/python/
├── shared/
│   ├── config.py
│   ├── nats_consumer.py
│   ├── progress.py
│   ├── storage.py
│   │
│   ├── ports/                           # Interfaces
│   │   ├── __init__.py
│   │   └── llm_port.py                  # LLM interface (generate only)
│   │
│   └── adapters/
│       ├── __init__.py
│       ├── entities.py
│       │
│       │  # LLM Adapters (ทำแค่ส่ง prompt + รับ response)
│       ├── gemini_llm.py                # Gemini implementation
│       ├── openai_llm.py                # OpenAI implementation (backup)
│       ├── llm_factory.py               # Factory: สร้างจาก env/config
│       │
│       │  # เดิม (ไม่แก้)
│       ├── whisper_adapter.py
│       ├── audio_adapter.py
│       ├── vad_adapter.py
│       │
│       │  # Legacy (ค่อยๆ migrate)
│       ├── gemini_adapter.py            # เก็บไว้ก่อน ไม่ลบ
│       │
│       │  # API Client Adapters
│       ├── subth_api.py                 # subth.com API client
│       └── suekk_api.py                 # suekk API client
│
├── subtitle_detect/                     # ไม่ใช้ LLM → ไม่ต้องแก้
│   ├── main.py
│   └── handler.py
│
├── subtitle_transcribe/                 # ใช้ LLM (refine) → migrate
│   ├── main.py
│   ├── handler.py
│   └── prompts.py                       # ย้าย prompt logic มาจาก gemini_adapter
│
├── subtitle_translate/                  # ใช้ LLM (translate) → migrate
│   ├── main.py
│   ├── handler.py
│   └── prompts.py                       # ย้าย prompt logic มาจาก gemini_adapter
│
├── title_translate/                     # NEW: ใช้ LLM → ใช้ Port ตั้งแต่แรก
│   ├── main.py
│   ├── handler.py
│   ├── prompts.py                       # klon8, cn, en prompts
│   └── cast_service.py                  # subth API cast management
│
└── tag_translate/                       # NEW: ใช้ LLM
    ├── main.py
    └── handler.py
```

---

## LLM Port — ทำแค่ "ส่ง prompt / รับ text"

```python
# shared/ports/llm_port.py

from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import Optional, List

@dataclass
class LLMResponse:
    """ผลลัพธ์จาก LLM"""
    text: str
    success: bool
    error: Optional[str] = None
    model: Optional[str] = None

class LLMPort(ABC):
    """
    Interface สำหรับ LLM — ทำแค่ส่ง prompt แล้วรับ text กลับ
    ไม่มี business logic (prompt building, response parsing)
    Business logic อยู่ใน handler ของแต่ละ service
    """

    @abstractmethod
    def generate(self, prompt: str) -> LLMResponse:
        """ส่ง prompt ไป LLM แล้วรับ text กลับ"""
        pass

    @abstractmethod
    def generate_with_safety_off(self, prompt: str) -> LLMResponse:
        """ส่ง prompt โดยปิด safety filter (สำหรับ 18+ content)"""
        pass

    @abstractmethod
    def get_model_name(self) -> str:
        pass
```

---

## Adapters — แค่ wrap API call

### Gemini

```python
# shared/adapters/gemini_llm.py

class GeminiLLM(LLMPort):
    def __init__(self, api_key: str, model: str = "gemini-2.5-flash"):
        self._api_key = api_key
        self._model_name = model
        self._client = None
        self._client_unsafe = None   # safety off

    def generate(self, prompt: str) -> LLMResponse:
        client = self._get_client()
        try:
            resp = client.generate_content(prompt)
            return LLMResponse(text=resp.text.strip(), success=True, model=self._model_name)
        except Exception as e:
            return LLMResponse(text="", success=False, error=str(e))

    def generate_with_safety_off(self, prompt: str) -> LLMResponse:
        client = self._get_client_unsafe()  # safety_settings = BLOCK_NONE
        ...
```

### OpenAI (backup)

```python
# shared/adapters/openai_llm.py

class OpenAILLM(LLMPort):
    def __init__(self, api_key: str, model: str = "gpt-4o-mini"):
        ...
    def generate(self, prompt: str) -> LLMResponse:
        resp = self.client.chat.completions.create(messages=[{"role":"user","content":prompt}])
        return LLMResponse(text=resp.choices[0].message.content, success=True)
```

### Factory

```python
# shared/adapters/llm_factory.py

def create_llm(provider: str = None, **kwargs) -> LLMPort:
    """
    สร้าง LLM adapter

    Usage:
        llm = create_llm()                          # ใช้ env LLM_PROVIDER
        llm = create_llm("gemini")                   # ระบุเอง
        llm = create_llm("openai", model="gpt-4o")   # ระบุ + config
    """
    provider = provider or os.getenv("LLM_PROVIDER", "gemini")

    if provider == "gemini":
        return GeminiLLM(
            api_key=kwargs.get("api_key") or os.getenv("GEMINI_API_KEY"),
            model=kwargs.get("model") or os.getenv("GEMINI_MODEL", "gemini-2.5-flash"),
        )
    elif provider == "openai":
        return OpenAILLM(
            api_key=kwargs.get("api_key") or os.getenv("OPENAI_API_KEY"),
            model=kwargs.get("model") or os.getenv("OPENAI_MODEL", "gpt-4o-mini"),
        )
    raise ValueError(f"Unknown LLM provider: {provider}")
```

---

## แต่ละ Service เลือก LLM คนละตัวได้

### .env

```bash
# Default LLM (ถ้า service ไม่ระบุ)
LLM_PROVIDER=gemini
GEMINI_API_KEY=AIzaSy...
GEMINI_MODEL=gemini-2.5-flash

# Override per service (ถ้าต้องการ)
TITLE_LLM_PROVIDER=openai          # title translate ใช้ OpenAI (Gemini ban 18+)
OPENAI_API_KEY=sk-...

SUBTITLE_LLM_PROVIDER=gemini       # subtitle ยังใช้ Gemini ได้ (ไม่ ban)
```

### Handler

```python
# subtitle_translate/handler.py
class SubtitleTranslateHandler:
    def __init__(self, ...):
        provider = os.getenv("SUBTITLE_LLM_PROVIDER")  # ถ้าไม่ set ใช้ default
        self.llm = create_llm(provider)

# title_translate/handler.py
class TitleTranslateHandler:
    def __init__(self, ...):
        provider = os.getenv("TITLE_LLM_PROVIDER")     # ใช้ OpenAI
        self.llm = create_llm(provider)
```

---

## Prompt Logic ย้ายไปอยู่ใน service (ไม่อยู่ใน adapter)

### Before (ผิด — prompt อยู่ใน adapter)

```python
# gemini_adapter.py (ปัจจุบัน)
class GeminiAdapter:
    def translate_cluster(self, lines, ...):
        prompt = self._build_cluster_prompt(...)   # ← prompt อยู่ใน adapter
        response = self._client.generate(prompt)
        return self._parse_cluster_response(...)   # ← parse อยู่ใน adapter
```

### After (ถูก — prompt อยู่ใน service)

```python
# subtitle_translate/prompts.py
def build_cluster_prompt(lines, previous_summary, source_lang, target_lang, context):
    """สร้าง prompt สำหรับ cluster translation"""
    return f"Translate these {source_lang} subtitles to {target_lang}..."

def parse_cluster_response(text, lines):
    """Parse LLM response กลับเป็น translated lines"""
    ...

# subtitle_translate/handler.py
class SubtitleTranslateHandler:
    def __init__(self, llm: LLMPort):
        self.llm = llm    # ไม่สนว่า Gemini หรือ OpenAI

    def _translate_cluster(self, lines, ...):
        prompt = build_cluster_prompt(lines, ...)      # ← prompt อยู่ใน service
        response = self.llm.generate(prompt)           # ← LLM ทำแค่ส่ง prompt
        return parse_cluster_response(response.text)   # ← parse อยู่ใน service
```

---

## สิ่งที่ต้องแก้ในแต่ละ service

### subtitle_detect — ไม่ต้องแก้ (ไม่ใช้ LLM)

### subtitle_transcribe — แก้เล็กน้อย

```
ปัจจุบัน:
  gemini = get_gemini_adapter()
  gemini.refine_batch(segments, lang)

ใหม่:
  llm = create_llm(os.getenv("SUBTITLE_LLM_PROVIDER"))
  prompt = build_refine_prompt(segments, lang)
  response = llm.generate(prompt)
  refined = parse_refine_response(response.text, segments)
```

**ย้าย**: `_build_refine_prompt()` + `_parse_translate_response()` จาก `gemini_adapter.py` → `subtitle_transcribe/prompts.py`

### subtitle_translate — แก้

```
ปัจจุบัน:
  gemini = get_gemini_adapter()
  gemini.translate_cluster(lines, summary, source, target)

ใหม่:
  llm = create_llm(os.getenv("SUBTITLE_LLM_PROVIDER"))
  prompt = build_cluster_prompt(lines, summary, source, target)
  response = llm.generate(prompt)
  translated, summary = parse_cluster_response(response.text, lines)
```

**ย้าย**: `_build_cluster_prompt()` + `_parse_cluster_response()` จาก `gemini_adapter.py` → `subtitle_translate/prompts.py`

### title_translate — ใหม่ (ใช้ Port ตั้งแต่แรก)

```python
llm = create_llm(os.getenv("TITLE_LLM_PROVIDER"))
prompt = build_klon8_prompt(title_en, tags, cast)
response = llm.generate_with_safety_off(prompt)   # ปิด safety สำหรับ 18+
title_th = parse_klon8_response(response.text, code)
```

---

## Legacy gemini_adapter.py

**ไม่ลบ** — เก็บไว้จนกว่า migrate ครบทุก service แล้ว mark deprecated:

```python
# shared/adapters/gemini_adapter.py
"""
DEPRECATED: ใช้ shared/ports/llm_port.py + shared/adapters/gemini_llm.py แทน
เก็บไว้สำหรับ backward compatibility จนกว่า migrate ครบ
"""
```

---

## Execution Order

| Step | สิ่งที่ทำ | กระทบ code เก่า |
|------|----------|----------------|
| 1 | สร้าง `shared/ports/llm_port.py` | ไม่กระทบ (เพิ่มไฟล์ใหม่) |
| 2 | สร้าง `shared/adapters/gemini_llm.py` | ไม่กระทบ (เพิ่มไฟล์ใหม่) |
| 3 | สร้าง `shared/adapters/llm_factory.py` | ไม่กระทบ (เพิ่มไฟล์ใหม่) |
| 4 | สร้าง `subtitle_transcribe/prompts.py` — ย้าย refine prompt | ไม่กระทบ (เพิ่มไฟล์ใหม่) |
| 5 | สร้าง `subtitle_translate/prompts.py` — ย้าย cluster prompt | ไม่กระทบ (เพิ่มไฟล์ใหม่) |
| 6 | แก้ `subtitle_transcribe/handler.py` — ใช้ LLMPort | **แก้ code เก่า** |
| 7 | แก้ `subtitle_translate/handler.py` — ใช้ LLMPort | **แก้ code เก่า** |
| 8 | ย้าย + สร้าง `title_translate/` | ไม่กระทบ (เพิ่ม service ใหม่) |
| 9 | ทดสอบทุก service | - |
| 10 | Mark `gemini_adapter.py` deprecated | ไม่กระทบ (แค่ comment) |

**Step 1-5**: เพิ่มไฟล์ใหม่ ไม่กระทบอะไร
**Step 6-7**: แก้ code เก่า แต่ behavior เหมือนเดิม (แค่เปลี่ยนทาง call)
**Step 8**: service ใหม่

---

## ข้อดีสุดท้าย

```
Gemini ban title translate?
  → .env: TITLE_LLM_PROVIDER=openai
  → restart title_translate
  → เสร็จ (ไม่แก้ code)

Gemini ban subtitle translate ด้วย?
  → .env: SUBTITLE_LLM_PROVIDER=openai
  → restart subtitle workers
  → เสร็จ

อยากใช้ local LLM (Ollama) ไม่เสียเงิน?
  → สร้าง shared/adapters/ollama_llm.py
  → .env: LLM_PROVIDER=ollama
  → ทุก service ใช้ได้ทันที
```
