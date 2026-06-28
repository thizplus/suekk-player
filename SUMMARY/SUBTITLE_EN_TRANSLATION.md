# Subtitle EN Translation — สรุปการเพิ่มแปล JP → EN

## สถานะ: API แก้แล้ว / Worker + Frontend ไม่ต้องแก้

---

## สิ่งที่แก้ไข (2 ไฟล์)

### 1. `_gofiber_starter/application/serviceimpl/subtitle_service_impl.go`

```go
// ก่อน (hardcode th อย่างเดียว)
func getTranslationTargets(sourceLanguage string) []string {
    if sourceLanguage == "th" { return []string{"en"} }
    return []string{"th"}
}

// หลัง (เพิ่ม en)
func getTranslationTargets(sourceLanguage string) []string {
    if sourceLanguage == "th" { return []string{"en"} }
    if sourceLanguage == "en" { return []string{"th"} }
    return []string{"th", "en"}  // ja, zh, ko, ru → ได้ทั้ง th และ en
}
```

### 2. `_gofiber_starter/domain/dto/subtitle.go`

```go
// ก่อน
translationPairs[lang.Code] = []string{"th"}

// หลัง
switch lang.Code {
case "th": translationPairs[lang.Code] = []string{"en"}
case "en": translationPairs[lang.Code] = []string{"th"}
default:   translationPairs[lang.Code] = []string{"th", "en"}
}
```

---

## Translation Pairs หลังแก้

| Source | Target ที่เลือกได้ |
|--------|-------------------|
| ja (ญี่ปุ่น) | th, **en** |
| zh (จีน) | th, **en** |
| ko (เกาหลี) | th, **en** |
| ru (รัสเซีย) | th, **en** |
| en (อังกฤษ) | th |
| th (ไทย) | en |

---

## Frontend — ไม่ต้องแก้

- Dropdown ดึง `translationPairs` จาก API (`GET /api/v1/subtitles/languages`)
- `LANGUAGE_LABELS["en"]` = "อังกฤษ" + `LANGUAGE_FLAGS["en"]` = flag มีอยู่แล้ว
- เมื่อ API คืน `["th", "en"]` → dropdown จะโชว์ทั้ง 2 ตัวเลือกอัตโนมัติ
- Service ส่ง `targetLanguages: ["en"]` ไป API ได้เลย

**ไฟล์ที่เกี่ยวข้อง (ไม่ต้องแก้):**
- `_vite_starter/src/features/subtitle/components/SubtitlePanel.tsx` — dropdown UI
- `_vite_starter/src/features/subtitle/hooks.ts` — useTranslate() mutation
- `_vite_starter/src/features/subtitle/service.ts` — API call
- `_vite_starter/src/constants/enums.ts` — LANGUAGE_LABELS, LANGUAGE_FLAGS

---

## Worker (Python) — ไม่ต้องแก้ แต่คุณภาพต่างกัน

Worker รองรับ multi-language อยู่แล้ว — routing logic:

```python
# prompts.py
if source_lang.code == "ja" and target_lang.code == "th":
    return _build_av_cluster_prompt(...)   # AV-specific กลอน 8
else:
    return _build_generic_cluster_prompt(...)  # Generic prompt
```

### เปรียบเทียบคุณภาพ

| Feature | JP → TH | JP → EN |
|---------|---------|---------|
| Prompt | AV-specific (กลอน 8, คำหยาบไทย) | Generic (natural translation) |
| Pre-translate moaning | dict ภาษาไทย | ข้าม → LLM แปลเอง |
| Post-process (formal→casual) | แทนที่คำทางการไทย | ข้าม (ไม่มี dict EN) |
| Silent failure detection | เช็ค JP chars ค้างใน TH | ข้าม |
| **ผลลัพธ์** | ดีมาก (customized) | ใช้ได้ดี (generic) |

**ไฟล์ที่เกี่ยวข้อง (ไม่ต้องแก้):**
- `_my_worker/python/subtitle_translate/handler.py`
- `_my_worker/python/subtitle_translate/prompts.py`
- `_my_worker/python/shared/entities.py` — LanguageCode รองรับ "en" อยู่แล้ว

---

## Flow: JP → EN (เหมือน JP → TH ทุกประการ)

```
User กดเลือก "อังกฤษ" ใน dropdown → กด "แปล"
  |
  v
Frontend: POST /api/v1/videos/{id}/subtitle/translate
  Body: { "targetLanguages": ["en"] }
  |
  v
API: สร้าง Subtitle record (language="en", type="translated", status="queued")
  |
  v
NATS: publish job → stream SUBTITLE_JOBS
  Payload: { target_languages: ["en"], source_language: "ja", ... }
  |
  v
Worker: รับ job → download ja.srt จาก S3
  → แปลด้วย generic prompt (LLM)
  → สร้าง en.srt
  → upload S3
  |
  v
Worker: callback → API update Subtitle.srt_path + status="ready"
  |
  v
WebSocket: progress update → Frontend แสดงผล
```

---

## Deploy

```bash
# Rebuild API container
ssh suekk "cd /opt/suekk-stream && git pull && docker compose up -d --build api"
```

Frontend ไม่ต้อง rebuild (ไม่มีการแก้ไข)

---

## อนาคต (Optional Enhancement)

ถ้าต้องการคุณภาพ EN ดีขึ้น:
1. เพิ่ม `_build_av_en_cluster_prompt()` ใน `prompts.py` — AV-specific English rules
2. เพิ่ม moaning dict ภาษา EN ใน `translation_dict.json`
3. เพิ่ม post-process สำหรับ EN (ถ้าจำเป็น)
