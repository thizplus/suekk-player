# Subtitle Pipeline — Known Issues & Bugs

> จาก code audit ละเอียด ทั้ง API side และ Python worker side
> วันที่ตรวจ: 2026-06-05
> อัพเดทล่าสุด: 2026-06-05 (แก้ไข CRITICAL bugs แล้ว)

---

## สรุปปัญหาทั้งหมด

| # | Severity | Component | Bug | Status |
|---|----------|-----------|-----|--------|
| 1 | ~~CRITICAL~~ | Consumer names | ตรวจสอบแล้ว — API สร้าง 3 consumers ถูกต้อง | **NOT A BUG** |
| 2 | **CRITICAL** | Translate input field | `source_srt_path` (API) vs `srt_path` (Python) | **FIXED** |
| 3 | **CRITICAL** | Transcribe completion | `entity_id` normalize ผิด -> `SubtitleID` ว่าง | **FIXED** |
| 4 | **CRITICAL** | Translate completion | Python worker ไม่ส่ง `subtitle_id` | **FIXED** |
| 5 | **CRITICAL** | entity_id normalization | Subscriber แปลง blindly ไม่ดู entity_type | **FIXED** |
| 6 | **HIGH** | Translate output format | API อ่าน `srt_path` ไม่ได้จาก `translations` dict | **FIXED** |
| 7 | **HIGH** | Translate architecture | 1 job = หลาย targets, API คาดหวัง per-language completion | **FIXED** |
| 8 | **MEDIUM** | RetryStuckSubtitles | ส่ง `language="auto"` ทำให้ output path ผิด | TODO |
| 9 | **MEDIUM** | Race condition | NATS reconnect -> duplicate subtitle records | TODO |
| 10 | ~~MEDIUM~~ | Chain sequencing | Translate trigger ก่อน DB commit | **RESOLVED** (ลบ auto-chain) |
| 11 | **LOW** | Stage classification | Shared stages classify ผิด type | TODO |
| 12 | **LOW** | Config mismatch | `GEMINI_MODEL` default ต่างกัน | TODO |
| 13 | ~~LOW~~ | current_language | Python worker ไม่ส่ง `current_language` | **FIXED** |

---

## CRITICAL Issues (ต้องแก้ก่อน chain จะทำงานได้)

---

### Bug 1: Consumer Name Mismatch — Workers Start ไม่ได้

**ปัญหา**: API สร้าง consumer ชื่อ `SUBTITLE_WORKER` (ตัวเดียว) แต่ Python workers ต้องการ 3 consumers แยก

**API สร้าง** (`_gofiber_starter/infrastructure/nats/types.go`):
```go
SubtitleConsumerName = "SUBTITLE_WORKER"  // consumer เดียว
```

**Python workers ต้องการ** (`_my_worker/python/shared/config.py`):
```python
"subtitle_detect":    ("SUBTITLE_JOBS", "SUBTITLE_DETECT_WORKER",    "jobs.subtitle.detect")
"subtitle_transcribe":("SUBTITLE_JOBS", "SUBTITLE_TRANSCRIBE_WORKER", "jobs.subtitle.transcribe")
"subtitle_translate": ("SUBTITLE_JOBS", "SUBTITLE_TRANSLATE_WORKER",  "jobs.subtitle.translate")
```

**ผลกระทบ**: Python workers พยายาม connect ถึง consumer `SUBTITLE_DETECT_WORKER` ที่ไม่มีอยู่ -> error "Consumer not found" -> workers ทำงานไม่ได้เลย

**วิธีแก้**: API ต้องสร้าง 3 consumers แยก:
- `SUBTITLE_DETECT_WORKER` filter `jobs.subtitle.detect`
- `SUBTITLE_TRANSCRIBE_WORKER` filter `jobs.subtitle.transcribe`
- `SUBTITLE_TRANSLATE_WORKER` filter `jobs.subtitle.translate`

---

### Bug 2: Translate Input Field Name Mismatch

