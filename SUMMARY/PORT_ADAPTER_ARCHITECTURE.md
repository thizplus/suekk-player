# Port/Adapter Architecture — AI Models

> ทุก AI model ที่ใช้ในระบบ เปลี่ยนได้โดยไม่แก้ business logic
> Path: `_my_worker/python/shared/`

---

## Overview

```
Business Logic (handlers)
    │
    ▼
  Port (interface)          ← กำหนดว่า "ต้องทำอะไรได้"
    │
    ▼
  Adapter (implementation)  ← กำหนดว่า "ทำยังไง + ใช้ model อะไร"
```

**เปลี่ยน Adapter** = เปลี่ยน AI model
**ไม่แก้ Port** = ไม่แก้ business logic

---

## Port / Adapter Map

| Port | Adapter | AI Model | ใช้ใน Service | เปลี่ยนจาก |
|------|---------|----------|--------------|-----------|
| `LLMPort` | `GeminiLLM` | Gemini 2.5 Flash | transcribe (refine), translate | `SUBTITLE_LLM_PROVIDER` env |
| `STTPort` | `WhisperAdapter` | faster-whisper (turbo/kotoba/large-v3) | detect, transcribe | constructor param `model=` |
| `VADPort` | `SileroVADAdapter` | Silero VAD | transcribe | สร้าง adapter ใหม่ |
| `AudioPort` | `AudioAdapter` | Demucs + FFmpeg | transcribe | สร้าง adapter ใหม่ |

---

## File Structure

```
_my_worker/python/shared/
├── entities.py                      # Domain entities (SubtitleLine, LanguageCode, etc.)
│
├── ports/                           # Interfaces — "ต้องทำอะไรได้"
│   ├── __init__.py
│   ├── llm_port.py                  # LLMPort: generate(prompt) → LLMResponse
│   ├── stt_port.py                  # STTPort: transcribe(audio, lang) → SubtitleLine[]
│   ├── vad_port.py                  # VADPort: detect_voice(audio) → VoiceSegment[]
│   └── audio_port.py               # AudioPort: separate_vocals(), cut_segment()
│
└── adapters/                        # Implementations — "ทำยังไง"
    ├── __init__.py
    ├── entities.py                  # Re-export จาก shared/entities.py (backward compat)
    │
    │  # LLM Adapters
    ├── gemini_llm.py                # GeminiLLM implements LLMPort
    ├── llm_factory.py               # create_llm("gemini") → LLMPort
    ├── gemini_adapter.py            # Legacy (deprecated, backward compat)
    │
    │  # STT Adapters
    ├── whisper_adapter.py           # WhisperAdapter implements STTPort
    │
    │  # VAD Adapters
    ├── vad_adapter.py               # SileroVADAdapter implements VADPort
    │
    │  # Audio Adapters
    └── audio_adapter.py             # AudioAdapter implements AudioPort
```

---

## Port Interfaces (Detail)

### LLMPort (`shared/ports/llm_port.py`)

```python
class LLMPort(ABC):
    def generate(self, prompt: str) -> LLMResponse:        # ส่ง prompt รับ text
    def generate_unsafe(self, prompt: str) -> LLMResponse:  # ปิด safety filter (18+)
    def get_model_name(self) -> str:
    def get_provider_name(self) -> str:

@dataclass
class LLMResponse:
    text: str
    success: bool
    error: Optional[str] = None
    model: Optional[str] = None
```

### STTPort (`shared/ports/stt_port.py`)

```python
class STTPort(ABC):
    def transcribe(self, audio_path, language) -> List[SubtitleLine]:
    def transcribe_segment(self, audio_path, language) -> Tuple[str, bool]:
    def detect_language(self, audio_path) -> Tuple[LanguageCode, float]:
    def get_model_name(self) -> str:
```

### VADPort (`shared/ports/vad_port.py`)

```python
class VADPort(ABC):
    def detect_voice(self, audio_path, threshold) -> List[VoiceSegment]:
    def find_gaps(self, vad_segments, subtitles) -> List[dict]:
```

### AudioPort (`shared/ports/audio_port.py`)

```python
class AudioPort(ABC):
    def separate_vocals(self, audio_path, output_path) -> bool:
    def cut_segment(self, audio_path, output_path, start, end) -> bool:
    def get_duration(self, audio_path) -> Optional[float]:
```

---

## LLM Factory (`shared/adapters/llm_factory.py`)

```python
from shared.adapters.llm_factory import create_llm

# Default (จาก env LLM_PROVIDER)
llm = create_llm()

# ระบุ provider
llm = create_llm("gemini")
llm = create_llm("openai")     # เมื่อ implement OpenAILLM

# Override config
llm = create_llm("gemini", model="gemini-2.0-flash", api_key="xxx")

# แต่ละ service เลือกคนละตัว
subtitle_llm = create_llm(os.getenv("SUBTITLE_LLM_PROVIDER"))   # gemini
title_llm = create_llm(os.getenv("TITLE_LLM_PROVIDER"))         # openai
```

---

## แต่ละ Service ใช้ AI อะไรบ้าง

### subtitle_detect
```
WhisperAdapter (STTPort) ─── detect_language()
```

### subtitle_transcribe
```
AudioAdapter (AudioPort) ─── separate_vocals()     [Demucs]
WhisperAdapter (STTPort) ─── transcribe()           [Whisper turbo]
SileroVADAdapter (VADPort) ─ detect_voice()         [Silero VAD]
WhisperAdapter (STTPort) ─── transcribe_segment()   [Whisper kotoba/large-v3]
GeminiLLM (LLMPort) ──────── generate()             [Gemini → refine]
```

### subtitle_translate
```
GeminiLLM (LLMPort) ──────── generate()             [Gemini → translate clusters]
```

---

## เพิ่ม LLM Provider ใหม่

### ตัวอย่าง: เพิ่ม OpenAI

1. สร้าง `shared/adapters/openai_llm.py`:
```python
class OpenAILLM(LLMPort):
    def generate(self, prompt): ...
    def generate_unsafe(self, prompt): ...
```

2. เพิ่มใน `shared/adapters/llm_factory.py`:
```python
elif provider == "openai":
    from shared.adapters.openai_llm import OpenAILLM
    return OpenAILLM(api_key=..., model=...)
```

3. เปลี่ยน `.env`:
```bash
SUBTITLE_LLM_PROVIDER=openai
OPENAI_API_KEY=sk-...
```

4. **ไม่ต้องแก้ handler หรือ prompts** — ทำงานเหมือนเดิม

---

## เพิ่ม STT Model ใหม่

### ตัวอย่าง: เพิ่ม whisper.cpp

1. สร้าง `shared/adapters/whisper_cpp_adapter.py`:
```python
class WhisperCppAdapter(STTPort):
    def transcribe(self, audio_path, language): ...
    def detect_language(self, audio_path): ...
```

2. ใช้ใน handler:
```python
stt = WhisperCppAdapter(model_path="...")
subtitles = stt.transcribe(audio_path, language)
```

3. **ไม่ต้องแก้ handler logic** — เรียก STTPort เหมือนเดิม
