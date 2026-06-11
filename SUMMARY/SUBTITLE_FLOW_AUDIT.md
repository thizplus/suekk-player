# Subtitle Flow Audit — UI + API + Worker

> ตรวจสอบ flow จริงจาก code ทั้ง Frontend, API, Python Worker
> วันที่: 2026-06-05

---

## ความตั้งใจ vs สถานะจริง

| สิ่งที่ต้องการ | สถานะจริงใน code |
|---------------|-----------------|
| 3 ปุ่มแยกอิสระ: Detect / Transcribe / Translate | UI มีปุ่มเดียว "สร้าง Subtitle" ที่ auto-chain ทั้งหมด |
| แต่ละ step เป็นอิสระ debug ง่าย | API ProgressBroadcaster ยัง auto-chain: detect -> auto transcribe -> auto translate |
| User เลือก step เอง | User เลือกได้แค่ "สร้าง" กับ "แปลเพิ่ม" |

---

## UI ปัจจุบัน (SubtitlePanel.tsx)

### ปุ่มที่มีจริง

| ปุ่ม | เงื่อนไข | สิ่งที่ทำ |
|------|---------|----------|
| **"สร้าง Subtitle"** | ยังไม่มี original subtitle | ถ้ายังไม่ detect -> `detectLanguage()` แล้วรอ auto-chain, ถ้า detect แล้ว -> `transcribe()` |
| **"แปล" (Select + Button)** | original subtitle `ready` + มีภาษาที่ยังไม่แปล | `translate(videoId, [targetLang])` |
| **Retry (RefreshCw icon)** | subtitle status = `queued`, `failed`, หรือ `ready` | ลบ subtitle เก่า แล้วสร้าง/แปลใหม่ |
| **Re-detect (RefreshCw icon)** | มี detectedLanguage แล้ว | `detectLanguage()` re-detect |
| **Set Language (Select)** | ยังไม่มี detectedLanguage | `setLanguage(videoId, lang)` ตั้งค่าเอง |

### ปุ่มที่ **ไม่มี** (ควรจะมี)

| ปุ่ม | ทำไมควรมี |
|------|----------|
| **"Detect Language" แยก** | ให้ user กด detect เฉยๆ โดยไม่ต้อง auto-chain ไป transcribe |
| **"Transcribe" แยก** | ให้ user กด transcribe เฉยๆ หลัง detect เสร็จ โดยไม่ auto-translate |
| **"Translate" ที่ไม่ต้องรอ original ready** | ตอนนี้ต้องมี original ready ก่อนถึงจะเห็นปุ่มแปล |

---

## Flow เมื่อกด "สร้าง Subtitle"

### กรณี 1: ยังไม่ detect language

```
User กด "สร้าง Subtitle"
  |
  ├── setPendingAutoTranslate = true
  ├── setCurrentStep = 'transcribing'
  |
  ▼
Frontend: detectLanguage.mutate(videoId)
  |
  ▼
API: POST /api/v1/videos/:id/subtitle/detect
  ├── Validate: video ready, has audio_path
  ├── Clear existing detected_language
  ├── Publish to NATS: jobs.subtitle.detect
  └── Return { videoId, message }
  |
  ▼
Python Detect Worker:
  ├── Download audio.wav from S3
  ├── Whisper detect language -> "ja" (0.97)
  ├── Publish completed to progress.subtitle.{videoID}
  └── ack message
  |
  ▼
API ProgressBroadcaster: handleDetectCompleted()
  ├── Update video.detected_language = "ja"          ✅ ทำงานถูก
  ├── AUTO-TRIGGER: TriggerTranscribe(videoID)        ⚠️ auto-chain
  |     ├── Create Subtitle record (status=queued)
  |     └── Publish to NATS: jobs.subtitle.transcribe
  |
  ▼
Python Transcribe Worker (14 stages):
  ├── Download audio -> Demucs -> Whisper -> VAD -> LLM -> SRT/VTT
  ├── Upload ja.srt + ja.vtt to S3
  ├── Publish completed to progress.subtitle.{videoID}
  └── ack message
  |
  ▼
API ProgressBroadcaster: handleTranscribeCompleted()
  ├── update.SubtitleID == "" ???                     ❌ BUG (see below)
  ├── IF SubtitleID found:
  |     ├── Update subtitle.srt_path, status=ready
  |     └── AUTO-TRIGGER: TriggerTranslation(videoID, ["th"])  ⚠️ auto-chain
  |
  ▼
Python Translate Worker:
  ├── Download ja.srt -> Gemini cluster translation
  ├── Upload th.srt + th.vtt to S3
  ├── Publish completed
  └── ack message
  |
  ▼
API ProgressBroadcaster: handleTranslateCompleted()
  ├── update.SubtitleID == "" ???                     ❌ BUG
  └── Update subtitle.status=ready (IF subtitleID found)
```

### กรณี 2: detect แล้ว ยังไม่มี original subtitle

