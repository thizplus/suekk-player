# Next Session TODO

> อ่านไฟล์นี้ก่อนเริ่มงาน session หน้า
> อัพเดทล่าสุด: 2026-06-06

---

## สิ่งที่ทำเสร็จแล้ว (Session 2026-06-06)

| # | งาน | Status |
|---|------|--------|
| 1 | สร้าง overview เอกสาร 6 ไฟล์ | DONE |
| 2 | แก้ subtitle pipeline bugs 7 ตัว (CRITICAL) + deploy | DONE |
| 3 | แก้ UI เป็น 3 ปุ่มแยก (Detect/Transcribe/Translate) + deploy | DONE |
| 4 | สร้าง Port/Adapter architecture (LLMPort, STTPort, VADPort, AudioPort) | DONE |
| 5 | Refactor subtitle workers ให้ใช้ Port/Adapter | DONE |
| 6 | ย้าย title_translate เข้า `_my_worker/python/` + ทดสอบแปลจริง 59 JAV สำเร็จ | DONE |
| 7 | สร้าง batch subtitle API endpoints (detect-all, transcribe-all, translate-all) + deploy | DONE |
| 8 | SSH access ทั้ง 2 server (suekk + subth) | DONE |
| 9 | ย้าย subth project → `_SUBTH_STREAM/` + clean git | DONE |

---

## งานที่ต้องทำ Session หน้า (เรียงตามลำดับ)

### 1. Frontend: ปรับ Queue Management — Subtitle Tab (สำคัญที่สุด)

**ปัญหา**: Queue page (`/queues`) Subtitle tab มีแค่ปุ่ม "Retry All" + "Queue Missing" ไม่มี batch detect/transcribe/translate แยก + ไม่มี category filter

**Backend API พร้อมแล้ว** (deploy ไปแล้ว):
```
GET  /api/v1/admin/queues/subtitle/stats?category=xxx
POST /api/v1/admin/queues/subtitle/detect-all?category=xxx&limit=50
POST /api/v1/admin/queues/subtitle/transcribe-all?category=xxx&limit=50
POST /api/v1/admin/queues/subtitle/translate-all?category=xxx&limit=50&target=th
```

**สิ่งที่ต้องทำ Frontend**:

| File | Action |
|------|--------|
| `_vite_starter/src/features/queue/service.ts` | เพิ่ม API calls: `getSubtitleStats()`, `batchDetect()`, `batchTranscribe()`, `batchTranslate()` |
| `_vite_starter/src/features/queue/hooks.ts` | เพิ่ม hooks: `useSubtitleStats()`, `useBatchDetect()`, `useBatchTranscribe()`, `useBatchTranslate()` |
| `_vite_starter/src/features/queue/types.ts` | เพิ่ม types: `SubtitleStatsResponse`, `BatchActionResponse` |
| `_vite_starter/src/constants/api-routes.ts` | เพิ่ม QUEUE_ROUTES: subtitle stats + batch endpoints |
| `_vite_starter/src/features/queue/pages/QueueManagementPage.tsx` | ปรับ `SubtitleTab` component ใหม่ |

**UI ที่ต้องการ**: ดูรายละเอียดใน `SUMMARY/PLAN_QUEUE_BATCH_SUBTITLE.md`

```
SubtitleTab:
├── CategorySelect (dropdown เลือกหมวดหมู่)
├── SubtitleStatsPanel
│   ├── "ยังไม่ detect: 20"    [Detect ทั้งหมด]
│   ├── "ยังไม่ transcribe: 30" [Transcribe ทั้งหมด]
│   └── "ยังไม่ translate: 10"  [Translate ทั้งหมด]
├── SubTabs (Failed / Processing / Queued)
└── JobTable
```

---

### 2. SubTH Redis Cache

**ปัญหา**: title_th ที่แปลไปอาจถูก cache ใน Redis ทำให้เว็บยังแสดง title เก่า

**ต้องตรวจสอบ**:
- SSH เข้า subth server: `ssh -i ~/.ssh/id_ed25519_suekk root@5.223.46.50`
- ดู Redis: `docker exec subth-redis redis-cli KEYS '*video*'`
- ถ้ามี cache → ต้อง clear หลังแปล title ใหม่

---

### 3. Test Subtitle Pipeline End-to-End

**ปัญหา**: subtitle pipeline ที่แก้ไป (3 ปุ่มแยก + entity_id fix) ยังไม่ได้ทดสอบจริง

**ขั้นตอนทดสอบ**:
1. เปิด https://player.suekk.com/videos
2. เลือก video ที่ยังไม่มี subtitle
3. กด **Detect Language** → ดูว่า detect สำเร็จไหม
4. กด **Transcribe** → ดูว่า SRT สร้างสำเร็จไหม
5. กด **Translate** → ดูว่าแปลสำเร็จไหม
6. ดู WebSocket progress bar ว่าแสดงถูกไหม
7. ตรวจ DB ว่า subtitle records ถูก update เป็น ready

**ต้องรัน subtitle workers บนเครื่อง local ก่อน**:
```bash
cd _my_worker
start_subtitle.bat
```

---

### 4. (Optional) เพิ่ม OpenAI LLM Adapter

สำหรับเมื่อ Gemini ban content 18+

**Files ที่ต้องสร้าง**:
- `_my_worker/python/shared/adapters/openai_llm.py` — implements LLMPort
- เพิ่มใน `llm_factory.py`

---

## เอกสารที่ต้องอ่าน

| เรื่อง | File |
|--------|------|
| **Queue batch plan** (UI layout + API endpoints) | `SUMMARY/PLAN_QUEUE_BATCH_SUBTITLE.md` |
| **Port/Adapter architecture** | `SUMMARY/PORT_ADAPTER_ARCHITECTURE.md` |
| **Subtitle pipeline flow** (3 services) | `SUMMARY/SUBTITLE_DETECT_FLOW.md`, `SUBTITLE_TRANSCRIBE_FLOW.md`, `SUBTITLE_TRANSLATE_FLOW.md` |
| **Subtitle bugs ที่แก้ไปแล้ว** | `SUMMARY/SUBTITLE_KNOWN_ISSUES.md` |
| **Server access** | `SUMMARY/SERVER_ACCESS.md` |
| **Title translate overview** | `SUMMARY/OVERVIEW_TITLE_TRANSLATE.md` |
| **NATS overview** | `SUMMARY/OVERVIEW_NATS.md` |

---

## Quick Reference

### SSH
```bash
ssh suekk                    # SUEKK Stream server (5.223.46.39)
ssh -i ~/.ssh/id_ed25519_suekk root@5.223.46.50   # SubTH server
```

### Deploy SUEKK
```bash
cd ___SUEKK_STREAM
git add . && git commit -m "message" && git push
ssh suekk "cd /opt/suekk-stream && git pull && docker compose up -d --build api frontend"
```

### Run Subtitle Workers (Local PC)
```bash
cd _my_worker
start_subtitle.bat   # opens 3 CMD windows (detect, transcribe, translate)
```

### Run Title Translate (Local PC)
```bash
cd _my_worker/python
python -m title_translate.main   # FastAPI port 8002
```
