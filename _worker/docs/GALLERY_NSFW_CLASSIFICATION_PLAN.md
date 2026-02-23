# Gallery NSFW Classification Plan

## Problem Statement

ปัจจุบันระบบ Gallery Worker สร้าง 100 ภาพจาก video โดยไม่มีการจัดประเภท (classification)

ทำให้เมื่อ SEO Worker ต้องการใช้ภาพ:
- ต้อง download ทุกภาพมาตรวจสอบ NSFW ทีละภาพ (ช้า)
- มักพบว่า 94 ภาพจาก 100 ภาพเป็น NSFW (ใช้ได้แค่ 6 ภาพ)
- พยายาม Smart Blur แต่ไม่ผ่าน Falconsai classifier

## Proposed Solution

**แก้ที่ต้นทาง**: ทำ NSFW classification ตอนสร้าง gallery เลย

### Phase 1: Add NSFW Classification to Gallery Worker

```
┌─────────────────────────────────────────────────────────────────┐
│                    Current Flow                                  │
├─────────────────────────────────────────────────────────────────┤
│  HLS → Extract 100 frames → Upload ALL to S3 → Done             │
│                                                                  │
│  Problem: SEO Worker ต้อง download + classify ทีหลัง            │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    Proposed Flow                                 │
├─────────────────────────────────────────────────────────────────┤
│  HLS → Extract frame → NSFW Check → Classify as safe/nsfw       │
│                      ↓                                           │
│              Upload to separate folders:                         │
│              - gallery/{code}/safe/001.jpg                      │
│              - gallery/{code}/nsfw/001.jpg                      │
│                      ↓                                           │
│              Update DB with counts: safe_count, nsfw_count      │
└─────────────────────────────────────────────────────────────────┘
```

### Phase 2: Adaptive Frame Extraction (Multi-Round Strategy)

**หลักการ:** ถ้า Round แรกได้ภาพ safe น้อย → ดึงภาพเพิ่มจากช่วงเวลาที่ต่างออกไป

```
┌─────────────────────────────────────────────────────────────────┐
│  ROUND 1: Standard Extraction (100 frames)                      │
│  ─────────────────────────────────────────                      │
│  Timeline: [5%]────────────────────────────────────────[95%]    │
│            ↓    ↓    ↓    ↓    ↓    ↓    ↓    ↓    ↓    ↓      │
│           f1   f10  f20  f30  f40  f50  f60  f70  f80  f100     │
│                                                                  │
│  Interval: (95% - 5%) / 100 = 0.9% per frame                    │
│  Result: safe_count = 6 (ไม่พอ!)                                 │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│  ROUND 2: Intro Focus (20 frames)                               │
│  ─────────────────────────────────                              │
│  Timeline: [0%]────────[15%]                                    │
│            ↓  ↓  ↓  ↓  ↓  ↓  ↓  ↓  ↓  ↓                        │
│                                                                  │
│  เหตุผล: ช่วง intro มักเป็นการพูดคุย, แนะนำตัว (safe)            │
│  Interval: 15% / 20 = 0.75% per frame                           │
│  Result: +4 safe images                                         │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│  ROUND 3: Outro Focus (15 frames)                               │
│  ─────────────────────────────────                              │
│  Timeline:                              [90%]────────[100%]     │
│                                          ↓  ↓  ↓  ↓  ↓  ↓      │
│                                                                  │
│  เหตุผล: ช่วงท้ายมักเป็น ending, credits (safe)                  │
│  Interval: 10% / 15 = 0.67% per frame                           │
│  Result: +2 safe images                                         │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│  ROUND 4: Gap Fill (30 frames)                                  │
│  ─────────────────────────────                                  │
│  Timeline: [5%]────────────────────────────────────────[95%]    │
│              ↓   ↓   ↓   ↓   ↓   ↓   ↓   ↓   ↓   ↓             │
│            (offset +0.45% จาก Round 1)                          │
│                                                                  │
│  เหตุผล: ดึงภาพระหว่าง frame เดิม (กลางๆ ระหว่าง Round 1)       │
│  Result: +3 safe images                                         │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│  TOTAL: 6 + 4 + 2 + 3 = 15 safe images ✓                        │
└─────────────────────────────────────────────────────────────────┘
```

**Algorithm:**

```go
type ExtractionRound struct {
    Name       string
    StartPct   float64  // % ของ video
    EndPct     float64
    FrameCount int
    Offset     float64  // offset จาก interval ปกติ (สำหรับ gap fill)
}

var extractionRounds = []ExtractionRound{
    // Round 1: Standard (กระจายทั้ง video)
    {Name: "standard", StartPct: 0.05, EndPct: 0.95, FrameCount: 100, Offset: 0},

    // Round 2: Intro focus (0-15%)
    {Name: "intro", StartPct: 0.00, EndPct: 0.15, FrameCount: 20, Offset: 0},

    // Round 3: Outro focus (90-100%)
    {Name: "outro", StartPct: 0.90, EndPct: 1.00, FrameCount: 15, Offset: 0},

    // Round 4: Gap fill (ระหว่าง Round 1)
    {Name: "gap_fill", StartPct: 0.05, EndPct: 0.95, FrameCount: 30, Offset: 0.5},

    // Round 5: Dense intro (ถ้ายังไม่พอ)
    {Name: "dense_intro", StartPct: 0.00, EndPct: 0.10, FrameCount: 30, Offset: 0.25},
}

func (h *GalleryHandler) extractWithRetry(ctx context.Context, job *models.GalleryJob) ([]ClassifiedImage, error) {
    var safeImages []ClassifiedImage
    var nsfwImages []ClassifiedImage
    minSafe := 12

    for _, round := range extractionRounds {
        if len(safeImages) >= minSafe {
            break // พอแล้ว หยุดได้
        }

        h.logger.Info("extraction round",
            "round", round.Name,
            "current_safe", len(safeImages),
            "target", minSafe,
        )

        // Extract frames for this round
        frames := h.extractFrames(ctx, job, round)

        // Classify each frame
        for _, frame := range frames {
            result := h.classifyImage(ctx, frame)
            if result.IsSafe {
                safeImages = append(safeImages, frame)
            } else {
                nsfwImages = append(nsfwImages, frame)
            }
        }
    }

    h.logger.Info("extraction complete",
        "total_safe", len(safeImages),
        "total_nsfw", len(nsfwImages),
        "rounds_used", getRoundsUsed(len(safeImages), minSafe),
    )

    return safeImages, nil
}
```

