# SUEKK Stream — NATS Message Queue Overview

> NATS JetStream เป็นหัวใจของระบบ ใช้สำหรับ Job Queue (transcode, subtitle, gallery, warmcache, reel)
> และ Pub/Sub สำหรับ real-time progress updates

---

## Architecture Overview

```
                        ┌──────────────────────────────┐
                        │       NATS Server             │
                        │  JetStream + Pub/Sub + KV     │
                        │  Port 4222 (client)           │
                        │  Port 8222 (monitoring)       │
                        └──────────┬───────────────────┘
                                   │
          ┌────────────────────────┼──────────────────────────┐
          │                        │                          │
    ┌─────┴──────┐          ┌──────┴──────┐           ┌──────┴──────┐
    │  API Server │          │  Go Worker  │           │Python Workers│
    │  (Publisher) │          │ (Consumer)  │           │ (Consumer)  │
    │             │          │             │           │             │
    │ - Publish   │  ──JS──> │ - Transcode │           │ - Detect    │
    │   jobs      │          │ - Gallery   │           │ - Transcribe│
    │ - Subscribe │  <─PS──  │             │           │ - Translate │
    │   progress  │          │ - Publish   │  ──JS──>  │             │
    │ - Broadcast │          │   warmcache │           │ - Publish   │
    │   WebSocket │          │   progress  │  <─PS──   │   progress  │
    └─────────────┘          └─────────────┘           └─────────────┘

JS = JetStream (persistent queue, pull-based)
PS = Pub/Sub (ephemeral, real-time)
```

---

## JetStream Streams

ทุก Stream ใช้: `FileStorage`, `WorkQueuePolicy` (ลบหลัง ACK), `Replicas: 1`

| Stream | Subjects | MaxAge | สร้างโดย | Purpose |
|--------|---------|--------|---------|---------|
| `TRANSCODE_JOBS` | `jobs.transcode` | 24h | API + Worker | Video transcoding queue |
| `SUBTITLE_JOBS` | `jobs.subtitle.detect`, `jobs.subtitle.transcribe`, `jobs.subtitle.translate` | 24h (API) / 7d (Worker) | API + Worker | Subtitle pipeline queue |
| `GALLERY_JOBS` | `jobs.gallery.generate` | 24h | API + Worker | Gallery frame extraction |
| `WARM_CACHE_JOBS` | `jobs.warmcache` | 24h | API | CDN cache warming |
| `REEL_JOBS` | `jobs.reel.export` | 24h | API | Reel MP4 export |
| `TRANSCODE_DLQ` | `dlq.transcode` | 30d | Worker | Dead letter queue (LimitsPolicy) |

---

## Consumers (Pull-Based)

ทุก Consumer ใช้: `AckExplicitPolicy`, `MaxAckPending` ตามตาราง

| Consumer Name | Stream | Filter Subject | MaxDeliver | AckWait | MaxAckPending | สร้างโดย |
|---------------|--------|---------------|-----------|---------|--------------|---------|
| `TRANSCODE_WORKER` | `TRANSCODE_JOBS` | `jobs.transcode` | 3 | 30 min | 10 | Worker |
| `GALLERY_WORKER` | `GALLERY_JOBS` | `jobs.gallery.generate` | 3 | 10 min | 1 | Worker |
| `SUBTITLE_DETECT_WORKER` | `SUBTITLE_JOBS` | `jobs.subtitle.detect` | 5 | 5 min | 1000 | API |
| `SUBTITLE_TRANSCRIBE_WORKER` | `SUBTITLE_JOBS` | `jobs.subtitle.transcribe` | 5 | 5 min | 1000 | API |
| `SUBTITLE_TRANSLATE_WORKER` | `SUBTITLE_JOBS` | `jobs.subtitle.translate` | 5 | 5 min | 1000 | API |
| `WARM_CACHE_WORKER` | `WARM_CACHE_JOBS` | `jobs.warmcache` | 5 | 5 min | 1000 | API |
| `REEL_WORKER` | `REEL_JOBS` | `jobs.reel.export` | 5 | 5 min | 1000 | API |
| `DLQ_NOTIFIER` | `TRANSCODE_DLQ` | `dlq.transcode` | 1 | - | - | API |

