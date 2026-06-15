# Title Translation Service — Overview

> แปลชื่อเรื่อง video เป็นภาษาไทย + จัดการ cast/tag/category translations
> **Working location**: `_my_worker/python/title_translate/`
> **SubTH API**: `https://api.subth.com/api/v1`
> **SubTH DB**: `5.223.46.50:5432/subth` (ต้อง SSH tunnel ถ้าเชื่อมตรง)

---

## สิ่งที่ระบบนี้ทำ

**FastAPI server** (port 8002) ที่ทำ 3 อย่างหลัก:

1. **แปลชื่อเรื่อง video** (Title Translation) — ใช้ LLMPort (Gemini/OpenAI)
2. **แปลชื่อ cast / tag / category** เป็นภาษาไทย — ใช้ LLMPort
3. **เชื่อมต่อกับ subth.com API** — สร้าง/อัพเดท cast, save translations

---

## Architecture

```
title_translate/ (FastAPI :8002)
│
├── LLMPort (shared/ports/)     ← Gemini/OpenAI swappable via env
├── SubTH DB (psycopg2)        ← cast/tag/category translations
└── SubTH API (REST)           ← cast CRUD, video update
         │                          │
         v                          v
  ┌─────────────┐          ┌──────────────────┐
  │ subth DB    │          │ api.subth.com    │
  │ PostgreSQL  │          │ (Production API) │
  │ port 5433   │          │ - Cast CRUD      │
  │ (via tunnel)│          │ - Video CRUD     │
  │             │          │ - Auth (JWT)     │
  │ Tables:     │          └──────────────────┘
  │ - videos    │
  │ - casts     │
  │ - tags      │
  │ - categories│
  │ - *_translations
  └─────────────┘
```

---

## Quick Start — วิธีใช้งานจริง

### 1. รัน Server

```bash
cd _my_worker/python
python -m title_translate.main
# Server: http://localhost:8002
# Docs: http://localhost:8002/docs
```

**Config**: โหลด `.env` จาก `_my_worker/.env` (shared กับ worker อื่น)
ต้องมี `GEMINI_API_KEY` อยู่ใน `.env`

### 2. แปลชื่อเรื่อง JAV (ทีละเรื่อง)

```bash
# ดึง video ที่ยังไม่มีชื่อไทยจาก SubTH
curl -s "https://api.subth.com/api/v1/videos?limit=5&missing_th=true&category=censored-jav"

# แปลกลอน 8 ผ่าน title_translate service
curl -X POST http://localhost:8002/api/klon8/translate-only \
  -H "Content-Type: application/json" \
  -d '{"title_en":"SQTE-694 This Girl Is Insane...","tags":["beautiful girl","creampie"],"source_type":"jav"}'

# Response: {"success":true,"data":{"title_th":"SQTE-694 ซับไทย นมงามล้นใจไวไฟเหลือเกิน"}}
```

### 3. Save กลับ SubTH API

```bash
# Login
TOKEN=$(curl -s -X POST https://api.subth.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@subth.com","password":"..."}' \
  | python -c "import sys,json; print(json.load(sys.stdin)['data']['token'])")

# PUT ด้วย field "titles" (map[lang]title)
curl -X PUT "https://api.subth.com/api/v1/videos/{video_id}" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"titles":{"th":"SQTE-694 ซับไทย นมงามล้นใจไวไฟเหลือเกิน"}}'
```

**สำคัญ**: SubTH API ใช้ field `titles` (ไม่ใช่ `title_th` หรือ `translations`)

### 4. ตรวจสอบก่อนรัน

| Check | วิธี |
|-------|------|
| Gemini API Key | ดู `_my_worker/.env` → `GEMINI_API_KEY` |
| มี video รอแปล? | `GET https://api.subth.com/api/v1/videos?limit=1&missing_th=true` → ดู `meta.total` |
| Login SubTH | `admin@subth.com` |

### 5. SSH Tunnel (เฉพาะต่อ DB ตรง)

```bash
ssh -i ~/.ssh/id_ed25519_suekk -f -N -L 5433:127.0.0.1:5433 root@5.223.46.50
```

> Endpoint `/api/klon8/translate-only` ไม่ต้องใช้ tunnel (ใช้แค่ Gemini API)
> Endpoint `/api/translate/*` ต้องต่อ DB (ใช้ tunnel)

---

## Title Translation — 3 Modes

### Mode 1: JAV — กลอน 8 (Default)

แปลชื่อหนัง JAV เป็น **กลอน 8 พยางค์** สไตล์หนังแผ่นไทยยุคเก่า

**Input**: `SQTE-694` + title EN + tags
**Output**: `SQTE-694 ซับไทย นมงามล้นใจไวไฟเหลือเกิน`

**กฎ**:
- คล้องจอง มีสัมผัสสระ ติดหู
- 8-16 พยางค์
- ใช้คำเล่น คำสแลง 18+ ได้เต็มที่
- ห้ามใส่ชื่อดาราในกลอน

### Mode 2: CN — Chinese AV

