# SEO Content Worker

Worker สำหรับ Auto-Generate E-E-A-T SEO Content ส่งไปที่ `api.subth.com`

## Architecture

```
_seo_worker/
├── cmd/worker/main.go          # Entry point
├── config/                     # Configuration
├── container/                  # Dependency Injection
├── domain/
│   ├── models/                 # Domain models
│   └── ports/                  # Interfaces
├── infrastructure/
│   ├── ai/                     # Gemini client
│   ├── tts/                    # ElevenLabs client
│   ├── embedding/              # pgvector client
│   ├── fetcher/                # HTTP fetchers
│   ├── publisher/              # Article publisher
│   ├── consumer/               # NATS consumer
│   ├── messenger/              # Progress publisher
│   └── storage/                # R2/S3 storage
└── use_cases/                  # Business logic
```

## Prerequisites

- Go 1.23+
- NATS with JetStream
- PostgreSQL with pgvector extension
- Gemini API key
- ElevenLabs API key (optional)
- R2/S3 storage (optional)

## Setup

1. Copy environment file:
```bash
cp .env.example .env
```

2. Edit `.env` with your credentials

3. Install dependencies:
```bash
make tidy
```

4. Run:
```bash
make dev
```

## Configuration

| Variable | Description | Required |
|----------|-------------|----------|
| `WORKER_ID` | Unique worker ID | Yes |
| `NATS_URL` | NATS server URL | Yes |
| `DATABASE_URL` | PostgreSQL connection string | Yes |
| `GEMINI_API_KEY` | Google Gemini API key | Yes |
| `GEMINI_MODEL` | `gemini-1.5-flash` (dev) or `gemini-1.5-pro` (prod) | Yes |
| `ELEVENLABS_API_KEY` | ElevenLabs API key | No |
| `ELEVENLABS_VOICE_ID` | Voice ID (default: `flat2.0`) | No |

## Workflow

1. Admin กด "Generate Article" → API publishes job to NATS
2. Worker consumes job
3. Fetch SRT from `api.suekk.com`
4. Fetch metadata from `api.subth.com`
5. Generate content with Gemini (JSON Mode)
6. Generate TTS audio (parallel)
7. Generate embedding (parallel)
8. Publish article to `api.subth.com`
9. Send progress updates via NATS

## NATS Subjects

- `seo.article.generate` - Job queue
- `seo.progress.{video_id}` - Progress updates

## Development

```bash
# Run in dev mode
make dev

# Build
make build

# Test
make test

# Lint
make lint
```

## Docker

```bash
# Build image
make docker-build

# Run container
make docker-run
```