---

## KV Bucket

| Bucket | Storage | TTL | Purpose |
|--------|---------|-----|---------|
| `WORKER_STATUS` | Memory | 30s | Worker heartbeat (hostname, GPU, disk, status, current job) |

Key pattern: `worker.{workerID}` — Worker เขียนทุก 5 วินาที, API อ่านสำหรับ Worker Monitoring page

---

## Pub/Sub Subjects (Non-JetStream, Ephemeral)

ใช้สำหรับ real-time progress updates — ไม่มี persistence, ถ้า API ไม่ได้ subscribe ก็หายไป

| Subject Pattern | Publisher | Description |
|----------------|-----------|-------------|
| `progress.{videoID}` | Go Worker (transcode) | Transcode progress |
| `progress.subtitle.{videoID}` | Python Workers | Subtitle progress |
| `progress.reel.{reelID}` | Reel Worker | Reel export progress |
| `progress.gallery.{videoID}` | Go Worker | Gallery progress |
| `progress.warmcache.{videoID}` | WarmCache Worker | Cache warming progress |

API subscribes ด้วย wildcard: `progress.>` (จับทุก subject ที่ขึ้นต้นด้วย `progress.`)

---

## Standard Job Message Format

ทุก job ที่ API publish ไป NATS ใช้ format เดียวกัน:

```json
{
  "_meta": {
    "job_id": "uuid-of-worker-job-record",
    "job_type": "transcode | gallery | warmcache | subtitle_detect | subtitle_transcribe | subtitle_translate | reel",
    "entity_type": "video | subtitle | reel",
    "entity_id": "uuid",
    "entity_code": "video-code-string",
    "priority": 1,
    "retry_count": 0,
    "max_retries": 3,
    "timeout_sec": 3600,
    "created_at": 1234567890
  },
  "input": { ... }
}
```

### input per Job Type

**Transcode** (`jobs.transcode`):
```json
{
  "input_path": "videos/{code}/original.mp4",
  "output_path": "hls/{code}/",
  "codec": "h264",
  "qualities": ["1080p", "720p", "480p"],
  "use_byte_range": false
}
```

**Subtitle Detect** (`jobs.subtitle.detect`):
```json
{
  "audio_path": "audio/{code}/audio.wav"
}
```

**Subtitle Transcribe** (`jobs.subtitle.transcribe`):
```json
{
  "video_id": "uuid",
  "video_code": "code",
  "audio_path": "audio/{code}/audio.wav",
  "language": "ja",
  "output_path": "subtitles/{code}/ja.srt",
  "refine_with_llm": true,
  "context": ""
}
```

**Subtitle Translate** (`jobs.subtitle.translate`):
```json
{
  "subtitle_ids": ["uuid1"],
  "video_id": "uuid",
  "video_code": "code",
  "source_srt_path": "subtitles/{code}/ja.srt",
  "source_language": "ja",
  "target_languages": ["th"],
  "output_path": "subtitles/{code}",
  "context": ""
}
```

**Gallery** (`jobs.gallery.generate`):
```json
{
  "hls_path": "hls/{code}/1080p/playlist.m3u8",
  "video_quality": "1080p",
  "duration": 3600,
  "output_path": "gallery/{code}/",
  "image_count": 100
}
```

**Warm Cache** (`jobs.warmcache`):
```json
{
  "hls_path": "hls/{code}/",
  "segment_counts": {"1080p": 150, "720p": 150},
  "priority": 1
}
```

**Reel** (`jobs.reel.export`):
```json
{
  "reel_id": "uuid",
  "video_code": "code",
  "segments": [...],
  "style": "letterbox",
  "line1": "...",
  "line2": "...",
  "tts_text": "...",
  "tts_voice": "...",
  "show_logo": true
}
```

---

## Progress Update Format (Worker -> API)

Workers publish JSON to `progress.*`:

