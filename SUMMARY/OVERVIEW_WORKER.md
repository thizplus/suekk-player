# SUEKK Stream — Worker System Overview

> Video Transcoding (Go + FFmpeg + GPU) + Subtitle Processing (Python + Whisper + Gemini)
> Paths: `_worker/` (production Go worker), `_my_worker/` (refactored workers)

---

## Architecture

```
                    NATS JetStream
                         |
         ┌───────────────┼───────────────────┐
         |               |                   |
   ┌─────┴─────┐  ┌─────┴──────┐  ┌────────┴────────┐
   │ Go Worker  │  │ Python     │  │ Python           │
   │ Transcode  │  │ Subtitle   │  │ Subtitle         │
   │ + Gallery  │  │ Detect     │  │ Transcribe       │
   │ + WarmCache│  │            │  │ + Translate       │
   └─────┬──────┘  └─────┬──────┘  └────────┬─────────┘
         |               |                   |
         └───────────────┼───────────────────┘
                         |
                    Host Machine (GPU)
```

- **Go Worker** (`_worker/`): Production transcode + gallery, runs on host for GPU access (NVENC)
- **Python Workers** (`_my_worker/python/`): Subtitle detect/transcribe/translate, uses CUDA for Whisper

---

## Two Worker Generations

| Feature | `_worker/` (Production) | `_my_worker/` (Refactored) |
|---------|------------------------|---------------------------|
| Language | Go | Go (transcode) + Python (subtitle) |
| Architecture | Clean arch + DI container | Separate binary per worker type |
| Job format | Direct struct fields | `{ "_meta": {...}, "input": {...} }` |
| Progress fields | `video_id`, `video_code` (legacy) | `entity_id`, `entity_code` (standard) |
| Status | Active production | Python subtitle workers active, Go workers in development |

---

## Go Worker (`_worker/`)

### Project Structure

```
_worker/
├── cmd/worker/main.go               # Entry point
├── config/config.go                  # Configuration
├── container/container.go            # DI container wiring
├── domain/
│   ├── constants/nats.go             # NATS stream/subject names
│   ├── models/job.go                 # TranscodeJob, GalleryJob structs
│   ├── models/quality.go             # Quality presets (1080p/720p/480p/360p)
│   └── errors/errors.go             # TranscodeError (retryable/permanent)
├── infrastructure/
│   ├── transcoder/ffmpeg_client.go   # FFmpeg + NVENC
│   ├── consumer/nats_consumer.go     # JetStream pull consumer
│   ├── consumer/gallery_consumer.go  # Gallery consumer
│   ├── messenger/nats_publisher.go   # Progress publisher
│   ├── storage/s3_client.go          # S3 via minio-go
│   ├── uploader/segment_watcher.go   # Parallel HLS segment upload
│   ├── gallery/service.go           # Gallery generation
│   ├── classifier/nsfw_classifier.go # NSFW image classification
│   ├── monitor/disk_monitor.go       # Disk usage monitoring
│   └── auth/auth_client.go           # API auth for internal calls
├── services/
│   ├── transcode_service.go          # Transcode orchestration
│   └── audio_service.go              # Audio extraction
├── use_cases/
│   ├── transcode_handler.go          # Main transcode use case (22 steps)
│   └── gallery_handler.go            # Gallery from HLS use case
└── cmd/                              # 20+ utility commands
    ├── backfill-transcode/
    ├── retry-failed/
    └── ...
```

### Entry Point & Lifecycle

1. Load `.env` via godotenv
2. Initialize slog logging (JSON + file + stdout)
3. Load config + create DI container
4. Start background services: disk monitor, heartbeat, cleanup
5. Start gallery consumer + main transcode consumer
6. Listen for SIGINT/SIGTERM -> graceful shutdown

**Worker ID** resolution: `WORKER_ID` env > `VAST_CONTAINERLABEL` > `HOSTNAME` + random hex

### DI Container Wiring Order

1. NATS connection (MaxReconnects=-1, ReconnectWait=2s)
2. PostgreSQL (direct sql.Open for status queries)
3. S3 storage (minio-go)
4. FFmpeg transcoder
5. NATS messenger (progress publisher)
6. WarmCache publisher + Subtitle publisher
7. PostgreSQL repository
8. TempManager (optional RAM disk)
9. Alert service (Discord/LINE webhook)
10. Disk monitor + Heartbeat publisher
11. JetStream consumers
12. Gallery service + Auth client
13. TranscodeHandler + GalleryHandler

---

### Video Transcode Flow (22 Steps)

