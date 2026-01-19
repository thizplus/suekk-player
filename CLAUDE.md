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

## Debugging History

### 2026-01-11: Ghost Worker Bug
- **ปัญหา**: Videos fail ทันทีเมื่ออัพโหลดหลายไฟล์
- **สาเหตุ**: Ghost `worker.exe` (PID 662720) ค้างอยู่บน Host
- **แก้ไข**: Kill process เก่า, ตรวจสอบ NATS connections ก่อนเริ่มงาน
- **บทเรียน**: ลบ Docker ไม่พอ ต้องตรวจสอบ process บน Host ด้วย
