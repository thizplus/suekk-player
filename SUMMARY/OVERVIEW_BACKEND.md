# SUEKK Stream — Backend API Overview

> Go Fiber REST API + WebSocket + NATS JetStream
> Path: `_gofiber_starter/`

---

## Project Structure

```
_gofiber_starter/
├── cmd/
│   ├── api/main.go                  # Entry point
│   ├── setup-bucket/main.go         # S3 bucket setup utility
│   ├── setup-cors/main.go           # CORS setup utility
│   └── tools/                       # One-off tools (clear/list gallery)
├── application/serviceimpl/         # Service implementations (business logic)
├── domain/
│   ├── models/                      # GORM entity structs (16 models)
│   ├── dto/                         # Request/Response DTOs + mappers
│   ├── ports/                       # Interfaces (Storage, Messaging, Notifier)
│   ├── repositories/                # Repository interfaces
│   └── services/                    # Service interfaces
├── infrastructure/
│   ├── nats/                        # NATS client, publisher, subscriber, DLQ
│   ├── messaging/                   # Port adapters (NATSJobQueue, Progress Pub/Sub)
│   ├── postgres/                    # GORM DB + repository implementations
│   ├── redis/                       # Redis client (whitelist cache)
│   ├── storage/                     # S3Storage + LocalStorage adapters
│   ├── telegram/                    # Telegram notification adapter
│   ├── transcoder/                  # FFmpeg probe (for video metadata)
│   └── websocket/                   # ProgressBroadcaster + WebSocketManager
├── interfaces/api/
│   ├── handlers/                    # HTTP handlers (20 files)
│   ├── middleware/                   # 6 middlewares
│   ├── routes/                      # Route registration (20 files)
│   └── websocket/                   # WebSocket upgrade handler
├── pkg/
│   ├── config/config.go             # Config struct
│   ├── di/container.go              # DI container (all wiring)
│   ├── logger/                      # slog + lumberjack
│   ├── scheduler/                   # GoCron scheduler
│   ├── settings/                    # Settings cache (RWMutex)
│   └── utils/                       # JWT, response helpers, validator
├── migrations/                      # Manual SQL patches (5 files)
├── Dockerfile                       # Multi-stage alpine build
└── docker-compose.yml               # API + NATS + PostgreSQL + MinIO
```

---

## Entry Point

**`cmd/api/main.go`** :
1. DI container `di.NewContainer()` + `container.Initialize()`
2. Fiber app (body limit 10 GB, streaming enabled)
3. Middleware chain: `RequestID -> Logger -> CORS`
4. `routes.SetupRoutes(app, handlers)`
5. Listen on `APP_PORT` (default `8080`)
6. Graceful shutdown via OS signals

---

## Database Models (16 Tables, GORM AutoMigrate)

| Model | Key Fields | Description |
|-------|------------|-------------|
| **User** | UUID, GoogleID, Email, Username, Password, Role (`user`/`admin`) | User accounts + Google OAuth |
| **Video** | UUID, Code (unique), Title, Duration, Status, HLSPath, DiskUsage, QualitySizes (JSONB), DetectedLanguage, GalleryStatus | Core video entity |
| **Subtitle** | UUID, VideoID FK, Language, Type (`original`/`translated`), SRTPath, Status, Confidence | Subtitle per language per video |
| **Category** | UUID, Name, Slug, ParentID (self-ref), SortOrder | Hierarchical categories |
| **WhitelistProfile** | UUID, Name, IsActive, Watermark config (enabled/URL/position/opacity) | Domain whitelist profiles |
| **ProfileDomain** | UUID, ProfileID FK, Domain (supports `*.example.com`) | Allowed embed domains |
| **PrerollAd** | UUID, ProfileID FK, Type (`video`/`image`), URL, Duration, SkipAfter, ClickURL | Pre-roll ads per profile |
| **AdImpression** | UUID, ProfileID FK, VideoCode, Domain, WatchDuration, Completed, Skipped, DeviceType | Ad analytics |
| **Reel** | UUID, VideoID FK, Title, Line1/Line2, TTSText/Voice, Segments (JSONB), Style, OutputPath, Status | Short clips for social media |
| **ReelTemplate** | UUID, Name, DefaultLayers (JSONB), BackgroundStyle, FontFamily | Reel visual templates |
| **WorkerJob** | UUID, JobType, EntityType, EntityID, Status, Progress, WorkerID, RetryCount, JobData/OutputData (JSONB) | Job tracking (Dual Write) |
| **SystemSetting** | UUID, Category, Key, Value, ValueType, IsSecret | Runtime system settings |
| **SettingAuditLog** | UUID, Category, Key, OldValue, NewValue, Reason, ChangedBy | Setting change history |
| **Task** | UUID, Title, Status, Priority, DueDate, UserID FK | Generic tasks |
| **File** | UUID, FileName, FileSize, MimeType, URL, CDNPath, UserID FK | File records |
| **ScheduledJob** | UUID, Name, CronExpr, Status, LastRun, NextRun | User-defined cron jobs |