```
1.  Idempotency check     - skip if status == ready
2.  Pre-transcode check   - cleanup zombie FFmpeg, check VRAM (>1000MB)
3.  Retry cleanup          - delete old S3 files + local temp (if retry)
4.  Update DB              - status -> processing
5.  Mark in-progress       - prevent TempManager cleanup
6.  Progress 0%            - "Starting..."
7.  Download source        - S3 -> local (progress 2-10%)
8.  Analyze video          - ffprobe: duration, dimensions, fps, bitrate, codec
9.  Select upload config   - turbo/balanced/stable based on duration
10. Start SegmentWatcher   - fsnotify watches output dir, uploads .ts as they appear
11. FFmpeg transcode       - single-pass multi-quality HLS (progress 12-87%)
12. Generate screenshots   - 10 per quality from local HLS
13. Generate thumbnail     - at 25% duration
14. Wait segment uploads   - flush buffer, segmentWatcher.Wait()
15. Upload playlists       - playlist.m3u8 per quality + master.m3u8 (uploaded last)
16. Cleanup local segments
17. Publish warm cache job - to WARM_CACHE_JOBS stream
18. Generate gallery       - extract frames + NSFW classification (3-tier)
19. Update DB              - status=ready, hls_path, thumbnail, disk_usage, quality_sizes
20. Extract audio          - WAV 16kHz mono -> upload to audio/{code}/audio.wav
21. Publish completion     - to NATS progress subject
22. Auto-trigger subtitle  - POST to API /videos/{id}/subtitle/transcribe (if enabled)
23. Delete original file   - only if audio extraction succeeded
```

### FFmpeg Command (Single-Pass Multi-Quality HLS)

```
ffmpeg -y
  -i {input}
  -filter_complex "[0:v]split=N[v0][v1]...; [v0]scale=1920:1080[v1080p]; ..."
  -map [v1080p] -map 0:a?
  -c:v h264_nvenc -preset p4 -rc vbr -cq 25 -rc-lookahead 20 -spatial-aq 1
  -g {gop} -keyint_min {gop} -maxrate {max} -bufsize {2x_max}
  -c:a aac (or copy if already aac)
  ... (repeat per quality)
  -f hls -hls_time 2 -hls_playlist_type vod
  -hls_flags independent_segments+temp_file
  -master_pl_name master.m3u8
  -var_stream_map "v:0,a:0,name:1080p v:1,a:1,name:720p ..."
  {outputDir}/%v/playlist.m3u8
```

**GPU mode**: `h264_nvenc`, preset `p4` (RTX 3060 Ti optimized)
**CPU mode**: `libx264`, preset `medium`, CRF 25
**Progress parsing**: `bufio.Reader.Read()` + regex `time=HH:MM:SS.cs`

### Quality Presets (Filtered by Source Resolution)

| Quality | Resolution | Video Bitrate | Audio |
|---------|-----------|--------------|-------|
| 1080p | 1920x1080 | ~4 Mbps | 192k |
| 720p | 1280x720 | ~2.5 Mbps | 128k |
| 480p | 854x480 | ~1 Mbps | 96k |
| 360p | 640x360 | ~600 kbps | 96k |

If source is 720p, only 720p and below are generated.

### SegmentWatcher (Parallel Upload)

- `fsnotify` watches output dir for `.ts` segments
- Upload queue: N goroutines (adaptive: 15-40 workers based on video duration)
- Rate limiter: token bucket (30-50 req/s)
- Backpressure: maxUncommitted limit, backs off up to 30s
- Error threshold: > 10 errors OR > 5% error rate -> job fails
- Playlists uploaded explicitly after all segments confirmed

### Adaptive Upload Config

| Profile | Duration | Workers | Max Uncommitted | Rate Limit |
|---------|----------|---------|-----------------|------------|
| Turbo | < 30 min | 40 | 400 | 50 req/s |
| Balanced | 30-90 min | 25 | 250 | 40 req/s |
| Stable | > 90 min | 15 | 150 | 30 req/s |

### NATS Consumer

- **Pull-based** JetStream consumer: `Fetch(1, MaxWait=30s)`
- `MaxDeliver: 3`, `AckWait: 30min`, `MaxAckPending: 10`
- InProgress heartbeat: every 5 min to prevent timeout on long jobs
- ACK strategy:
  - Success -> `msg.Ack()`
  - Retryable error + deliveries < 3 -> `msg.Nak()`
  - Permanent error or deliveries >= 3 -> DLQ + `msg.Term()`
- Pause/Resume: disk monitor can pause consumer on disk-full

### Gallery Generation (3-Tier NSFW Classification)

**Phase 1** (minutes 0-10): 10 frames/min -> classify -> keep `super_safe` + `safe`
**Phase 2** (minutes 10-30): 10 frames/min -> classify -> keep `nsfw` only
Upload to: `gallery/{code}/super_safe/`, `gallery/{code}/safe/`, `gallery/{code}/nsfw/`

