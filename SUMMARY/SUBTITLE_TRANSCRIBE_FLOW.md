# Subtitle Transcribe — Service Flow

> ถอดเสียงเป็นข้อความ (Speech-to-Text) พร้อม VAD gap detection + LLM refinement
> Path: `_my_worker/python/subtitle_transcribe/`

---

## Flow

```
User กด "Transcribe" บน UI (ต้อง detect ก่อน)
    │
    ▼
Frontend: POST /api/v1/videos/:id/subtitle/transcribe
    │
    ▼
API: SubtitleService.TriggerTranscribe()
    ├── Validate: has audio_path, detected_language not empty
    ├── Create Subtitle record (status=queued)
    └── Publish to NATS: jobs.subtitle.transcribe
    │
    ▼
NATS Stream: SUBTITLE_JOBS
Consumer: SUBTITLE_TRANSCRIBE_WORKER
    │
    ▼
Python Worker: subtitle_transcribe/handler.py (14 stages)
    │
    ├── [1]  Initialize                                (0%)
    ├── [2]  Download audio.wav from S3                (5%)
    ├── [3]  ★ Demucs voice separation                (10%)
    ├── [4]  ★ Whisper transcribe                     (20-50%)
    ├── [5]  Cap subtitle duration (>7s → trim)        (55%)
    ├── [6]  ★ Silero VAD detection                   (60%)
    ├── [7]  Cut gap audio segments (FFmpeg)            (65%)
    ├── [7b] ★ Whisper re-transcribe gaps             (70%)
    ├── [8]  Merge gap results                         (75%)
    ├── [9]  Smart fix (remove empty/noise)            (78%)
    ├── [10] ★ LLM refine (fix transcription errors)  (82%)
    ├── [11] Generate SRT + VTT files                  (88%)
    ├── [12] Upload to S3                              (95%)
    ├── [13] Publish completed                         (100%)
    └── Cleanup temp files
    │
    ▼
API: ProgressBroadcaster.handleTranscribeCompleted()
    └── Update subtitle.srt_path, subtitle.status = ready
    │
    ▼
WebSocket → Frontend แสดง "🇯🇵 ญี่ปุ่น [ต้นฉบับ] [ready]"
```

---

## AI Models ที่ใช้ (5 ตัว)

| Step | AI Model | Port | Adapter | File | เปลี่ยนได้? |
|------|----------|------|---------|------|------------|
| 3 | **Demucs** (htdemucs) | `AudioPort` | `AudioAdapter` | `shared/adapters/audio_adapter.py` | เปลี่ยน model ได้ |
| 4 | **Whisper turbo** | `STTPort` | `WhisperAdapter` | `shared/adapters/whisper_adapter.py` | เปลี่ยน model ได้ |
| 6 | **Silero VAD** | `VADPort` | `SileroVADAdapter` | `shared/adapters/vad_adapter.py` | เปลี่ยน VAD ได้ |
| 7b | **Whisper kotoba/large-v3** | `STTPort` | `WhisperAdapter` | `shared/adapters/whisper_adapter.py` | เปลี่ยน model ได้ |
| 10 | **Gemini LLM** (refine) | `LLMPort` | `GeminiLLM` | `shared/adapters/gemini_llm.py` | เปลี่ยน provider ได้ |

### Step 3 — Demucs (Voice Separation)

```python
# subtitle_transcribe/handler.py line 304
audio_adapter = AudioAdapter(temp_dir=job_dir)      # implements AudioPort
success = audio_adapter.separate_vocals(local_audio, local_audio_clean)
```

