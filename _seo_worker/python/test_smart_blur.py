#!/usr/bin/env python3
"""
Test Smart Blur with new aggressive settings
Downloads original NSFW images and applies blur, then verifies with Falconsai
"""
import os
import sys
import requests
from io import BytesIO
from PIL import Image
import numpy as np
import cv2
from pathlib import Path

# Add parent dir for imports
sys.path.insert(0, str(Path(__file__).parent))
from image_selector import SmartBlur

OUTPUT_DIR = "output/blurred"
os.makedirs(OUTPUT_DIR, exist_ok=True)

# Test images - NSFW images that need blur (from previous run)
# Using direct gallery URLs (will need presigned URLs)
TEST_IMAGES = [
    "009", "010", "025", "028", "063", "073", "076"
]

# E2 bucket base path
BUCKET_BASE = "https://s3.ap-southeast-1.idrivee2.com/suekk-01/gallery/utywgage"

def get_presigned_url_or_direct(image_num: str) -> str:
    """Try to get image - this would need presigned URL in production"""
    # For testing, let's try to use existing local downloaded images
    local_path = f"output/originals/{image_num}.jpg"
    if os.path.exists(local_path):
        return local_path
    return None


def main():
    print("[1] Loading SmartBlur with new settings...")
    smart_blur = SmartBlur(output_dir=OUTPUT_DIR)
    smart_blur.load_model()

    if smart_blur.detector is None:
        print("[ERROR] Could not load NudeNet detector")
        return

    print(f"    Settings: blur_radius=75, expand_percent=0.40, 5 passes + pixelation + color overlay")

    # Check for local original images
    originals_dir = "output/originals"
    if not os.path.exists(originals_dir):
        print(f"\n[ERROR] No originals directory found at {originals_dir}")
        print("    Please first download the original NSFW images to test.")
        print("    You can use: python test_nudenet.py <presigned_url>")
        return

    # Find all original images
    original_files = [f for f in os.listdir(originals_dir) if f.endswith('.jpg')]
    print(f"\n[2] Found {len(original_files)} original images in {originals_dir}")

    if not original_files:
        print("[ERROR] No images found to process")
        return

    blurred_count = 0

    for filename in sorted(original_files):
        filepath = os.path.join(originals_dir, filename)
        print(f"\n{'='*60}")
        print(f"Processing: {filename}")
        print(f"{'='*60}")

        # Load image
        image = Image.open(filepath).convert("RGB")
        print(f"  Size: {image.width}x{image.height}")

        # Detect and blur
        was_blurred, output_path = smart_blur.process_image(image, filename)

        if was_blurred:
            print(f"  [OK] Blurred and saved to: {output_path}")
            blurred_count += 1
        else:
            print(f"  [SKIP] No NSFW regions detected")

    print(f"\n{'='*60}")
    print(f"Done! Blurred {blurred_count}/{len(original_files)} images")
    print(f"Output directory: {OUTPUT_DIR}")
    print(f"\nNow run verify_nsfw.py to check if they pass Falconsai")


if __name__ == "__main__":
    main()
