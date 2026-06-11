# Subtitle Translate — Service Flow

> แปลซับไตเติ้ลจากภาษาต้นฉบับเป็นภาษาเป้าหมาย ใช้ LLM cluster-based translation
> Path: `_my_worker/python/subtitle_translate/`

---

## Flow

```
User เลือกภาษา + กด "แปล" บน UI (ต้อง transcribe เสร็จก่อน)
    │
    ▼
Frontend: POST /api/v1/videos/:id/subtitle/translate
    Body: { "targetLanguages": ["th"] }
    │
    ▼
API: SubtitleService.TriggerTranslation()
    ├── Validate: original subtitle ready
    ├── Create Subtitle record per target lang (status=queued)
    └── Publish to NATS: jobs.subtitle.translate
    │
    ▼
NATS Stream: SUBTITLE_JOBS
Consumer: SUBTITLE_TRANSLATE_WORKER
    │
    ▼
Python Worker: subtitle_translate/handler.py
    │
    ├── [1] Initialize                                  (0%)
    ├── [2] Download source .srt from S3                (5%)
    ├── [3] Load speakers.json (optional)               (—)
    ├── [4] Per target language:
    │   ├── Split into clusters (gap > 3s, max 30 lines)
    │   ├── Per cluster:
    │   │   ├── ★ LLM translate cluster                (10-80%)
    │   │   └── Pass summary to next cluster
    │   ├── Fix segment overlaps
    │   ├── Generate SRT + VTT
    │   └── Upload to S3                                (+10%)
    ├── [5] Publish completed (per language)             (100%)
    └── Cleanup temp files
    │
    ▼
API: ProgressBroadcaster.handleTranslateCompleted()
    └── Update subtitle.srt_path, subtitle.status = ready
    │
    ▼
WebSocket → Frontend แสดง "🇹🇭 ไทย [แปล] [ready]"
```

---

## AI Model ที่ใช้ (1 ตัว)

| Step | AI Model | Port | Adapter | File | เปลี่ยนได้? |
|------|----------|------|---------|------|------------|
| 4 | **Gemini LLM** (translate) | `LLMPort` | `GeminiLLM` | `shared/adapters/gemini_llm.py` | เปลี่ยน provider ได้ |

### LLM Translation — Cluster-Based

```python
# subtitle_translate/handler.py line 257-276
llm = get_llm()    # create_llm() → LLMPort (Gemini/OpenAI/Local)

translated = await self._translate_with_clusters(
    segments=segments,
    source_lang=LanguageCode("ja"),
    target_lang=LanguageCode("th"),
    context=context,
    llm=llm,        # ส่ง LLMPort เข้าไป (ไม่ผูกกับ provider ใดๆ)
)
```

**ทำไมต้อง cluster?**
- แปลทีละบรรทัด → เสียบริบท (ตัวละครพูดอะไร, สถานการณ์อะไร)
- แปลทั้งไฟล์ → ใหญ่เกินสำหรับ LLM
- Cluster (30 บรรทัด) → ได้บริบทพอดี + ส่ง summary ต่อ cluster ถัดไป

**Cluster Logic** (`subtitle_translate/handler.py`):
```
Split: gap > 3 seconds หรือ > 30 lines → cluster ใหม่
Scene change: gap > 10 seconds → reset summary

Per cluster:
  1. Build prompt (lines + previous_summary + speaker info)
  2. LLM generate → translated lines + new summary
  3. Pass summary to next cluster
```

**Prompt Logic** (`subtitle_translate/prompts.py`):
```python
def build_cluster_prompt(lines, previous_summary, source_lang, target_lang, ...):
    """
    Translate these {source} subtitles to {target}.
    Previous context: {summary}
    Lines (index|text|gender|speaker): ...
    """

def parse_cluster_response(response_text, lines):
    """Parse: index|translated_text + Summary: ..."""
```

---

## Key Files

| File | Description |
|------|-------------|
| `_my_worker/python/subtitle_translate/main.py` | Entry point — NATS consumer |
| `_my_worker/python/subtitle_translate/handler.py` | Handler — cluster translation pipeline |
| `_my_worker/python/subtitle_translate/prompts.py` | LLM prompts — cluster prompt + parse (ใช้ LLMPort) |
| `_my_worker/python/shared/adapters/gemini_llm.py` | GeminiLLM (implements LLMPort) |
| `_my_worker/python/shared/adapters/llm_factory.py` | Factory: create_llm("gemini") |
| `_my_worker/python/shared/ports/llm_port.py` | LLMPort interface |
| `_my_worker/python/shared/entities.py` | Domain entities (SubtitleLine, LanguageCode) |

---

## Input / Output

**NATS Input** (`jobs.subtitle.translate`):
```json
{ "_meta": { "job_type": "subtitle_translate", "entity_type": "video", "entity_id": "<video_uuid>" },
  "input": {
    "subtitle_ids": ["<subtitle_uuid_for_th>"],
    "video_id": "<video_uuid>", "video_code": "CODE",
    "source_srt_path": "subtitles/{code}/ja.srt",
    "source_language": "ja",
    "target_languages": ["th"],
    "output_path": "subtitles/{code}"
  } }
```

**Files uploaded to S3** (per target language):
- `subtitles/{code}/{target_lang}.srt`
- `subtitles/{code}/{target_lang}.vtt`

**Progress Output** (per language, `progress.video.{videoID}`):
```json
{ "status": "completed", "subtitle_id": "<uuid>", "video_id": "<uuid>",
  "current_language": "th",
  "output": { "translations": {"th": "subtitles/{code}/th.srt"}, "source_language": "ja" } }
```

---

## Translation Target Rules

| Source Language | Target Language |
|---------------|----------------|
| th (ไทย) | en (อังกฤษ) |
| ja, en, zh, ko, ru | th (ไทย) |

---

## Speaker Tagging (Optional)

ถ้ามี `speakers.json` บน S3 → tag แต่ละ subtitle ด้วย gender + speaker_id
→ LLM ใช้ gender เพื่อเลือกคำให้ถูก (เช่น ครับ/ค่ะ ในภาษาไทย)

---

## เปลี่ยน LLM Provider

```bash
# .env
SUBTITLE_LLM_PROVIDER=gemini    # default
# SUBTITLE_LLM_PROVIDER=openai  # ถ้า Gemini ban
```

เปลี่ยนแค่ env → restart worker → LLM provider เปลี่ยน
Prompt logic (prompts.py) ไม่ต้องแก้ — ส่ง prompt เหมือนเดิมไม่ว่าจะใช้ LLM ตัวไหน