```json
{
  "job_id": "uuid",
  "job_type": "subtitle_detect | subtitle_transcribe | subtitle_translate | ...",
  "entity_type": "video | subtitle | reel",
  "entity_id": "uuid",
  "entity_code": "code",
  "worker_id": "worker-hostname-abc123",
  "status": "processing | completed | failed",
  "progress": 42.5,
  "stage": "downloading | transcribing | translating | ...",
  "message": "กำลังถอดเสียง...",
  "updated_at": "2026-06-05T10:00:00Z",
  "error": "error message (if failed)",
  "error_details": {
    "code": "SUBTITLE_TRANSCRIBE_FAILED",
    "stage": "transcribing",
    "is_retryable": true
  },
  "output": { ... }
}
```

### output per Completion Type

**Transcode completed**:
```json
{
  "hls_path": "hls/{code}/",
  "thumbnail_path": "hls/{code}/thumbnail.jpg",
  "audio_path": "audio/{code}/audio.wav",
  "duration": 3600,
  "disk_usage": 2147483648,
  "quality_sizes": {"1080p": 1000000, "720p": 500000},
  "segment_counts": {"1080p": 150}
}
```

**Detect completed**:
```json
{
  "language": "ja",
  "confidence": 0.97
}
```

**Transcribe completed**:
```json
{
  "srt_path": "subtitles/{code}/ja.srt",
  "vtt_path": "subtitles/{code}/ja.vtt",
  "segments": 234,
  "duration": 3600.5,
  "language": "ja"
}
```

**Translate completed**:
```json
{
  "translations": {"th": "subtitles/{code}/th.srt"},
  "vtt_paths": {"th": "subtitles/{code}/th.vtt"},
  "source_language": "ja",
  "segments_count": 234
}
```

---

## Progress Throttling (Python Workers)

Python workers ใช้ `ProgressThrottler` เพื่อไม่ให้ส่ง progress ถี่เกินไป:

- ส่งเสมอถ้า progress == 0 (first update)
- ส่งถ้าเวลาผ่านไป >= 2 วินาที
- ส่งถ้า progress ข้าม threshold 5% (เช่น 10% -> 15%)
- `completed` และ `failed` ส่งทันทีเสมอ (bypass throttle)

---

## Dual Write Pattern

ทุกครั้งที่ API publish job ไป NATS จะทำ 2 อย่างพร้อมกัน:

```
1. สร้าง WorkerJob record ใน DB (status=pending) -> ได้ job_id
2. Publish ไป NATS โดยใส่ job_id ใน _meta
3. ถ้า publish สำเร็จ -> mark WorkerJob เป็น queued
4. ถ้า publish ล้มเหลว -> mark WorkerJob เป็น failed
```

เมื่อ Worker ส่ง progress กลับมา -> `ProgressBroadcaster` update `WorkerJob` record:
- `processing` -> `MarkAsStarted(workerID)` + `UpdateProgress(progress, stage, message)`
- `completed` -> `MarkAsCompleted(outputJSON)`
- `failed` -> `MarkAsFailed(error, errorCode, stage)`

ทำให้สามารถ track สถานะ job ทั้งหมดผ่าน DB ได้ ไม่ต้องพึ่ง NATS อย่างเดียว

---

## ACK/NAK/Term Strategy

### Go Worker (Transcode + Gallery)

| Situation | Action | ผลลัพธ์ |
|-----------|--------|---------|
| Success | `msg.Ack()` | Job removed from queue |
| Retryable error + deliveries < 3 | `msg.Nak()` | Re-queue for retry |
| Permanent error (404, corrupt file) | `msg.Term()` + publish to DLQ | Job terminated |
| Max deliveries reached (>= 3) | `msg.Term()` + publish to DLQ | Job terminated |
| JSON unmarshal error | `msg.Term()` | Job terminated (no DLQ) |

### Python Workers (Subtitle)

| Situation | Action | ผลลัพธ์ |
|-----------|--------|---------|
| Success | `msg.ack()` | Job removed from queue |
| Handler exception + deliveries < 3 | `msg.nak(delay=30)` | Retry after 30s |
| Handler exception + deliveries >= 3 | `msg.term()` | Job terminated |
| JSON decode error | `msg.term()` | Job terminated |

### In-Progress Heartbeat