**Timestamp Deduplication:**

```go
// ป้องกันภาพซ้ำ: track timestamps ที่ใช้ไปแล้ว
type TimestampTracker struct {
    used      map[int]bool  // timestamp (seconds) ที่ใช้แล้ว
    minGap    int           // minimum gap between frames (seconds)
}

func (t *TimestampTracker) IsAvailable(timestamp float64) bool {
    sec := int(timestamp)
    // Check if any nearby timestamp was used
    for i := sec - t.minGap; i <= sec + t.minGap; i++ {
        if t.used[i] {
            return false
        }
    }
    return true
}

func (t *TimestampTracker) Mark(timestamp float64) {
    t.used[int(timestamp)] = true
}
```

### Phase 3: Smart Segment Detection (Future Enhancement)

```
┌─────────────────────────────────────────────────────────────────┐
│  If still not enough after Round 5:                             │
│                                                                  │
│  Option A: Scene Detection                                      │
│    - ใช้ FFmpeg scene detection หา "talking scenes"             │
│    - ดึงภาพจากช่วงที่มี scene change น้อย (มักเป็นการพูดคุย)    │
│                                                                  │
│  Option B: Audio Analysis                                       │
│    - วิเคราะห์ audio หาช่วงที่มีเสียงพูด (speech)               │
│    - ดึงภาพจากช่วงที่มี speech (มักเป็น safe)                   │
│                                                                  │
│  Option C: Accept Fewer Images                                  │
│    - ถ้า video เป็น NSFW เกือบทั้งหมด                           │
│    - ยอมรับ safe images น้อยกว่า 12                             │
│    - Flag video ว่า "limited_gallery"                           │
└─────────────────────────────────────────────────────────────────┘
```

---

## System Architecture: 2 Entry Points

**สำคัญ:** ระบบมี 2 ทางในการสร้าง gallery ต้องแก้ทั้ง 2 จุด

```
┌─────────────────────────────────────────────────────────────────┐
│  Entry Point 1: Auto-generation (Transcode)                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Video Upload → Transcode Worker → generateAndUploadGallery()  │
│                                    ↓                            │
│                          transcoder/gallery.go                  │
│                          (ใช้ Local video file - เร็ว)          │
│                                    ↓                            │
│                          ┌─────────────────┐                    │
│                          │ NSFW Classifier │ ← NEW              │
│                          └─────────────────┘                    │
│                                    ↓                            │
│                          Upload safe/ + nsfw/ to S3             │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  Entry Point 2: Manual Trigger (Frontend)                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Frontend กดปุ่ม → API → PublishGalleryJob() → NATS Queue       │
│                                                ↓                │
│                               gallery_handler.go:ProcessJob()   │
│                               (ใช้ HLS จาก S3 - ช้ากว่า)        │
│                                                ↓                │
│                               ┌─────────────────┐               │
│                               │ NSFW Classifier │ ← NEW         │
│                               └─────────────────┘               │
│                                                ↓                │
│                               Upload safe/ + nsfw/ to S3        │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Shared NSFW Classifier Module

ทั้ง 2 path ใช้ classifier ร่วมกัน:

```
_worker/
├── infrastructure/
│   └── classifier/              ← NEW: Shared module
│       ├── nsfw_classifier.go   # Go wrapper (call Python)
│       ├── classify_batch.py    # Python NudeNet + logic
│       └── types.go             # ClassificationResult struct
│
├── infrastructure/transcoder/
│   └── gallery.go               # Entry Point 1 (แก้ไข)
│
└── use_cases/
    └── gallery_handler.go       # Entry Point 2 (แก้ไข)
```

### Integration Flow

```go
// ทั้ง 2 entry points เรียกใช้แบบเดียวกัน:

// 1. Extract frames (existing logic)
frames := extractFrames(ctx, videoSource, timestamps)

// 2. Classify all frames (NEW)
results, err := classifier.ClassifyBatch(ctx, framesDir)
// results = map[string]ClassificationResult

// 3. Separate safe/nsfw
safeFrames, nsfwFrames := classifier.SeparateByResult(frames, results)

// 4. If not enough safe → extract more (Multi-Round)
if len(safeFrames) < minSafeImages {
    // ... adaptive extraction logic
}

