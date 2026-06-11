# Subtitle Detect — Service Flow

> ตรวจจับภาษาจากเสียง (ไม่ถอดเสียงจริง แค่ detect ว่าเป็นภาษาอะไร)
> Path: `_my_worker/python/subtitle_detect/`

---

## Flow

```
User กด "Detect Language" บน UI
    │
    ▼
Frontend: POST /api/v1/videos/:id/subtitle/detect
    │
    ▼
API: SubtitleService.TriggerDetectLanguage()
    ├── Validate: video ready, has audio_path
    ├── Clear existing detected_language
    └── Publish to NATS: jobs.subtitle.detect
    │
    ▼
NATS Stream: SUBTITLE_JOBS
Consumer: SUBTITLE_DETECT_WORKER
    │
    ▼
Python Worker: subtitle_detect/handler.py
    │
    ├── [1] Download audio.wav from S3         (0-10%)
    ├── [2] ★ Whisper detect language          (10-70%)
    ├── [3] Publish completed                  (100%)
    └── Cleanup temp files
    │
    ▼
API: ProgressBroadcaster.handleDetectCompleted()
    └── Update video.detected_language = "ja"
    │
    ▼
WebSocket → Frontend แสดง "🇯🇵 ญี่ปุ่น"
```

---

## AI Model ที่ใช้

| Step | AI Model | Port | Adapter | File |
|------|----------|------|---------|------|
| 2 | **faster-whisper turbo** | `STTPort` | `WhisperAdapter` | `shared/adapters/whisper_adapter.py` |

### Whisper Usage Detail

```python
# subtitle_detect/handler.py line 150
adapter = WhisperAdapter(model="turbo", device="cuda")
language, confidence = adapter.detect_language(audio_path)
# Returns: ("ja", 0.97)
```

- **Model**: `deepdml/faster-whisper-large-v3-turbo-ct2`
- **Mode**: Detection only (ไม่ transcribe จริง แค่ดูว่าภาษาอะไร)
- **Compute**: `int8_float16` on CUDA
- **Fallback**: ถ้า detect ล้มเหลว → return `("ja", 0.0)`

---

## Key Files

| File | Description |
|------|-------------|
| `_my_worker/python/subtitle_detect/main.py` | Entry point — NATS consumer |
| `_my_worker/python/subtitle_detect/handler.py` | Handler — download audio + Whisper detect |
| `_my_worker/python/shared/adapters/whisper_adapter.py` | WhisperAdapter (implements STTPort) |
| `_my_worker/python/shared/ports/stt_port.py` | STTPort interface |
| `_gofiber_starter/application/serviceimpl/subtitle_service_impl.go` | API: TriggerDetectLanguage() |
| `_gofiber_starter/infrastructure/websocket/progress_broadcaster.go` | API: handleDetectCompleted() |

---

## Input / Output

**NATS Input** (`jobs.subtitle.detect`):
```json
{ "_meta": { "job_type": "subtitle_detect", "entity_type": "video", "entity_id": "<video_uuid>" },
  "input": { "audio_path": "audio/{code}/audio.wav" } }
```

**Progress Output** (`progress.subtitle.{videoID}`):
```json
{ "status": "completed", "output": { "language": "ja", "confidence": 0.97 } }
```

---

## ไม่ใช้ LLM — ใช้แค่ Whisper STT