- **Model**: `htdemucs` (Meta's Hybrid Transformer Demucs)
- **Command**: `demucs -n htdemucs --two-stems vocals -o {out} {input}`
- **Timeout**: 600 seconds (10 min)
- **Fallback**: ถ้า Demucs ล้มเหลว → ใช้ original audio ต่อ

### Step 4 — Whisper Transcribe (Main)

```python
# subtitle_transcribe/handler.py line 321
whisper = WhisperAdapter(model="turbo", device="cuda")   # implements STTPort
subtitles = whisper.transcribe(audio_path, LanguageCode("ja"))
```

- **Model**: `deepdml/faster-whisper-large-v3-turbo-ct2`
- **Options**: `vad_filter=True`, `word_timestamps=True`, `no_speech_threshold=0.3`
- **Hallucination filter**: Regex patterns กรอง noise

### Step 6 — Silero VAD

```python
# subtitle_transcribe/handler.py line 509
vad = SileroVADAdapter()                               # implements VADPort
vad_segments = vad.detect_voice(audio_path)
gaps = vad.find_gaps(vad_segments, segments)
```

- **Model**: `snakers4/silero-vad` via `torch.hub`
- **Purpose**: หาช่วงเสียงที่ Whisper ถอดไม่ได้ แล้ว re-transcribe

### Step 7b — Whisper Gap Re-transcribe

```python
# subtitle_transcribe/handler.py line 524-543
if language.code == "ja":
    gap_whisper = WhisperAdapter(model="kotoba", device="cuda")   # implements STTPort
else:
    gap_whisper = WhisperAdapter(model="large-v3", device="cuda") # implements STTPort
text, is_valid = gap_whisper.transcribe_segment(gap_audio, language)
```

- **Japanese**: `kotoba-tech/kotoba-whisper-v2.0-faster` (less hallucination)
- **Other**: `large-v3` (standard Whisper)

### Step 10 — LLM Refine ★ (เปลี่ยน provider ได้ผ่าน Port)

```python
# subtitle_transcribe/handler.py line 375-376
from subtitle_transcribe.prompts import refine_subtitles
llm = get_llm()                    # create_llm() → LLMPort (Gemini/OpenAI/Local)
segments = refine_subtitles(llm, segments, lang_code)
```

- **Provider**: จาก env `SUBTITLE_LLM_PROVIDER` (default: gemini)
- **Prompt**: อยู่ที่ `subtitle_transcribe/prompts.py`
- **Purpose**: แก้ typo + errors จาก Whisper
- **Fallback**: ถ้า LLM ล้มเหลว → keep original text

---

## Key Files

| File | Description |
|------|-------------|
| `_my_worker/python/subtitle_transcribe/main.py` | Entry point — NATS consumer |
| `_my_worker/python/subtitle_transcribe/handler.py` | Handler — 14-stage pipeline |
| `_my_worker/python/subtitle_transcribe/prompts.py` | LLM prompts — refine logic (ใช้ LLMPort) |
| `_my_worker/python/shared/adapters/whisper_adapter.py` | WhisperAdapter (implements STTPort) |
| `_my_worker/python/shared/adapters/vad_adapter.py` | SileroVADAdapter (implements VADPort) |
| `_my_worker/python/shared/adapters/audio_adapter.py` | AudioAdapter (implements AudioPort) |
| `_my_worker/python/shared/adapters/gemini_llm.py` | GeminiLLM (implements LLMPort) |
| `_my_worker/python/shared/adapters/llm_factory.py` | Factory: create_llm("gemini") |
| `_my_worker/python/shared/ports/stt_port.py` | STTPort interface |
| `_my_worker/python/shared/ports/vad_port.py` | VADPort interface |
| `_my_worker/python/shared/ports/audio_port.py` | AudioPort interface |
| `_my_worker/python/shared/ports/llm_port.py` | LLMPort interface |
| `_my_worker/python/shared/entities.py` | Domain entities (SubtitleLine, LanguageCode, etc.) |

---

## Input / Output

**NATS Input** (`jobs.subtitle.transcribe`):
```json
{ "_meta": { "job_type": "subtitle_transcribe", "entity_type": "subtitle", "entity_id": "<subtitle_uuid>" },
  "input": {
    "video_id": "<video_uuid>", "video_code": "CODE",
    "audio_path": "audio/{code}/audio.wav",
    "language": "ja",
    "output_path": "subtitles/{code}/ja.srt",
    "refine_with_llm": true
  } }
```

**Files uploaded to S3**:
- `subtitles/{code}/{lang}.srt`
- `subtitles/{code}/{lang}.vtt`

**Progress Output** (`progress.subtitle.{videoID}`):
```json
{ "status": "completed", "subtitle_id": "<uuid>", "video_id": "<uuid>",
  "output": { "srt_path": "subtitles/{code}/ja.srt", "vtt_path": "...", "segments": 234, "language": "ja" } }
```

---

## เปลี่ยน LLM Provider

```bash
# .env
SUBTITLE_LLM_PROVIDER=gemini    # default
# SUBTITLE_LLM_PROVIDER=openai  # ถ้า Gemini ban
```

เปลี่ยนแค่ env → restart worker → LLM provider เปลี่ยน
Business logic (handler.py, prompts.py) ไม่ต้องแก้
