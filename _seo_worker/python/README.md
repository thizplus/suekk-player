# Image Selector for SEO Worker

AI-powered image selection for cover and gallery images.

## Features

1. **NSFW Filtering** - คัดรูปที่โป๊เกินไปออก (threshold 0.3)
2. **Face Detection** - เลือกรูปที่เห็นหน้านักแสดงชัด
3. **Aesthetic Scoring** - เลือกรูปที่คุณภาพดี คมชัด

## Installation

```bash
cd python
pip install -r requirements.txt
```

## Usage

### Score Single Image

```bash
python image_selector.py --url https://example.com/image.jpg
```

Output:
```json
{
  "url": "https://example.com/image.jpg",
  "filename": "image.jpg",
  "nsfw_score": 0.15,
  "face_score": 0.72,
  "aesthetic_score": 0.68,
  "combined_score": 0.75,
  "is_safe": true
}
```

### Select Cover + Gallery from URLs

```bash
# Create input file
echo '["url1.jpg", "url2.jpg", ...]' > gallery_urls.json

# Run selection
python image_selector.py --input gallery_urls.json --output selected.json
```

Output:
```json
{
  "cover": {
    "url": "best_cover.jpg",
    "nsfw_score": 0.05,
    "face_score": 0.85,
    "aesthetic_score": 0.78,
    "combined_score": 0.86,
    "is_safe": true
  },
  "gallery": [
    {"url": "img1.jpg", ...},
    {"url": "img2.jpg", ...},
    ...
  ],
  "stats": {
    "total_images": 100,
    "safe_images": 85,
    "processing_time": 45.2
  }
}
```

## Integration with Go SEO Worker

### Environment Variables
```bash
# .env file
IMAGE_SELECTOR_PYTHON=python    # or /usr/bin/python3
IMAGE_SELECTOR_SCRIPT=python/image_selector.py
IMAGE_SELECTOR_DEVICE=cuda      # or cpu
```

### Go Implementation
The Go SEO Handler automatically calls the Python script via `ImageSelectorPort`:

```go
// infrastructure/imageselector/python_selector.go
// - Writes image URLs to temp JSON file
// - Calls Python script with --input and --output
// - Parses JSON result into models.ImageSelectionResult
```

### Manual Call (for testing)
```go
// Call Python script from Go
cmd := exec.Command("python", "python/image_selector.py",
    "--input", inputFile,
    "--output", outputFile)
err := cmd.Run()
```

## Algorithm

1. **Input**: 100 images from gallery/{code}/
2. **Step 1 (NSFW Check)**: Filter images with nsfw_score > 0.3
3. **Step 2 (Face Detection)**: Score face visibility (0-1)
4. **Step 3 (Aesthetic Score)**: Score quality using CLIP (0-1)
5. **Output**:
   - **Cover**: Best safe image with face_score > 0.2 and highest combined_score
   - **Gallery**: 12 diverse images spread across the gallery

## Score Weights

- NSFW: 40% (lower is better)
- Face: 30% (higher is better)
- Aesthetic: 30% (higher is better)

## Models Used

- **NSFW**: `Falconsai/nsfw_image_detection` (HuggingFace)
- **Face**: OpenCV Haar Cascade
- **Aesthetic**: OpenAI CLIP (ViT-B/32)
