# Fix Plan: Subtitle Workers — All Bugs

> เปรียบเทียบระบบเก่า `_subtitle/` (ทำงานได้) กับระบบใหม่ `_my_worker/python/` (มี bugs)
> วันที่: 2026-06-06

---

## Bugs ทั้งหมด (4 ตัว)

### Bug 1: Gemini API Key "Expired"

**Root Cause:** Key `AQ.Ab8RN6I7...` ไม่ใช่ Gemini API key format
- Gemini API key ปกติ: `AIzaSy...`
- Key `AQ.Ab8...` อาจเป็น OAuth token หรือ key ของ service อื่น
- Key เก่าใน `_subtitle/.env` ที่ใช้งานได้: `AIzaSyBoG2TRIoTCRaFGgi32rCotuQMMVts9O0w`

**Fix:**
```bash
# _my_worker/.env — เปลี่ยนกลับเป็น key ที่ใช้งานได้
GEMINI_API_KEY=AIzaSyBoG2TRIoTCRaFGgi32rCotuQMMVts9O0w
```

**หรือ** ถ้า user ยืนยันว่า `AQ.Ab8...` เป็น key จริง ต้องเช็คว่า:
1. เป็น key ของ Gemini AI Studio หรือ Vertex AI
2. ถ้าเป็น Vertex AI ต้องใช้ SDK แบบอื่น (ไม่ใช่ `google.generativeai`)

**File:** `_my_worker/.env` line 129

---

### Bug 2: Demucs ไม่ทำงาน (ไม่ใช้ GPU)

**Root Cause:** ขาด flag `-d cuda` ในคำสั่ง Demucs

| System | Command |
|--------|---------|
| **OLD** (ทำงานได้) | `demucs -d cuda --two-stems=vocals -o output input.wav` |
| **NEW** (พัง) | `demucs -n htdemucs --two-stems vocals -o output input.wav` |

ความแตกต่าง:
1. **ไม่มี `-d cuda`** → Demucs ใช้ CPU → ช้ามาก → timeout 600s
2. **timeout=600** → subprocess.run ตัด Demucs ทิ้งเมื่อเกิน 10 นาที

**Fix:** `_my_worker/python/shared/adapters/audio_adapter.py`
```python
# Line 64-70: เพิ่ม -d cuda
cmd = [
    "demucs",
    "-d", "cuda",           # <-- เพิ่ม! ระบบเก่ามี
    "-n", model,
    "--two-stems", "vocals",
    "-o", str(demucs_out),
    str(audio_path),
]

# Line 74-79: เพิ่ม timeout เป็น 1800 (30 นาที)
result = subprocess.run(
    cmd,
    capture_output=True,
    text=True,
    timeout=1800,  # เดิม 600 → เพิ่มเป็น 1800
)
```

**File:** `_my_worker/python/shared/adapters/audio_adapter.py` line 64-79

---

### Bug 3: Processing Timeout 10 นาที

**Root Cause:** Backend API สร้าง NATS consumer ด้วย ack_wait สั้นเกินไป

| System | ack_wait | Heartbeat |
|--------|----------|-----------|
| **OLD** | 1800s (30 นาที) — worker สร้าง consumer เอง | HeartbeatPublisher ทุก 5s |
| **NEW** | ??? — API สร้าง consumer, worker ไม่ควบคุม | msg.in_progress() ทุก 30s |

Transcribe pipeline สำหรับ video 2 ชม.:
- Demucs: 3-5 นาที (GPU) หรือ 30+ นาที (CPU)
- Whisper: 1-3 นาที
- Gemini refine: 1-2 นาที
- **รวม: 5-10 นาที (GPU) หรือ 35+ นาที (CPU)**

**Fix 2 จุด:**

1. **Backend API** — เพิ่ม ack_wait สำหรับ subtitle consumers
   - File: `_gofiber_starter/infrastructure/nats/client.go`
   - เปลี่ยน ack_wait เป็น 3600s (60 นาที) สำหรับ SUBTITLE_TRANSCRIBE_WORKER

2. **Worker heartbeat** — ลดจาก 30s เป็น 10s
   - File: `_my_worker/python/shared/nats_consumer.py`
   - `msg.in_progress()` ทุก 10 วินาทีแทน 30 วินาที

---

### Bug 4: SRT File Not Found

**Root Cause:** เกิดจาก Bug 2 (Demucs timeout) ทำให้ cleanup ทำงานก่อน SRT ถูกเขียน

Flow ที่เกิด:
1. Demucs subprocess timeout → catch exception → fallback to original audio
2. ก่อนหน้า: job `a7410582` completed (340s) → cleanup ลบ job_dir
3. job ใหม่ `xxx` เริ่ม → Whisper ทำงานได้ → เขียน SRT ไปที่ job_dir ของ job ก่อน (ที่ถูกลบไปแล้ว)

จริงๆ ปัญหาอาจมาจาก **2 jobs ที่ run ซ้อนกัน** (NATS redeliver) — job แรก cleanup ลบ temp ของ job ที่สอง

**Fix:** ไม่ต้องแก้ถ้า Bug 2 + Bug 3 ถูก fix — เพราะ Demucs จะไม่ timeout และ NATS จะไม่ redeliver

---

## Execution Plan (เรียงตาม priority)

### Step 1: Fix Gemini API Key
```
File: _my_worker/.env
Action: ถาม user ว่า key ไหนถูก → ใส่ key ที่ถูกต้อง
```

### Step 2: Fix Demucs GPU + Timeout
```
File: _my_worker/python/shared/adapters/audio_adapter.py
Action: เพิ่ม `-d cuda` + timeout 1800s
```

### Step 3: Fix NATS ack_wait
```
File: _gofiber_starter/infrastructure/nats/client.go
Action: เพิ่ม ack_wait สำหรับ subtitle consumers เป็น 3600s
```

### Step 4: Test Full Pipeline
```
1. Restart 3 workers
2. Trigger transcribe DLDSS-504
3. Verify: Demucs (GPU) → Whisper → Gemini refine → SRT upload → status=ready
4. Trigger translate (ja→th)
5. Verify: SRT download → Gemini translate → Thai SRT upload → status=ready
```

---

## Key Differences: OLD vs NEW

| Aspect | OLD (`_subtitle/`) | NEW (`_my_worker/python/`) |
|--------|-------------------|---------------------------|
| Processes | 1 (handles all 3 types) | 3 separate processes |
| `.env` path | `_subtitle/.env` (explicit) | `_my_worker/.env` (search-based) |
| GEMINI_KEY | `AIzaSy...` (valid) | `AQ.Ab8...` (invalid format) |
| GEMINI_MODEL | `gemini-2.0-flash` | `gemini-2.5-flash` |
| Demucs GPU | `-d cuda` (explicit) | Missing! (uses CPU) |
| Demucs timeout | None (Popen, real-time) | 600s (subprocess.run) |
| NATS ack_wait | 1800s (worker creates) | API-controlled (unknown) |
| Heartbeat | KV-based, 5s | msg.in_progress(), 30s |
| Pre-flight check | Yes (Gemini, S3, Whisper) | No |
| LLM loading | Singleton via Container | New instance per call |
| Safety settings | Enum-based | Dict-based (string) |