แปลตรงๆ แบบ 18+ ภาษาพูด

**Input**: `MD-0312` + title EN
**Output**: `MD-0312 ซับไทย ร่างกายบริสุทธิ์ถูกขายเพื่อฝังศพพ่อ`

### Mode 3: EN — Western/Pornhub

แปลตรงๆ + ทับศัพท์ชื่อ cast

**Input**: title EN + cast name
**Output**: `Eva Elfie (อีวา เอลฟี่) สาวบลอนด์โดนเย็ด ซับไทย`

---

## API Endpoints

### Title Translation (`/api/klon8`)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/translate-only` | แปล title (ไม่ save DB, ใช้แค่ Gemini) |

### Batch Translation (`/api/translate`)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/tags/pending?limit=50` | แปล tags ที่ยังไม่มีไทย → save SubTH DB |
| POST | `/casts/pending?limit=50` | แปล casts → save SubTH DB |
| POST | `/categories/pending` | แปล categories → save SubTH DB |
| POST | `/sync-all?limit=100` | แปลทุก type ที่ pending |
| POST | `/direct/tags` | แปลตรง ไม่ต้องต่อ DB |
| POST | `/direct/casts` | แปลตรง ไม่ต้องต่อ DB |

### Query Translation (`/api/query`)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/translate` | แปลคำค้นหาไทย→อังกฤษ |
| GET | `/detect-lang?query=xxx` | ตรวจว่าเป็นไทยหรืออังกฤษ |

---

## Cast Name Translation

### ชื่อญี่ปุ่น vs ชื่อฝรั่ง

| Type | Input | Thai Output | Rule |
|------|-------|-------------|------|
| Japanese | `Mikami Yua` | `ยัว มิคามิ` | **สลับลำดับ** |
| Western | `Abella Danger` | `อาเบลล่า เดนเจอร์` | **ไม่สลับ** |
| Username | `babyjee` | `babyjee` | **ไม่แปล** |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GEMINI_API_KEY` | - | **required** |
| `GEMINI_MODEL` | `gemini-2.5-flash` | LLM model |
| `LLM_PROVIDER` / `TITLE_LLM_PROVIDER` | `gemini` | LLM provider |
| `DATABASE_URL` | `postgresql://...@localhost:5433/subth` | SubTH DB (ต้อง tunnel) |
| `API_SUBTH_URL` | `https://api.subth.com/api/v1` | SubTH API |
| `API_EMAIL` | - | SubTH login |
| `API_PASSWORD` | - | SubTH password |
| `HOST` | `0.0.0.0` | Server host |
| `PORT` | `8002` | Server port |

Config โหลดจาก `_my_worker/.env` (shared), `_my_worker/python/.env`, หรือ `.env` ใน cwd

---

## File Structure

```
_my_worker/python/title_translate/
├── main.py              # FastAPI entry point (uvicorn :8002)
├── config.py            # Pydantic Settings (loads shared .env)
├── database.py          # psycopg2 connection to SubTH DB
├── entities.py          # Tag, Cast, Category, VideoTitle, TranslationResult
├── dependencies.py      # LLM singleton (via shared LLMPort)
├── prompts.py           # All LLM prompts (klon8, CN, EN, batch)
├── repositories/
│   ├── tag_repo.py      # Tag DB operations
│   ├── cast_repo.py     # Cast DB operations
│   ├── category_repo.py # Category DB operations
│   └── video_title_repo.py
└── routes/
    ├── klon8.py         # /api/klon8/* — title translation
    ├── translate.py     # /api/translate/* — batch translation
    └── query.py         # /api/query/* — search query translation
```

---

## SubTH API — Video Update Format

```bash
# PUT /api/v1/videos/{id}
# Body: { "titles": { "th": "ชื่อภาษาไทย" } }

curl -X PUT "https://api.subth.com/api/v1/videos/{id}" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"titles":{"th":"SQTE-694 ซับไทย นมงามล้นใจไวไฟเหลือเกิน"}}'
```

**Field name**: `titles` (map[string]string) — ไม่ใช่ `title_th`, `translations`, หรือ `th`

---

## Important: Windows Encoding

**ห้ามใช้ curl ส่งภาษาไทยจาก bash บน Windows** — จะเพี้ยน!
ต้องใช้ **Python requests** เท่านั้น:

```python
import requests
# Save title กลับ SubTH
requests.put(f"{API}/videos/{vid}",
    json={"titles": {"th": title_th}},
    headers={"Authorization": f"Bearer {token}"},
)
```

---

## Tested & Verified (2026-06-15)

| Step | Status | Detail |
|------|--------|--------|
| รัน server | OK | `python -m title_translate.main` (port 8002) |
| แปลกลอน 8 + cast | OK | `/api/klon8/translate-only` + cast auto-transliterate |
| Save SubTH | OK | Python requests + `{"titles":{"th":"..."}}` |
| Batch 22 videos | OK | 22/22 แปล + save สำเร็จ |
| Cast auto-format | OK | `Kuroshima Rei` → `Rei Kuroshima (เรย์ คุโรชิมะ)` |