// 5. Upload to separate folders
uploadToS3(safeFrames, "gallery/{code}/safe/")
uploadToS3(nsfwFrames[:30], "gallery/{code}/nsfw/")  // Max 30
```

---

## Technical Implementation

### 1. Database Schema Changes

**Video model (gofiber_starter):**
```go
// เพิ่ม fields
GallerySafeCount int    `gorm:"default:0"`  // จำนวนภาพ safe
GalleryNsfwCount int    `gorm:"default:0"`  // จำนวนภาพ nsfw
GalleryStatus    string `gorm:"size:20;default:'pending'"` // pending|processing|ready
```

### 2. Storage Structure

**Current:**
```
gallery/{videoCode}/
├── 001.jpg
├── 002.jpg
├── ...
└── 100.jpg
```

**Proposed:**
```
gallery/{videoCode}/
├── safe/
│   ├── 001.jpg
│   ├── 005.jpg
│   └── ...
├── nsfw/
│   ├── 002.jpg
│   ├── 003.jpg
│   └── ...
└── metadata.json  # classification results
```

### 3. NSFW Classification Service

**Option A: Python Microservice (Recommended)**
```
┌──────────────────────────────────────────────────────────┐
│  python_nsfw_classifier/                                 │
│  ├── main.py              # FastAPI server               │
│  ├── classifier.py        # NudeNet + Falconsai          │
│  └── requirements.txt                                    │
│                                                          │
│  Endpoints:                                              │
│  - POST /classify         # Classify single image        │
│  - POST /classify-batch   # Classify multiple images     │
│                                                          │
│  Response: { "is_safe": true, "nsfw_score": 0.12 }       │
└──────────────────────────────────────────────────────────┘
```

**Option B: Embedded in Go Worker**
- Use Python subprocess for each image
- Slower but simpler deployment

### 4. GalleryJob Update

```go
type GalleryJob struct {
    // ... existing fields ...

    // New fields
    ClassifyNSFW    bool `json:"classify_nsfw"`     // Enable NSFW classification
    MinSafeImages   int  `json:"min_safe_images"`   // Minimum safe images required (default 12)
    MaxExtraFrames  int  `json:"max_extra_frames"`  // Max additional frames to try (default 50)
}
```

### 5. Gallery Handler Changes

```go
func (h *GalleryHandler) ProcessJob(ctx context.Context, job *models.GalleryJob) error {
    // 1. Extract frames (existing logic)
    frames, err := h.extractFramesFromHLS(ctx, job, outputDir, progressCallback)

    // 2. NEW: Classify each frame
    safeFrames := []string{}
    nsfwFrames := []string{}

    for _, frame := range frames {
        result, err := h.classifyImage(ctx, frame)
        if result.IsSafe {
            safeFrames = append(safeFrames, frame)
        } else {
            nsfwFrames = append(nsfwFrames, frame)
        }
    }

    // 3. NEW: If not enough safe images, extract more
    if len(safeFrames) < job.MinSafeImages {
        extraFrames := h.extractExtraFrames(ctx, job, len(safeFrames), job.MinSafeImages)
        // ... classify extra frames ...
    }

    // 4. Upload to separate folders
    h.uploadGalleryImages(ctx, safeFrames, job.OutputPath+"/safe", job.VideoCode)
    h.uploadGalleryImages(ctx, nsfwFrames, job.OutputPath+"/nsfw", job.VideoCode)

    // 5. Update DB with counts
    h.updateVideoGallery(ctx, job.VideoID, len(safeFrames), len(nsfwFrames))
}
```

---

## SEO Worker Impact

### Current SEO Worker Flow:
```
1. Get gallery images from S3
2. Download each image
3. Run NSFW classification
4. Filter safe images
5. Try to blur NSFW images (often fails)
6. Use whatever images are available
```

### New SEO Worker Flow:
```
1. Get safe_count from DB
2. If safe_count >= 12:
     → Download only from gallery/{code}/safe/
     → Use directly (no classification needed!)
3. If safe_count < 12:
     → Download safe images
     → Consider using Smart Blur on NSFW images
     → Or just use fewer images
```

---

## Implementation Order

### Sprint 1: Shared Classifier Module (2-3 days)
```
infrastructure/classifier/
├── classify_batch.py    # Python NudeNet
├── nsfw_classifier.go   # Go wrapper (subprocess)
└── types.go             # Structs
```
- [ ] สร้าง `classify_batch.py` - NudeNet batch classification
- [ ] สร้าง `nsfw_classifier.go` - Go wrapper เรียก Python subprocess
- [ ] สร้าง `types.go` - ClassificationResult, ClassificationStats
- [ ] ทดสอบกับ sample images

### Sprint 2: Entry Point 1 - Transcode Path (2 days)
```
infrastructure/transcoder/gallery.go  ← แก้ไข
```
- [ ] แก้ `GenerateGallery()` เพิ่ม classification step
- [ ] เพิ่ม Multi-Round extraction logic
- [ ] แยก upload safe/ และ nsfw/ folders
- [ ] จำกัด nsfw ไม่เกิน 30 ภาพ
- [ ] เพิ่ม logging stats

### Sprint 3: Entry Point 2 - NATS Job Path (2 days)
```
use_cases/gallery_handler.go  ← แก้ไข
```
- [ ] แก้ `ProcessJob()` เพิ่ม classification step
- [ ] เพิ่ม Multi-Round extraction logic
- [ ] แยก upload safe/ และ nsfw/ folders
- [ ] จำกัด nsfw ไม่เกิน 30 ภาพ
- [ ] เพิ่ม logging stats

### Sprint 4: Database & API Updates (1 day)
```
gofiber_starter/
├── domain/models/video.go        # เพิ่ม fields
├── domain/dto/video.go           # เพิ่ม DTO
└── interfaces/api/handlers/      # แก้ API response
```
- [ ] เพิ่ม `GallerySafeCount`, `GalleryNsfwCount` fields
- [ ] อัพเดท `UpdateGallery` API รับ counts ใหม่
- [ ] Migration script

### Sprint 5: SEO Worker Update (1 day)
```
_seo_worker/
├── infrastructure/imageselector/  # แก้ไข
└── use_cases/seo_handler.go       # แก้ไข
```
- [ ] ใช้ pre-classified images จาก safe/ folder
- [ ] ลบ classification logic เดิม (ไม่จำเป็นแล้ว)
- [ ] ใช้ safe_count จาก DB

### Sprint 6: Backfill Existing Videos (Optional)
- [ ] สร้าง batch job สำหรับ video เก่า
- [ ] Classify และ re-organize folders
- [ ] Update DB counts

---

## Configuration

```yaml
# config.yaml
gallery:
  enabled: true
  image_count: 100
  min_safe_images: 12
  max_extra_frames: 50

nsfw_classifier:
  service_url: "http://localhost:8000"
  timeout: 30s

  # Classification thresholds
  nsfw_threshold: 0.3     # Score above this = NSFW

  # Models to use
  use_nudenet: true       # Fast, region-based
  use_falconsai: false    # Slow, whole-image (use for verification only)