```
User กด "สร้าง Subtitle"
  |
  ▼
Frontend: transcribe.mutate(videoId)
  |
  ▼
API: POST /api/v1/videos/:id/subtitle/transcribe
  (เหมือนด้านบน ข้าม detect step)
```

---

## ปัญหาที่พบจาก code

---

### ปัญหา 1: UI ไม่มี 3 ปุ่มแยก

**สถานะจริง**: มีปุ่ม "สร้าง Subtitle" ปุ่มเดียวที่ทำทั้ง detect + transcribe (auto-chain)
และปุ่ม "แปล" ที่แยกออกมาแต่ต้องรอ original ready ก่อน

**สิ่งที่ขาด**:
- ปุ่ม "Detect Language" แยก — ให้ user ตรวจจับภาษาก่อน แล้วตัดสินใจว่าจะ transcribe หรือไม่
- ปุ่ม "Transcribe" แยก — ให้ user สั่ง transcribe โดยไม่ auto-translate
- ทั้ง 3 ปุ่มควรเป็นอิสระจากกัน ไม่ auto-chain

---

### ปัญหา 2: API ยัง Auto-Chain ใน ProgressBroadcaster

**File**: `progress_broadcaster.go` lines 574-588, 629-670

```go
// handleDetectCompleted -> line 580
_, err = pb.subtitleService.TriggerTranscribe(ctx, videoID)  // AUTO!

// handleTranscribeCompleted -> line 658
_, err = pb.subtitleService.TriggerTranslation(ctx, videoID, translateReq)  // AUTO!
```

**ปัญหา**: แม้จะแยก worker เป็น 3 ตัวแล้ว แต่ API ยัง auto-trigger step ถัดไปเสมอ
ทำให้ไม่สามารถ:
- Detect แล้วหยุด (เพื่อตรวจสอบว่า detect ถูกไหม)
- Transcribe แล้วหยุด (เพื่อตรวจสอบ/แก้ไข SRT ก่อนแปล)

**วิธีแก้**: ลบ auto-trigger ออกจาก ProgressBroadcaster ให้ user กดปุ่มเองทุก step

---

### ปัญหา 3: SubtitleID ว่างเสมอ (CRITICAL — chain จะไม่ทำงาน)

**Root cause**: Python worker ส่ง `entity_id` แต่ API subscriber normalize เป็น `video_id`
ส่วน `subtitle_id` ไม่เคยถูก set

**File**: `subscriber.go` lines 81-86
```go
if update.EntityID != "" && update.VideoID == "" {
    update.VideoID = update.EntityID  // subtitle UUID ถูกใส่เป็น VideoID!
}
```

**ผลกระทบ**:
- `handleTranscribeCompleted()` → early return เพราะ `SubtitleID == ""`
- `handleTranslateCompleted()` → early return เพราะ `SubtitleID == ""`
- Subtitle record ไม่เคยถูก update เป็น `ready`
- SRT path ไม่เคยถูกบันทึกใน DB

---

### ปัญหา 4: Translate input field mismatch

**API ส่ง**: `source_srt_path` (JSON key)
**Python อ่าน**: `srt_path` (JSON key)

Translate worker จะได้ path ว่าง -> download fail

---

### ปัญหา 5: Translate output format mismatch

**Python ส่ง**: `output.translations = {"th": "subtitles/code/th.srt"}`
**API อ่าน**: `output.srt_path` (string)

SRT path ไม่เข้า DB แม้จะแก้ SubtitleID แล้ว

---

### ปัญหา 6: Consumer name mismatch

**API สร้าง**: `SUBTITLE_WORKER` (1 consumer)
**Python ต้องการ**: `SUBTITLE_DETECT_WORKER`, `SUBTITLE_TRANSCRIBE_WORKER`, `SUBTITLE_TRANSLATE_WORKER` (3 consumers)

Python workers อาจ start ไม่ได้เลย ถ้า consumer ไม่มี

---

### ปัญหา 7: Frontend auto-translate logic ซ้ำกับ API

**SubtitlePanel.tsx** line 80-101:
```tsx
// Auto-translate: เมื่อ transcribe เสร็จ และยังไม่มี translation
useEffect(() => {
  if (pendingAutoTranslate && originalSubtitle?.status === 'ready' && translatedSubtitles.length === 0) {
    translate.mutate(...)  // Frontend ก็ auto-translate!
  }
}, [pendingAutoTranslate, originalSubtitle, ...])
```

**ProgressBroadcaster** line 658:
```go
// API ก็ auto-translate!
pb.subtitleService.TriggerTranslation(ctx, videoID, translateReq)
```

ทั้ง Frontend และ API ต่างก็ auto-trigger translate -> อาจเกิด **duplicate translate jobs**

---

### ปัญหา 8: Progress bar แสดง step 1/2, 2/2 แต่ไม่ match กับ 3 steps แยก

