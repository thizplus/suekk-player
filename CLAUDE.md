# Project: SUEKK Stream - Video Streaming Platform

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                        Docker                                │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐        │
│  │   API   │  │  NATS   │  │ Postgres│  │  MinIO  │        │
│  │ :8080   │  │ :4222   │  │ :5432   │  │ :9000   │        │
│  └────┬────┘  └────┬────┘  └─────────┘  └─────────┘        │
│       │            │                                        │
└───────┼────────────┼────────────────────────────────────────┘
        │            │
        │            │ (JetStream + Pub/Sub)
        │            │
   ┌────┴────────────┴────┐
   │   Worker (Host)      │  ← รันบน Host เพื่อใช้ GPU
   │   go run ./cmd/worker│
   └──────────────────────┘
```

## Key Components

| Component | Location | Description |
|-----------|----------|-------------|
| API (Go Fiber) | `_gofiber_starter/` | REST API + WebSocket |
| Worker | `_worker/` | Video transcoding (FFmpeg + GPU) |
| Frontend | `_vite_starter/` | React + TypeScript |
| Docker Compose | `docker-compose.yml` (root) | API, NATS, PostgreSQL, MinIO |

## Video Status Flow

```
pending → queued → processing → ready
                       ↓
                    failed
```

| Status | ความหมาย | Timeout |
|--------|----------|---------|
| `pending` | เพิ่ง upload, ยังไม่ publish job | 5 นาที |
| `queued` | job อยู่ใน NATS queue รอ worker | 60 นาที |
| `processing` | worker กำลัง transcode | 30 นาที |
| `ready` | transcode สำเร็จ | - |
| `failed` | ล้มเหลว | - |

## NATS Configuration

- **JetStream Stream**: `TRANSCODE_JOBS` - สำหรับ job queue (durable)
- **Pub/Sub Subject**: `progress.*` - สำหรับ progress updates
- **Consumer**: `WORKER` - durable consumer สำหรับ worker
- **Worker Concurrency**: 1 (default, เพื่อไม่ให้ VRAM เต็ม)

---

## Troubleshooting Guide

### ปัญหา: Videos fail ทันทีโดยมี worker_id ว่าง

**อาการ:**
- อัพโหลดหลายไฟล์ บางไฟล์ fail ทันที
- Log แสดง `worker_id: ""` (ว่างเปล่า)
- ลบ Docker กี่รอบก็ไม่หาย

**สาเหตุ:** Ghost Worker Process
- มี `worker.exe` เก่าจากการรัน `go build` หรือ `go run` ค้างอยู่บน Host
- Ghost worker แย่ง job จาก NATS แล้วส่ง failed กลับมา
- อยู่บน Host ไม่ใช่ใน Docker จึงไม่ถูกลบไปด้วย

**วิธีตรวจสอบและแก้ไข:**

```bash
# 1. ดู NATS connections
curl http://localhost:8222/connz

# 2. หา process ที่ connect port 4222
netstat -ano | findstr "4222"

# 3. ตรวจสอบ process ที่น่าสงสัย
wmic process where "ProcessId=<PID>" get Name,CommandLine

# 4. Kill ghost worker
taskkill /F /PID <PID>
```

**วิธีป้องกัน:**
- ก่อนเริ่ม worker ใหม่ ให้ตรวจสอบว่าไม่มี process เก่าค้าง
- ใช้ `Ctrl+C` ปิด worker แทนการปิด terminal ตรงๆ

---

### ปัญหา: ไม่รู้ว่า log มาจากไหน

**วิธีแก้:** เปิด `AddSource: true` ใน slog

```go
// pkg/logger/logger.go
opts := &slog.HandlerOptions{
    Level:     level,
    AddSource: true, // แสดง file/line ใน log
}
```

**ผลลัพธ์:**
```json
{"source":{"function":"...","file":"/app/internal/nats/consumer.go","line":185},"msg":"..."}
```

---

### คำสั่งที่ใช้บ่อย

```bash
# ดู Docker containers
docker ps --format "table {{.Names}}\t{{.Status}}"

# ดู API logs
docker logs suekk_stream-api-1 --tail 50 -f

# ดู NATS connections
curl http://localhost:8222/connz

# ดู NATS streams
curl http://localhost:8222/jsz?streams=true

# รัน worker
cd _worker && go run ./cmd/worker

# Rebuild Docker
docker-compose down -v && docker-compose up --build -d
```

---

## Important Notes

1. **Worker รันบน Host** ไม่ใช่ใน Docker เพราะต้องใช้ GPU (NVENC)
2. **docker-compose.yml อยู่ที่ root** ไม่ใช่ใน `_gofiber_starter/`
3. **worker_id** ใช้ระบุว่า progress มาจาก worker ตัวไหน - ถ้าว่างแสดงว่ามีปัญหา
4. **NATS Monitor** อยู่ที่ `http://localhost:8222` - ใช้ดู connections และ streams
5. **Storage** ใช้ IDrive E2 (S3-compatible) สำหรับเก็บ original videos และ HLS segments

---

## Gallery Generation from HLS