```

---

## Estimated Impact

| Metric | Before | After |
|--------|--------|-------|
| Safe images per video | ~6 | 12+ |
| SEO Worker classification time | 30-60s | 0s (pre-classified) |
| Gallery generation time | 2-3 min | 4-5 min (+classification) |
| Storage overhead | 1x | ~1.1x (metadata.json) |

---

## Design Decisions (Confirmed)

### 1. Folder Structure: ใช้ `safe/` และ `nsfw/` folders

**Decision:** ใช้ folder แยก ไม่ใช้ suffix

**เหตุผล:**
- ทำ ACL (Access Control) ได้ง่าย
- `safe/` → Public access
- `nsfw/` → ต้องมี Signed URL หรือ Membership
- ง่ายต่อการ list files, backup, migrate

```
gallery/{videoCode}/
├── safe/           ← Public CDN
│   ├── 001.jpg
│   └── ...
├── nsfw/           ← Signed URL only
│   ├── 002.jpg
│   └── ...
└── metadata.json
```

### 2. NSFW Classification Strategy: Two-Tier System

**Decision:** NudeNet (Primary) + Falconsai (Cover Gatekeeper)

```
┌─────────────────────────────────────────────────────────────────┐
│  Classification Flow                                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  All Images ──► NudeNet (Fast) ──► safe/ or nsfw/               │
│                                                                  │
│  Cover Candidate ──► Falconsai (Slow) ──► Final Cover           │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**เหตุผล:**
- **NudeNet**: เร็ว, ระบุจุดได้ดี (bounding boxes) - ใช้กับทุกภาพ
- **Falconsai**: มองบริบทเก่งกว่า แต่ช้า - ใช้เฉพาะ Cover ที่จะโชว์บน Google
- ลดความเสี่ยงโดนแบนได้ 100% โดยไม่ทำให้ระบบช้าเกินไป

```go
// Classification logic
func (h *GalleryHandler) classifyImage(ctx context.Context, imagePath string) (*ClassificationResult, error) {
    // Step 1: NudeNet (always)
    nudenetResult := h.nudenetClassify(imagePath)

    // For gallery images: NudeNet is enough
    return nudenetResult
}

func (h *GalleryHandler) selectCoverImage(ctx context.Context, safeImages []string) (string, error) {
    // Step 1: Sort by face_score, aesthetic_score
    candidates := h.rankBestCandidates(safeImages, 3) // Top 3

    // Step 2: Verify with Falconsai (strict check for Google)
    for _, candidate := range candidates {
        falconsaiResult := h.falconsaiClassify(candidate)
        if falconsaiResult.Score < 0.2 { // Very strict for cover
            return candidate, nil
        }
    }

    // Fallback: use safest candidate
    return candidates[0], nil
}
```

### 3. Failed Classification: Safety First

**Decision:** ถ้า classifier ล้มเหลว → ถือว่าเป็น **NSFW**

**เหตุผล:**
- ป้องกันภาพหลุดไปที่หน้า Public
- Better safe than sorry
- สามารถ re-classify ทีหลังได้

```go
func (h *GalleryHandler) classifyImage(ctx context.Context, imagePath string) (*ClassificationResult, error) {
    result, err := h.nudenetClassify(imagePath)
    if err != nil {
        // Classification failed → treat as NSFW
        h.logger.Warn("classification failed, treating as NSFW",
            "image", imagePath,
            "error", err,
        )
        return &ClassificationResult{
            IsSafe:    false,
            NsfwScore: 1.0,
            Error:     err.Error(),
        }, nil
    }
    return result, nil
}
```

### 4. Re-run Existing Videos: Optional Backfill

**Decision:** ไม่บังคับ backfill ทันที แต่มี option

**แนวทาง:**
- Video ใหม่ → ใช้ระบบใหม่เลย
- Video เก่า → ทำ lazy classification เมื่อ SEO Worker ต้องการ
- Admin option → Manual trigger backfill สำหรับ video สำคัญ

---

## Additional Design Decisions

### 5. NSFW Storage Limit: เก็บไม่เกิน 30 ภาพ

**Decision:** เก็บ `nsfw/` folder ไว้ แต่จำกัดไม่เกิน 30 ภาพ

**เหตุผล:**
- ประหยัด storage (ไม่เก็บ 70-90 ภาพ NSFW ทั้งหมด)
- มีไว้ใช้สำหรับ Members (Signed URL)
- เลือกเก็บภาพที่มี quality ดีที่สุด (face_score, aesthetic_score)

```go
const MaxNsfwImages = 30

func (h *GalleryHandler) selectBestNsfwImages(nsfwImages []ClassifiedImage) []ClassifiedImage {
    if len(nsfwImages) <= MaxNsfwImages {
        return nsfwImages
    }

    // Sort by quality score (face + aesthetic)
    sort.Slice(nsfwImages, func(i, j int) bool {
        scoreI := nsfwImages[i].FaceScore + nsfwImages[i].AestheticScore
        scoreJ := nsfwImages[j].FaceScore + nsfwImages[j].AestheticScore
        return scoreI > scoreJ
    })

    // Keep top 30
    return nsfwImages[:MaxNsfwImages]
}
```

**Storage Structure:**
```
gallery/{videoCode}/
├── safe/           ← ทุกภาพ safe (12-20 ภาพ)
├── nsfw/           ← เฉพาะ top 30 ภาพ (เรียงตาม quality)
└── metadata.json
```

### 6. Analytics: Simple Logging

**Decision:** เก็บ Simple Logging สำหรับ tuning threshold ในอนาคต

**เหตุผล:**
- Track False Positive/Negative เพื่อปรับ `nsfw_threshold`
- ไม่ต้องซับซ้อน แค่ log ตัวเลขพื้นฐาน

```go
type ClassificationStats struct {
    VideoCode       string    `json:"video_code"`
    TotalFrames     int       `json:"total_frames"`
    SafeCount       int       `json:"safe_count"`
    NsfwCount       int       `json:"nsfw_count"`
    RoundsUsed      int       `json:"rounds_used"`
    AvgNsfwScore    float64   `json:"avg_nsfw_score"`
    ProcessingTime  float64   `json:"processing_time_sec"`
    Timestamp       time.Time `json:"timestamp"`
}

// Log to file for future analysis
func (h *GalleryHandler) logClassificationStats(stats ClassificationStats) {
    h.logger.Info("classification_stats",
        "video_code", stats.VideoCode,
        "total", stats.TotalFrames,
        "safe", stats.SafeCount,
        "nsfw", stats.NsfwCount,
        "rounds", stats.RoundsUsed,
        "avg_score", stats.AvgNsfwScore,
        "time_sec", stats.ProcessingTime,
    )
}
```

**Log Output Example:**
```json
{
  "level": "INFO",
  "msg": "classification_stats",
  "video_code": "ABC123",
  "total": 165,
  "safe": 15,
  "nsfw": 150,
  "rounds": 4,
  "avg_score": 0.72,
  "time_sec": 45.2
}
```

