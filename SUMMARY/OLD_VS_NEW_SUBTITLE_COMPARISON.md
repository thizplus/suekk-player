# Old vs New Subtitle System — Feature Comparison

> ตรวจสอบละเอียดทุก feature ที่ระบบเก่า (_subtitle/) มี แต่ระบบใหม่ (_my_worker/python/) ยังไม่มี
> วันที่: 2026-06-06

---

## สรุปภาพรวม

ระบบใหม่มี architecture ที่ดีกว่า (Port/Adapter, LLM Factory) แต่ **ขาด business logic สำคัญ 70%** ที่ระบบเก่ามี โดยเฉพาะส่วนที่เกี่ยวกับ AV content

---

## 1. Translation Prompts (ขาดทั้งหมด)

### ระบบเก่ามี 5 prompts เฉพาะทาง

| Prompt | ไฟล์ | บรรทัด | รายละเอียด |
|--------|------|--------|-----------|
| JP->TH (AV) | `config/prompts.py` | 8-39 | 40 บรรทัด เจาะจง AV, keyword mapping, gender rules |
| EN->TH (AV) | `config/prompts.py` | 42-68 | fuck->เย็ด, cock->ควย, pussy->หี |
| TH->EN (AV) | `config/prompts.py` | 71-96 | Reverse translation |
| Generic (AV) | `config/prompts.py` | 98-118 | Fallback สำหรับภาษาอื่น |
| Translate+Summary | `gemini_client.py` | 981+ | 100+ บรรทัด, cluster context |

### ระบบใหม่มีแค่ 1 prompt generic

| Prompt | ไฟล์ | บรรทัด | รายละเอียด |
|--------|------|--------|-----------|
| Generic translate | `subtitle_translate/prompts.py` | 14-36 | 15 บรรทัด, ไม่มี AV rules |
| Generic cluster | `subtitle_translate/prompts.py` | 39-74 | มี summary แต่ไม่มี gender rules |

### กฎสำคัญที่หายไป

```
ระบบเก่า prompt สั่งว่า:
- ห้ามใช้ "ค่ะ/คะ/ครับ/จ๊ะ/จ้ะ" เด็ดขาด!
- ถ้าไม่แน่ใจเพศ -> ถือว่าเป็นหญิง (ใช้ "ฉัน")
- ตัวละครหญิง: "ฉัน" + "นะ", "สิ", "เหรอ", "ล่ะ"
- ตัวละครชาย: "ผม" + "นะ", "ว่ะ", "สิ"
- บังคับ keywords: เย็ด, ควย, หี, แตก, อม, เสียว
- เสียงคราง: สลับใช้หลายคำ ไม่ซ้ำ
- Self-reference: 先生 -> "ฉัน" (ไม่ใช่ "อาจารย์"), ママ -> "ฉัน" (ไม่ใช่ "แม่")

ระบบใหม่:
- ไม่มีกฎเหล่านี้เลย -> ครับ/ค่ะ ปน 87 บรรทัด
```

**ผลกระทบ**: ครับ/ค่ะ ปนกัน, สรรพนามผิด, แปลไม่เป็นธรรมชาติ

---

## 2. Moaning Sound Dictionary (ขาดทั้งหมด)

### ระบบเก่า

| ไฟล์ | รายละเอียด |
|------|-----------|
| `domain/constants/translation_mappings.py` | MOANING_SOUNDS dict 60+ entries |
| `infrastructure/gemini_client.py` line 398-423 | `_pre_translate_moaning_sounds()` |
| `infrastructure/gemini_client.py` line 360-396 | `_is_moaning_sound()` + pattern detection |

```python
# ตัวอย่าง MOANING_SOUNDS dict
'あ': '(เสียงคราง)'
'ああ': '(เสียงคราง)'
'あぁぁ': '(เสียงคราง)'
'ん': '(เสียงครวญ)'
'んーーー': '(เสียงครวญ)'
'はぁ': '(หายใจหอบ)'
'きもちいい': 'เสียวจัง'
```