**NSFW Classifier**: Python subprocess `classify_batch.py`
Thresholds: `NsfwThreshold=0.3`, `SuperSafeThreshold=0.15`
Max images: 20 NSFW + 10 safe

### Error Handling

| Type | Examples | NATS Action |
|------|----------|-------------|
| **Retryable** | Download fail (non-404), upload fail, OOM, no space | `msg.Nak()` |
| **Permanent** | 404 source file, corrupt video, max retries | DLQ + `msg.Term()` |

---

## Python Subtitle Workers (`_my_worker/python/`)

### Structure

```
_my_worker/python/
├── shared/                          # Common code
│   ├── config.py                    # Config loader (.env)
│   ├── nats_consumer.py             # NATS JetStream pull consumer
│   ├── progress.py                  # Progress publisher (throttled)
│   ├── storage.py                   # S3 client (boto3)
│   └── adapters/
│       ├── whisper_adapter.py       # faster-whisper (turbo/large-v3/kotoba)
│       ├── gemini_adapter.py        # Google Gemini LLM
│       ├── audio_adapter.py         # Demucs + FFmpeg
│       ├── vad_adapter.py           # Silero VAD
│       └── entities.py              # SubtitleLine, LanguageCode
├── subtitle_detect/
│   ├── main.py                      # Entry point
│   └── handler.py                   # Language detection handler
├── subtitle_transcribe/
│   ├── main.py                      # Entry point
│   └── handler.py                   # Transcription handler (14 stages)
└── subtitle_translate/
    ├── main.py                      # Entry point
    └── handler.py                   # Translation handler
```

### Common Infrastructure

**NATS Consumer** (`shared/nats_consumer.py`):
- Pull-based, 1 message at a time, 5s timeout
- InProgress heartbeat every 30s
- Retry: `msg.nak(delay=30)` for retryable, `msg.term()` after 3 deliveries
- Job format: `{ "_meta": {...}, "input": {...} }`

**Progress Publisher** (`shared/progress.py`):
- Publishes to `progress.{entity_type}.{entity_id}`
- Throttled: >= 5% change OR >= 2s elapsed
- Terminal states (completed/failed) bypass throttle

**S3 Storage** (`shared/storage.py`):
- boto3 with adaptive retry (max 10 attempts)
- Manual retry for 503/500/429 (3 attempts, exponential wait)
- Auto-detect content-type from extension

---

### Language Detection Worker

**Input**: `audio/{code}/audio.wav` from S3
**Model**: `faster-whisper turbo` (detection mode only)
**Output**: `{ "language": "ja", "confidence": 0.95 }`
**Fallback**: If detection fails -> `("ja", 0.0)`

### Transcription Worker (14-Stage Pipeline)

| Stage | Progress | Description |
|-------|----------|-------------|
| 1. Initialize | 0% | Setup temp dir |
| 2. Download audio | 5% | S3 -> local `audio.wav` |
| 3. Demucs | 10% | Voice separation (`htdemucs`, `--two-stems vocals`), 10min timeout, fallback to original |
| 4. Whisper transcribe | 20-50% | `faster-whisper turbo`, VAD filter, hallucination filtering |
| 5. Cap duration | 55% | Trim subtitles > 7s |
| 6-8. VAD gaps | 60-75% | Silero VAD -> find gaps -> cut segments -> re-transcribe with `kotoba-whisper` (ja) or `large-v3` (other) |
| 9. Smart fix | 78% | Filter empty/noise segments |
| 10. LLM refine | 82% | Gemini batch refine (30 lines/call) |
| 11. Generate SRT/VTT | 88% | Fix overlaps, generate files |
| 12. Upload | 95% | Upload `{lang}.srt` + `{lang}.vtt` to `subtitles/{code}/` |
| 13. Complete | 100% | Output: `{srt_path, vtt_path, segments, duration, language}` |

**Whisper Models**:

| Name | Model | Compute | Word Timestamps |
|------|-------|---------|-----------------|
| turbo | `deepdml/faster-whisper-large-v3-turbo-ct2` | int8_float16 | Yes |
| kotoba | `kotoba-tech/kotoba-whisper-v2.0-faster` | int8_float16 | No (crashes) |
| large-v3 | standard large-v3 | int8_float16 | Yes |

**Hallucination Filter**: Regex patterns for common Whisper hallucinations
- `ご視聴.*ありがとう`, `アダルトビデオ`, `チャンネル登録`, `(XRXR){2,}`, etc.

### Translation Worker