**Future Use:**
- ถ้า `avg_nsfw_score` ต่ำแต่ยังถูก classify เป็น nsfw → ลด threshold
- ถ้ามี report ว่าภาพ safe หลุดไป → เพิ่ม threshold

---

## Summary: All Decisions Confirmed

| # | Topic | Decision |
|---|-------|----------|
| 1 | Folder Structure | ใช้ `safe/` และ `nsfw/` folders |
| 2 | Classification | NudeNet (all) + Falconsai (cover only) |
| 3 | Failed Classification | ถือว่า NSFW (Safety First) |
| 4 | Backfill | Optional, lazy classification |
| 5 | NSFW Storage | เก็บไม่เกิน 30 ภาพ (top quality) |
| 6 | Analytics | Simple Logging สำหรับ tuning |

**Status: Ready for Implementation**

---

*Created: 2026-02-23*
*Updated: 2026-02-23 - Added design decisions based on review*
*Updated: 2026-02-24 - Added Three-Tier Super Safe system for Google SafeSearch compliance*

---

## 📊 Implementation Status

### ✅ IMPLEMENTED (Phase 1-2 + Phase 3 Complete)

| Feature | File | Status |
|---------|------|--------|
| Two-Tier Classification (safe/nsfw) | `types.go` | ✅ Done |
| Dual Model (Falconsai + NudeNet) | `classify_batch.py` | ✅ Done |
| Threshold 0.3 | `DefaultConfig()` | ✅ Done |
| Multi-Round Extraction (5 rounds) | `gallery_handler.go` | ✅ Done |
| MinSafeImages: 12, MaxNsfwImages: 30 | Config | ✅ Done |
| Quality sorting (face + aesthetic) | `nsfw_classifier.go` | ✅ Done |
| Safety First (errors → nsfw) | `gallery_handler.go` | ✅ Done |
| ProcessJobWithClassification() | `gallery_handler.go` | ✅ Done |
| GenerateGalleryWithClassification() | `gallery_classified.go` | ✅ Done |
| **Three-Tier (super_safe/)** | `gallery_classified.go`, `gallery_handler.go` | ✅ Done |
| **SuperSafeThreshold 0.15** | `types.go`, `classify_batch.py` | ✅ Done |
| **IsSuperSafe field** | `types.go`, `classify_batch.py` | ✅ Done |
| **MinSuperSafeImages: 10** | `gallery_classified.go` | ✅ Done |
| **Face requirement for super_safe** | `classify_batch.py`, `nsfw_classifier.go` | ✅ Done |
| Upload to /super_safe/, /safe/, /nsfw/ | `gallery_handler.go`, `gallery_classified.go` | ✅ Done |

### ⏳ PENDING

| Feature | Description | Priority |
|---------|-------------|----------|
| **FalconsaiScore/NudenetScore** | แยกเก็บ score แต่ละ model ใน result | LOW |
| **metadata.json** | บันทึก classification results | LOW |

### Current Storage Structure (Three-Tier - Implemented)

```
gallery/{videoCode}/
├── super_safe/         ← score < 0.15 + face (Public SEO)
│   ├── 001.jpg
│   └── ...
├── safe/               ← score 0.15-0.3 (Lazy load)
│   ├── 005.jpg
│   └── ...
├── nsfw/               ← score >= 0.3 (Member only)
│   ├── 010.jpg
│   └── ...
└── (metadata.json - not yet)
```

---

---

## 🆕 [PROPOSED] Phase 3: Three-Tier Image Safety System (Google SafeSearch Compliance)

> **ปัญหาใหม่:** Google Cloud Vision ไม่ได้ดูแค่ explicit content
> แต่ดู **Suggestive Content** (ภาพชี้นำ, สีหน้าเคลิ้ม, ท่าทางใกล้ชิด) ด้วย

### Current System (Two-Tier)

```
nsfw_score < 0.3  →  /safe/   (Public + Member)
nsfw_score >= 0.3 →  /nsfw/   (Member only)
```

**ปัญหา:** ภาพที่ score 0.2 อาจถูก Google ตีว่า "Racy" (suggestive)

### Proposed System (Three-Tier)

```
┌─────────────────────────────────────────────────────────────────┐
│  THREE-TIER CLASSIFICATION                                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  SUPER_SAFE = nsfw_score < 0.15 AND face_score > 0.1            │
│  ─────────────────────────────────────────────────────────────  │
│  • ต้องมีหน้าคนในภาพ (ไม่ใช่ภาพห้องเปล่า/ฉากหลัง)              │
│  • ใช้เป็น Thumbnail/OG Image                                   │
│  • Google Bot เห็นได้                                           │
│                                                                  │
│  SAFE = nsfw_score < 0.3 AND NOT SUPER_SAFE                     │
│  ─────────────────────────────────────────────────────────────  │
│  • ภาพที่ score 0.15-0.3 หรือ ไม่มีหน้าคน                       │
│  • ซ่อนหลังปุ่ม "ดูเพิ่มเติม"                                   │
│  • Google Bot ไม่เห็น URL                                       │
│                                                                  │
│  NSFW = nsfw_score >= 0.3                                       │
│  ─────────────────────────────────────────────────────────────  │
│  • ภาพที่ explicit                                              │
│  • Signed URL only (Member)                                     │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### ⚠️ สำคัญ: Super Safe ต้องมีหน้าคน

```
ปัญหาที่เคยเจอ:
─────────────────────────────────────────────────────────────────
❌ ภาพห้องเปล่า (nsfw_score: 0.02, face_score: 0.0)
   → ผ่าน threshold 0.15 แต่ไม่มีคน = ไม่ควรเป็น super_safe

❌ ภาพฉากหลัง/เฟอร์นิเจอร์ (nsfw_score: 0.05, face_score: 0.0)
   → ผ่าน threshold 0.15 แต่ไม่มีคน = ไม่ควรเป็น super_safe

✅ ภาพคนยืนคุย (nsfw_score: 0.08, face_score: 0.35)
   → ผ่านทั้ง 2 เงื่อนไข = super_safe ✓

✅ ภาพหน้านักแสดง (nsfw_score: 0.12, face_score: 0.45)
   → ผ่านทั้ง 2 เงื่อนไข = super_safe ✓
