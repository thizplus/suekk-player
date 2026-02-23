#!/usr/bin/env python3
"""
NSFW Batch Classifier
Classifies all images in a folder using NudeNet
Outputs JSON result to stdout or file

Usage:
    python classify_batch.py --input /path/to/images --output result.json
    python classify_batch.py --input /path/to/images  # Output to stdout
"""
import os
import sys
import json
import argparse
import time
from pathlib import Path
from typing import Dict, List, Any

import cv2
import numpy as np
from PIL import Image


# ═══════════════════════════════════════════════════════════════════════════════
# Configuration
# ═══════════════════════════════════════════════════════════════════════════════

NSFW_THRESHOLD = 0.3  # Score above this = NSFW

# NudeNet labels that indicate NSFW content (NudeNet v2/v3 format)
NSFW_LABELS = [
    "EXPOSED_BREAST_F",
    "EXPOSED_GENITALIA_F",
    "EXPOSED_GENITALIA_M",
    "EXPOSED_BUTTOCKS",
    "EXPOSED_ANUS",
    "COVERED_BREAST_F",  # Also filter for safety
]

# Supported image extensions
IMAGE_EXTENSIONS = {'.jpg', '.jpeg', '.png', '.webp'}


# ═══════════════════════════════════════════════════════════════════════════════
# NudeNet Classifier
# ═══════════════════════════════════════════════════════════════════════════════

class NSFWClassifier:
    """NSFW classifier using NudeNet"""

    def __init__(self):
        self.detector = None
        self.face_cascade = None
        self._loaded = False

    def load(self):
        """Load NudeNet model (lazy loading)"""
        if self._loaded:
            return

        try:
            from nudenet import NudeDetector
            self.detector = NudeDetector()

            # Load face cascade for face detection
            cascade_path = cv2.data.haarcascades + 'haarcascade_frontalface_default.xml'
            self.face_cascade = cv2.CascadeClassifier(cascade_path)

            self._loaded = True
            print("[OK] NudeNet loaded", file=sys.stderr)
        except Exception as e:
            print(f"[ERROR] Failed to load NudeNet: {e}", file=sys.stderr)
            raise

    def classify(self, image_path: str) -> Dict[str, Any]:
        """
        Classify a single image
        Returns: {filename, is_safe, nsfw_score, face_score, aesthetic_score, error}
        """
        filename = os.path.basename(image_path)

        try:
            # Load image
            img = cv2.imread(image_path)
            if img is None:
                return {
                    "filename": filename,
                    "is_safe": False,  # Safety first
                    "nsfw_score": 1.0,
                    "face_score": 0.0,
                    "aesthetic_score": 0.0,
                    "error": "Failed to load image"
                }

            # Run NudeNet detection
            detections = self.detector.detect(img)

            # Calculate NSFW score based on detected regions
            nsfw_score = self._calculate_nsfw_score(detections)
            is_safe = nsfw_score < NSFW_THRESHOLD

            # Calculate face score
            face_score = self._calculate_face_score(img)

            # Simple aesthetic score (based on image quality)
            aesthetic_score = self._calculate_aesthetic_score(img)

            return {
                "filename": filename,
                "is_safe": is_safe,
                "nsfw_score": round(nsfw_score, 4),
                "face_score": round(face_score, 4),
                "aesthetic_score": round(aesthetic_score, 4),
                "error": ""
            }

        except Exception as e:
            return {
                "filename": filename,
                "is_safe": False,  # Safety first
                "nsfw_score": 1.0,
                "face_score": 0.0,
                "aesthetic_score": 0.0,
                "error": str(e)
            }

    def _calculate_nsfw_score(self, detections: List[Dict]) -> float:
        """Calculate overall NSFW score from detections"""
        if not detections:
            return 0.0

        max_nsfw_score = 0.0
        for det in detections:
            if det['class'] in NSFW_LABELS:
                max_nsfw_score = max(max_nsfw_score, det['score'])

        return max_nsfw_score

    def _calculate_face_score(self, img: np.ndarray) -> float:
        """Calculate face visibility score (0-1)"""
        if self.face_cascade is None:
            return 0.0

        try:
            gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
            faces = self.face_cascade.detectMultiScale(
                gray,
                scaleFactor=1.1,
                minNeighbors=5,
                minSize=(50, 50)
            )

            if len(faces) == 0:
                return 0.0

            # Score based on face size relative to image
            img_h, img_w = img.shape[:2]
            img_area = img_h * img_w

            max_face_ratio = 0.0
            for (x, y, w, h) in faces:
                face_area = w * h
                face_ratio = face_area / img_area
                max_face_ratio = max(max_face_ratio, face_ratio)

            # Normalize: face taking 5-20% of image is ideal
            # Score peaks at ~10% and decreases for very large/small faces
            if max_face_ratio < 0.01:
                return max_face_ratio * 10  # Too small
            elif max_face_ratio > 0.5:
                return 0.5  # Too large (cropped face)
            else:
                return min(1.0, max_face_ratio * 5)

        except Exception:
            return 0.0

    def _calculate_aesthetic_score(self, img: np.ndarray) -> float:
        """Simple aesthetic score based on image properties"""
        try:
            # Check image sharpness using Laplacian variance
            gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
            laplacian_var = cv2.Laplacian(gray, cv2.CV_64F).var()

            # Normalize sharpness (higher is better, cap at 1.0)
            sharpness = min(1.0, laplacian_var / 500)

            # Check brightness
            brightness = np.mean(gray) / 255.0
            # Penalize too dark or too bright
            brightness_score = 1.0 - abs(brightness - 0.5) * 2

            # Combined score
            return (sharpness * 0.6 + brightness_score * 0.4)

        except Exception:
            return 0.5  # Default middle score


