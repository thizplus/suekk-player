# Plan: Queue Management — Batch Subtitle Actions

> ปัญหา: Queue page มีแค่ "Queue Missing" ปุ่มเดียว ไม่มี batch detect/transcribe/translate แยก
> และไม่มี category filter

---

## สิ่งที่ต้องการ

### ปุ่ม Batch Actions (ทำทีเดียวหลาย video)

| ปุ่ม | สิ่งที่ทำ | API ที่ต้องสร้าง |
|------|----------|----------------|
| **Detect ทั้งหมด** | กด detect language ให้ video ที่ยังไม่ detect | `POST /admin/queues/subtitle/detect-all` |
| **Transcribe ทั้งหมด** | กด transcribe ให้ video ที่ detect แล้วแต่ยังไม่มี SRT | `POST /admin/queues/subtitle/transcribe-all` |
| **Translate ทั้งหมด** | กด translate ให้ video ที่มี SRT แล้วแต่ยังไม่แปล | `POST /admin/queues/subtitle/translate-all` |

### Category Filter

| Filter | สิ่งที่ทำ |
|--------|----------|
| เลือก category | filter video ตาม category ก่อนทำ batch action |
| เลือก "ทั้งหมด" | ทำทุก video (default) |

---

## Backend — API Endpoints ใหม่

### 1. Batch Detect
```
POST /api/v1/admin/queues/subtitle/detect-all
Query: ?category=xxx&limit=50
Logic:
  1. Query videos ที่ status=ready + has audio_path + detected_language=""
  2. Filter by category (optional)
  3. Loop: TriggerDetectLanguage(videoID) ทีละตัว
  4. Return { queued: N, skipped: N, errors: [] }
```

### 2. Batch Transcribe
```
POST /api/v1/admin/queues/subtitle/transcribe-all
Query: ?category=xxx&limit=50
Logic:
  1. Query videos ที่ detected_language != "" + ไม่มี original subtitle (status=ready)
  2. Filter by category (optional)
  3. Loop: TriggerTranscribe(videoID) ทีละตัว
  4. Return { queued: N, skipped: N, errors: [] }
```

### 3. Batch Translate
```
POST /api/v1/admin/queues/subtitle/translate-all
Query: ?category=xxx&limit=50&target=th
Logic:
  1. Query videos ที่มี original subtitle (status=ready) + ไม่มี translated subtitle
  2. Filter by category (optional)
  3. Loop: TriggerTranslation(videoID, [targetLang]) ทีละตัว
  4. Return { queued: N, skipped: N, errors: [] }
```

### 4. Get Categories (for dropdown)
```
GET /api/v1/categories
(มีอยู่แล้ว)
```

### 5. Get Subtitle Stats by Category
```
GET /api/v1/admin/queues/subtitle/stats
Query: ?category=xxx
Return:
{
  total_videos: 100,
  detected: 80,
  not_detected: 20,
  transcribed: 50,
  not_transcribed: 30,
  translated: 40,
  not_translated: 10,
}
```

---

## Frontend — Subtitle Tab ใหม่

### UI Layout

```
┌──────────────────────────────────────────────────────────────┐
│ ซับไตเติ้ล                                                    │
│                                                              │
│ ┌──────────────────────┐                                     │
│ │ หมวดหมู่: [ทั้งหมด ▾] │  ← category dropdown              │
│ └──────────────────────┘                                     │
│                                                              │
│ ┌────────────────────────────────────────────────────────┐   │
│ │ สถานะ Subtitle                                         │   │
│ │                                                        │   │
│ │ 🔍 ยังไม่ detect: 20     [Detect ทั้งหมด]               │   │
│ │ 📝 ยังไม่ transcribe: 30  [Transcribe ทั้งหมด]          │   │
│ │ 🌐 ยังไม่ translate: 10   [Translate ทั้งหมด]           │   │
│ │ ✅ ครบแล้ว: 40                                          │   │
│ └────────────────────────────────────────────────────────┘   │
│                                                              │
│ ┌─────┬──────────┬─────────┐                                │
│ │ Failed │ Processing │ Queued │  ← sub-tabs               │
│ └─────┴──────────┴─────────┘                                │
│ [Table: job list per status]                                 │
│                                                              │
│ ปุ่ม: [Retry All] [Clear Stuck] [ลบ Failed]                  │
└──────────────────────────────────────────────────────────────┘
```

### Components

```
SubtitleTab
├── CategorySelect          — dropdown เลือกหมวดหมู่
├── SubtitleStatsPanel      — แสดงจำนวน detect/transcribe/translate
│   ├── StatRow (detect)    — จำนวน + ปุ่ม "Detect ทั้งหมด"
│   ├── StatRow (transcribe)— จำนวน + ปุ่ม "Transcribe ทั้งหมด"
│   └── StatRow (translate) — จำนวน + ปุ่ม "Translate ทั้งหมด"
├── SubTabs (Failed/Processing/Queued)
└── JobTable                — ตาราง jobs
```

---

## Files ที่ต้องแก้/สร้าง

### Backend (Go)

| File | Action |
|------|--------|
| `interfaces/api/handlers/queue_handler.go` | เพิ่ม BatchDetect, BatchTranscribe, BatchTranslate, GetSubtitleStats |
| `interfaces/api/routes/queue_routes.go` | เพิ่ม routes |
| `application/serviceimpl/queue_service_impl.go` | เพิ่ม batch logic |
| `domain/services/queue_service.go` | เพิ่ม interface methods |

### Frontend (React)

| File | Action |
|------|--------|
| `features/queue/pages/QueueManagementPage.tsx` | ปรับ SubtitleTab ใหม่ |
| `features/queue/hooks.ts` | เพิ่ม useBatchDetect, useBatchTranscribe, useBatchTranslate, useSubtitleStats |
| `features/queue/service.ts` | เพิ่ม API calls |
| `features/queue/types.ts` | เพิ่ม types |

---

## Execution Order

1. **Backend**: สร้าง `GET /subtitle/stats` + `POST /detect-all` + `POST /transcribe-all` + `POST /translate-all`
2. **Frontend**: เพิ่ม hooks + service calls
3. **Frontend**: ปรับ SubtitleTab UI
4. **Test**: ทดสอบกับ production
5. **Deploy**: push + rebuild

---

## Notes

- Batch actions ควรมี `limit` parameter (default 50) เพื่อไม่ให้ queue ท่วม
- ทุก action ต้อง validate ก่อน (เช่น transcribe ต้อง detect ก่อน)
- Progress แสดงผ่าน WebSocket เหมือนเดิม
- Category filter ใช้ SUEKK categories (ไม่ใช่ subth categories)