ทั้ง Go และ Python workers ส่ง `msg.InProgress()` เป็นระยะเพื่อป้องกัน AckWait timeout:
- Go Worker: ทุก 5 นาที
- Python Workers: ทุก 30 วินาที

---

## DLQ (Dead Letter Queue)

เฉพาะ Transcode jobs เท่านั้นที่มี DLQ:

```
Worker: msg exceeds MaxDeliver (3 ครั้ง)
  -> publish DLQJob to "dlq.transcode" (JetStream)
  -> msg.Term()

API: DLQSubscriber listens on "dlq.transcode"
  -> sends Telegram alert
  -> msg.Ack()
```

**DLQ Message Format**:
```json
{
  "original_job": { "video_id": "...", "video_code": "...", "input_path": "...", ... },
  "error": "error message",
  "attempts": 3,
  "worker_id": "worker-id",
  "failed_at": 1234567890,
  "stage": "download | presign | transcode | upload | watcher"
}
```

Subtitle/Gallery/WarmCache/Reel ไม่มี DLQ — ใช้ `WorkerJob` DB record tracking แทน

---

## Progress Data Flow (API Side)

```
NATS progress.> subscription
        |
  nats/subscriber.go: handleMessage()
    - Unmarshal JSON -> ProgressUpdate
    - Normalize entity_id -> video_id (backward compat)
    - Parse raw "output" field for completed jobs
        |
  messaging/nats_progress_sub.go: natsHandler()
    - Convert ProgressUpdate -> ports.ProgressData
    - Extract output fields (duration, disk_usage, srt_path, etc.)
        |
  websocket/progress_broadcaster.go: handleProgressUpdate()
    - Route to handler by type:
      |-- isSubtitleProgress? -> handleSubtitleProgress()
      |-- Quality == "gallery"? -> handleGalleryProgress()
      |-- Quality == "warmcache"? -> handleWarmCacheProgress()
      |-- ReelID != ""? -> handleReelProgress()
      |-- else -> transcode progress
    - Update WorkerJob DB record (Dual Write)
    - Broadcast WebSocket message to all clients
    - On completed/failed: trigger next step (subtitle chain)
```

### Type Detection Logic

| Condition | Detected Type |
|-----------|--------------|
| `JobType` in `[subtitle_detect, subtitle_transcribe, subtitle_translate]` | Subtitle |
| `SubtitleID != ""` | Subtitle |
| `Stage` in `[detecting, transcribing, translating, vad, fixing, refining, generating]` | Subtitle |
| `Quality == "gallery"` | Gallery |
| `Quality == "warmcache"` | WarmCache |
| `ReelID != ""` | Reel |
| else | Transcode |

---

## Video Status Protection

เมื่อ video มีสถานะ `ready` แล้ว จะ **ไม่ถูก revert เป็น failed** แม้ subtitle/gallery job จะล้มเหลว:

```go
// progress_broadcaster.go
if update.Status == "failed" && currentVideo.Status == "ready" {
    // SKIP — do NOT update video status
    // Only update WorkerJob record
}
```

ป้องกันไม่ให้ video ที่ transcode สำเร็จแล้วกลับเป็น failed เพราะ subtitle ล้มเหลว

---

## NATS Connection Management

### API Server
- 1 NATS connection สำหรับทุกอย่าง (publisher + subscriber + DLQ)
- `MaxReconnects(-1)`, `ReconnectWait(2s)`

### Go Worker
- **3 NATS connections แยกกัน**:
  1. Shared connection: Messenger + WarmCachePublisher + SubtitlePublisher + Heartbeat
  2. Transcode consumer: own connection
  3. Gallery consumer: own connection
- ทุก connection: `MaxReconnects(-1)`, `ReconnectWait(2s)`

### Python Workers
- 1 NATS connection per worker process
- `max_reconnect_attempts=-1` (infinite reconnect)
- Workers ไม่สร้าง streams/consumers — expect ว่า API สร้างให้แล้ว

---

## Stream Creation Responsibility

