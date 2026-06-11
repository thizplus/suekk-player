# SUEKK Stream — Subtitle Pipeline Overview

> Detect -> Transcribe -> Translate (Auto-chained by API)
> Workers: Python (faster-whisper + Gemini)
> Orchestration: API ProgressBroadcaster

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                        API (Orchestrator)                        │
│                                                                  │
│  User clicks "Detect"                                            │
│       │                                                          │
│       ▼                                                          │
│  SubtitleService.TriggerDetectLanguage()                         │
│       │  publish to jobs.subtitle.detect                         │
│       ▼                                                          │
│  ┌─────────────────┐    progress.subtitle.{videoID}              │
│  │  Detect Worker   │ ────────────────────────────►              │
│  │  (Python)        │                              │             │
│  └─────────────────┘                               │             │
│                                          ProgressBroadcaster     │
│                                          handleDetectCompleted() │
│                                                │                 │
│                                     auto-trigger│                │
│                                                ▼                 │
│  SubtitleService.TriggerTranscribe()                             │
│       │  publish to jobs.subtitle.transcribe                     │
│       ▼                                                          │
│  ┌─────────────────┐    progress.subtitle.{videoID}              │
│  │ Transcribe Worker│ ────────────────────────────►              │
│  │  (Python)        │                              │             │
│  └─────────────────┘                               │             │
│                                          ProgressBroadcaster     │
│                                          handleTranscribeCompleted│
│                                                │                 │
│                                     auto-trigger│                │
│                                                ▼                 │
│  SubtitleService.TriggerTranslation()                            │
│       │  publish to jobs.subtitle.translate                      │
│       ▼                                                          │
│  ┌─────────────────┐    progress.subtitle.{videoID}              │
│  │ Translate Worker │ ────────────────────────────►              │
│  │  (Python)        │                              │             │
│  └─────────────────┘                               │             │
│                                          ProgressBroadcaster     │
│                                          handleTranslateCompleted │
│                                                │                 │
│                                          (chain ends)            │
└──────────────────────────────────────────────────────────────────┘
```

**Key Design**: Workers ทำแค่งานเดียวแล้วส่งผลกลับ API เป็นคน orchestrate chain ทั้งหมด

---

## Step 1: Language Detection

### Trigger

```
POST /api/v1/videos/:id/subtitle/detect
```

### Validation (API Side)

1. Video must exist
2. Video must have `audio_path` (set during transcode — audio extracted as WAV 16kHz mono)
3. Video status must be `ready`
4. Clears existing `detected_language` (for re-detect)

### NATS Publish

**Subject**: `jobs.subtitle.detect`
**Stream**: `SUBTITLE_JOBS`

```json
{
  "_meta": {
    "job_id": "uuid",
    "job_type": "subtitle_detect",
    "entity_type": "video",
    "entity_id": "<video_uuid>",
    "entity_code": "<video_code>",
    "timeout_sec": 300
  },
  "input": {
    "audio_path": "audio/{code}/audio.wav"
  }
}
```

### Worker Processing (Python `subtitle_detect/handler.py`)

| Step | Stage | Progress | Message | Detail |
|------|-------|----------|---------|--------|
| 1 | `initializing` | 0% | "เริ่มตรวจจับภาษา" | Setup temp dir |
| 2 | `downloading` | 10% | "กำลังดาวน์โหลดเสียง..." | Download audio.wav from S3 |
| 3 | `detecting` | 30% | "กำลังตรวจจับภาษา..." | Load faster-whisper turbo model |
| 4 | `detecting` | 70% | "ตรวจพบภาษา: {lang}" | Run `model.transcribe(language=None)` -> read `info.language` |
| 5 | completed | 100% | "สำเร็จ" | Cleanup temp |

**Model Used**: `faster-whisper turbo` (`deepdml/faster-whisper-large-v3-turbo-ct2`)
- `vad_filter=True` เพื่อข้าม silence
- ไม่ transcribe จริง แค่ detect language จาก audio
- **Fallback**: ถ้า detect ล้มเหลว -> return `("ja", 0.0)`

### Completion Output

Published to `progress.subtitle.{videoID}`:
```json
{
  "status": "completed",
  "job_type": "subtitle_detect",
  "entity_id": "<video_uuid>",
  "entity_code": "<video_code>",
  "output": {
    "language": "ja",
    "confidence": 0.97
  }
}
```

### API on Completed (ProgressBroadcaster)

1. `subtitleService.HandleDetectComplete()`:
   - Update `video.detected_language = "ja"` in DB
2. **Auto-trigger next step**:
   - `subtitleService.TriggerTranscribe(videoID)` -> publishes transcribe job

---

## Step 2: Transcription

### Trigger

Auto-triggered after detect, or manual:
```
POST /api/v1/videos/:id/subtitle/transcribe
```

### Validation (API Side)

1. Video must have `audio_path`
2. `video.detected_language` must NOT be empty (ต้อง detect ก่อน)
3. No existing original subtitle with status `ready` or in-progress
4. Creates `Subtitle` DB record: `type=original`, `language=detected_lang`, `status=queued`

### NATS Publish

**Subject**: `jobs.subtitle.transcribe`

```json
{
  "_meta": {
    "job_id": "uuid",
    "job_type": "subtitle_transcribe",
    "entity_type": "subtitle",
    "entity_id": "<subtitle_uuid>",
    "entity_code": "<video_code>",
    "timeout_sec": 1800
  },
  "input": {
    "video_id": "<video_uuid>",
    "video_code": "<video_code>",
    "audio_path": "audio/{code}/audio.wav",
    "language": "ja",
    "output_path": "subtitles/{code}/ja.srt",
    "refine_with_llm": true,
    "context": ""
  }
}
```

### Worker Processing (Python `subtitle_transcribe/handler.py`) — 14 Stages

| Step | Stage | Progress | Message | Detail |
|------|-------|----------|---------|--------|
| 1 | `initializing` | 0% | "เริ่มถอดเสียง" | Setup temp dir |
| 2 | `downloading` | 5% | "กำลังดาวน์โหลดเสียง..." | Download audio.wav from S3 |
| 3 | `demucs` | 10% | "กำลังแยกเสียงพูด (Demucs)" | Voice separation (if `USE_DEMUCS=true`) |
| 4a | `transcribing` | 15% | "กำลังตรวจจับภาษา..." | Only if language=="auto" |
| 4b | `transcribing` | 20% | "กำลังถอดเสียง ({lang})..." | Main Whisper transcription |
| 4c | `transcribing` | 50% | "ถอดเสียงได้ {N} บรรทัด" | Transcription complete |
| 5 | `processing` | 55% | "กำลังตัดซับไตเติ้ลยาว" | Cap segments > 7 seconds |
| 6 | `vad` | 60% | "กำลังตรวจหาเสียงที่หายไป (VAD)" | Silero VAD (if `USE_VAD=true`) |
| 7a | `gaps` | 65% | "กำลังประมวลผล {N} ช่องว่าง" | Find speech gaps not covered by subtitles |
| 7b | `gaps` | 70% | "กำลังถอดเสียงช่องว่าง" | Re-transcribe gaps with specialized model |
| 8 | `merging` | 75% | "กำลังรวมซับไตเติ้ล" | Merge gap results into main subtitles |
| 9 | `fixing` | 78% | "กำลังแก้ไขซับไตเติ้ล" | Remove empty/noise segments |
| 10 | `refining` | 82% | "กำลังปรับปรุงข้อความ (LLM)" | Gemini batch refine (if `USE_REFINE=true`) |
| 11 | `generating` | 88% | "กำลังสร้างไฟล์ SRT/VTT" | Fix overlaps, generate SRT + VTT |
| 12 | `uploading` | 95% | "กำลังอัพโหลด..." | Upload to S3 |
| 13 | completed | 100% | "สำเร็จ" | Cleanup temp |

### Detailed Stage Logic

#### Stage 3 — Demucs (Voice Separation)

```bash
demucs -n htdemucs --two-stems vocals -o {demucs_out} {audio_path}
```

- Model: `htdemucs` (Meta's Hybrid Transformer Demucs)
- Timeout: 600 seconds (10 min)
- Output: `{demucs_out}/htdemucs/{stem}/vocals.wav`
- Convert to 16kHz mono WAV via FFmpeg
- **Fallback**: ถ้า Demucs ล้มเหลวหรือ timeout -> ใช้ original audio ต่อ (silent fallback)

#### Stage 4 — Whisper Transcription

**Model**: `faster-whisper turbo` (`deepdml/faster-whisper-large-v3-turbo-ct2`, `int8_float16`)

**Options**:
```python
model.transcribe(
    audio_path,
    language=language,          # "ja", "en", etc.
    vad_filter=True,
    vad_parameters={"min_silence_duration_ms": 500},
    condition_on_previous_text=False,
    word_timestamps=True,
    no_speech_threshold=0.3,
    temperature=0
)
```

**Hallucination Filter** (applied per segment):
```python
HALLUCINATION_PATTERNS = [
    r"ご視聴.*ありがとう",      # "Thanks for watching"
    r"アダルトビデオ",           # Adult video
    r"チャンネル登録",           # Subscribe
    r"(XRXR){2,}",             # Repeated noise
    r"^(あ|ああ|うう|んん){3,}$", # Moaning sounds
    # ... more patterns
]
```

Segments matching any pattern are filtered out.

#### Stage 5 — Cap Duration

Segments longer than `MAX_SUBTITLE_DURATION` (default 7.0s) -> trim end to `start + 7.0`

#### Stage 6-8 — VAD Gap Detection & Re-transcription

1. **Silero VAD**: `torch.hub.load('snakers4/silero-vad')` -> `detect_voice(audio_path)`
2. **Find gaps**: Compare VAD speech segments with existing subtitles -> find speech NOT covered
3. **Filter**: Only gaps >= 0.5 seconds
4. **Cut audio**: `ffmpeg -ss {start} -i {audio} -t {duration} -c:a pcm_s16le -ar 16000 -ac 1`
5. **Re-transcribe each gap**:
   - Japanese: `kotoba-whisper-v2.0-faster` (less hallucination)
   - Other languages: `large-v3`
   - Options: `vad_filter=False`, `no_speech_threshold=0.5`
6. **Merge** gap results into main subtitle list

#### Stage 9 — Smart Fix

Remove segments where `text.strip()` is empty or <= 1 character

#### Stage 10 — LLM Refine

- **Model**: Gemini (from `GEMINI_MODEL` env, default `gemini-2.0-flash`)
- **Batch size**: 30 lines per API call
- **Prompt**: "Fix transcription errors in these {lang} subtitles... index|text"
- **Fallback**: ถ้า Gemini ล้มเหลว -> keep original text

#### Stage 11 — Generate SRT/VTT

- `fix_segment_overlaps()`: Sort by start time, trim overlapping end times
  - Minimum 1ms gap between segments
  - If trimmed end <= start, set end = start + 0.05s
- SRT format: `HH:MM:SS,mmm --> HH:MM:SS,mmm`
- VTT format: `WEBVTT` header + `HH:MM:SS.mmm --> HH:MM:SS.mmm`

#### Stage 12 — Upload to S3

- Path fix: `rsplit("/", 1)[0]` (ไม่ใช้ `Path.parent` เพราะ Windows backslash bug)
- Upload `{base_path}/{language}.srt`
- Upload `{base_path}/{language}.vtt`

### Completion Output

```json
{
  "status": "completed",
  "job_type": "subtitle_transcribe",
  "entity_id": "<subtitle_uuid>",
  "entity_code": "<video_code>",
  "subtitle_id": "<subtitle_uuid>",
  "output": {
    "srt_path": "subtitles/{code}/ja.srt",
    "vtt_path": "subtitles/{code}/ja.vtt",
    "segments": 234,
    "duration": 3600.5,
    "language": "ja"
  }
}
```

### API on Completed (ProgressBroadcaster)

1. `subtitleService.HandleTranscribeComplete()`:
   - Set `subtitle.srt_path = srt_path`
   - Set `subtitle.status = ready`
   - Update `subtitle.language` if worker confirmed
   - **Critical**: requires `SubtitleID` in progress message, otherwise skips with warning
2. **Auto-trigger next step**:
   - Source `"th"` -> translate to `["en"]`
   - Source anything else -> translate to `["th"]`
   - `subtitleService.TriggerTranslation(videoID, targetLanguages)`

---

## Step 3: Translation

### Trigger

Auto-triggered after transcribe, or manual:
```
POST /api/v1/videos/:id/subtitle/translate
Body: { "target_languages": ["th"] }
```

### Validation (API Side)

1. Original subtitle must exist with `status=ready`
2. Target language must be in supported list
3. Skip already-existing ready translations
4. Creates `Subtitle` DB record per target: `type=translated`, `source_language=ja`, `status=queued`

### NATS Publish

**Subject**: `jobs.subtitle.translate`

```json
{
  "_meta": {
    "job_id": "uuid",
    "job_type": "subtitle_translate",
    "entity_type": "video",
    "entity_id": "<video_uuid>",
    "entity_code": "<video_code>",
    "timeout_sec": 900
  },
  "input": {
    "subtitle_ids": ["<subtitle_uuid_for_th>"],
    "video_id": "<video_uuid>",
    "video_code": "<video_code>",
    "source_srt_path": "subtitles/{code}/ja.srt",
    "source_language": "ja",
    "target_languages": ["th"],
    "output_path": "subtitles/{code}",
    "context": ""
  }
}
```

### Worker Processing (Python `subtitle_translate/handler.py`)

| Step | Stage | Progress | Message | Detail |
|------|-------|----------|---------|--------|
| 1 | `initializing` | 0% | "เริ่มแปลซับไตเติ้ล" | Setup temp dir |
| 2 | `downloading` | 5% | "กำลังดาวน์โหลดซับไตเติ้ลต้นฉบับ" | Download source .srt from S3 |
| 3 | — | — | — | Load speakers.json (optional) |
| 4 | `translating` | 10-80% | "กำลังแปลเป็นภาษา{ชื่อ}" | Per-target Gemini cluster translation |
| 4b | `uploading` | +10% | "กำลังอัพโหลดซับไตเติ้ลภาษา{ชื่อ}" | Upload SRT + VTT per target |
| 5 | completed | 100% | "สำเร็จ" | Cleanup temp |

### Cluster-Based Translation (Detail)

**Why clusters?** แปลทีละบรรทัดจะเสียบริบท แปลทั้งไฟล์ก็ใหญ่เกินไปสำหรับ LLM

1. **Split into clusters**: Group segments by proximity
   - New cluster when gap > 3.0 seconds between segments
   - New cluster when cluster reaches 30 lines
2. **Scene change detection**: If gap > 10 seconds -> reset context summary
3. **Per-cluster translation via Gemini**:
   - Pass `previous_summary` from previous cluster for continuity
   - Include speaker gender/ID if available

**Gemini Prompt Format**:
```
Translate these {source_lang} subtitles to {target_lang}.
Video context: {context}
[Previous context: {previous_summary}]