### Video Status Flow

```
pending -> queued -> processing -> ready
                         |
                       failed -> dead_letter (after 3 retries)
```

---

## API Endpoints (All Routes)

### Auth & Users (`/api/v1/auth`, `/api/v1/users`)

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | `/auth/register` | Register user | - |
| POST | `/auth/login` | Login (email/password) | - |
| GET | `/auth/google` | Redirect to Google OAuth | - |
| GET | `/auth/google/callback` | Google OAuth callback | - |
| GET | `/auth/me` | Get current user | JWT |
| GET | `/users/profile` | Get profile | JWT |
| PUT | `/users/profile` | Update profile | JWT |
| DELETE | `/users/profile` | Delete account | JWT |
| POST | `/users/set-password` | Set password (Google-only users) | JWT |
| GET | `/users/` | List all users | JWT + Admin |

### Videos (`/api/v1/videos`)

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | `/videos/` or `/videos/upload` | Upload video | JWT |
| POST | `/videos/batch` | Batch upload | JWT |
| GET | `/videos/` | List videos (paginated, filterable) | JWT |
| GET | `/videos/my` | My videos | JWT |
| GET | `/videos/stats` | Video statistics | JWT |
| GET | `/videos/:id` | Get video by ID | JWT |
| PUT | `/videos/:id` | Update video (title, category) | JWT |
| DELETE | `/videos/:id` | Delete video + S3 files | JWT |
| POST | `/videos/:id/transcode` | Queue transcode job | JWT |
| POST | `/videos/:id/warm-cache` | Queue CDN cache warming | JWT |
| POST | `/videos/:id/generate-gallery` | Queue gallery generation | JWT |
| POST | `/videos/:id/regenerate-gallery` | Re-generate gallery | JWT |
| GET | `/videos/ready` | List ready videos (public) | - |
| GET | `/videos/code/:code` | Get video by code (public) | - |
| GET | `/videos/embed/:code` | Get embed data (public) | - |