# ═══════════════════════════════════════════════════════════════════════════════
# Batch Processing
# ═══════════════════════════════════════════════════════════════════════════════

def get_image_files(input_path: str) -> List[str]:
    """Get all image files from input path (file or directory)"""
    input_path = Path(input_path)

    if input_path.is_file():
        return [str(input_path)]

    if input_path.is_dir():
        files = []
        for ext in IMAGE_EXTENSIONS:
            files.extend(input_path.glob(f"*{ext}"))
            files.extend(input_path.glob(f"*{ext.upper()}"))
        return sorted([str(f) for f in files])

    return []


def classify_batch(input_path: str) -> Dict[str, Any]:
    """
    Classify all images in input path
    Returns BatchResult as dict
    """
    start_time = time.time()

    # Get image files
    image_files = get_image_files(input_path)
    if not image_files:
        return {
            "results": {},
            "stats": {
                "total_images": 0,
                "safe_count": 0,
                "nsfw_count": 0,
                "error_count": 0,
                "avg_nsfw_score": 0.0,
                "processing_time_sec": 0.0
            },
            "output_path": input_path
        }

    # Load classifier
    classifier = NSFWClassifier()
    classifier.load()

    # Process each image
    results = {}
    total_nsfw_score = 0.0
    safe_count = 0
    nsfw_count = 0
    error_count = 0

    for i, image_path in enumerate(image_files):
        result = classifier.classify(image_path)
        filename = result["filename"]
        results[filename] = result

        if result["error"]:
            error_count += 1
        elif result["is_safe"]:
            safe_count += 1
        else:
            nsfw_count += 1

        total_nsfw_score += result["nsfw_score"]

        # Progress to stderr (every 10 images)
        if (i + 1) % 10 == 0:
            print(f"[PROGRESS] {i + 1}/{len(image_files)} images processed", file=sys.stderr)

    processing_time = time.time() - start_time

    # Calculate stats
    total_images = len(image_files)
    avg_nsfw_score = total_nsfw_score / total_images if total_images > 0 else 0.0

    return {
        "results": results,
        "stats": {
            "total_images": total_images,
            "safe_count": safe_count,
            "nsfw_count": nsfw_count,
            "error_count": error_count,
            "avg_nsfw_score": round(avg_nsfw_score, 4),
            "processing_time_sec": round(processing_time, 2)
        },
        "output_path": input_path
    }


# ═══════════════════════════════════════════════════════════════════════════════
# Main
# ═══════════════════════════════════════════════════════════════════════════════

def main():
    parser = argparse.ArgumentParser(description="NSFW Batch Classifier")
    parser.add_argument("--input", "-i", required=True, help="Input folder or image file")
    parser.add_argument("--output", "-o", help="Output JSON file (default: stdout)")
    parser.add_argument("--threshold", "-t", type=float, default=0.3, help="NSFW threshold (default: 0.3)")

    args = parser.parse_args()

    # Update threshold if specified
    global NSFW_THRESHOLD
    NSFW_THRESHOLD = args.threshold

    # Validate input
    if not os.path.exists(args.input):
        print(json.dumps({"error": f"Input path does not exist: {args.input}"}))
        sys.exit(1)

    # Run classification
    try:
        result = classify_batch(args.input)

        # Output result
        output_json = json.dumps(result, ensure_ascii=False, indent=2)

        if args.output:
            with open(args.output, 'w', encoding='utf-8') as f:
                f.write(output_json)
            print(f"[OK] Results written to {args.output}", file=sys.stderr)
        else:
            print(output_json)

        # Print summary to stderr
        stats = result["stats"]
        print(f"\n[SUMMARY]", file=sys.stderr)
        print(f"  Total: {stats['total_images']}", file=sys.stderr)
        print(f"  Safe: {stats['safe_count']}", file=sys.stderr)
        print(f"  NSFW: {stats['nsfw_count']}", file=sys.stderr)
        print(f"  Errors: {stats['error_count']}", file=sys.stderr)
        print(f"  Avg NSFW Score: {stats['avg_nsfw_score']}", file=sys.stderr)
        print(f"  Time: {stats['processing_time_sec']}s", file=sys.stderr)

    except Exception as e:
        print(json.dumps({"error": str(e)}))
        sys.exit(1)


if __name__ == "__main__":
    main()
