# Test Plan: Subtitle Pipeline End-to-End (Production)

> Target: ทดสอบ 1 JAV video ครบทั้ง 3 steps (detect -> transcribe -> translate)
> Environment: Production API (`api.suekk.com`) + Local Workers (GPU)
> Date: 2026-06-06

---

## Test Video

| Field | Value |
|-------|-------|
| ID | `60a4aa06-d6bc-49e7-a921-3abd178f2fd8` |
| Code | `rwdvw5m9` |
| Title | DLDSS-504 |
| Category | Censored JAV |
| Status | ready |
| Audio | `audio/rwdvw5m9/audio.wav` |

---

## Pre-conditions

- [x] API server running (`api.suekk.com`)
- [x] NATS server running (`nats://5.223.46.39:4222`)
- [x] 3 subtitle workers running on local PC:
  - `subtitle_detect` (task: b6eapr014)
  - `subtitle_transcribe` (task: brbgds24v)
  - `subtitle_translate` (task: blhwlue9p)
- [x] Video has audio (`hasAudio: true`)
- [x] Video has NO subtitles yet (`subtitles: []`)

---

## Test Steps

### Step 1: Detect Language

**Action:**
```bash
curl -X POST -H "Authorization: Bearer TOKEN" \
  "https://api.suekk.com/api/v1/videos/60a4aa06-d6bc-49e7-a921-3abd178f2fd8/subtitle/detect"
```

**Status:** TRIGGERED (job submitted, waiting for worker)

**Expected:**
- [ ] API returns `{"success":true, "data":{"message":"Language detection job submitted"}}`
- [ ] Worker picks up job from NATS stream `SUBTITLE_JOBS` subject `jobs.subtitle.detect`
- [ ] Worker downloads audio from S3
- [ ] Whisper detects language (expected: `ja` for JAV)
- [ ] Worker reports result via API callback
- [ ] DB updated: `detected_language = "ja"`

**Verify:**
```bash
curl -H "Authorization: Bearer TOKEN" \
  "https://api.suekk.com/api/v1/videos/60a4aa06-d6bc-49e7-a921-3abd178f2fd8/subtitles"
# Expected: availableLanguages contains detected language
```

---

### Step 2: Transcribe

**Action:** (trigger AFTER detect completes)
```bash
curl -X POST -H "Authorization: Bearer TOKEN" \
  "https://api.suekk.com/api/v1/videos/60a4aa06-d6bc-49e7-a921-3abd178f2fd8/subtitle/transcribe"
```

**Expected:**
- [ ] API returns success + job submitted
- [ ] Worker picks up from `jobs.subtitle.transcribe`
- [ ] Worker downloads audio from S3
- [ ] Whisper transcribes full audio (turbo model)
- [ ] Gap re-transcription (kotoba model for Japanese)
- [ ] SRT file generated and uploaded to S3: `subtitles/rwdvw5m9/ja.srt`
- [ ] DB: subtitle record created (language=ja, status=ready)

**Verify:**
```bash
curl -H "Authorization: Bearer TOKEN" \
  "https://api.suekk.com/api/v1/videos/60a4aa06-d6bc-49e7-a921-3abd178f2fd8/subtitles"
# Expected: subtitles array has entry with language=ja, status=ready
```

---

### Step 3: Translate (ja -> th)

**Action:** (trigger AFTER transcribe completes)
```bash
curl -X POST -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"targetLanguages":["th"]}' \
  "https://api.suekk.com/api/v1/videos/60a4aa06-d6bc-49e7-a921-3abd178f2fd8/subtitle/translate"
```

**Expected:**
- [ ] API returns success + job submitted
- [ ] Worker picks up from `jobs.subtitle.translate`
- [ ] Worker downloads source SRT from S3
- [ ] Gemini (gemini-2.5-flash) translates ja -> th
- [ ] Translated SRT uploaded to S3: `subtitles/rwdvw5m9/th.srt`
- [ ] DB: subtitle record created (language=th, status=ready, isTranslation=true)

**Verify:**
```bash
curl -H "Authorization: Bearer TOKEN" \
  "https://api.suekk.com/api/v1/videos/60a4aa06-d6bc-49e7-a921-3abd178f2fd8/subtitles"
# Expected: subtitles array has entries for both ja and th
```

---

## Batch Endpoint Test (Frontend Queue Tab)

After single video test passes, test batch endpoints:

### Test A: Batch Detect (limit=1)
```bash
curl -X POST -H "Authorization: Bearer TOKEN" \
  "https://api.suekk.com/api/v1/admin/queues/subtitle/detect-all?category=5c9702c0-9826-4686-861e-797e50e1bbe2&limit=1"
```
- [ ] Returns `{"queued":1, "skipped":0}`
- [ ] Worker processes the job

### Test B: Batch Transcribe (limit=1)
```bash
curl -X POST -H "Authorization: Bearer TOKEN" \
  "https://api.suekk.com/api/v1/admin/queues/subtitle/transcribe-all?category=5c9702c0-9826-4686-861e-797e50e1bbe2&limit=1"
```
- [ ] Returns `{"queued":1, "skipped":0}` (or 0 if none pending)

### Test C: Batch Translate (limit=1)
```bash
curl -X POST -H "Authorization: Bearer TOKEN" \
  "https://api.suekk.com/api/v1/admin/queues/subtitle/translate-all?category=5c9702c0-9826-4686-861e-797e50e1bbe2&limit=1&target=th"
```
- [ ] Returns `{"queued":1, "skipped":0}`

### Test D: Stats Accuracy
```bash
curl -H "Authorization: Bearer TOKEN" \
  "https://api.suekk.com/api/v1/admin/queues/subtitle/stats?category=5c9702c0-9826-4686-861e-797e50e1bbe2"
```
- [ ] Numbers decrease after batch actions complete

---

## Worker Task IDs (for monitoring)

| Worker | Task ID | Status |
|--------|---------|--------|
| detect | b6eapr014 | running |
| transcribe | brbgds24v | running |
| translate | blhwlue9p | running |

---

## Known Issues to Watch

1. **Ghost worker** - ถ้ามี worker เก่าค้างจะแย่ง job
2. **entity_id normalization** - detect uses video_id, transcribe/translate may use subtitle_id
3. **S3 path backslash** - Windows backslash in S3 path (fixed previously)
4. **NATS consumer timeout** - ถ้า job ใหญ่ (>30 min) อาจ timeout

---

## Current Status

| Step | Status | Notes |
|------|--------|-------|
| Workers started | DONE | 3 workers connected to NATS |
| Detect DLDSS-504 | QUEUED | Job submitted, detect worker processing older job (jakzefe6) first |
| Transcribe DLDSS-504 | PENDING | Wait for detect |
| Translate DLDSS-504 | PENDING | Wait for transcribe |