### สถานการณ์
- Video ที่ transcode ไปแล้วไม่มี gallery (ไฟล์ต้นฉบับถูกลบหลัง transcode)
- เหลือแค่ HLS files (m3u8, .ts segments) บน S3

### แนวทาง: สร้าง Gallery จาก HLS Segments

**ข้อดี:**
- ไม่ต้อง re-upload ไฟล์ต้นฉบับ
- ใช้ไฟล์ที่มีอยู่แล้วบน S3

**วิธีการ:**

#### 1. FFmpeg Extract Frames จาก HLS
```bash
# ดึง 100 frames จาก HLS playlist (กระจายตลอดทั้ง video)
ffmpeg -i "https://cdn.example.com/{code}/1080p/playlist.m3u8" \
  -vf "select='not(mod(n\,INTERVAL))',scale=1920:-1" \
  -frames:v 100 \
  -q:v 2 \
  gallery/%03d.jpg

# INTERVAL = total_frames / 100
```

#### 2. Worker Job Type ใหม่: `gallery_generate`
```
Job Flow:
1. API สร้าง job type: "gallery_generate"
2. Worker รับ job → download HLS จาก S3/CDN
3. FFmpeg extract 100 frames
4. Upload ไป S3: gallery/{video_code}/001.jpg - 100.jpg
5. Update DB: galleryPath, galleryCount
```

#### 3. API Endpoint
```
POST /api/v1/videos/{id}/generate-gallery
- เพิ่ม video เข้า queue สำหรับ generate gallery
- ใช้ HLS ที่มีอยู่แล้ว (ไม่ต้อง re-transcode)
```

#### 4. Batch Generate (สำหรับ video เก่าทั้งหมด)
```
POST /api/v1/admin/generate-galleries
Body: { "minDuration": 1200 }  // เฉพาะ video > 20 นาที

- Loop videos ที่ status=ready และ galleryCount=0
- สร้าง gallery job ทีละตัว
```

### Implementation Priority

| Step | Task | Effort |
|------|------|--------|
| 1 | เพิ่ม job type `gallery_generate` ใน NATS | Low |
| 2 | Worker: FFmpeg extract frames จาก HLS | Medium |
| 3 | Worker: Upload gallery images to S3 | Low |
| 4 | API: POST /videos/{id}/generate-gallery | Low |
| 5 | API: Batch generate endpoint | Low |
| 6 | Frontend: ปุ่ม "Generate Gallery" ใน VideoDetailSheet | Low |

### Notes
- ใช้ quality สูงสุดที่มี (1080p > 720p > 480p)
- Gallery images: 1920x1080 หรือ aspect ratio เดิม
- JPEG quality: 85-90 (balance size vs quality)

### Implementation Status

#### API Side (DONE)
- ✅ `_gofiber_starter/infrastructure/nats/types.go` - GalleryJob struct + constants
- ✅ `_gofiber_starter/infrastructure/nats/publisher.go` - PublishGalleryJob()
- ✅ `_gofiber_starter/infrastructure/nats/client.go` - gallery stream setup
- ✅ `_gofiber_starter/interfaces/api/handlers/video_handler.go` - GenerateGallery handler
- ✅ `_gofiber_starter/interfaces/api/routes/video_routes.go` - POST /:id/generate-gallery

#### Worker Side (DONE)

**Files modified/created:**

| File | Action | Description |
|------|--------|-------------|
| `_worker/domain/models/job.go` | ✅ DONE | GalleryJob struct |
| `_worker/domain/constants/nats.go` | ✅ DONE | Gallery stream constants |
| `_worker/use_cases/gallery_handler.go` | ✅ DONE | Gallery job handler |
| `_worker/infrastructure/consumer/gallery_consumer.go` | ✅ DONE | Separate gallery consumer |
| `_worker/container/container.go` | ✅ DONE | Wire up gallery handler + consumer |
| `_worker/config/config.go` | ✅ DONE | Add S3_PUBLIC_ENDPOINT config |

### Environment Variables (Worker)

```bash
# S3 public endpoint for HLS access (CDN URL)
S3_PUBLIC_ENDPOINT=https://cdn.example.com
```

### Usage

1. **Generate gallery for single video:**
   ```bash
   curl -X POST http://localhost:8080/api/v1/videos/{id}/generate-gallery \
     -H "Authorization: Bearer TOKEN"
   ```

2. **Worker will:**
   - Receive job from NATS stream `GALLERY_JOBS`
   - Download HLS from S3/CDN
   - Extract 100 frames using FFmpeg
   - Upload to S3: `gallery/{video_code}/001.jpg - 100.jpg`
   - Update database via API

---

## Debugging History

### 2026-01-11: Ghost Worker Bug
- **ปัญหา**: Videos fail ทันทีเมื่ออัพโหลดหลายไฟล์
- **สาเหตุ**: Ghost `worker.exe` (PID 662720) ค้างอยู่บน Host
- **แก้ไข**: Kill process เก่า, ตรวจสอบ NATS connections ก่อนเริ่มงาน
- **บทเรียน**: ลบ Docker ไม่พอ ต้องตรวจสอบ process บน Host ด้วย
