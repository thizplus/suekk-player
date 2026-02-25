#!/usr/bin/env python3
"""
Test NudeNet detection on local blurred images
Compare with original to see detection boxes
"""
import os
import glob
from PIL import Image
import numpy as np
import cv2
from nudenet import NudeDetector

BLURRED_DIR = "output/blurred"
OUTPUT_DIR = "output/debug_boxes"

os.makedirs(OUTPUT_DIR, exist_ok=True)

print("[1] Loading NudeNet...")
detector = NudeDetector()
print("    [OK] NudeNet loaded")

BLUR_LABELS = [
    "FEMALE_BREAST_EXPOSED",
    "FEMALE_GENITALIA_EXPOSED",
    "MALE_GENITALIA_EXPOSED",
    "BUTTOCKS_EXPOSED",
    "ANUS_EXPOSED",
    "FEMALE_BREAST_COVERED",
]

# Get all blurred images
blurred_files = glob.glob(os.path.join(BLURRED_DIR, "*_blurred.jpg"))
print(f"\n[2] Found {len(blurred_files)} blurred images")

for filepath in blurred_files:
    filename = os.path.basename(filepath)
    print(f"\n{'='*60}")
    print(f"Checking: {filename}")
    print(f"{'='*60}")

    # Load blurred image
    img = cv2.imread(filepath)
    if img is None:
        print("  [ERROR] Could not load image")
        continue

    print(f"  Size: {img.shape[1]}x{img.shape[0]}")

    # Run detection on blurred image
    detections = detector.detect(img)

    print(f"  Detections: {len(detections)}")

    blur_regions = []
    for det in detections:
        label = det['class']
        score = det['score']
        box = det['box']
        should_blur = label in BLUR_LABELS and score > 0.3

        status = "[SHOULD_BLUR]" if should_blur else "[skip]"
        print(f"    {status} | {label:30} | score={score:.3f} | box={box}")

        if should_blur:
            blur_regions.append(det)

    # If still detecting NSFW after blur, the blur didn't work well
    if blur_regions:
        print(f"\n  [WARNING]  WARNING: Still detecting {len(blur_regions)} NSFW regions after blur!")
        print(f"     This means the blur didn't cover these areas properly.")
    else:
        print(f"\n  [OK] No NSFW regions detected - blur worked!")

    # Draw all detections on image for visualization
    img_debug = img.copy()
    for det in detections:
        box = det['box']
        x1, y1, x2, y2 = int(box[0]), int(box[1]), int(box[2]), int(box[3])

        # Color based on whether it should be blurred
        color = (0, 0, 255) if det['class'] in BLUR_LABELS else (0, 255, 0)
        cv2.rectangle(img_debug, (x1, y1), (x2, y2), color, 2)

        label = f"{det['class'][:20]} ({det['score']:.2f})"
        cv2.putText(img_debug, label, (x1, y1-5), cv2.FONT_HERSHEY_SIMPLEX, 0.4, color, 1)

    # Save debug image
    debug_path = os.path.join(OUTPUT_DIR, f"debug_{filename}")
    cv2.imwrite(debug_path, img_debug)
    print(f"  Debug image saved: {debug_path}")

print(f"\n{'='*60}")
print("Done! Check output/debug_boxes/ for visualizations")