**Flow**:
1. ก่อนส่ง LLM -> เช็คว่าเป็นเสียงครางไหม
2. ถ้าใช่ -> แปลจาก dict เลย (ไม่ต้องเรียก API)
3. ส่งเฉพาะบทพูดจริงไป LLM

### ระบบใหม่

- ไม่มี dict เลย
- ส่งเสียงครางทุกบรรทัดไป Gemini API
- เปลือง API + ผลไม่ consistent

**ผลกระทบ**: เปลือง API calls, เสียงครางแปลไม่สม่ำเสมอ

---

## 3. Softening Mechanism (ขาดทั้งหมด)

### ระบบเก่า

| ไฟล์ | รายละเอียด |
|------|-----------|
| `infrastructure/gemini_client.py` line 84-100 | SOFT_REPLACEMENTS dict 60+ mappings |
| `infrastructure/gemini_client.py` line 346-354 | `_soften_text()` method |
| `infrastructure/gemini_client.py` line 321-329 | `_is_prohibited_content_error()` |

```python
# SOFT_REPLACEMENTS ตัวอย่าง
'ちんちん' -> 'あそこ'
'オマンコ' -> 'あそこ'
'セックス' -> '行為'
```

**Flow เมื่อ Gemini block**:
1. ส่ง text ปกติ -> Gemini block (PROHIBITED_CONTENT)
2. Soften text (แทนคำโป๊ด้วยคำอ้อม)
3. ส่งใหม่ -> ผ่าน
4. ถ้ายัง block -> ใช้ generic prompt + retry

### ระบบใหม่

- `generate_unsafe()` fallback ไป `generate()` แค่ครั้งเดียว
- ไม่มี softening
- ไม่มี retry with modified text
- ถ้า block -> ใช้ text เดิม (ไม่แปล)

**ผลกระทบ**: Gemini block แล้วบรรทัดนั้นไม่ถูกแปลเลย (เห็นจาก test: finish_reason=8)

---

## 4. Silent Failure Detection (ขาดทั้งหมด)

### ระบบเก่า

| ไฟล์ | รายละเอียด |
|------|-----------|
| `infrastructure/gemini_client.py` line 425-453 | `_is_still_source_language()` |

```python
# ตรวจสอบว่าแปลจริงหรือยัง
jp_ratio > 0.3 and th_ratio < 0.1 -> "ไม่ได้แปล!"
-> retry ด้วย softened text
```

### ระบบใหม่

- ไม่มีการตรวจสอบ
- output อาจยังเป็น JP แต่ถูก save เป็น "th.srt"
- จาก test: Hybrid มี 91/562 บรรทัดที่ยังเป็น JP

**ผลกระทบ**: subtitle ภาษาไทยอาจมี JP ปนโดยไม่รู้ตัว

---

## 5. Missing Lines Recovery (ขาดทั้งหมด)

### ระบบเก่า

| ไฟล์ | รายละเอียด |
|------|-----------|
| `infrastructure/gemini_client.py` line 455-524 | `_find_missing_lines()` + `_retry_missing_lines()` |

**Flow**:
1. หลัง translate -> เช็คว่าได้ครบทุกบรรทัดไหม
2. ถ้าขาด -> retry เฉพาะบรรทัดที่หาย
3. prompt: "ต้องแปลทุก index ให้ครบ"

### ระบบใหม่

- ไม่ตรวจสอบ
- ถ้า LLM ข้ามบรรทัด -> ใช้ text เดิม (JP) ไม่มี retry

**ผลกระทบ**: บรรทัดที่หายไปจะเป็น JP ใน TH subtitle

---

## 6. Speaker Diarization (ขาดทั้งหมด)

### ระบบเก่า

