#!/usr/bin/env python3
"""
NSFW Batch Classifier for Gallery Worker

This script classifies images for NSFW content using NudeNet and face detection.

Classification Tiers:
- super_safe: score < 0.15 AND has face detected
- safe: score 0.15-0.30 OR (score < 0.15 without face)
- nsfw: score >= 0.30

Usage:
    python classify_batch.py --images img1.jpg img2.jpg img3.jpg
    python classify_batch.py --images *.jpg --super-safe-threshold 0.15 --safe-threshold 0.30

Output:
    JSON array of classification results to stdout
"""

import argparse
import json
import sys
from pathlib import Path
from typing import List, Dict, Any

# Lazy imports to improve startup time
_nudenet = None
_face_detector = None


def get_nudenet():
    """Lazy load NudeNet classifier"""
    global _nudenet
    if _nudenet is None:
        try:
            from nudenet import NudeClassifier
            _nudenet = NudeClassifier()
        except ImportError:
            sys.stderr.write("Warning: NudeNet not installed. Using fallback.\n")
            _nudenet = "fallback"
    return _nudenet


def get_face_detector():
    """Lazy load face detector (RetinaFace or cv2)"""
    global _face_detector
    if _face_detector is None:
        try:
            import cv2
            face_cascade = cv2.CascadeClassifier(
                cv2.data.haarcascades + 'haarcascade_frontalface_default.xml'
            )
            _face_detector = ("cv2", face_cascade)
        except Exception:
            _face_detector = "fallback"
    return _face_detector


def detect_face(image_path: str) -> bool:
    """Detect if image contains a face"""
    detector = get_face_detector()

    if detector == "fallback":
        return False

    try:
        detector_type, cascade = detector
        import cv2
        img = cv2.imread(image_path)
        if img is None:
            return False

        gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
        faces = cascade.detectMultiScale(gray, 1.1, 4)
        return len(faces) > 0
    except Exception:
        return False


def classify_image(image_path: str) -> float:
    """Get NSFW score for an image (0.0 = safe, 1.0 = NSFW)"""
    classifier = get_nudenet()

    if classifier == "fallback":
        # Fallback: return low score
        return 0.1

    try:
        result = classifier.classify(image_path)
        # NudeNet returns dict like {image_path: {'unsafe': 0.7, 'safe': 0.3}}
        if image_path in result:
            return result[image_path].get('unsafe', 0.0)
        return 0.0
    except Exception as e:
        sys.stderr.write(f"Warning: Classification failed for {image_path}: {e}\n")
        return 0.0


def classify_batch(
    image_paths: List[str],
    super_safe_threshold: float = 0.15,
    safe_threshold: float = 0.30
) -> List[Dict[str, Any]]:
    """
    Classify a batch of images.

    Args:
        image_paths: List of image file paths
        super_safe_threshold: Score below this + face = super_safe
        safe_threshold: Score below this = safe, above = nsfw

    Returns:
        List of classification results
    """
    results = []

    for path in image_paths:
        if not Path(path).exists():
            sys.stderr.write(f"Warning: Image not found: {path}\n")
            continue

        # Get NSFW score
        score = classify_image(path)

        # Detect face
        has_face = detect_face(path)

        # Determine classification
        if score < super_safe_threshold and has_face:
            classification = "super_safe"
        elif score < safe_threshold:
            classification = "safe"
        else:
            classification = "nsfw"

        results.append({
            "filename": Path(path).name,
            "classification": classification,
            "score": round(score, 4),
            "has_face": has_face
        })

    return results


def main():
    parser = argparse.ArgumentParser(description="NSFW Batch Classifier")
    parser.add_argument(
        "--images",
        nargs="+",
        required=True,
        help="Image paths to classify"
    )
    parser.add_argument(
        "--super-safe-threshold",
        type=float,
        default=0.15,
        help="Threshold for super_safe classification (default: 0.15)"
    )
    parser.add_argument(
        "--safe-threshold",
        type=float,
        default=0.30,
        help="Threshold for safe vs nsfw (default: 0.30)"
    )
    parser.add_argument(
        "--output",
        type=str,
        default=None,
        help="Output file path (default: stdout)"
    )

    args = parser.parse_args()

    # Classify images
    results = classify_batch(
        args.images,
        super_safe_threshold=args.super_safe_threshold,
        safe_threshold=args.safe_threshold
    )

    # Output as JSON
    output_json = json.dumps(results, indent=2, ensure_ascii=False)

    if args.output:
        with open(args.output, 'w', encoding='utf-8') as f:
            f.write(output_json)
    else:
        print(output_json)


if __name__ == "__main__":
    main()
