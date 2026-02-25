#!/usr/bin/env python3
"""
Test NudeNet detection on a single image
"""
import sys
import requests
from io import BytesIO
from PIL import Image
import numpy as np
import cv2

# Test URL (use one of the presigned URLs)
TEST_URL = sys.argv[1] if len(sys.argv) > 1 else None

if not TEST_URL:
    print("Usage: python test_nudenet.py <image_url>")
    sys.exit(1)

print(f"[1] Loading image from URL...")
response = requests.get(TEST_URL, timeout=15)
image = Image.open(BytesIO(response.content)).convert("RGB")
print(f"    Image size: {image.width}x{image.height}")

print(f"\n[2] Loading NudeNet...")
from nudenet import NudeDetector
detector = NudeDetector()
print("    [OK] NudeNet loaded")

print(f"\n[3] Running detection...")
img_array = np.array(image)
img_bgr = cv2.cvtColor(img_array, cv2.COLOR_RGB2BGR)

detections = detector.detect(img_bgr)

print(f"\n[4] Results ({len(detections)} detections):")
print("-" * 60)

BLUR_LABELS = [
    "EXPOSED_BREAST_F",
    "EXPOSED_GENITALIA_F",
    "EXPOSED_GENITALIA_M",
    "EXPOSED_BUTTOCKS",
    "EXPOSED_ANUS",
    "COVERED_BREAST_F",
]

blur_regions = []
for det in detections:
    label = det['class']
    score = det['score']
    box = det['box']
    should_blur = label in BLUR_LABELS and score > 0.3

    status = "🔴 BLUR" if should_blur else "⚪ skip"
    print(f"  {status} | {label:30} | score={score:.3f} | box={box}")

    if should_blur:
        blur_regions.append(det)

print("-" * 60)
print(f"\nTotal: {len(detections)} detections, {len(blur_regions)} to blur")

if blur_regions:
    print(f"\n[5] Drawing boxes on image...")

    # Draw boxes
    for region in blur_regions:
        box = region['box']
        x1, y1, x2, y2 = int(box[0]), int(box[1]), int(box[2]), int(box[3])

        # Draw red rectangle
        cv2.rectangle(img_bgr, (x1, y1), (x2, y2), (0, 0, 255), 3)

        # Add label
        label = f"{region['class']} ({region['score']:.2f})"
        cv2.putText(img_bgr, label, (x1, y1-10), cv2.FONT_HERSHEY_SIMPLEX, 0.5, (0, 0, 255), 2)

    # Save with boxes
    output_path = "output/test_detection_boxes.jpg"
    cv2.imwrite(output_path, img_bgr)
    print(f"    Saved: {output_path}")

    # Also save blurred version
    print(f"\n[6] Creating blurred version...")
    img_blur = img_bgr.copy()

    for region in blur_regions:
        box = region['box']
        x1, y1, x2, y2 = int(box[0]), int(box[1]), int(box[2]), int(box[3])

        # Ensure bounds
        h, w = img_blur.shape[:2]
        x1, y1 = max(0, x1), max(0, y1)
        x2, y2 = min(w, x2), min(h, y2)

        if x2 > x1 and y2 > y1:
            roi = img_blur[y1:y2, x1:x2]

            # Heavy blur + pixelation
            blur_size = 61
            blurred_roi = cv2.GaussianBlur(roi, (blur_size, blur_size), 0)

            # Pixelate
            small = cv2.resize(blurred_roi, (max(1, (x2-x1)//15), max(1, (y2-y1)//15)))
            pixelated = cv2.resize(small, (x2-x1, y2-y1), interpolation=cv2.INTER_NEAREST)

            img_blur[y1:y2, x1:x2] = pixelated

    blur_output_path = "output/test_detection_blurred.jpg"
    cv2.imwrite(blur_output_path, img_blur)
    print(f"    Saved: {blur_output_path}")

else:
    print("\n⚠️  No regions to blur detected!")
