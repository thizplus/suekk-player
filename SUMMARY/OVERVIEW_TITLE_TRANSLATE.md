# Title Translation Service — Overview

> แปลชื่อเรื่อง video เป็นภาษาไทย + จัดการ cast/tag/category translations
> **Working location**: `D:\Admin\Desktop\MY PROJECT\_suekk_bot\python_translate\`
> **Copy ใน SUEKK**: `_my_worker/python/title_translate/` (refactored, ใช้ LLMPort)
> **SubTH API**: `https://api.subth.com/api/v1`
> **SubTH DB**: `5.223.46.50:5432/subth` (ต้อง SSH tunnel ถ้าเชื่อมตรง)

---

## สิ่งที่ระบบนี้ทำ

**FastAPI server** (port 8002) ที่ทำ 3 อย่างหลัก:

1. **แปลชื่อเรื่อง video** (Title Translation) — ใช้ Gemini LLM
2. **แปลชื่อ cast / tag / category** เป็นภาษาไทย — ใช้ Gemini LLM
3. **เชื่อมต่อกับ subth.com API** — สร้าง/อัพเดท cast, save translations

---

## Architecture

```
┌──────────────────────────────────────────────────────┐
│           Title Translation Service                   │
│           FastAPI (port 8002)                         │
│                                                      │
│  Routes:                                              │
│    /api/klon8/*      — แปลชื่อเรื่อง (กลอน 8 / CN / EN)│
│    /api/translate/*  — แปล cast / tag / category      │
│    /api/cast/*       — จัดการ cast translations        │
│    /api/tag/*        — จัดการ tag translations         │
│    /api/category/*   — จัดการ category translations    │
│    /api/query/*      — query ข้อมูลจาก DB             │
│                                                      │
│  Dependencies:                                        │
│    ├── PostgreSQL (subth DB, port 5433)               │
│    ├── Gemini API (google-generativeai)               │
│    └── subth.com API (REST)                          │
└──────────────────────────────────────────────────────┘
         │                          │
         ▼                          ▼
  ┌─────────────┐          ┌──────────────────┐
  │ subth DB    │          │ api.subth.com    │
  │ PostgreSQL  │          │ (Production API) │
  │ port 5433   │          │                  │
  │             │          │ - Cast CRUD      │
  │ Tables:     │          │ - Video CRUD     │
  │ - videos    │          │ - Auth (JWT)     │
  │ - casts     │          │                  │
  │ - tags      │          └──────────────────┘
  │ - categories│
  │ - video_translations
  │ - cast_translations
  │ - tag_translations
  └─────────────┘
```

---

## Title Translation — 3 Modes

### Mode 1: JAV — กลอน 8 (Default)

แปลชื่อหนัง JAV เป็น **กลอน 8 พยางค์** สไตล์หนังแผ่นไทยยุคเก่า

**Input**: `ABF-295` + title EN + tags + cast
**Output**: `ABF-295 ซับไทย คันตรงหูเอ็นดูที่หรรม Minami Aizawa (มินามิ ไอซาวะ)`

**กฎ**:
- คล้องจอง มีสัมผัสสระ ติดหู
- 8-16 พยางค์
- ใช้คำเล่น คำสแลง 18+ ได้เต็มที่
- ห้ามใส่ชื่อดาราในกลอน

### Mode 2: CN — Chinese AV

แปลตรงๆ แบบ 18+ ภาษาพูด

**Input**: `MD-0312-AD` + title EN
**Output**: `MD-0312-AD ซับไทย ร่างกายบริสุทธิ์ถูกขายเพื่อฝังศพพ่อ CastName`

### Mode 3: EN — Western/Pornhub

แปลตรงๆ ไม่มี video code

**Input**: title EN + cast name
**Output**: `Eva Elfie (อีวา เอลฟี่) สาวบลอนด์โดนเย็ด ซับไทย`

---

## Cast Name Translation

### Flow เมื่อเจอ cast ใหม่

```
1. Search cast ใน subth.com API
2. ถ้าไม่มี → ทับศัพท์ด้วย Gemini → สร้าง cast ผ่าน API
3. ถ้ามีแต่ไม่มีชื่อไทย → ทับศัพท์ → update ผ่าน API
4. Return: "Yua Mikami (ยัว มิคามิ)"
```

