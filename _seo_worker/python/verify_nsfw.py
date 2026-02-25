#!/usr/bin/env python3
"""
Verify blurred images pass NSFW check
"""
import glob
from PIL import Image
from transformers import pipeline

BLURRED_DIR = "output/blurred"
NSFW_THRESHOLD = 0.3

print("[1] Loading NSFW classifier (Falconsai)...")
classifier = pipeline(
    "image-classification",
    model="Falconsai/nsfw_image_detection",
    device=0  # CUDA
)
print("    [OK] NSFW classifier loaded")

# Get all blurred images
blurred_files = glob.glob(f"{BLURRED_DIR}/*_blurred.jpg")
print(f"\n[2] Checking {len(blurred_files)} blurred images...")
print("=" * 60)

passed = 0
failed = 0

for filepath in blurred_files:
    filename = filepath.split("\\")[-1]

    # Load image
    image = Image.open(filepath).convert("RGB")

    # Run NSFW classification
    results = classifier(image)

    # Get NSFW score
    nsfw_score = 0.0
    for r in results:
        label = r["label"].lower()
        if label in ["nsfw", "porn", "sexy", "hentai"]:
            nsfw_score = r["score"]
            break

    # Check if passed
    is_safe = nsfw_score < NSFW_THRESHOLD
    status = "[PASS]" if is_safe else "[FAIL]"

    if is_safe:
        passed += 1
    else:
        failed += 1

    print(f"  {status} {filename:25} nsfw_score={nsfw_score:.3f}")

print("=" * 60)
print(f"\nResult: {passed}/{len(blurred_files)} passed (threshold={NSFW_THRESHOLD})")

if failed > 0:
    print(f"[WARNING] {failed} images still detected as NSFW!")
else:
    print("[OK] All blurred images are safe for SEO!")
