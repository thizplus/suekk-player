# Subtitle Pipeline R&D Roadmap (2026)

## Current Pipeline

```text
Demucs
↓
Whisper Turbo
↓
Silero VAD
↓
Kotoba Whisper
↓
Gemini Refine
```

คะแนนโดยรวม

```text
Accuracy: 9/10
Speed:    6/10
Cost:     8/10
```

---

# Goal

ลด Processing Time

```text
40-70%
```

โดยยังรักษา Subtitle Quality

---

# Option A (Low Risk)

## Replace Demucs → BS-Roformer

Current

```text
Demucs
```

Test

```text
BS-Roformer
```

เหตุผล

* Community แยก vocal ได้ดีกว่า Demucs หลายเคส
* ลดเสียง BGM รั่ว
* ลด hallucination ของ Whisper

โดยเฉพาะ

```text
Anime
Drama
JAV
Music-heavy content
```

Expected

```text
Subtitle Accuracy +3~8%
Processing Time ใกล้เคียงเดิม
```

Priority

HIGH

---

# Option B (Best ROI)

## Remove Gemini

Current

```text
Gemini Refine
```

Replace

```text
Qwen3 14B Q4
```

หรือ

```text
Qwen3 32B Q4
```

เหตุผล

* ไม่มี moderation
* ไม่ rewrite เกินจำเป็น
* local
* batch ได้

Expected

```text
Latency ลด
Cost ลด
Stability เพิ่ม
```

Priority

VERY HIGH

---

# Option C (Fastest)

## Whisper Turbo Only

Current

```text
Whisper Turbo
↓
Gap Detection
↓
Kotoba
```

Test

```text
Kotoba Only
```

หรือ

```text
Whisper Turbo Only
```

Benchmark

Kotoba v2 เร็วกว่า Whisper Large-v3 ประมาณ

```text
6.3x
```

โดยยังรักษา WER ใกล้เคียงเดิม

Expected

```text
Latency ลดมาก
```

Priority

MEDIUM

---

# Option D (Most Interesting)

## Canary 1B v2

Model

```text
NVIDIA Canary
```

จุดเด่น

* Multilingual ASR
* Built-in Speech Translation
* ลด hallucination จาก non-speech training

Benchmark

Canary 1B v2

```text
10x faster
```

กว่า Whisper Large-v3

ในบาง benchmark ของ NVIDIA

ข้อเสีย

```text
Japanese ยังไม่พิสูจน์เท่า Kotoba
```

Priority

MEDIUM

---

# Option E (Experimental)

## Direct Speech Translation

Current

```text
JP Audio
↓
JP Subtitle
↓
Translate
↓
TH Subtitle
```

Test

```text
JP Audio
↓
Speech Translation
↓
EN Subtitle
```

ด้วย

```text
Canary
```

หรือ

```text
SeamlessM4T
```

ข้อดี

* ตัด pipeline ออกหลาย stage

ข้อเสีย

* ยังไม่เหมาะ production

Priority

LOW

---

# JAV-Specific Improvements

## Problem

JAV Audio

```text
Voice
+
Moaning
+
Breathing
+
BGM
+
Bed Noise
```

Whisper มักคิดว่า

```text
เสียงคราง = speech
```

ทำให้ subtitle มั่ว

---

## Recommendation

เพิ่ม Pre-filter

ก่อน STT

```text
BS-Roformer
↓
Silero VAD
↓
Whisper
```

แล้วเพิ่ม

```python
min_speech_duration_ms=400
```

ใน VAD

เพื่อตัด

```text
breath
moan
short noises
```

ออก

---

# A/B Tests

## Test #1

Current

```text
Demucs
↓
Whisper Turbo
↓
Kotoba
```

VS

```text
BS-Roformer
↓
Kotoba
```

---

## Test #2

Current

```text
Gemini
```

VS

```text
Qwen3 14B
```

---

## Test #3

Current

```text
Whisper Turbo
↓
Kotoba
```

VS

```text
Kotoba Only
```

---

## Test #4

Current

```text
Whisper Turbo
```

VS

```text
Canary 1B
```

---

# My Priority Order

1. Replace Gemini → Qwen3
2. Demucs → BS-Roformer
3. Benchmark Kotoba-only
4. Benchmark Canary
5. Explore Speech Translation

````

Expected Overall Gain

```text
Latency  -30% ถึง -60%
Cost     -50%+
Quality  +5% ถึง +15%
````
