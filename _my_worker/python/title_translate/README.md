# Title Translate Service

แปลชื่อ video เป็นภาษาไทย (กลอน 8) + ทับศัพท์ชื่อนักแสดง สำหรับ SubTH

## Quick Start

```bash
cd _my_worker/python
python -m title_translate.main
# Server: http://localhost:8002
```

ต้องมี `GEMINI_API_KEY` ใน `_my_worker/.env` (shared กับ worker อื่น)

---

## วิธีแปล video จาก SubTH (ใช้ Python script)

**สำคัญ: ห้ามใช้ curl ส่งภาษาไทยจาก bash บน Windows — ต้องใช้ Python requests เท่านั้น**

### แปลทั้งหมดที่ยังไม่มีไทย (batch)

```python
import requests, time

API = "https://api.subth.com/api/v1"
TRANSLATE = "http://localhost:8002/api/klon8/translate-only"

# Login
token = requests.post(f"{API}/auth/login", json={
    "email": "admin@subth.com",
    "password": "..."
}).json()["data"]["token"]
headers = {"Authorization": f"Bearer {token}"}

# ดึง video ที่ยังไม่มีไทย
videos = requests.get(f"{API}/videos", params={
    "missing_th": "true",
    "category": "censored-jav",
    "limit": 50,
}).json()["data"]

for v in videos:
    # แปล (service จะทับศัพท์ cast ให้อัตโนมัติ)
    resp = requests.post(TRANSLATE, json={
        "title_en": v["title"],
        "tags": [t["name"] for t in v.get("tags", [])],
        "cast_name": v["casts"][0]["name"] if v.get("casts") else "",
        "source_type": "jav",
    }).json()

    if resp["success"]:
        # Save กลับ SubTH
        requests.put(f"{API}/videos/{v['id']}",
            json={"titles": {"th": resp["data"]["title_th"]}},
            headers=headers,
        )
        print(f"[OK] {resp['data']['title_th']}")

    time.sleep(0.5)
```

### แปลทีละเรื่อง (ไม่ save)

```python
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
- `SQTE-694 ซับไทย สาวสวยขาอ่อน อมก่อนแล้วแตกใน Hikari Tomoe (ฮิคาริ โทโมเอะ)`
- `FNS-229 ซับไทย น้องสาวนมโตโดนพี่ชายขยี้ Sonoka Misaki (โซโนกะ มิซากิ)`

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

| Variable | Description |
|----------|-------------|
| `GEMINI_API_KEY` | **required** |
| `GEMINI_MODEL` | default `gemini-2.5-flash` |
| `API_SUBTH_URL` | default `https://api.subth.com/api/v1` |
| `API_EMAIL` | SubTH admin email |
| `API_PASSWORD` | SubTH admin password |

---

## เช็คก่อนรัน

```bash
# มี video รอแปลกี่รายการ?
curl "https://api.subth.com/api/v1/videos?limit=1&missing_th=true" | python -m json.tool | grep total
```
