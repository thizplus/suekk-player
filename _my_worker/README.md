# SUEKK Stream Workers

Unified worker infrastructure for SUEKK Stream video processing platform.

## Architecture

```
_my_worker/
├── go/                        # Go workers
│   ├── shared/               # Shared code for all Go workers
│   │   ├── config/           # Base configuration
│   │   ├── domain/           # Domain types + Ports
│   │   │   └── ports/        # Port interfaces (abstractions)
│   │   ├── infrastructure/   # Shared adapters
│   │   │   ├── nats/         # NATS consumer, progress
│   │   │   ├── s3/           # S3 storage
│   │   │   └── cli/          # CLI runner, progress printer
│   │   └── pkg/              # Utilities
│   │       ├── cleanup/      # Temp file cleanup
│   │       └── logger/       # Structured logging
│   ├── warmcache/            # Warmcache worker
│   ├── transcode/            # Transcode worker
│   ├── gallery/              # Gallery worker
│   └── reel/                 # Reel worker
├── python/                   # Python workers
│   ├── shared/               # Shared Python code
│   ├── subtitle_detect/      # Language detection
│   ├── subtitle_transcribe/  # Whisper transcription
│   └── subtitle_translate/   # Translation
└── docker-compose.yml        # Single command start
```

## Design Principles

### Clean Architecture + Port/Adapter Pattern

Every worker follows Clean Architecture:

```
cmd/           → Entry points (NATS or CLI)
config/        → Configuration
domain/        → Business entities and Port interfaces
application/   → Business logic (Handler)
infrastructure/ → Adapter implementations
```

### Why Port/Adapter?

1. **Swappable Components**: Change S3 to GCS? Just create new adapter
2. **Testable**: Mock ports for unit tests
3. **CLI Testing**: Test worker with JSON file before NATS

## Workers

| Worker | Language | Job Type | Description |
|--------|----------|----------|-------------|
| warmcache | Go | `warmcache` | CDN cache warming |
| transcode | Go | `transcode` | Video transcoding (FFmpeg) |
| gallery | Go | `gallery` | Frame extraction + NSFW classification |
| reel | Go | `reel` | Reel video composition |
| subtitle_detect | Python | `subtitle_detect` | Language detection (Whisper) |
| subtitle_transcribe | Python | `subtitle_transcribe` | Transcription (Whisper + LLM) |
| subtitle_translate | Python | `subtitle_translate` | Translation (OpenAI) |

## Quick Start

### 1. Environment Variables

```bash
# Copy example env
cp .env.example .env

# Edit with your values
```

### 2. Run with Docker Compose

```bash
# Start all workers
docker-compose up -d

# View logs
docker-compose logs -f warmcache
```

### 3. Run Individual Worker (Development)

```bash
# Go worker
cd go/warmcache
go run ./cmd

# Python worker
cd python/subtitle_detect
python main.py
```

### 4. CLI Testing Mode

```bash
# Test with JSON file (no NATS required)
cd go/warmcache
go run ./cmd/cli -input testdata/test_job.json -verbose
```

## Job Message Format

All workers receive jobs in this format:

```json
{
  "_meta": {
    "job_id": "uuid",
    "job_type": "warmcache",
    "entity_type": "video",
    "entity_id": "uuid",
    "entity_code": "ABC123",
    "priority": 2,
    "retry_count": 0,
    "max_retries": 3,
    "timeout_sec": 600,
    "created_at": 1709040000
  },
  "input": {
    // Job-specific input fields
  }
}
```

## Progress Updates

Workers send progress via NATS Pub/Sub:

```json
{
  "job_id": "uuid",
  "job_type": "warmcache",
  "entity_type": "video",
  "entity_id": "uuid",
  "worker_id": "warmcache-host-1",
  "status": "processing",
  "progress": 45.5,
  "stage": "warming",
  "message": "กำลัง warm 100/200 segments..."
}
```

## Standard Stages

Each job type has defined stages:

### Warmcache
| Stage | Progress | Description |
|-------|----------|-------------|
| initializing | 0-5% | Setup |
| warming | 5-80% | Fetch segments |
| verifying | 80-95% | Verify cache |
| finalizing | 95-99% | Update DB |
| completed | 100% | Done |

### Transcode
| Stage | Progress | Description |
|-------|----------|-------------|
| initializing | 0-2% | Setup |
| downloading | 2-10% | Download original |
| analyzing | 10-12% | Analyze metadata |
| transcoding_1080p | 12-45% | Encode 1080p |
| transcoding_720p | 45-65% | Encode 720p |
| transcoding_480p | 65-80% | Encode 480p |
| uploading | 80-95% | Upload to S3 |
| completed | 100% | Done |

## Related Documents

- `_gofiber_starter/docs/___WORKER_INTERFACE_STANDARD.md` - Full standard
- `_gofiber_starter/docs/___WORKER_INPUT_STANDARD.md` - Input formats
- `_gofiber_starter/docs/___NATS_MESSAGE_STANDARD.md` - NATS configuration