### ชื่อญี่ปุ่น vs ชื่อฝรั่ง

| Type | Input | Thai Output | Rule |
|------|-------|-------------|------|
| Japanese | `Mikami Yua` | `ยัว มิคามิ` | **สลับลำดับ** (นามสกุล->ชื่อ เป็น ชื่อ->นามสกุล) |
| Western | `Abella Danger` | `อาเบลล่า เดนเจอร์` | **ไม่สลับ** |
| Username | `babyjee` | `babyjee` | **ไม่แปล** |

---

## Tag / Category Translation

แปล batch ทีละ 50 items ผ่าน Gemini:

| Type | Example EN | Example TH |
|------|-----------|-----------|
| Tag | "Big Tits" | "นมใหญ่" |
| Tag | "Massage" | "นวด" |
| Category | "Drama" | "ดราม่า" |

---

## API Endpoints

### Title Translation (`/api/klon8`)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/translate-only` | แปล title โดยไม่ต้องมี video_id (ไม่ save DB) |
| POST | `/single` | แปล title ทีละเรื่อง (save ได้) |
| POST | `/batch` | แปล batch หลายเรื่อง |
| POST | `/pending` | แปล video ที่ยังไม่มีชื่อไทย (auto-fetch จาก DB) |
| POST | `/by-id/{video_id}` | แปลโดย fetch ข้อมูลจาก subth API |
| GET | `/lookup/tag` | ค้นหาแปล tag จาก cache |
| GET | `/lookup/category` | ค้นหาแปล category จาก cache |

### Batch Translation (`/api/translate`)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/tags/pending` | แปล tags ที่ยังไม่มีภาษาไทย (save DB) |
| POST | `/casts/pending` | แปล casts ที่ยังไม่มีภาษาไทย (save DB) |
| POST | `/categories/pending` | แปล categories (save DB) |
| POST | `/video-titles/pending` | แปล video titles (save DB) |
| POST | `/tags/retranslate` | แปลใหม่ทั้งหมด (overwrite) |
| POST | `/casts/retranslate` | แปลใหม่ทั้งหมด (overwrite) |
| POST | `/sync-all` | แปลทุก type ที่ยังไม่มี |
| POST | `/direct/tags` | แปลตรง (ไม่ save DB) |
| POST | `/direct/casts` | แปลตรง (ไม่ save DB) |
| POST | `/direct/text` | แปล text ทั่วไป |
| POST | `/api/casts/retranslate-all` | Re-translate ผ่าน production API |

### Cast Management (`/api/cast`)

จัดการ cast ผ่าน subth.com API (search, create, update translations)

### Tag / Category Management (`/api/tag`, `/api/category`)

จัดการ tag/category translations

---

## Database (subth)

**Connection**: `postgresql://postgres:postgres@localhost:5433/subth`

### Tables ที่ใช้

| Table | Description |
|-------|-------------|
| `videos` | Video records |
| `video_translations` | `(video_id, lang, title)` — ชื่อเรื่องแต่ละภาษา |
| `casts` | Cast records |
| `cast_translations` | `(cast_id, lang, name)` — ชื่อ cast แต่ละภาษา |
| `tags` | Tag records |
| `tag_translations` | `(tag_id, lang, name)` — ชื่อ tag แต่ละภาษา |
| `categories` | Category records |
| `category_translations` | `(category_id, lang, name)` |
| `video_casts` | Many-to-many: video <-> cast |
| `video_tags` | Many-to-many: video <-> tag |

---

## External API Connection (subth.com)

### Auth
```
POST https://api.subth.com/api/v1/auth/login
Body: { "email": "admin@subth.com", "password": "..." }
Response: { "data": { "token": "JWT..." } }
```

### Cast Operations
```
GET  /api/v1/casts/search?q=Mikami&lang=en&limit=1
GET  /api/v1/casts/{id}
POST /api/v1/casts              (create with translations)
PUT  /api/v1/casts/{id}         (update translations)
```

### Video Operations
```
GET  /api/v1/videos/{id}        (fetch title + casts + tags)
PUT  /api/v1/videos/{id}        (update titles: {"th": "..."})
```