```tsx
{pendingAutoTranslate && currentStep === 'transcribing' && (
  <p>ขั้นตอน 1/2: ถอดเสียง → จะแปลอัตโนมัติเมื่อเสร็จ</p>
)}
{currentStep === 'translating' && (
  <p>ขั้นตอน 2/2: กำลังแปลภาษา</p>
)}
```

ถ้าเปลี่ยนเป็น 3 ปุ่มแยก -> progress bar นี้ไม่ต้องแสดง step 1/2 2/2 อีก

---

## สรุปสิ่งที่ต้องแก้ (เรียงตามลำดับ)

### Phase 1: แก้ Bug ที่ทำให้ chain พัง (ต้องแก้ก่อน)

| # | ที่ | แก้อะไร |
|---|-----|--------|
| 1 | API `nats/client.go` | สร้าง 3 consumers: `SUBTITLE_DETECT_WORKER`, `SUBTITLE_TRANSCRIBE_WORKER`, `SUBTITLE_TRANSLATE_WORKER` |
| 2 | API `subscriber.go` | แก้ `entity_id` normalization ให้ดู `entity_type` — subtitle -> `SubtitleID`, video -> `VideoID` |
| 3 | Python `translate/handler.py` | แก้ `input_data.get("srt_path")` -> `input_data.get("source_srt_path")` |
| 4 | API `nats_progress_sub.go` | แก้ให้อ่าน `output.translations` dict สำหรับ translate completion |

### Phase 2: เปลี่ยนเป็น 3 ปุ่มอิสระ (ตามที่ต้องการ)

| # | ที่ | แก้อะไร |
|---|-----|--------|
| 5 | API `progress_broadcaster.go` | ลบ auto-trigger ใน `handleDetectCompleted()` และ `handleTranscribeCompleted()` |
| 6 | Frontend `SubtitlePanel.tsx` | เพิ่ม 3 ปุ่มแยก: Detect / Transcribe / Translate |
| 7 | Frontend `SubtitlePanel.tsx` | ลบ `pendingAutoTranslate` logic และ useEffect auto-translate |
| 8 | Frontend `SubtitlePanel.tsx` | ลบ step indicator "ขั้นตอน 1/2, 2/2" |

### Phase 3: UI Improvement

| # | ที่ | แก้อะไร |
|---|-----|--------|
| 9 | Frontend | แสดง detected language + confidence หลัง detect เสร็จ |
| 10 | Frontend | ปุ่ม Transcribe: disable ถ้ายัง detect ไม่เสร็จ |
| 11 | Frontend | ปุ่ม Translate: disable ถ้ายัง transcribe ไม่เสร็จ |
| 12 | Frontend | แสดง progress bar แยกต่าง event type (detect/transcribe/translate) |

---

## Flow ที่ควรจะเป็น (หลังแก้)

```
┌─────────────────────────────────────────────────────┐
│ Video Detail Sheet                                   │
│                                                     │
│ Subtitle Section:                                    │
│                                                     │
│  ┌──────────────────────────────────────────────┐   │
│  │ ภาษา: [ยังไม่ตรวจจับ]                        │   │
│  │ [🔍 Detect Language]  [ตั้งค่าภาษา ▾]         │   │
│  └──────────────────────────────────────────────┘   │
│                                                     │
│  (หลัง detect เสร็จ: "🇯🇵 ญี่ปุ่น (97%)")           │
│                                                     │
│  ┌──────────────────────────────────────────────┐   │
│  │ [📝 Transcribe]  สร้าง SRT จากเสียง           │   │
│  └──────────────────────────────────────────────┘   │
│                                                     │
│  (หลัง transcribe เสร็จ: แสดง original subtitle)    │
│  🇯🇵 ญี่ปุ่น [ต้นฉบับ] [ready] [⬇️] [✏️]           │
│                                                     │
│  ┌──────────────────────────────────────────────┐   │
│  │ [🌐 แปล] เลือกภาษา: [🇹🇭 ไทย ▾]  [แปล]      │   │
│  └──────────────────────────────────────────────┘   │
│                                                     │
│  (หลัง translate เสร็จ)                              │
│  🇹🇭 ไทย [แปล] [ready] [⬇️] [✏️]                   │
│                                                     │
└─────────────────────────────────────────────────────┘
```

### Flow ใหม่:
```
User กด "Detect Language"
  -> API publish detect job
  -> Worker detect -> completed
  -> API update video.detected_language
  -> (หยุด) User เห็นภาษาที่ detect ได้

User กด "Transcribe"
  -> API validate: ต้อง detect ก่อน
  -> API publish transcribe job
  -> Worker transcribe -> completed
  -> API update subtitle.status=ready
  -> (หยุด) User เห็น original subtitle พร้อม download/edit

User กด "แปล" + เลือกภาษา
  -> API validate: ต้องมี original ready
  -> API publish translate job
  -> Worker translate -> completed
  -> API update translated subtitle
  -> User เห็น translated subtitle
```

**ข้อดี**: แต่ละ step เป็นอิสระ ถ้า step ไหนพัง ก็รู้ทันที ไม่ต้อง debug ทั้ง chain