─────────────────────────────────────────────────────────────────
```

### Face Score Logic (มีอยู่แล้วใน classify_batch.py)

```python
def _calculate_face_score(self, img: np.ndarray) -> float:
    """
    ใช้ Haar Cascade ตรวจจับหน้า

    Returns:
        0.0  = ไม่เจอหน้าเลย (ภาพห้อง/ฉากหลัง)
        0.1+ = เจอหน้าเล็กๆ (หน้า < 1% ของภาพ)
        0.3+ = เจอหน้าขนาดกลาง (หน้า 5-10% ของภาพ)
        0.5+ = เจอหน้าใหญ่ (หน้า > 10% ของภาพ)
    """
    faces = self.face_cascade.detectMultiScale(gray, ...)

    if len(faces) == 0:
        return 0.0  # ไม่มีหน้าคน

    # คำนวณ ratio ของหน้าต่อภาพทั้งหมด
    face_ratio = face_area / img_area
    return min(1.0, face_ratio * 5)
```

### Super Safe Selection Criteria

```python
# เงื่อนไขสำหรับ SUPER_SAFE
def is_super_safe(result):
    return (
        result.nsfw_score < 0.15 and      # ต้อง safe มากๆ
        result.face_score > 0.1 and        # ต้องมีหน้าคน
        result.error == ""                 # ไม่มี error
    )

# ภาพที่ไม่ผ่าน super_safe แต่ยัง safe
def is_safe_only(result):
    return (
        result.nsfw_score < 0.30 and      # safe threshold
        not is_super_safe(result)          # แต่ไม่ผ่าน super_safe
    )
```

### Storage Structure Update

```
gallery/{videoCode}/
├── super_safe/         ← NEW: สำหรับ Public SEO (score < 0.15)
│   ├── 001.jpg
│   └── ...
├── safe/               ← Borderline (score 0.15-0.3)
│   ├── 005.jpg
│   └── ...
├── nsfw/               ← Member only (score >= 0.3)
│   ├── 010.jpg
│   └── ...
└── metadata.json
```

### Implementation Changes

#### 1. Update Classifier Constants

```go
// infrastructure/classifier/types.go

const (
    // Three-Tier thresholds
    SuperSafeThreshold = 0.15  // สำหรับ Public Featured
    SafeThreshold      = 0.30  // สำหรับ Member

    // Minimum requirements
    MinSuperSafeImages = 10    // ต้องมีอย่างน้อย 10 ภาพ super safe
    MinSafeImages      = 12    // ต้องมีอย่างน้อย 12 ภาพ safe
    MaxNsfwImages      = 30    // เก็บไม่เกิน 30 ภาพ nsfw
)
```

#### 2. Update ClassificationResult & Config

```go
// infrastructure/classifier/types.go

// ClassifierConfig - เพิ่ม MinSuperSafeImages
type ClassifierConfig struct {
    PythonPath         string
    ScriptPath         string
    NsfwThreshold      float64 // 0.3
    SuperSafeThreshold float64 // NEW: 0.15
    MinFaceScore       float64 // NEW: 0.1
    Timeout            int
    MaxNsfwImages      int     // 30
    MinSafeImages      int     // 12
    MinSuperSafeImages int     // NEW: 10 (ต้องมีอย่างน้อย 10 ภาพ super_safe)
}

func DefaultConfig() ClassifierConfig {
    return ClassifierConfig{
        PythonPath:         "python",
        ScriptPath:         "infrastructure/classifier/classify_batch.py",
        NsfwThreshold:      0.3,
        SuperSafeThreshold: 0.15,  // NEW
        MinFaceScore:       0.1,   // NEW
        Timeout:            90,
        MaxNsfwImages:      30,
        MinSafeImages:      12,
        MinSuperSafeImages: 10,    // NEW
    }
}

type ClassificationResult struct {
    Filename       string  `json:"filename"`
    IsSuperSafe    bool    `json:"is_super_safe"`   // NEW: < 0.15 + face
    IsSafe         bool    `json:"is_safe"`         // < 0.30
    NsfwScore      float64 `json:"nsfw_score"`
    FalconsaiScore float64 `json:"falconsai_score"` // NEW: for tracking
    NudenetScore   float64 `json:"nudenet_score"`   // NEW: for tracking
    FaceScore      float64 `json:"face_score"`
    AestheticScore float64 `json:"aesthetic_score"`
    Error          string  `json:"error,omitempty"`
}

// SeparatedImages อัพเดทเป็น 3 ระดับ
type SeparatedImages struct {
    SuperSafe []ClassificationResult `json:"super_safe"` // < 0.15 + face
    Safe      []ClassificationResult `json:"safe"`       // 0.15-0.3 or no face
    Nsfw      []ClassificationResult `json:"nsfw"`       // >= 0.3
    Error     []ClassificationResult `json:"error"`
}
```

#### 3. Update Python Classifier

```python
# infrastructure/classifier/classify_batch.py

SUPER_SAFE_THRESHOLD = 0.15  # NEW
SAFE_THRESHOLD = 0.30
MIN_FACE_SCORE = 0.1  # NEW: ต้องมีหน้าคนในภาพ

def classify(self, image_path: str) -> Dict[str, Any]:
    # ... existing classification logic ...

    nsfw_score = max(falconsai_score, nudenet_score)
    face_score = self._calculate_face_score(cv_image)  # มีอยู่แล้ว

    # Three-tier classification
    # ⚠️ SUPER_SAFE ต้องมีหน้าคน (face_score > 0.1)
    # ป้องกันภาพห้องเปล่า/ฉากหลังหลุดไปเป็น featured image
    is_super_safe = (
        nsfw_score < SUPER_SAFE_THRESHOLD and
        face_score > MIN_FACE_SCORE  # ต้องมีหน้าคน!
    )
    is_safe = nsfw_score < SAFE_THRESHOLD

    return {
        "filename": filename,
        "is_super_safe": is_super_safe,  # NEW: ต้องมีหน้าคน
        "is_safe": is_safe,
        "nsfw_score": round(nsfw_score, 4),
        "falconsai_score": round(falconsai_score, 4),
        "nudenet_score": round(nudenet_score, 4),
        "face_score": round(face_score, 4),  # มีอยู่แล้ว
        "aesthetic_score": round(aesthetic_score, 4),
        "error": ""
    }