### Auth Token
- Auto-login เมื่อ token หมดอายุ (401 -> re-login -> retry)
- Token cache ใน memory

---

## Environment Variables

| Variable | Value | Description |
|----------|-------|-------------|
| `HOST` | `0.0.0.0` | Server host |
| `PORT` | `8002` | Server port |
| `DATABASE_URL` | `postgresql://...@localhost:5433/subth` | subth database |
| `GEMINI_API_KEY` | `AIzaSy...` | Google Gemini API key |
| `GEMINI_MODEL` | `gemini-2.5-flash` | LLM model |
| `BATCH_SIZE` | `50` | Items per batch |
| `API_SUBTH_URL` | `https://api.subth.com/api/v1` | subth production API |
| `API_EMAIL` | `admin@subth.com` | API login |
| `API_PASSWORD` | `...` | API password |
| `API_SUEKK_EMAIL` | `info@thizplus.com` | SUEKK API login |
| `API_SUEKK_PASSWORD` | `...` | SUEKK API password |

---

## File Structure

```
python_translate/
├── main.py                          # Entry point (uvicorn)
├── requirements.txt                 # Dependencies
├── .env                             # Config
├── data/
│   └── translations_th.json         # Cache file (tag/category translations)
├── app/
│   ├── core/
│   │   ├── config.py                # Pydantic Settings
│   │   └── database.py              # psycopg2 connection
│   ├── domain/
│   │   ├── entities.py              # Tag, Cast, Category, VideoTitle, TranslationResult
│   │   └── interfaces.py            # Abstract interfaces (ITranslationClient, repos)
│   ├── services/
│   │   ├── gemini_client.py         # Gemini API wrapper (translate tags/casts/categories/titles)
│   │   ├── translation_service.py   # Batch translation + DB update orchestration
│   │   └── klon8_service.py         # Title translation (กลอน 8 / CN / EN) + cast management
│   ├── repositories/
│   │   ├── video_title_repository.py
│   │   ├── tag_repository.py
│   │   ├── cast_repository.py
│   │   └── category_repository.py
│   └── api/
│       ├── main.py                  # FastAPI app
│       └── routes/
│           ├── klon8.py             # Title translation endpoints
│           ├── translate.py         # Batch translation endpoints
│           ├── cast.py              # Cast management
│           ├── tag.py               # Tag management
│           ├── category.py          # Category management
│           └── query.py             # Data query endpoints
└── scripts/
    ├── import_translations.py       # Import from JSON
    ├── translate_jav_titles.py      # Batch translate JAV titles
    ├── retranslate_all_casts.py     # Re-translate all casts
    └── fix_suekk_description.py     # Fix descriptions
```

---

## Dependencies

```
fastapi>=0.104.0
uvicorn>=0.24.0
pydantic>=2.5.0
pydantic-settings>=2.1.0
psycopg2-binary>=2.9.9
google-generativeai>=0.3.0
python-dotenv>=1.0.0
```

---

## ข้อพิจารณาสำหรับการย้ายเข้า SUEKK Stream Workers

### สิ่งที่ต่างจาก subtitle workers

| Item | Subtitle Workers | Title Translate |
|------|-----------------|-----------------|
| Architecture | NATS consumer (pull jobs) | FastAPI server (HTTP) |
| Trigger | NATS job queue | HTTP API call |
| Database | SUEKK DB (via API) | subth DB (direct psycopg2) |
| GPU | Yes (Whisper, CUDA) | No (Gemini API only) |
| Dependencies | boto3, nats, torch, faster-whisper | fastapi, psycopg2, google-generativeai |

### Options สำหรับการย้าย

**Option A: ย้ายเป็น FastAPI server แยก** (ง่ายสุด)
- Copy folder เข้า `_my_worker/python/title_translate/`
- แก้ .env ให้ชี้ DB/API ที่ถูก
- รัน `python main.py` แยกต่างหาก
- ไม่ต้องเปลี่ยน architecture

