#!/usr/bin/env python3
"""
NSFW Batch Classifier
Classifies all images in a folder using Falconsai + NudeNet (dual model)
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
from typing import Dict, List, Any, Optional

import cv2
import numpy as np
from PIL import Image


# ═══════════════════════════════════════════════════════════════════════════════
# Configuration (Three-Tier System)
# ═══════════════════════════════════════════════════════════════════════════════

# Three-Tier Thresholds
SUPER_SAFE_THRESHOLD = 0.15  # Score below this + face = super safe (Public SEO)
NSFW_THRESHOLD = 0.3         # Score above this = NSFW
MIN_FACE_SCORE = 0.1         # Minimum face score for super_safe

# NudeNet labels that indicate NSFW content (NudeNet v2/v3 format)
# Used as secondary detection
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
# Dual Model NSFW Classifier (Falconsai + NudeNet)
# ═══════════════════════════════════════════════════════════════════════════════

class NSFWClassifier:
    """
    NSFW classifier using dual models:
    - Falconsai/nsfw_image_detection (primary, more accurate)
    - NudeNet (secondary, region-based detection)
    """

    def __init__(self):
        self.falconsai_model = None
        self.nudenet_detector = None
        self.face_cascade = None
        self._loaded = False

    def load(self):
        """Load all models (lazy loading)"""
        if self._loaded:
            return

        # 1. Load Falconsai model (primary NSFW classifier)
        try:
            from transformers import pipeline
            import torch

            device = 0 if torch.cuda.is_available() else -1
            self.falconsai_model = pipeline(
                "image-classification",
                model="Falconsai/nsfw_image_detection",
                device=device
            )
            print("[OK] Falconsai NSFW model loaded", file=sys.stderr)
        except Exception as e:
            print(f"[WARN] Could not load Falconsai model: {e}", file=sys.stderr)

        # 2. Load NudeNet (secondary, region-based)
        try:
            from nudenet import NudeDetector
            self.nudenet_detector = NudeDetector()
            print("[OK] NudeNet loaded", file=sys.stderr)
        except Exception as e:
            print(f"[WARN] Could not load NudeNet: {e}", file=sys.stderr)

        # 3. Load face cascade for face detection
        try:
            cascade_path = cv2.data.haarcascades + 'haarcascade_frontalface_default.xml'
            self.face_cascade = cv2.CascadeClassifier(cascade_path)
            print("[OK] Face cascade loaded", file=sys.stderr)
        except Exception as e:
            print(f"[WARN] Could not load face cascade: {e}", file=sys.stderr)

        self._loaded = True

    def classify(self, image_path: str) -> Dict[str, Any]:
        """
        Classify a single image using dual models (Three-Tier System)
        Returns: {filename, is_super_safe, is_safe, nsfw_score, face_score, aesthetic_score, error}

        Three-Tier Classification:
        - super_safe: nsfw_score < 0.15 AND face_score > 0.1 (Public SEO)
        - safe: nsfw_score < 0.30 (Lazy load)
        - nsfw: nsfw_score >= 0.30 (Member only)
        """
        filename = os.path.basename(image_path)

        try:
            # Load image with PIL for Falconsai
            pil_image = Image.open(image_path).convert("RGB")

            # Load image with OpenCV for NudeNet
            cv_image = cv2.imread(image_path)
            if cv_image is None:
                return {
                    "filename": filename,
                    "is_super_safe": False,
                    "is_safe": False,
                    "nsfw_score": 1.0,
                    "face_score": 0.0,
                    "aesthetic_score": 0.0,
                    "error": "Failed to load image"
                }

            # 1. Falconsai classification (primary)
            falconsai_score = self._score_falconsai(pil_image)

            # 2. NudeNet detection (secondary)
            nudenet_score = self._score_nudenet(cv_image)

            # Combined NSFW score: use MAX of both models (stricter)
            nsfw_score = max(falconsai_score, nudenet_score)

            # Calculate face score (ต้องมีหน้าคนสำหรับ super_safe)
            face_score = self._calculate_face_score(cv_image)

            # Simple aesthetic score
            aesthetic_score = self._calculate_aesthetic_score(cv_image)

            # Three-Tier Classification
            # super_safe: ต้องมีหน้าคน + nsfw ต่ำมาก (ป้องกันภาพห้องเปล่า)
            is_super_safe = (
                nsfw_score < SUPER_SAFE_THRESHOLD and
                face_score > MIN_FACE_SCORE
            )
            is_safe = nsfw_score < NSFW_THRESHOLD

            return {
                "filename": filename,
                "is_super_safe": is_super_safe,
                "is_safe": is_safe,
                "nsfw_score": round(nsfw_score, 4),
                "face_score": round(face_score, 4),
                "aesthetic_score": round(aesthetic_score, 4),
                "error": ""
            }

        except Exception as e:
            return {
                "filename": filename,
                "is_super_safe": False,
                "is_safe": False,
                "nsfw_score": 1.0,
                "face_score": 0.0,
                "aesthetic_score": 0.0,
                "error": str(e)
            }

    def _score_falconsai(self, pil_image: Image.Image) -> float:
        """Score image using Falconsai model (0=safe, 1=nsfw)"""
        if self.falconsai_model is None:
            return 0.0

        try:
            results = self.falconsai_model(pil_image)
            for r in results:
                label = r["label"].lower()
                if label in ["nsfw", "porn", "sexy", "hentai"]:
                    return r["score"]
            return 0.0
        except Exception as e:
            print(f"[WARN] Falconsai error: {e}", file=sys.stderr)
            return 0.0

    def _score_nudenet(self, cv_image: np.ndarray) -> float:
        """Score image using NudeNet region detection (0=safe, 1=nsfw)"""
        if self.nudenet_detector is None:
            return 0.0

        try:
            detections = self.nudenet_detector.detect(cv_image)

            if not detections:
                return 0.0

            max_nsfw_score = 0.0
            for det in detections:
                if det['class'] in NSFW_LABELS:
                    max_nsfw_score = max(max_nsfw_score, det['score'])

            return max_nsfw_score
        except Exception as e:
            print(f"[WARN] NudeNet error: {e}", file=sys.stderr)
            return 0.0

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
            if max_face_ratio < 0.01:
                return max_face_ratio * 10
            elif max_face_ratio > 0.5:
                return 0.5
            else:
                return min(1.0, max_face_ratio * 5)

        except Exception:
            return 0.0

    def _calculate_aesthetic_score(self, img: np.ndarray) -> float:
        """Simple aesthetic score based on image properties"""
        try:
            gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
            laplacian_var = cv2.Laplacian(gray, cv2.CV_64F).var()
            sharpness = min(1.0, laplacian_var / 500)

            brightness = np.mean(gray) / 255.0
            brightness_score = 1.0 - abs(brightness - 0.5) * 2

            return (sharpness * 0.6 + brightness_score * 0.4)

        except Exception:
            return 0.5


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
    parser = argparse.ArgumentParser(description="NSFW Batch Classifier (Falconsai + NudeNet)")
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