```

**หมายเหตุ:** `face_score` มีการคำนวณอยู่แล้วใน `_calculate_face_score()` ใช้ Haar Cascade ตรวจจับหน้า

#### 4. Update SeparateResults

```go
// infrastructure/classifier/nsfw_classifier.go

const (
    SuperSafeThreshold = 0.15
    SafeThreshold      = 0.30
    MinFaceScore       = 0.1  // ต้องมีหน้าคนในภาพ
)

func (c *NSFWClassifier) SeparateResults(results map[string]ClassificationResult) *SeparatedImages {
    separated := &SeparatedImages{
        SuperSafe: make([]ClassificationResult, 0),
        Safe:      make([]ClassificationResult, 0),
        Nsfw:      make([]ClassificationResult, 0),
        Error:     make([]ClassificationResult, 0),
    }

    for _, result := range results {
        if result.Error != "" {
            // Error → treat as NSFW (safety first)
            separated.Error = append(separated.Error, result)

        } else if result.NsfwScore < SuperSafeThreshold && result.FaceScore > MinFaceScore {
            // ⚠️ SUPER SAFE: ต้องผ่านทั้ง 2 เงื่อนไข
            // 1. nsfw_score < 0.15 (ไม่มีเนื้อหาอันตราย)
            // 2. face_score > 0.1 (มีหน้าคนในภาพ)
            // ป้องกันภาพห้องเปล่า/ฉากหลังหลุดไปเป็น featured image
            separated.SuperSafe = append(separated.SuperSafe, result)

        } else if result.NsfwScore < SafeThreshold {
            // SAFE: ไม่ผ่าน super_safe แต่ยัง safe
            // - อาจเป็นภาพห้อง (no face)
            // - หรือ score 0.15-0.3 (borderline)
            separated.Safe = append(separated.Safe, result)

        } else {
            // NSFW: >= 0.3
            separated.Nsfw = append(separated.Nsfw, result)
        }
    }

    return separated
}
```

**Logic สำคัญ:**
```
ภาพห้องเปล่า (nsfw: 0.05, face: 0.0) → SAFE (ไม่ใช่ super_safe)
ภาพคนคุยกัน (nsfw: 0.08, face: 0.35) → SUPER_SAFE ✓
ภาพ borderline (nsfw: 0.20, face: 0.40) → SAFE (nsfw > 0.15)
```

#### 5. Update Multi-Round Extraction (หาจนกว่าจะครบ)

```go
// use_cases/gallery_handler.go

// ProcessJobWithClassification - อัพเดทให้หา super_safe จนครบ
func (h *GalleryHandler) ProcessJobWithClassification(ctx context.Context, job *models.GalleryJob) error {
    // ... existing setup ...

    // Track ทั้ง 3 ระดับ
    var allSuperSafeResults []classifier.ClassificationResult  // NEW
    var allSafeResults []classifier.ClassificationResult
    var allNsfwResults []classifier.ClassificationResult

    for _, round := range extractionRounds {
        // ⚠️ เงื่อนไขหยุดใหม่: ต้องได้ทั้ง super_safe และ safe
        hasEnoughSuperSafe := len(allSuperSafeResults) >= classifierConfig.MinSuperSafeImages  // >= 10
        hasEnoughSafe := len(allSafeResults) + len(allSuperSafeResults) >= classifierConfig.MinSafeImages  // >= 12

        if hasEnoughSuperSafe && hasEnoughSafe {
            break  // ครบแล้ว หยุดได้
        }

        h.logger.Info("extraction round",
            "round", round.name,
            "current_super_safe", len(allSuperSafeResults),
            "current_safe", len(allSafeResults),
            "target_super_safe", classifierConfig.MinSuperSafeImages,
            "target_safe", classifierConfig.MinSafeImages,
        )

        // Extract frames for this round
        frameCount := h.extractRoundFramesFromHLS(...)

        // Classify all frames
        result, _ := nsfwClassifier.ClassifyBatch(ctx, allFramesDir)

        // Separate into 3 tiers
        separated := nsfwClassifier.SeparateResults(result.Results)

        // Move files to appropriate directories
        h.moveClassifiedFilesThreeTier(allFramesDir, superSafeDir, safeDir, nsfwDir, separated)

        // Accumulate results
        allSuperSafeResults = append(allSuperSafeResults, separated.SuperSafe...)
        allSafeResults = append(allSafeResults, separated.Safe...)
        allNsfwResults = append(allNsfwResults, separated.Nsfw...)

        h.logger.Info("round complete",
            "round", round.name,
            "super_safe_found", len(separated.SuperSafe),
            "safe_found", len(separated.Safe),
            "total_super_safe", len(allSuperSafeResults),
            "total_safe", len(allSafeResults),
        )
    }

    // Log final stats
    h.logger.Info("extraction complete",
        "total_super_safe", len(allSuperSafeResults),
        "total_safe", len(allSafeResults),
        "total_nsfw", len(allNsfwResults),
        "rounds_used", roundsUsed,
        "super_safe_target_met", len(allSuperSafeResults) >= classifierConfig.MinSuperSafeImages,
    )

    // ... upload and continue ...
}
```

#### 6. Target Counts

| Tier | Minimum Target | หยุดเมื่อ | เหตุผล |
|------|---------------|-----------|--------|
| **super_safe** | 10 ภาพ | ได้ครบ 10 | สำหรับ Public SEO (featured, gallery) |
| **safe** (รวม super_safe) | 12 ภาพ | ได้ครบ 12 | Backward compatible |
| **nsfw** | ไม่จำกัด (เก็บ max 30) | - | Member only |

**Logic:**
```go
// หยุด extraction เมื่อ:
// 1. super_safe >= 10 ภาพ (มีภาพคนสำหรับ Public)
// 2. total_safe (super_safe + safe) >= 12 ภาพ (backward compatible)

stopCondition := len(superSafe) >= 10 && (len(superSafe) + len(safe)) >= 12
```

#### 7. Handling Edge Cases

```go
// กรณี video เป็น NSFW เกือบทั้งหมด (ไม่มี super_safe พอ)

