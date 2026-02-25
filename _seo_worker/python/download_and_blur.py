#!/usr/bin/env python3
"""
Download original images from E2 storage and apply Smart Blur
Then verify with Falconsai NSFW classifier
"""
import os
import sys
from pathlib import Path
from io import BytesIO

import boto3
from botocore.config import Config
from PIL import Image
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

# Add parent dir for imports
sys.path.insert(0, str(Path(__file__).parent))
from image_selector import SmartBlur

# E2 Configuration
E2_ENDPOINT = os.getenv("SUEKK_STORAGE_ENDPOINT", "https://s3.ap-southeast-1.idrivee2.com")
E2_ACCESS_KEY = os.getenv("SUEKK_STORAGE_ACCESS_KEY")
E2_SECRET_KEY = os.getenv("SUEKK_STORAGE_SECRET_KEY")
E2_BUCKET = os.getenv("SUEKK_STORAGE_BUCKET", "suekk-01")

# Output directories
ORIGINALS_DIR = "output/originals"
BLURRED_DIR = "output/blurred"

# Images to test (NSFW images from previous run)
TEST_IMAGES = [
    "gallery/utywgage/009.jpg",
    "gallery/utywgage/010.jpg",
    "gallery/utywgage/025.jpg",
    "gallery/utywgage/028.jpg",
    "gallery/utywgage/063.jpg",
    "gallery/utywgage/073.jpg",
    "gallery/utywgage/076.jpg",
]


def create_s3_client():
    """Create S3 client for E2 storage"""
    return boto3.client(
        's3',
        endpoint_url=E2_ENDPOINT,
        aws_access_key_id=E2_ACCESS_KEY,
        aws_secret_access_key=E2_SECRET_KEY,
        config=Config(signature_version='s3v4'),
        region_name='auto'
    )


def download_image(s3_client, key: str) -> Image.Image:
    """Download image from S3 bucket"""
    response = s3_client.get_object(Bucket=E2_BUCKET, Key=key)
    image_data = response['Body'].read()
    return Image.open(BytesIO(image_data)).convert("RGB")


def main():
    os.makedirs(ORIGINALS_DIR, exist_ok=True)
    os.makedirs(BLURRED_DIR, exist_ok=True)

    print("[1] Connecting to E2 storage...")
    try:
        s3_client = create_s3_client()
        print("    [OK] Connected to E2 storage")
    except Exception as e:
        print(f"    [ERROR] Failed to connect: {e}")
        return

    print("\n[2] Loading SmartBlur...")
    smart_blur = SmartBlur(output_dir=BLURRED_DIR)
    smart_blur.load_model()

    if smart_blur.detector is None:
        print("[ERROR] Could not load NudeNet detector")
        return

    print("    Settings: blur_radius=75, expand_percent=0.40, 5 passes + pixelation + gray overlay")

    print(f"\n[3] Processing {len(TEST_IMAGES)} images...")
    print("=" * 60)

    blurred_count = 0

    for key in TEST_IMAGES:
        filename = key.split("/")[-1]
        print(f"\nDownloading: {key}")

        try:
            # Download image
            image = download_image(s3_client, key)
            print(f"  Size: {image.width}x{image.height}")

            # Save original
            original_path = os.path.join(ORIGINALS_DIR, filename)
            image.save(original_path, "JPEG", quality=95)
            print(f"  Saved original: {original_path}")

            # Apply blur
            was_blurred, blurred_path = smart_blur.process_image(image, filename)

            if was_blurred:
                print(f"  [OK] Blurred: {blurred_path}")
                blurred_count += 1
            else:
                print(f"  [SKIP] No NSFW regions detected")

        except Exception as e:
            print(f"  [ERROR] {e}")

    print("=" * 60)
    print(f"\nDone! Blurred {blurred_count}/{len(TEST_IMAGES)} images")
    print(f"Originals: {ORIGINALS_DIR}")
    print(f"Blurred: {BLURRED_DIR}")
    print("\n[4] Now running Falconsai verification...")

    # Run verification
    os.system("python python/verify_nsfw.py")


if __name__ == "__main__":
    main()