**ปัญหา**: API ส่ง field ชื่อ `source_srt_path` แต่ Python worker อ่าน `srt_path`

**API sends** (`types.go`):
```go
type SubtitleTranslateInput struct {
    SourceSRTPath string `json:"source_srt_path"`  // <- ชื่อนี้
}
```

**Python reads** (`subtitle_translate/handler.py`):
```python
srt_path = input_data.get("srt_path", "")  // <- แต่อ่านชื่อนี้!
```

**ผลกระทบ**: Translate worker ได้ `srt_path = ""` ทุกครั้ง -> download from S3 ด้วย path ว่าง -> fail ทันที

**วิธีแก้**: แก้ Python worker ให้อ่าน `source_srt_path` หรือแก้ API ให้ส่ง `srt_path`

---

### Bug 3: Transcribe Completion — `SubtitleID` ว่างเสมอ

**ปัญหา**: Chain ที่ควรจะเป็น:
```
Python worker ส่ง entity_id = subtitle_uuid
-> API subscriber แปลง entity_id เป็น... video_id?!
-> SubtitleID ว่าง
-> handleTranscribeCompleted() return early
-> DB ไม่ update, chain หยุด
```

**Root cause** (`subscriber.go`):
```go
// Normalize entity_id -> video_id (backward compat)
if update.EntityID != "" && update.VideoID == "" {
    update.VideoID = update.EntityID  // subtitle UUID ถูกยัดใส่ VideoID!
}
```

สำหรับ transcribe job, `entity_id` = subtitle UUID (ไม่ใช่ video UUID)
แต่ subscriber แปลง blindly เป็น `VideoID` ทำให้:
1. `VideoID` กลายเป็น subtitle UUID (ผิด)
2. `SubtitleID` ยังว่างอยู่

**ใน progress_broadcaster.go**:
```go
func (pb *ProgressBroadcaster) handleTranscribeCompleted(update ports.ProgressData) {
    if update.SubtitleID == "" {
        logger.Warn("Transcribe completed but no subtitle_id")
        return  // <- EARLY EXIT! DB never updated
    }
}
```

**ผลกระทบ**:
- Subtitle record ไม่ถูก update เป็น `ready`
- `srt_path` ไม่ถูกบันทึกใน DB
- Auto-trigger translate ถูกข้าม (เพราะ subtitle ยังไม่ ready)
- **Chain ตายตรงนี้ทุกครั้ง**

**วิธีแก้ (ต้องแก้ทั้ง 2 ฝั่ง)**:

Option A — Python worker ส่ง `subtitle_id` field เพิ่ม:
```python
# progress.py - เพิ่ม subtitle_id ใน ProgressUpdate
"subtitle_id": self.subtitle_id  # set from meta.entity_id when job_type is transcribe
```

Option B — API subscriber แยก logic ตาม entity_type:
```go
if update.EntityType == "subtitle" {
    update.SubtitleID = update.EntityID  // ไม่ใช่ VideoID
} else {
    update.VideoID = update.EntityID
}
```

---

### Bug 4: Translate Completion — ไม่มี `subtitle_id`

**ปัญหา**: เหมือน Bug 3 แต่สำหรับ translate

- Translate job ใช้ `entity_type=video`, `entity_id=video_uuid`
- Python worker ส่ง `entity_id` = video UUID (ถูกต้องสำหรับ VideoID)
- แต่ไม่มี `subtitle_id` field ใน Python `ProgressUpdate` dataclass เลย

**ใน progress_broadcaster.go**:
```go
func (pb *ProgressBroadcaster) handleTranslateCompleted(update ports.ProgressData) {
    if update.SubtitleID == "" {
        logger.Warn("Translate completed but no subtitle_id")
        return  // <- EARLY EXIT! DB never updated
    }
}
```

**ผลกระทบ**: Translated subtitle records ไม่เคยถูก mark เป็น `ready`