**Option B: แปลงเป็น NATS consumer** (integrate กับ SUEKK Stream)
- สร้าง stream `TITLE_TRANSLATE_JOBS`
- API publish job เมื่อ video ใหม่ถูกสร้าง
- Worker consume + แปล + update DB
- ต้องเขียน consumer + handler ใหม่

**Option C: เป็น scheduled job** (batch)
- Cron job ทุก X นาที
- Query video ที่ยังไม่มีชื่อไทย -> แปล batch -> save
- ไม่ต้องเปลี่ยน architecture มาก

### Recommendation

**Option A** เหมาะสุดเป็น step แรก:
- ย้าย folder เข้ามา
- ปรับ config/env
- เพิ่มใน `start_subtitle.bat` หรือสร้าง `start_translate.bat`
- ค่อย integrate กับ NATS ทีหลัง (ถ้าต้องการ)

---

## วิธีใช้งานจริง (Quick Start)

### 1. แปลชื่อเรื่อง JAV (Batch — ใช้บ่อยสุด)

ใช้ script ที่ `_suekk_bot/python_translate/` เชื่อม SubTH API โดยตรง (ไม่ต้องเชื่อม DB)

```bash
cd "D:/Admin/Desktop/MY PROJECT/_suekk_bot/python_translate"
python scripts/translate_jav_titles.py --batch 20 --start 1 --max-pages 10
```

**Flow**: Login SubTH API → ดึง video ที่ missing_th=true → Gemini แปลกลอน 8 → PUT update title_th กลับ API

**Options**:
- `--batch 20` — จำนวน video ต่อ page
- `--start 1` — เริ่มจาก page ที่เท่าไหร่
- `--max-pages 10` — จำนวน page สูงสุด

### 2. รัน FastAPI Server (สำหรับ translate ทีละเรื่อง)

```bash
cd "D:/Admin/Desktop/MY PROJECT/_suekk_bot/python_translate"
python main.py
# Server: http://localhost:8002
# Docs: http://localhost:8002/docs
```

### 3. ตรวจสอบก่อนรัน

| Check | Command |
|-------|---------|
| API key ยังใช้ได้? | ดู `.env` → `GEMINI_API_KEY` ถ้า expired ใช้ key จาก `_my_worker/.env` |
| มี video รอแปลกี่รายการ? | `GET https://api.subth.com/api/v1/videos?limit=1&category=censored-jav&missing_th=true` → ดู `meta.total` |
| Login ได้? | email: `admin@subth.com` |

### 4. Gemini API Key

- **Key เก่า (expired)**: `AIzaSyBoG2TRIoT...` — หมดอายุแล้ว
- **Key ใหม่ (working)**: ดูใน `_my_worker/.env` → `GEMINI_API_KEY`
- ถ้า key หมดอายุอีก ไป Google AI Studio สร้างใหม่

### 5. SSH Tunnel (ถ้าต้องเชื่อม DB ตรง)

SubTH PostgreSQL ไม่เปิด port จากข้างนอก ต้องใช้ SSH tunnel:

```bash
# สร้าง tunnel: local:5433 → server:5433 (SubTH postgres)
ssh -i ~/.ssh/id_ed25519_suekk -f -N -L 5433:127.0.0.1:5433 root@5.223.46.50

# ทดสอบ
psql postgresql://postgres:postgres@localhost:5433/subth
```

> **หมายเหตุ**: Script `translate_jav_titles.py` ไม่ต้องใช้ tunnel เพราะเชื่อมผ่าน SubTH API ไม่ได้เชื่อม DB ตรง

---

## 2 Versions ของ Title Translate

| | Old (`_suekk_bot/python_translate/`) | New (`_my_worker/python/title_translate/`) |
|---|---|---|
| **Status** | **ใช้งานจริง** | Refactored (ใช้ LLMPort) |
| **LLM** | google.generativeai (deprecated) | LLMPort + Factory |
| **Script แปล batch** | `scripts/translate_jav_titles.py` | ไม่มี (ยังไม่ port) |
| **DB connection** | psycopg2 ตรง | psycopg2 ตรง |
| **ใช้ SubTH API** | Yes (script ใช้) | Yes (config มี) |

**สรุป**: ใช้ Old version (`_suekk_bot/python_translate/`) สำหรับแปล batch จริง