Rules:
- Natural translation, not literal
- Consider speaker gender
- Output format: index|translated_text
- After translations, provide 1-2 sentence summary

Lines (index|text|gender|speaker):
0|こんにちは|female|SPEAKER_01
1|今日はいい天気ですね|male|SPEAKER_02
...
```

**Response Parsing**:
```
0|สวัสดีค่ะ
1|วันนี้อากาศดีนะครับ
Summary: สองคนทักทายกันและพูดเรื่องอากาศ
```

- Parse `index|text` lines
- Extract `Summary:` section (max 100 chars) -> pass as `previous_summary` to next cluster
- **Fallback**: ถ้า Gemini ล้มเหลว -> return original untranslated text

### Speaker Tagging (Optional)

ถ้ามี `speaker_info_path` -> download `speakers.json`:
```json
{
  "speakers": {"SPEAKER_01": {"gender": "male"}},
  "segments": [{"start": 0.0, "end": 1.5, "speaker": "SPEAKER_01", "gender": "male"}]
}
```

Each subtitle segment gets `speaker_id` and `gender` by time-overlap matching -> passed to Gemini for gendered translation (เช่น ครับ/ค่ะ in Thai)

### Files Uploaded to S3

Per target language:
- `{output_path}/{target_lang}.srt` (e.g., `subtitles/CODE/th.srt`)
- `{output_path}/{target_lang}.vtt` (e.g., `subtitles/CODE/th.vtt`)

### Completion Output

```json
{
  "status": "completed",
  "job_type": "subtitle_translate",
  "entity_id": "<video_uuid>",
  "subtitle_id": "<subtitle_uuid_for_th>",
  "current_language": "th",
  "output": {
    "translations": {"th": "subtitles/{code}/th.srt"},
    "vtt_paths": {"th": "subtitles/{code}/th.vtt"},
    "source_language": "ja",
    "segments_count": 234
  }
}
```

### API on Completed (ProgressBroadcaster)

1. `subtitleService.HandleTranslationComplete()`:
   - Set `subtitle.srt_path = srt_path`
   - Set `subtitle.status = ready`
2. **Chain ends here** — no further auto-trigger

---

## Whisper Models Summary

| Key | Model | Compute Type | word_timestamps | Usage |
|-----|-------|-------------|-----------------|-------|
| `turbo` | `deepdml/faster-whisper-large-v3-turbo-ct2` | int8_float16 | Yes | Main transcription + detect |
| `kotoba` | `kotoba-tech/kotoba-whisper-v2.0-faster` | int8_float16 | **No** (crashes) | Japanese gap re-transcription |
| `large-v3` | `large-v3` | int8_float16 | Yes | Non-Japanese gap re-transcription |

GPU fallback: ถ้า CUDA ล้มเหลว -> auto fallback to CPU with `compute_type="int8"`

---

## Supported Languages

| Code | Language | Flag |
|------|----------|------|
| ja | Japanese | JP |
| en | English | GB |
| th | Thai | TH |
| zh | Chinese | CN |
| ko | Korean | KR |
| ru | Russian | RU |

### Auto Translation Target

| Source Language | Target Language |
|---------------|----------------|
| th | en |
| ja, en, zh, ko, ru | th |

---

## S3 File Paths

| File | S3 Path | Created By |
|------|---------|------------|
| Original audio | `audio/{code}/audio.wav` | Go Transcode Worker (step 20) |
| Original SRT | `subtitles/{code}/{lang}.srt` | Python Transcribe Worker |
| Original VTT | `subtitles/{code}/{lang}.vtt` | Python Transcribe Worker |
| Translated SRT | `subtitles/{code}/{target_lang}.srt` | Python Translate Worker |
| Translated VTT | `subtitles/{code}/{target_lang}.vtt` | Python Translate Worker |

---

## Error Handling & Fallbacks

| Stage | Error | Behavior |
|-------|-------|----------|
| Demucs | Timeout (>10min) or crash | Silent fallback to original audio |
| Whisper | CUDA OOM | Auto fallback to CPU |
| VAD gaps | Any error | Skip gaps, use original subtitles |
| Gemini refine | API error | Keep original text |
| Gemini translate | API error | Return original untranslated text |
| S3 download | 503/500/429 | 3 retries with 10s/20s wait |
| Worker crash | Any | `msg.nak(delay=30)` -> retry after 30s (max 3 times) |

### Error Codes Published

| Code | Stage |
|------|-------|
| `SUBTITLE_DETECT_FAILED` | Detection |
| `SUBTITLE_TRANSCRIBE_FAILED` | Transcription |
| `SUBTITLE_TRANSLATE_FAILED` | Translation |

---

## Full Chain Timeline (Typical Video)

```
00:00  User clicks "Detect"
00:00  API publishes detect job -> NATS
00:05  Detect worker downloads audio (2-3 sec)
00:10  Whisper detects language: ja (confidence 0.97)
00:12  Detect completed -> API auto-triggers Transcribe
00:12  API creates Subtitle record, publishes transcribe job
00:15  Transcribe worker downloads audio
00:25  Demucs voice separation (30-120 sec)
02:00  Whisper transcription (1-5 min for 30min video)
04:00  VAD gap detection + re-transcription (1-2 min)
05:00  LLM refine (30 sec - 2 min)
05:30  Generate SRT/VTT + upload to S3
05:45  Transcribe completed -> API auto-triggers Translate
05:45  API creates Subtitle record for "th", publishes translate job
05:50  Translate worker downloads ja.srt
06:00  Cluster translation via Gemini (30 sec - 3 min)
07:00  Upload th.srt + th.vtt to S3
07:10  Translate completed -> chain ends
```

Total: ~7-10 minutes for a 30-minute video (varies by length and GPU speed)

---

## Known Issues / TODO

1. **NATS reconnect -> duplicate job**: Worker completes job but ACK fails due to reconnect -> job re-delivered -> processed again
2. **SubtitleID missing in progress**: If Python worker doesn't send `subtitle_id`, `handleTranscribeCompleted()` skips DB update
3. **Gemini model inconsistency**: `config.py` default = `gemini-2.0-flash`, `gemini_adapter.py` fallback = `gemini-2.5-flash`
4. **Config attrs not in dataclass**: `cluster_size` and `cluster_gap_sec` read via `getattr(config, ..., default)` but not defined in Config -> always use defaults (30, 3.0)
5. **No batch subtitle status view**: No UI page to see subtitle status across all videos
6. **Demucs timeout**: Some videos take >10 min -> fallback to original (lower quality subtitles)
7. **HTTP callbacks still in code**: Legacy `POST /internal/subtitles/:id/callback/*` endpoints exist alongside NATS progress — could cause confusion

---

## Environment Variables (Subtitle Workers)

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `NATS_URL` | `nats://localhost:4222` | Yes | NATS server |
| `S3_ENDPOINT` | - | Yes | IDrive E2 endpoint |
| `S3_ACCESS_KEY` | - | Yes | S3 credentials |
| `S3_SECRET_KEY` | - | Yes | S3 credentials |
| `S3_BUCKET` | `suekk` | Yes | Bucket name |
| `S3_REGION` | `us-east-1` | No | S3 region |
| `GEMINI_API_KEY` | - | Yes (transcribe/translate) | Google Gemini API key |
| `GEMINI_MODEL` | `gemini-2.0-flash` | No | LLM model |
| `WHISPER_MODEL` | `turbo` | No | Main STT model |
| `WHISPER_DEVICE` | `cuda` | No | GPU/CPU |
| `WHISPER_COMPUTE_TYPE` | `int8_float16` | No | Compute precision |
| `GAP_MODEL` | `kotoba` | No | Japanese gap model |
| `GAP_MODEL_FALLBACK` | `large-v3` | No | Non-Japanese gap model |
| `USE_DEMUCS` | `true` | No | Voice separation toggle |
| `USE_VAD` | `true` | No | VAD gap detection toggle |
| `USE_REFINE` | `true` | No | LLM refine toggle |
| `MAX_SUBTITLE_DURATION` | `7.0` | No | Max segment length (seconds) |
| `MIN_GAP_DURATION` | `0.5` | No | Min gap to re-transcribe |
| `TEMP_DIR` | `/tmp/subtitle` | No | Temp file directory |

---

## Starting Workers

```bash
# Windows: start all 3 workers in separate windows
cd _my_worker
start_subtitle.bat

# Or manually:
cd _my_worker/python/subtitle_detect && python main.py
cd _my_worker/python/subtitle_transcribe && python main.py
cd _my_worker/python/subtitle_translate && python main.py
```

Workers connect to existing NATS streams/consumers (created by API on startup). ไม่ต้อง start API ก่อนก็ได้ แต่ streams ต้องมีอยู่แล้ว