| ไฟล์ | รายละเอียด |
|------|-----------|
| `infrastructure/speaker_diarization.py` | SpeechBrain ECAPA-TDNN + gender detection |
| `infrastructure/speaker_subprocess.py` | รัน diarization ใน subprocess (คืน VRAM) |
| `adapters/audio/speaker_adapter.py` | DiarizationPort adapter |
| `services/speaker_service.py` | Role analysis (วิเคราะห์บทบาท) |

**Flow**:
1. Demucs -> Diarization -> แยก SPEAKER_00, SPEAKER_01
2. Detect gender: male/female (pitch-based 165Hz threshold)
3. Save speakers.json -> S3
4. Tag ทุก subtitle: `(F, SPEAKER_01)` หรือ `(M, SPEAKER_00)`
5. Translate: LLM เห็น gender tag -> เลือก ฉัน/ผม ถูก

### ระบบใหม่

- `SubtitleLine` มี field `gender`, `speaker_id` แล้ว
- `handler.py` มี code อ่าน speakers.json แล้ว
- **แต่ไม่มีขั้นตอนสร้าง speakers.json!**
- Transcribe handler ไม่เรียก diarization
- ทุก subtitle มี gender=None

**ผลกระทบ**: LLM ไม่รู้เพศ -> ครับ/ค่ะ ปน

---

## 7. Role Analysis (ขาดทั้งหมด)

### ระบบเก่า

| ไฟล์ | รายละเอียด |
|------|-----------|
| `services/speaker_service.py` | วิเคราะห์บทบาทจาก dialogue |

**Flow**:
1. วิเคราะห์บทพูดแต่ละ speaker
2. ใช้ keyword scoring: "先生", "ママ", "お姉ちゃん"
3. Map: SPEAKER_00 -> "Teacher", SPEAKER_01 -> "Student"
4. ส่ง role info ไป translate prompt

### ระบบใหม่

- ไม่มี role analysis
- ไม่มี keyword scoring
- Self-reference แปลผิด (先生 -> "อาจารย์" แทน "ฉัน")

---

## 8. Translation Logging (ขาดทั้งหมด)

### ระบบเก่า

| ไฟล์ | รายละเอียด |
|------|-----------|
| `infrastructure/gemini_client.py` line 88-263 | `init_translation_log()`, `_log_translation()` |

**Output files**:
- `translation_log_{timestamp}.txt` - ทุก request/response ของ LLM
- `failed_translations.log` - บรรทัดที่แปลไม่สำเร็จ
- Debug SRT: `{lang}_1_raw_whisper.srt`, `{lang}_2_after_gaps.srt`

**Log format per cluster**:
```
CLUSTER #1 | Time: 12.3s | Lines: 10 -> 10
[PREV SUMMARY] Woman expressing affection
>>> INPUT (JA):
  [1] (F, A) 大好きよ
  [2] (M, B) もっとちょうだい
<<< OUTPUT (TH):
  [1] ชอบนะ
  [2] อีกนะ
[NEW SUMMARY] Woman expressing affection, man asking for more.
```

### ระบบใหม่

- ไม่มี translation log
- ไม่มี failed log
- ไม่มี debug SRT
- debug ปัญหา translate ไม่ได้เลย

---

## 9. Error Handling & Retry (ลดลงมาก)

### ระบบเก่า

| Feature | ไฟล์ | รายละเอียด |
|---------|------|-----------|
| Exponential backoff | `gemini_client.py` line 269-320 | MAX_RETRIES=5, 2s->60s |
| Rate limit detection | `gemini_client.py` line 311-319 | 429, resource exhausted |
| Prohibited content | `gemini_client.py` line 321-329 | Detect + soften + retry |
| Refusal detection | `gemini_client.py` line 331-344 | Detect LLM refusal keywords |
| Partial completion | `gemini_client.py` | Retry if < 80% translated |

### ระบบใหม่

| Feature | ไฟล์ | รายละเอียด |
|---------|------|-----------|
| Single fallback | `gemini_llm.py` line 77-91 | unsafe -> normal (1 retry) |

**ขาด**: exponential backoff, rate limit handling, softening, refusal detection, partial completion check