if len(allSuperSafeResults) < classifierConfig.MinSuperSafeImages {
    // Option A: ใช้ภาพที่มี face สูงสุดจาก safe (แม้ score > 0.15)
    // เลือกภาพที่ "safe ที่สุด" จากที่มี
    fallbackImages := selectBestFallbackImages(allSafeResults, classifierConfig.MinSuperSafeImages - len(allSuperSafeResults))
    allSuperSafeResults = append(allSuperSafeResults, fallbackImages...)

    // Option B: Flag video ว่า "limited_public_gallery"
    h.logger.Warn("not enough super_safe images",
        "video_code", job.VideoCode,
        "super_safe_count", len(allSuperSafeResults),
        "target", classifierConfig.MinSuperSafeImages,
    )
}

// selectBestFallbackImages: เลือกจาก safe ที่มี face สูงสุด และ nsfw ต่ำสุด
func selectBestFallbackImages(safeResults []classifier.ClassificationResult, count int) []classifier.ClassificationResult {
    // Sort by: face_score DESC, nsfw_score ASC
    sort.Slice(safeResults, func(i, j int) bool {
        scoreI := safeResults[i].FaceScore - safeResults[i].NsfwScore
        scoreJ := safeResults[j].FaceScore - safeResults[j].NsfwScore
        return scoreI > scoreJ
    })

    if len(safeResults) < count {
        return safeResults
    }
    return safeResults[:count]
}
```

#### 5. Update Gallery Upload

```go
// infrastructure/transcoder/gallery_classified.go

func UploadClassifiedGallery(
    ctx context.Context,
    result *ClassifiedGalleryResult,
    remotePrefix string,
    uploader GalleryUploader,
    logger *slog.Logger,
) (superSafeUploaded, safeUploaded, nsfwUploaded int, err error) {

    // Upload super_safe images (for Public SEO)
    superSafeRemote := filepath.ToSlash(filepath.Join(remotePrefix, "super_safe"))
    superSafeCount, _, _ := UploadGallery(ctx, result.SuperSafeDir, superSafeRemote, uploader, logger)

    // Upload safe images (borderline, lazy load)
    safeRemote := filepath.ToSlash(filepath.Join(remotePrefix, "safe"))
    safeCount, _, _ := UploadGallery(ctx, result.SafeDir, safeRemote, uploader, logger)

    // Upload nsfw images (member only)
    nsfwRemote := filepath.ToSlash(filepath.Join(remotePrefix, "nsfw"))
    nsfwCount, _, _ := UploadGallery(ctx, result.NsfwDir, nsfwRemote, uploader, logger)

    logger.Info("three-tier gallery uploaded",
        "remote_prefix", remotePrefix,
        "super_safe", superSafeCount,
        "safe", safeCount,
        "nsfw", nsfwCount,
    )

    return superSafeCount, safeCount, nsfwCount, nil
}
```

#### 6. Update Database Schema

```sql
-- migrations/002_add_three_tier_gallery.sql

ALTER TABLE videos ADD COLUMN gallery_super_safe_count INT DEFAULT 0;
-- gallery_safe_count already exists
-- gallery_nsfw_count already exists

-- Optional: Add flag for public-ready
ALTER TABLE videos ADD COLUMN gallery_public_ready BOOLEAN DEFAULT FALSE;

-- Index for API queries
CREATE INDEX idx_videos_gallery_public ON videos(gallery_public_ready);
```

### API Response Update

```go
// gofiber_starter/domain/dto/video.go

type GalleryInfoDTO struct {
    SuperSafeCount int  `json:"superSafeCount"` // สำหรับ Public Featured
    SafeCount      int  `json:"safeCount"`      // สำหรับ Public Lazy Load
    NsfwCount      int  `json:"nsfwCount"`      // สำหรับ Member
    PublicReady    bool `json:"publicReady"`    // >= 10 super_safe images
}

// API: GET /api/v1/videos/{id}/gallery
type VideoGalleryResponse struct {
    // Public pages ใช้ตัวนี้
    PublicImages []GalleryImageDTO `json:"publicImages"` // super_safe only

    // "ดูเพิ่มเติม" button (lazy load)
    BorderlineImages []GalleryImageDTO `json:"borderlineImages,omitempty"` // safe

    // Member pages ใช้ตัวนี้
    AllImages []GalleryImageDTO `json:"allImages,omitempty"` // super_safe + safe

    // Info
    Info GalleryInfoDTO `json:"info"`
}
```

### Frontend Integration

```tsx
// SEO Worker Article Generation

interface ArticleImages {
  featuredImage: string      // First super_safe image
  galleryImages: string[]    // All super_safe images (for Schema.org)
  lazyImages: string[]       // safe images (hidden from bot)
}

// Use only super_safe for public SEO content
const publicImages = await api.getGallery(videoId, { tier: 'super_safe' })

// Alt text rules (from FRONTEND_IMPLEMENTATION_PLAN.md)
const safeAltText = generateSafeAlt(video.title, video.cast[0].name)
```

### Priority & Impact

| Change | Priority | Impact |
|--------|----------|--------|
| Add `super_safe/` folder | HIGH | Google SafeSearch compliance |
| Update classifier constants | HIGH | Core logic change |
| Update Python classifier | HIGH | Add `is_super_safe` field |
| Update Go wrapper | MEDIUM | Parse new field |
| Update upload logic | MEDIUM | 3 folders instead of 2 |
| Update DB schema | MEDIUM | New counts |
| Backfill existing videos | LOW | Optional, lazy migration |

### Summary

```
BEFORE (Two-Tier):
  /safe/  (< 0.3)  →  Public + Member
  /nsfw/  (>= 0.3) →  Member only

AFTER (Three-Tier):
  /super_safe/ (< 0.15 + face) →  Public SEO (Google Bot sees)
  /safe/       (0.15 - 0.3)    →  Public Lazy (hidden from Bot)
  /nsfw/       (>= 0.3)        →  Member only
```

**เป้าหมาย:** ภาพที่ Google Bot เห็นต้องสะอาด 100%
- ไม่มี suggestive content
- ไม่มี arousal expression
- เห็นหน้าชัด ไม่มี intimate contact