**วิธีแก้**: Python `ProgressUpdate` ต้องมี `subtitle_id` field เพิ่ม โดยสำหรับ translate job ต้องส่ง subtitle_id ของแต่ละภาษาที่แปลเสร็จ

---

### Bug 5: `entity_id` Normalization ทำลาย Subtitle UUID

**ปัญหา**: นี่คือ root cause ของ Bug 3 และ Bug 4

**`subscriber.go`**:
```go
// This normalization is too aggressive:
if update.EntityID != "" && update.VideoID == "" {
    update.VideoID = update.EntityID
}
if update.EntityCode != "" && update.VideoCode == "" {
    update.VideoCode = update.EntityCode
}
```

Logic นี้ assume ว่า `entity_id` = `video_id` เสมอ ซึ่งไม่จริงสำหรับ transcribe jobs (`entity_type=subtitle`)

**วิธีแก้**: ต้อง normalize ตาม `entity_type`:
```go
switch update.EntityType {
case "subtitle":
    update.SubtitleID = update.EntityID
    // VideoID ต้องมาจาก field อื่น (เช่น video_id ที่ worker ส่งมา)
case "video":
    update.VideoID = update.EntityID
    update.VideoCode = update.EntityCode
case "reel":
    update.ReelID = update.EntityID
}
```

---

## HIGH Severity Issues

---

### Bug 6: Translate Output Format Mismatch

**ปัญหา**: Python worker ส่ง `output.translations` (dict) แต่ API อ่าน `output.srt_path` (string)

**Python sends**:
```json
{
  "output": {
    "translations": {"th": "subtitles/code/th.srt"},
    "vtt_paths": {"th": "subtitles/code/th.vtt"},
    "source_language": "ja"
  }
}
```

**API reads** (`nats_progress_sub.go`):
```go
if srtPath, ok := update.RawOutput["srt_path"].(string); ok {
    data.SRTPath = srtPath  // <- ไม่เคย match เพราะ key คือ "translations" ไม่ใช่ "srt_path"
}
```

**ผลกระทบ**: แม้จะแก้ Bug 4 แล้ว SRT path ก็จะว่างใน DB

**วิธีแก้**: ต้องเลือกทาง:
- A) Python worker ส่ง completion แยกทีละภาษา พร้อม `srt_path` string
- B) API อ่าน `translations` dict แล้ว loop update ทีละ subtitle record

---

### Bug 7: One Translate Job = Multiple Languages, But API Expects Per-Subtitle Completion

**ปัญหา**: API สร้าง subtitle record แยกทีละภาษา (เช่น `th`, `en`) แต่ translate worker ทำทุกภาษาใน job เดียว แล้วส่ง completion 1 ครั้ง

**ผลกระทบ**: ถ้ามี 2 target languages, subtitle record ของภาษาที่ 2 จะค้างอยู่ที่ `queued` ตลอดไป

**วิธีแก้**:
- A) Worker ส่ง completion แยกทีละภาษา (preferred — ตรงกับ API architecture)
- B) API `HandleTranslationComplete()` parse `translations` dict แล้ว update ทุก record

---

## MEDIUM Severity Issues

---

### Bug 8: RetryStuckSubtitles ส่ง `language="auto"`

**ปัญหา**: `RetryStuckSubtitles()` bypass `TriggerTranscribe()` validation แล้วส่ง `language="auto"` ตรง

**ผลกระทบ**: `output_path` จะเป็น `subtitles/{code}/auto.srt` ซึ่งผิด (ควรเป็น `ja.srt`)
Worker จะ detect language ใหม่ แล้วเปลี่ยน path เป็น `ja.srt` แต่ DB record ยังชี้ path เก่า

---

### Bug 9: Race Condition — NATS Reconnect + Duplicate Jobs

**ปัญหา**: `orchestrateSubtitleChain` รันใน goroutine แยก ถ้า NATS reconnect ส่ง completed ซ้ำ จะมี 2 goroutines เรียก `TriggerTranscribe()` พร้อมกัน