**Input**: Source `.srt` from S3
**Method**: Cluster-based translation via Gemini

1. Group nearby segments (gap < 3s, max 30 lines/cluster)
2. Translate each cluster with context summary from previous cluster
3. Scene change detection: gap > 10s resets context
4. Prompt format: `index|text|gender|speaker_id` -> returns `index|translated_text` + `Summary: [context]`

**Supported languages**: ja, en, zh, ko, th
**Output**: `{ "translations": {"th": "path"}, "vtt_paths": {...}, "source_language": "ja" }`
**Fallback**: On Gemini error, keep original text

### Gemini Adapter

- Library: `google.generativeai`
- Model: from `GEMINI_MODEL` env (default: `gemini-2.5-flash`)
- Functions: `translate_batch()`, `refine_batch()`, `translate_cluster()`
- Batch size: 30 lines per API call
- Fallback: return original text on error

---

## Gallery from HLS (GalleryHandler)

For retroactive gallery generation from existing HLS:

1. Download `playlist.m3u8` from S3
2. Parse `#EXTINF:` durations to build segment timeline
3. For each target timestamp: find segment -> get presigned URL -> FFmpeg extract 1 frame
4. Upload to `gallery/{code}/` folders
5. 30s timeout per frame extraction

---

## Configuration (Environment Variables)

| Variable | Default | Purpose |
|----------|---------|---------|
| `NATS_URL` | `nats://localhost:4222` | NATS server |
| `S3_ENDPOINT` | - | IDrive E2 S3-compatible |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | - | S3 credentials |
| `S3_BUCKET` | `suekk` | Storage bucket |
| `USE_GPU` | `true` | NVENC vs libx264 |
| `FFMPEG_PRESET` | `p4` | NVENC quality preset |
| `HLS_TIME` | `2` | Segment duration (seconds) |
| `WORKER_CONCURRENCY` | `1` | Parallel jobs (GPU constraint) |
| `WHISPER_MODEL` | `turbo` | Main STT model |
| `WHISPER_DEVICE` | `cuda` | GPU/CPU for Whisper |
| `GEMINI_API_KEY` | - | For transcribe/translate |
| `GEMINI_MODEL` | `gemini-2.0-flash` | LLM model |
| `USE_DEMUCS` | `true` | Voice separation |
| `USE_VAD` | `true` | VAD gap detection |
| `USE_REFINE` | `true` | LLM refinement step |
| `AUTO_SUBTITLE_ENABLED` | `false` | Auto-trigger subtitle after transcode |
| `DISK_MONITOR_ENABLED` | `true` | Pause on disk-full |
| `TEMP_PATH` | `/tmp/worker` | Temp files directory |

---

## Start Scripts

**`_my_worker/start_subtitle.bat`** (Windows):
```bat
start "Subtitle Detect" cmd /k "cd /d %~dp0python\subtitle_detect && python main.py"
start "Subtitle Transcribe" cmd /k "cd /d %~dp0python\subtitle_transcribe && python main.py"
start "Subtitle Translate" cmd /k "cd /d %~dp0python\subtitle_translate && python main.py"
```

Opens 3 separate CMD windows, one per worker.

---

## NATS Streams Used by Workers

| Stream | Subject | Consumer | Purpose |
|--------|---------|----------|---------|
| `TRANSCODE_JOBS` | `jobs.transcode` | `WORKER` | Video transcode |
| `TRANSCODE_DLQ` | `dlq.transcode` | - | Dead letter queue |
| `SUBTITLE_JOBS` | `jobs.subtitle.detect` | `SUBTITLE_DETECT_WORKER` | Language detection |
| `SUBTITLE_JOBS` | `jobs.subtitle.transcribe` | `SUBTITLE_TRANSCRIBE_WORKER` | Transcription |
| `SUBTITLE_JOBS` | `jobs.subtitle.translate` | `SUBTITLE_TRANSLATE_WORKER` | Translation |
| `WARM_CACHE_JOBS` | `jobs.warmcache` | `WARM_CACHE_WORKER` | CDN cache warming |
| `GALLERY_JOBS` | `jobs.gallery.generate` | `GALLERY_WORKER` | Gallery generation |
| (Pub/Sub) | `progress.{entity_id}` | - | Real-time progress |

---

## Important Notes

1. **Worker runs on Host** (not Docker) because it needs GPU access (NVENC + CUDA)
2. **Kill old worker before starting new one**: `taskkill /F /IM worker.exe` (ghost worker bug)
3. **NATS streams are created by API**, workers only connect to existing consumers
4. **Concurrency = 1** by default to avoid VRAM exhaustion
5. **Demucs timeout = 10 min**: Falls back to original audio if voice separation takes too long