---

## 10. Refine Prompts (ลดลงมาก)

### ระบบเก่ามี 4 refine prompts

| Prompt | ภาษา | พิเศษ |
|--------|------|-------|
| REFINE_JP_PROMPT | Japanese | AV context, แก้ kanji, แปล EN/ZH ปนเป็น JP |
| REFINE_EN_PROMPT | English | แก้ Whisper errors + แปลภาษาอื่นเป็น EN |
| REFINE_ZH_PROMPT | Chinese | แก้ + แปลเป็น ZH |
| REFINE_GENERIC_PROMPT | อื่นๆ | Generic |

### ระบบใหม่มี 1 prompt

| Prompt | ภาษา | พิเศษ |
|--------|------|-------|
| build_refine_prompt | Generic | แก้ Whisper errors เฉพาะ JP (หลังปรับ prompt ใหม่) |

**ขาด**: ไม่มีความสามารถแปลภาษาปนใน refine, ไม่มี AV context

---

## สรุป: Priority ในการ Port กลับ

### Priority 1 — แก้ได้ทันที (แค่ copy + ปรับ)

| # | Feature | ผลที่ได้ | ไฟล์ที่ต้องแก้ |
|---|---------|---------|--------------|
| 1 | **AV Translation Prompt** (ห้าม ครับ/ค่ะ, gender rules) | แก้ ครับ/ค่ะ ปน 90% | `subtitle_translate/prompts.py` |
| 2 | **Moaning Dict** (60+ entries pre-translate) | ลด API calls + consistent | `subtitle_translate/handler.py` |
| 3 | **Self-Reference Rules** (先生->ฉัน) | แปลบทบาทถูก | `subtitle_translate/prompts.py` |
| 4 | **Softening + Retry** | แก้ Gemini block | `shared/adapters/gemini_llm.py` |

### Priority 2 — ใช้เวลาปานกลาง

| # | Feature | ผลที่ได้ | ไฟล์ที่ต้องแก้ |
|---|---------|---------|--------------|
| 5 | **Silent Failure Detection** | ตรวจจับ output ไม่แปล | `subtitle_translate/handler.py` |
| 6 | **Missing Lines Recovery** | retry บรรทัดที่หาย | `subtitle_translate/handler.py` |
| 7 | **Translation Logging** | debug ได้ | `subtitle_translate/handler.py` |

### Priority 3 — ใช้เวลามาก (ต้อง port infrastructure)

| # | Feature | ผลที่ได้ | ไฟล์ที่ต้องแก้ |
|---|---------|---------|--------------|
| 8 | **Speaker Diarization** | แยกเพศจริง | `subtitle_transcribe/handler.py` + new adapter |
| 9 | **Role Analysis** | วิเคราะห์บทบาท | new service |
| 10 | **Language-Specific Refine** (JP/EN/ZH) | refine คุณภาพดีขึ้น | `subtitle_transcribe/prompts.py` |

---

## Reference: ไฟล์ระบบเก่าที่ต้องอ่าน

| ไฟล์ | มีอะไร |
|------|--------|
| `_subtitle/config/prompts.py` | ทุก prompt (JP, EN, TH, ZH, generic, refine, translate) |
| `_subtitle/domain/constants/translation_mappings.py` | MOANING_SOUNDS, SOFT_REPLACEMENTS dicts |
| `_subtitle/infrastructure/gemini_client.py` | Error handling, softening, logging, retry, moaning pre-translate |
| `_subtitle/infrastructure/speaker_diarization.py` | SpeechBrain diarization + gender detection |
| `_subtitle/services/llm_service.py` | Cluster grouping, translate_segments, refine_segments |
| `_subtitle/services/speaker_service.py` | Role analysis, keyword scoring |
| `_subtitle/use_cases/translate_flow.py` | Speaker-aware translation flow |
| `_subtitle/use_cases/transcribe_flow.py` | Diarization integration in transcribe |