**ผลกระทบ**: อาจสร้าง subtitle record ซ้ำ (race between check and create)

**วิธีแก้**: ใช้ DB unique constraint หรือ distributed lock

---

### Bug 10: Chain Sequencing — Translate Trigger ก่อน DB Update

**ปัญหา**: `handleTranscribeCompleted()` เรียก:
1. `HandleTranscribeComplete()` -> update subtitle status = ready
2. `TriggerTranslation()` -> check subtitle status == ready

ถ้า step 1 fail (เพราะ Bug 3), step 2 จะ fail ด้วย ("original subtitle is not ready")

**ผลกระทบ**: Chain ตาย silently, log แค่ warning

---

## LOW Severity Issues

---

### Bug 11: Stage Misclassification

**ปัญหา**: Shared stages (`initializing`, `downloading`, `uploading`) จาก detect/translate workers ถูก classify เป็น `subtitle_transcribe`

**ผลกระทบ**: WorkerJob record อาจถูก lookup ผิด type ในช่วงเริ่มต้น job (ไม่กระทบ terminal states)

---

### Bug 12: GEMINI_MODEL Default Inconsistency

**ปัญหา**:
- `config.py` default: `gemini-2.0-flash`
- `gemini_adapter.py` default: `gemini-2.5-flash`

`GeminiAdapter` อ่าน env var `GEMINI_MODEL` ตรง ไม่ผ่าน Config object -> ถ้า env ไม่ set จะใช้ `gemini-2.5-flash`

**ผลกระทบ**: ใช้ model ที่ไม่ตรงกับที่ตั้งใจ (แต่ production มี env var set อยู่แล้ว)

---

### Bug 13: `current_language` ไม่ถูกส่ง

**ปัญหา**: Python `ProgressUpdate` dataclass ไม่มี `current_language` field

**ผลกระทบ**: API fallback ว่าง -> auto-translate target lang อาจผิด (minor edge case)

---

## Root Cause Analysis

ปัญหาส่วนใหญ่มาจาก **1 จุดหลัก**:

> Python `ProgressUpdate` dataclass ใช้ `entity_id` สำหรับทุกอย่าง แต่ API subscriber normalize `entity_id -> video_id` แบบ blindly โดยไม่ดู `entity_type`

**ทำให้**:
1. Subtitle UUID หายไป (ถูกยัดเป็น VideoID)
2. `SubtitleID` ว่างเสมอ
3. ทุก completion handler ที่ต้องการ `SubtitleID` -> early exit
4. DB ไม่ update -> chain หยุด

**Fix ที่ต้องทำ (เรียงตามลำดับ)**:
1. API: สร้าง 3 consumers แยก (Bug 1)
2. API: แก้ `entity_id` normalization ให้ดู `entity_type` (Bug 5 — fixes Bug 3, 4)
3. Python: เพิ่ม `subtitle_id` field ใน `ProgressUpdate` (redundant safety)
4. Python: แก้ translate handler อ่าน `source_srt_path` (Bug 2)
5. API/Python: ตกลง output format สำหรับ translate completion (Bug 6, 7)

---

## Chain Flow — ก่อนแก้ vs หลังแก้

### ก่อนแก้ (ปัจจุบัน)
```
Detect ─── [OK] ──→ auto-trigger Transcribe
Transcribe ─── [FAIL: SubtitleID empty, DB not updated] ──→ X (chain dies)
Translate ─── [FAIL: never triggered because transcribe not ready] ──→ X
```

### หลังแก้ (เป้าหมาย)
```
Detect ─── completed ──→ API updates video.detected_language
                         ──→ auto-trigger Transcribe
Transcribe ─── completed ──→ API updates subtitle.status=ready, subtitle.srt_path
                             ──→ auto-trigger Translate(th)
Translate ─── completed ──→ API updates translated subtitle.status=ready per language
                            ──→ chain ends
```
