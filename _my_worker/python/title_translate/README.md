# Title Translate Service

แปลชื่อ video เป็นภาษาไทย (กลอน 8) + ทับศัพท์ชื่อนักแสดง สำหรับ SubTH

## Quick Start

```bash
cd _my_worker/python

# 1. รัน service
python -m title_translate.main
# Server: http://localhost:8002

# 2. แปล batch (ทดสอบก่อน)
python batch_translate.py --limit 5 --dry-run

# 3. แปลจริง (save กลับ SubTH)
python batch_translate.py --limit 50
```

ต้องมี `GEMINI_API_KEY`, `API_EMAIL`, `API_PASSWORD` ใน `_my_worker/.env`

---

## Batch Translate Script (แนะนำ)

ใช้ `batch_translate.py` สำหรับแปลหลายรายการ — โหลด credentials จาก `.env` อัตโนมัติ

```bash
cd _my_worker/python

# แปลทั้งหมดที่รอ (default 50 รายการ)
python batch_translate.py

# กำหนดจำนวน
python batch_translate.py --limit 100

# เฉพาะ category
python batch_translate.py --category censored-jav
python batch_translate.py --category uncensored-jav

# ทดสอบ (ไม่ save กลับ SubTH)
python batch_translate.py --dry-run

# ปรับ delay ระหว่างรายการ (default 0.5 วินาที)
python batch_translate.py --delay 1.0
```

### ตัวอย่างผลลัพธ์

```
[LOGIN] OK - admin@subth.com
[QUERY] พบ 8 รายการรอแปล (ดึงมา 8)
[1/8] [OK] SNOS-256 ซับไทย เมียพี่มีชู้หนูขอรุมรัก Hana Kuraki (ฮานะ คุรากิ)
[2/8] [OK] DSOD-008 ซับไทย ยาปลุกฤทธิ์แรง แฉะแข่งเหงื่อไหล Mei Itsukaichi (เมอิ อิตสึกะอิจิ)
...
[SUMMARY] Total: 8 | Success: 8 | Failed: 0
```

---

## แปลทีละเรื่อง (API ตรง)

**สำคัญ: ห้ามใช้ curl ส่งภาษาไทยจาก bash บน Windows — ต้องใช้ Python requests เท่านั้น**

```python
import requests

resp = requests.post("http://localhost:8002/api/klon8/translate-only", json={
    "title_en": "SQTE-694 This Girl Is Insane...",
    "tags": ["beautiful girl", "creampie"],
    "cast_name": "Tomoe Hikari",       # ชื่อจาก SubTH API
    "source_type": "jav",              # jav / cn / en
}).json()

# ผลลัพธ์:
# title_th: "SQTE-694 ซับไทย สาวสวยขาอ่อน อมก่อนแล้วแตกใน Hikari Tomoe (ฮิคาริ โทโมเอะ)"
# cast_formatted: "Hikari Tomoe (ฮิคาริ โทโมเอะ)"
```

---

## Output Format

```
{CODE} ซับไทย {กลอน8} {CastEN} ({CastTH})
```

ตัวอย่าง:
- `SNOS-275 ซับไทย หน้าสวยใสใจอยากคลุกวงใน Saika Kawakita (ไซกะ คาวาคิตะ)`
- `FNS-222 ซับไทย ฟิตเนสสุดหื่น คลึงคลิตจนคราง Ranran Fujii (รันรัน ฟูจิอิ)`

---

## Cast Auto-Transliterate

เมื่อส่ง `cast_name` มา service จะ:

1. Search cast ใน SubTH API
2. ถ้ายังไม่มีชื่อไทย → ทับศัพท์ด้วย Gemini
3. สลับชื่อ JP order: `Kuroshima Rei` → `Rei Kuroshima`
4. Save ชื่อไทยกลับ SubTH API
5. Return: `Rei Kuroshima (เรย์ คุโรชิมะ)`

---

## Source Types

| Type | ใช้กับ | Format |
|------|--------|--------|
| `jav` | JAV (default) | กลอน 8 คล้องจอง |
| `cn` | Chinese AV | แปลตรง 18+ ภาษาพูด |
| `en` | Western/Pornhub | แปลตรง + ทับศัพท์ cast |

---

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/` | Service info (provider, model) |
| `GET` | `/health` | Health check |
| `POST` | `/api/klon8/translate-only` | แปล title 1 รายการ (ไม่ต้องต่อ DB) |
| `POST` | `/api/translate/tags/pending` | แปล tags ที่ยังไม่มี TH (ต้องต่อ subth DB) |
| `POST` | `/api/translate/casts/pending` | แปล casts ที่ยังไม่มี TH (ต้องต่อ subth DB) |
| `POST` | `/api/translate/categories/pending` | แปล categories (ต้องต่อ subth DB) |
| `POST` | `/api/translate/sync-all` | แปลทั้งหมด (ต้องต่อ subth DB) |
| `POST` | `/api/translate/direct/tags` | แปล tags ตรง (ไม่ save DB) |
| `POST` | `/api/translate/direct/casts` | แปล casts ตรง (ไม่ save DB) |
| `POST` | `/api/query/translate` | แปล search query ไทย→อังกฤษ |

---

## SubTH API — Save Format

```python
requests.put(f"https://api.subth.com/api/v1/videos/{video_id}",
    json={"titles": {"th": "ชื่อไทย"}},   # field "titles" ไม่ใช่ "title_th"
    headers={"Authorization": f"Bearer {token}"},
)
```

---

## Environment Variables

โหลดจาก `_my_worker/.env` อัตโนมัติ ตัวที่ใช้:

| Variable | Description | Required |
|----------|-------------|----------|
| `GEMINI_API_KEY` | Gemini API key | **Yes** |
| `GEMINI_MODEL` | default `gemini-2.5-flash` | No |
| `API_SUBTH_URL` | default `https://api.subth.com/api/v1` | No |
| `API_EMAIL` | SubTH admin email | Yes (batch) |
| `API_PASSWORD` | SubTH admin password | Yes (batch) |

---

## เช็คก่อนรัน

```bash
# มี video รอแปลกี่รายการ?
curl "https://api.subth.com/api/v1/videos?limit=1&missing_th=true" | python -m json.tool | grep total

# service รันอยู่ไหม?
curl http://localhost:8002/health
```