| Stream | สร้างโดย API | สร้างโดย Worker | หมายเหตุ |
|--------|-------------|---------------|---------|
| `TRANSCODE_JOBS` | Yes | Yes (Go Worker) | ทั้งคู่สร้าง idempotent |
| `SUBTITLE_JOBS` | Yes | Yes (Go Worker, แต่ไม่ถูกเรียก) | Python workers ไม่สร้าง ต้องมีอยู่แล้ว |
| `GALLERY_JOBS` | Yes | Yes (Go Worker) | ทั้งคู่สร้าง |
| `WARM_CACHE_JOBS` | Yes | No (SetupStream ไม่ถูกเรียก) | API ต้องสร้าง |
| `REEL_JOBS` | Yes | No | API สร้างอย่างเดียว |
| `TRANSCODE_DLQ` | No | Yes (Go Worker) | Worker สร้างตอน startup |

---

## Timeout Constants (API Side)

| Job Type | Timeout | ใช้ใน `_meta.timeout_sec` |
|----------|---------|--------------------------|
| Transcode | 3600s (1 ชม.) | `TimeoutTranscode` |
| Gallery | 1800s (30 นาที) | `TimeoutGallery` |
| WarmCache | 600s (10 นาที) | `TimeoutWarmCache` |
| Subtitle Detect | 300s (5 นาที) | `TimeoutSubtitleDetect` |
| Subtitle Transcribe | 1800s (30 นาที) | `TimeoutSubtitleTranscribe` |
| Subtitle Translate | 900s (15 นาที) | `TimeoutSubtitleTranslate` |
| Reel | 1200s (20 นาที) | `TimeoutReel` |

---

## Monitoring

- **NATS Monitor**: `http://localhost:8222`
- **API Endpoint**: `GET /api/v1/monitoring/jetstream` — แสดงสถานะ streams ทั้งหมด
- **API Endpoint**: `GET /api/v1/monitoring/queue` — แสดง queue stats
- **Connections**: `curl http://localhost:8222/connz` — ดู connections ทั้งหมด
- **Streams**: `curl http://localhost:8222/jsz?streams=true` — ดู stream stats

---

## NATS CLI Management (on Server)

### Prerequisites

`nats` CLI ถูกติดตั้งบน server แล้วที่ `/usr/local/bin/nats`

```bash
# SSH เข้า server
ssh -i ~/.ssh/id_ed25519_suekk root@5.223.46.39
```

### Purge Stream (ลบ messages ทั้งหมดใน stream)

```bash
# Purge ทุก message ใน SUBTITLE_JOBS
nats -s nats://localhost:4222 stream purge SUBTITLE_JOBS -f

# Purge streams อื่น
nats -s nats://localhost:4222 stream purge TRANSCODE_JOBS -f
nats -s nats://localhost:4222 stream purge GALLERY_JOBS -f
nats -s nats://localhost:4222 stream purge WARM_CACHE_JOBS -f
nats -s nats://localhost:4222 stream purge REEL_JOBS -f
```

### ดูรายละเอียด Stream

```bash
# ดูสถานะ stream (messages, bytes, consumers)
nats -s nats://localhost:4222 stream info SUBTITLE_JOBS

# ดูทุก streams
nats -s nats://localhost:4222 stream list

# ดู messages ที่ค้างอยู่ใน stream
nats -s nats://localhost:4222 stream view SUBTITLE_JOBS
```

### จัดการ Consumers

```bash
# ดูรายชื่อ consumers ของ stream
nats -s nats://localhost:4222 consumer list SUBTITLE_JOBS

# ดูรายละเอียด consumer
nats -s nats://localhost:4222 consumer info SUBTITLE_JOBS SUBTITLE_TRANSLATE_WORKER
```

### เมื่อไหร่ควร Purge

| สถานการณ์ | ทำอะไร |
|-----------|--------|
| มี job ซ้ำค้างใน queue | `stream purge` แล้ว restart worker |
| Worker ค้าง/crash แล้ว job redeliver ซ้ำ | `stream purge` เฉพาะ stream นั้น |
| ต้องการ reset ระบบทั้งหมด | purge ทุก stream + restart workers |
| Debug ปัญหา job ไม่ถูก process | `stream info` + `consumer info` ดู pending |