### DLQ (Dead Letter Queue)

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/videos/dlq/` | List DLQ videos | JWT |
| POST | `/videos/dlq/:id/retry` | Retry DLQ video | JWT |
| DELETE | `/videos/dlq/:id` | Delete DLQ entry | JWT |

### Subtitles (`/api/v1/subtitles`, `/api/v1/videos/:id/subtitle/*`)

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/subtitles/languages` | Supported languages | - |
| GET | `/embed/videos/:code/subtitles` | Get subtitles for embed player | - |
| POST | `/videos/:id/subtitle/detect` | Trigger language detection | JWT |
| POST | `/videos/:id/subtitle/language` | Manually set language | JWT |
| POST | `/videos/:id/subtitle/transcribe` | Trigger transcription | JWT |
| POST | `/videos/:id/subtitle/translate` | Trigger translation | JWT |
| GET | `/videos/:id/subtitles` | List subtitles for video | JWT |
| DELETE | `/subtitles/:id` | Delete subtitle | JWT |
| GET | `/subtitles/:id/content` | Get SRT content | JWT |
| PUT | `/subtitles/:id/content` | Update SRT content | JWT |
| POST | `/admin/subtitles/retry-stuck` | Retry stuck subtitles | JWT |

### Worker Callbacks (Internal, no auth)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/internal/subtitles/job-started` | Worker reports job started |
| POST | `/internal/videos/:id/subtitle/callback/detect` | Detection complete |
| POST | `/internal/subtitles/:id/callback/transcribe` | Transcription complete |
| POST | `/internal/subtitles/:id/callback/translate` | Translation complete |
| POST | `/internal/subtitles/:id/callback/failed` | Subtitle job failed |
| PATCH | `/internal/videos/:id/gallery` | Gallery update from worker |

### Direct Upload (`/api/v1/direct-upload`)

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | `/direct-upload/init` | Init multipart upload, get presigned part URLs | JWT |
| POST | `/direct-upload/complete` | Complete multipart upload, create video record | JWT |
| DELETE | `/direct-upload/abort` | Abort upload | JWT |
| GET | `/config/upload-limits` | Get upload size limits | JWT |

### Queue Management (`/api/v1/admin/queues`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/admin/queues/stats` | Queue statistics |
| GET | `/admin/queues/workers/online` | Online workers (from NATS KV) |
| GET/POST | `/admin/queues/transcode/*` | Transcode queue: failed list, retry all, retry one, purge |
| GET/POST/DELETE | `/admin/queues/subtitle/*` | Subtitle queue: stuck, failed, retry all, clear all, queue missing |
| GET/POST | `/admin/queues/warm-cache/*` | Cache warming: pending, failed, warm one, warm all |
| GET/POST | `/admin/queues/gallery/*` | Gallery queue: processing, failed, retry all |
| GET/POST | `/admin/queues/reel/*` | Reel queue: exporting, failed, retry all |

### Worker Jobs (`/api/v1/worker-jobs`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/worker-jobs/stats` | Job stats (by type) |
| GET | `/worker-jobs/stats/all` | All stats |
| GET | `/worker-jobs/` | List jobs (filterable) |
| GET | `/worker-jobs/:id` | Get job detail |
| POST | `/worker-jobs/:id/cancel` | Cancel job |
| POST | `/worker-jobs/:id/retry` | Retry job |
| DELETE | `/worker-jobs/orphaned` | Delete orphaned |
| DELETE | `/worker-jobs/completed` | Delete completed |
| DELETE | `/worker-jobs/failed` | Delete failed |

### Whitelist & Ads

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/embed/config` | Get embed config for domain (whitelist check) | - |
| POST | `/ads/impression` | Record ad impression/skip | - |
| CRUD | `/whitelist/profiles/*` | Profile management | JWT |
| CRUD | `/whitelist/profiles/:id/domains` | Domain management | JWT |
| CRUD | `/whitelist/profiles/:id/prerolls` | Preroll ad management | JWT |
| POST | `/whitelist/cache/clear` | Clear all whitelist cache | JWT |
| GET | `/ads/stats/*` | Ad stats, device stats, rankings, skip distribution | JWT |

### Categories (`/api/v1/categories`)

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/categories/` | List categories | - |
| GET | `/categories/tree` | Tree structure | - |
| GET | `/categories/:id` | By ID | - |
| GET | `/categories/slug/:slug` | By slug | - |
| POST | `/categories/` | Create | JWT |
| PUT | `/categories/:id` | Update | JWT |
| PUT | `/categories/reorder` | Reorder | JWT |
| DELETE | `/categories/:id` | Delete | JWT |

### Reels (`/api/v1/reels`)

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/reels/templates/` | List templates | - |
| CRUD | `/reels/*` | Create, list, get, update, delete reels | JWT |
| POST | `/reels/:id/export` | Export reel to MP4 | JWT |
| GET | `/videos/:id/reels` | List reels for video | JWT |

### Settings (`/api/v1/settings`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/settings/` | Get all settings |
| GET | `/settings/categories` | List categories |
| GET | `/settings/:category` | Get by category |
| PUT | `/settings/:category` | Update category settings |
| POST | `/settings/:category/reset` | Reset to defaults |
| GET | `/settings/audit-logs` | Change history |
| POST | `/settings/reload-cache` | Force reload cache |

### HLS Streaming & CDN Proxy

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/hls/:code/access` | Get JWT stream token |
| GET | `/api/v1/hls/:code/gallery` | Get gallery presigned URLs |
| GET | `/api/v1/hls/verify` | Verify stream token |
| GET | `/hls/:code/*` | Proxy HLS segments from S3 |
| GET | `/subtitles/:code/*` | Proxy subtitle files |
| GET | `/stream/reels/:code/*` | Proxy reel files |
| GET | `/gallery/:code/*` | Proxy gallery images |

### WebSocket

| Path | Description |
|------|-------------|
| `ws://host/ws` | Real-time progress (video, subtitle, reel, gallery) |

### Monitoring

| Method | Path | Description |
|--------|------|-------------|
| GET | `/monitoring/jetstream` | JetStream stream status |
| GET | `/monitoring/queue` | Queue stats |
| GET | `/monitoring/health` | Health check |

---

## Middleware Chain

```
RequestID -> Logger -> CORS -> [Protected (JWT)] -> [AdminOnly]
```

| Middleware | Description |
|-----------|-------------|
| **RequestID** | Generate `X-Request-ID`, attach to context for tracing |
| **Logger** | Structured request/response logging via slog |
| **CORS** | CORS headers |
| **Protected()** | JWT validation, sets `c.Locals("user")` |
| **AdminOnly()** | Role check: requires `admin` role |
| **EmbedWhitelist** | Domain whitelist check for embed, sets signed `suekk_stream` cookie (HMAC, SameSite=None) |

---

## NATS JetStream Streams

| Stream | Subject(s) | Consumer | Timeout | Purpose |
|--------|-----------|----------|---------|---------|
| `TRANSCODE_JOBS` | `jobs.transcode` | `TRANSCODE_WORKER` | 3600s | Video transcoding |
| `SUBTITLE_JOBS` | `jobs.subtitle.detect` | `SUBTITLE_DETECT_WORKER` | 300s | Language detection |
| `SUBTITLE_JOBS` | `jobs.subtitle.transcribe` | `SUBTITLE_TRANSCRIBE_WORKER` | 1800s | Transcription |
| `SUBTITLE_JOBS` | `jobs.subtitle.translate` | `SUBTITLE_TRANSLATE_WORKER` | 900s | Translation |
| `GALLERY_JOBS` | `jobs.gallery.generate` | `GALLERY_WORKER` | 1800s | Gallery extraction |
| `WARM_CACHE_JOBS` | `jobs.warmcache` | `WARM_CACHE_WORKER` | 600s | CDN cache warming |
| `REEL_JOBS` | `jobs.reel.export` | `REEL_WORKER` | 1200s | Reel MP4 export |

### KV Bucket: `WORKER_STATUS`
- Workers publish heartbeats (hostname, GPU, disk, status, current job)
- API reads for worker monitoring page

### Progress Pub/Sub: `progress.>`
- Workers publish real-time progress to `progress.{entity_type}.{entity_id}`
- API subscribes -> `ProgressBroadcaster` -> WebSocket broadcast

### Dual Write Pattern
```
API publish job:
  1. Create WorkerJob in DB (status=pending) -> get job_id
  2. Publish to NATS with job_id (status->queued)
  3. Worker picks up -> progress updates -> ProgressBroadcaster updates WorkerJob record
  4. On complete/fail -> final DB update
```

### Subtitle Orchestration Chain (ProgressBroadcaster)
```
detect completed -> HandleDetectComplete() -> auto-trigger TriggerTranscribe()
transcribe completed -> HandleTranscribeComplete() -> auto-trigger TriggerTranslation()
translate completed -> HandleTranslationComplete() -> done
```
Workers do one job and report back. API orchestrates the chain.

---

## WebSocket Events

| Event Type | Payload | Trigger |
|-----------|---------|---------|
| `video_progress` | videoId, videoCode, type, status, progress, message, quality | Transcode/gallery/warmcache progress |
| `subtitle_progress` | videoId, subtitleId, language, stage, status, progress | Subtitle pipeline progress |
| `reel_progress` | reelId, videoCode, status, progress, outputUrl | Reel export progress |
| `transcode:completed` | videoId, videoCode | Video became ready |
| `transcode:failed` | videoId, videoCode, error | Transcode failed |

---

## Service Layer

| Service | Key Methods |
|---------|-------------|
| **UserService** | Register, Login, GoogleOAuth, GetProfile, UpdateProfile, SetPassword |
| **VideoService** | Upload, CreateVideo, ListWithFilters, Update, Delete, GetStats, CheckStorageQuota |
| **SubtitleService** | TriggerDetect/Transcribe/Translate, HandleComplete callbacks, GetContent, UpdateContent |
| **QueueService** | Unified queue management across all job types |
| **ReelService** | CRUD + Export (publish NATS job) |
| **WorkerJobService** | Full job lifecycle, stats, cleanup |
| **WhitelistService** | IsDomainAllowed (Redis cache), Profile CRUD, Ad stats |
| **SettingService** | Get/Set by category, InitializeDefaults, in-memory cache |
| **CategoryService** | CRUD + Tree structure + Reorder |
| **StorageService** | Cleanup scheduled job |

---

## Background Jobs (GoCron Scheduler)

| Job | Schedule | Description |
|-----|----------|-------------|
| Storage Cleanup | `0 3 * * *` (3 AM daily) | Delete temp files > 24h, failed video files > 7 days |
| Video Stuck Detector | Every 30 seconds | Mark `processing` > 60 min or `pending` > 10 min as `failed` |
| Subtitle Stuck Detector | Every 30 seconds | Mark subtitle jobs `processing` > 10 min as `failed` |
| User-defined Jobs | Configurable cron | From `jobs` table, managed via API |

---

## External Integrations

| Integration | Purpose | Implementation |
|------------|---------|----------------|
| **S3-Compatible** (IDrive E2) | Video, HLS, gallery, subtitle, reel storage | `infrastructure/storage/s3_storage.go` |
| **PostgreSQL** | Primary database | GORM via `infrastructure/postgres/` |
| **NATS JetStream** | Job queue + pub/sub progress | `infrastructure/nats/` |
| **Redis** | Whitelist domain cache | `infrastructure/redis/` |
| **Google OAuth** | Social login | `application/serviceimpl/user_service_impl.go` |
| **Telegram Bot** | Alerts (DLQ, transcode complete/fail, worker offline) | `infrastructure/telegram/notifier.go` |
| **Cloudflare CDN** | HLS streaming, signed cookie access control | Config: `CDN_BASE_URL`, `STREAM_COOKIE_*` |

---

## CDN Access Control Flow

```
1. Embed player requests GET /api/v1/embed/:code/info
2. EmbedWhitelist middleware checks domain in whitelist (Redis cache)
3. If allowed -> sets signed cookie (HMAC: suekk_stream, SameSite=None, HttpOnly)
4. Player requests HLS via CDN -> Cloudflare WAF validates cookie
5. Alternatively: JWT stream token via GET /api/v1/hls/:code/access
```

---

## Docker

**Dockerfile**: Multi-stage alpine build
- Builder: `golang:1.22-alpine`
- Runtime: `alpine:3.19` (non-root user `api`)
- Healthcheck: `wget --spider http://localhost:8080/health`
- No FFmpeg included (transcoding done by separate worker on host)

**docker-compose.yml** (root level):
- API container (port 8080)
- NATS + JetStream (ports 4222, 8222)
- PostgreSQL (port 5432)
- MinIO (ports 9000, 9001) — local S3
