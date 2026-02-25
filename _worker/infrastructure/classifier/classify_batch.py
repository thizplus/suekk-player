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

# Perceptual Hash for deduplication
try:
    import imagehash
    IMAGEHASH_AVAILABLE = True
except ImportError:
    IMAGEHASH_AVAILABLE = False
    print("[WARN] imagehash not installed. Run: pip install imagehash", file=sys.stderr)


# ═══════════════════════════════════════════════════════════════════════════════
# Configuration (Three-Tier System)
# ═══════════════════════════════════════════════════════════════════════════════

# Three-Tier Thresholds
SUPER_SAFE_THRESHOLD = 0.15  # Score below this + face = super safe (Public SEO)
NSFW_THRESHOLD = 0.3         # Score above this = NSFW
MIN_FACE_SCORE = 0.1         # Minimum face score for super_safe

# NudeNet labels that indicate NSFW content (NudeNet v3 format)
# Updated to match actual NudeNet output labels
NSFW_LABELS = [
    # Exposed body parts (high severity)
    "FEMALE_BREAST_EXPOSED",
    "FEMALE_GENITALIA_EXPOSED",
    "MALE_GENITALIA_EXPOSED",
    "BUTTOCKS_EXPOSED",
    "ANUS_EXPOSED",
    # Covered but still suggestive (medium severity)
    "FEMALE_BREAST_COVERED",
    "BELLY_EXPOSED",
]

# Supported image extensions
IMAGE_EXTENSIONS = {'.jpg', '.jpeg', '.png', '.webp'}

# Deduplication settings
PHASH_THRESHOLD = 8  # Hamming distance threshold (0=identical, lower=more strict)


# ═══════════════════════════════════════════════════════════════════════════════
# Image Deduplication using Perceptual Hash (pHash)
# ═══════════════════════════════════════════════════════════════════════════════

def compute_phash(image_path: str) -> Optional[Any]:
    """Compute perceptual hash for an image"""
    if not IMAGEHASH_AVAILABLE:
        return None
    try:
        img = Image.open(image_path)
        return imagehash.phash(img)
    except Exception as e:
        print(f"[PHASH] Error computing hash for {image_path}: {e}", file=sys.stderr)
        return None


def deduplicate_images(image_files: List[str], threshold: int = PHASH_THRESHOLD, verbose: bool = False) -> List[str]:
    """
    Remove duplicate/similar images using perceptual hash.
    Returns list of unique image paths.

    Args:
        image_files: List of image file paths
        threshold: Hamming distance threshold (lower = more strict)
        verbose: Print deduplication details
    """
    if not IMAGEHASH_AVAILABLE:
        print("[DEDUP] imagehash not available, skipping deduplication", file=sys.stderr)
        return image_files

    if len(image_files) <= 1:
        return image_files

    print(f"[DEDUP] Computing hashes for {len(image_files)} images...", file=sys.stderr)

    # Compute hashes for all images
    hashes = []
    for path in image_files:
        h = compute_phash(path)
        hashes.append((path, h))

    # Group similar images
    unique_images = []
    duplicate_count = 0
    used = set()

    for i, (path, h) in enumerate(hashes):
        if path in used:
            continue
        if h is None:
            # Include images with hash errors
            unique_images.append(path)
            continue

        # Mark this image as used
        used.add(path)
        unique_images.append(path)

        # Find and mark all similar images
        for j in range(i + 1, len(hashes)):
            other_path, other_h = hashes[j]
            if other_path in used or other_h is None:
                continue

            # Calculate hamming distance
            distance = h - other_h

            if distance <= threshold:
                # Mark as duplicate
                used.add(other_path)
                duplicate_count += 1
                if verbose:
                    print(f"[DEDUP] {os.path.basename(other_path)} is similar to {os.path.basename(path)} (distance={distance})", file=sys.stderr)

    print(f"[DEDUP] Removed {duplicate_count} duplicates, {len(unique_images)} unique images remain", file=sys.stderr)

    return unique_images


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
        self.skip_mosaic = False
        self.skip_pov = False

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

    def classify(self, image_path: str, verbose: bool = False) -> Dict[str, Any]:
        """
        Classify a single image using dual models (Three-Tier System)
        Returns: {filename, is_super_safe, is_safe, nsfw_score, face_score, aesthetic_score, error}

        Three-Tier Classification:
        - super_safe: nsfw_score < 0.15 AND face_score > 0.1 AND no mosaic (Public SEO)
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
                    "falconsai_score": 0.0,
                    "nudenet_score": 0.0,
                    "mosaic_detected": False,
                    "mosaic_score": 0.0,
                    "classification": "error",
                    "reason": "Failed to load image",
                    "error": "Failed to load image"
                }

            # 1. Falconsai classification (general NSFW detection)
            falconsai_score = self._score_falconsai(pil_image)

            # 2. NudeNet detection (actual nudity detection - more accurate)
            nudenet_score = self._score_nudenet(cv_image, filename, verbose)

            # Combined NSFW score: Weighted Logic (trust NudeNet more)
            # NudeNet ดีกว่าในการ detect nudity จริง, Falconsai มี bias สูง
            if nudenet_score < 0.25:
                # NudeNet บอกว่าไม่โป๊เลย → ลดอิทธิพล Falconsai 70%
                nsfw_score = falconsai_score * 0.3
            elif nudenet_score > 0.6:
                # NudeNet เห็น nudity ชัดเจน → เชื่อ NudeNet 100%
                nsfw_score = nudenet_score
            else:
                # กรณีคลุมเครือ (0.25 - 0.6) → ถ่วงน้ำหนัก NudeNet 70%, Falconsai 30%
                nsfw_score = (nudenet_score * 0.7) + (falconsai_score * 0.3)

            # Calculate face score and get face data (ต้องมีหน้าคนสำหรับ super_safe)
            face_score, face_data = self._calculate_face_score(cv_image)

            # Simple aesthetic score
            aesthetic_score = self._calculate_aesthetic_score(cv_image)

            # 3. Mosaic/Censorship detection (catches censored NSFW content)
            if self.skip_mosaic:
                mosaic_detected, mosaic_score, mosaic_details = False, 0.0, "skipped"
            else:
                mosaic_detected, mosaic_score, mosaic_details = self._detect_mosaic(cv_image, verbose)

            # 4. POV (Point of View) detection (catches suggestive POV compositions)
            if self.skip_pov:
                pov_detected, pov_score, pov_details = False, 0.0, "skipped"
            else:
                pov_detected, pov_score, pov_details = self._detect_pov(cv_image, face_data, verbose)

            # Three-Tier Classification with detailed reasoning
            # super_safe: ต้องมีหน้าคน + nsfw ต่ำมาก + ไม่มี mosaic + ไม่ใช่ POV
            is_super_safe = (
                nsfw_score < SUPER_SAFE_THRESHOLD and
                face_score > MIN_FACE_SCORE and
                not mosaic_detected and
                not pov_detected
            )
            is_safe = nsfw_score < NSFW_THRESHOLD and not mosaic_detected

            # Determine classification and reason
            if mosaic_detected:
                # Mosaic detected = definitely not super_safe
                classification = "nsfw"
                reason = f"mosaic detected ({mosaic_details})"
            elif pov_detected:
                # POV detected = not super_safe, move to safe
                classification = "safe"
                reason = f"POV composition detected ({pov_details})"
            elif is_super_safe:
                classification = "super_safe"
                reason = f"nsfw={nsfw_score:.4f}<{SUPER_SAFE_THRESHOLD} AND face={face_score:.4f}>{MIN_FACE_SCORE} AND clean"
            elif nsfw_score < NSFW_THRESHOLD:
                classification = "safe"
                if nsfw_score >= SUPER_SAFE_THRESHOLD:
                    reason = f"nsfw={nsfw_score:.4f}>={SUPER_SAFE_THRESHOLD} (too high for super_safe)"
                else:
                    reason = f"face={face_score:.4f}<={MIN_FACE_SCORE} (no face detected)"
            else:
                classification = "nsfw"
                reason = f"nsfw={nsfw_score:.4f}>={NSFW_THRESHOLD}"

            # Verbose logging per image
            if verbose:
                dominant_model = "falconsai" if falconsai_score >= nudenet_score else "nudenet"
                flags = []
                if mosaic_detected:
                    flags.append("MOSAIC")
                if pov_detected:
                    flags.append("POV")
                flags_str = f" [{','.join(flags)}]" if flags else ""
                print(f"[CLASSIFY] {filename}: "
                      f"falcon={falconsai_score:.4f} nude={nudenet_score:.4f} "
                      f"combined={nsfw_score:.4f} (from {dominant_model}) "
                      f"face={face_score:.4f} mosaic={mosaic_score:.3f} pov={pov_score:.2f}{flags_str} → {classification.upper()} "
                      f"({reason})", file=sys.stderr)

            return {
                "filename": filename,
                "is_super_safe": is_super_safe,
                "is_safe": is_safe,
                "nsfw_score": round(nsfw_score, 4),
                "face_score": round(face_score, 4),
                "aesthetic_score": round(aesthetic_score, 4),
                "falconsai_score": round(falconsai_score, 4),
                "nudenet_score": round(nudenet_score, 4),
                "mosaic_detected": mosaic_detected,
                "mosaic_score": round(mosaic_score, 4),
                "pov_detected": pov_detected,
                "pov_score": round(pov_score, 4),
                "classification": classification,
                "reason": reason,
                "error": ""
            }

        except Exception as e:
            if verbose:
                print(f"[ERROR] {filename}: {e}", file=sys.stderr)
            return {
                "filename": filename,
                "is_super_safe": False,
                "is_safe": False,
                "nsfw_score": 1.0,
                "face_score": 0.0,
                "aesthetic_score": 0.0,
                "falconsai_score": 0.0,
                "nudenet_score": 0.0,
                "mosaic_detected": False,
                "mosaic_score": 0.0,
                "pov_detected": False,
                "pov_score": 0.0,
                "classification": "error",
                "reason": str(e),
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
                    return float(r["score"])  # Convert numpy to native float
            return 0.0
        except Exception as e:
            print(f"[WARN] Falconsai error: {e}", file=sys.stderr)
            return 0.0

    def _score_nudenet(self, cv_image: np.ndarray, filename: str = "", verbose: bool = False) -> float:
        """Score image using NudeNet region detection (0=safe, 1=nsfw)"""
        if self.nudenet_detector is None:
            if verbose:
                print(f"[NUDENET] {filename}: detector NOT LOADED!", file=sys.stderr)
            return 0.0

        try:
            detections = self.nudenet_detector.detect(cv_image)

            if not detections:
                if verbose:
                    print(f"[NUDENET] {filename}: no detections found", file=sys.stderr)
                return 0.0

            # Log all detections for debugging
            if verbose:
                det_summary = [(d['class'], round(d['score'], 3)) for d in detections]
                print(f"[NUDENET] {filename}: found {len(detections)} detections: {det_summary}", file=sys.stderr)

            max_nsfw_score = 0.0
            for det in detections:
                if det['class'] in NSFW_LABELS:
                    max_nsfw_score = max(max_nsfw_score, float(det['score']))  # Convert numpy

            if verbose and max_nsfw_score > 0:
                print(f"[NUDENET] {filename}: NSFW score = {max_nsfw_score:.4f}", file=sys.stderr)

            return max_nsfw_score
        except Exception as e:
            print(f"[WARN] NudeNet error for {filename}: {e}", file=sys.stderr)
            return 0.0

    def _calculate_face_score(self, img: np.ndarray) -> tuple:
        """
        Calculate face visibility score (0-1) and return face data.
        Returns (score: float, faces: list of (x, y, w, h))
        """
        if self.face_cascade is None:
            return 0.0, []

        try:
            gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
            faces = self.face_cascade.detectMultiScale(
                gray,
                scaleFactor=1.1,
                minNeighbors=5,
                minSize=(50, 50)
            )

            if len(faces) == 0:
                return 0.0, []

            # Convert to list of tuples
            face_list = [(int(x), int(y), int(w), int(h)) for (x, y, w, h) in faces]

            # Score based on face size relative to image
            img_h, img_w = img.shape[:2]
            img_area = img_h * img_w

            max_face_ratio = 0.0
            for (x, y, w, h) in face_list:
                face_area = w * h
                face_ratio = face_area / img_area
                max_face_ratio = max(max_face_ratio, face_ratio)

            # Normalize: face taking 5-20% of image is ideal
            if max_face_ratio < 0.01:
                score = float(max_face_ratio * 10)
            elif max_face_ratio > 0.5:
                score = 0.5
            else:
                score = float(min(1.0, max_face_ratio * 5))

            return score, face_list

        except Exception:
            return 0.0, []

    def _calculate_aesthetic_score(self, img: np.ndarray) -> float:
        """Simple aesthetic score based on image properties"""
        try:
            gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
            laplacian_var = float(cv2.Laplacian(gray, cv2.CV_64F).var())  # Convert numpy
            sharpness = min(1.0, laplacian_var / 500)

            brightness = float(np.mean(gray)) / 255.0  # Convert numpy
            brightness_score = 1.0 - abs(brightness - 0.5) * 2

            return float(sharpness * 0.6 + brightness_score * 0.4)  # Ensure native float

        except Exception:
            return 0.5

    def _detect_mosaic(self, img: np.ndarray, verbose: bool = False) -> tuple:
        """
        Detect mosaic/pixelation censorship in image.
        Returns (is_mosaic_detected: bool, mosaic_ratio: float, details: str)

        Mosaic censorship creates characteristic patterns:
        - Blocky square regions with uniform colors
        - Sharp edges at regular grid intervals
        - Typically in skin-tone colored regions
        """
        try:
            img_h, img_w = img.shape[:2]

            # Skip very small images
            if img_w < 100 or img_h < 100:
                return False, 0.0, "image too small"

            # Convert to different color spaces
            gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
            hsv = cv2.cvtColor(img, cv2.COLOR_BGR2HSV)

            # Method 1: Detect skin-tone mosaic regions
            # Skin tone in HSV: H=0-25, S=40-170, V=80-255
            lower_skin = np.array([0, 40, 80], dtype=np.uint8)
            upper_skin = np.array([25, 170, 255], dtype=np.uint8)
            skin_mask = cv2.inRange(hsv, lower_skin, upper_skin)

            # Also check for pink/red tones (another common skin range)
            lower_skin2 = np.array([170, 40, 80], dtype=np.uint8)
            upper_skin2 = np.array([180, 170, 255], dtype=np.uint8)
            skin_mask2 = cv2.inRange(hsv, lower_skin2, upper_skin2)
            skin_mask = cv2.bitwise_or(skin_mask, skin_mask2)

            # Method 2: Detect blocky patterns in skin regions
            # Use multiple block sizes to catch different mosaic resolutions
            best_mosaic_score = 0.0
            best_details = ""

            for block_size in [8, 12, 16, 20]:
                mosaic_blocks = 0
                skin_blocks = 0

                for y in range(0, img_h - block_size, block_size // 2):  # Overlapping windows
                    for x in range(0, img_w - block_size, block_size // 2):
                        # Check if this region has significant skin tone
                        skin_region = skin_mask[y:y+block_size, x:x+block_size]
                        skin_ratio = np.sum(skin_region > 0) / (block_size * block_size)

                        if skin_ratio < 0.3:  # Skip non-skin regions
                            continue

                        skin_blocks += 1

                        # Analyze the block for mosaic pattern
                        block = gray[y:y+block_size, x:x+block_size]

                        # Split into sub-blocks
                        half = block_size // 2
                        quarter = block_size // 4

                        # Get sub-regions
                        sub_blocks = [
                            block[:half, :half],
                            block[:half, half:],
                            block[half:, :half],
                            block[half:, half:]
                        ]

                        # Calculate variance within each sub-block
                        sub_vars = [np.var(sb) for sb in sub_blocks]
                        sub_means = [np.mean(sb) for sb in sub_blocks]

                        # Mosaic characteristics:
                        # 1. Low variance within sub-blocks (uniform color)
                        # 2. Different means between sub-blocks
                        # 3. Sharp edges at boundaries

                        avg_var = np.mean(sub_vars)
                        max_var = max(sub_vars)
                        mean_range = max(sub_means) - min(sub_means)

                        # Real mosaic blocks: low internal variance + color difference
                        # Relax variance slightly to catch compression artifacts
                        if max_var < 120 and avg_var < 80 and mean_range > 15:
                            # Check edge sharpness at block boundaries
                            h_edge = np.abs(float(np.mean(block[half-1, :])) - float(np.mean(block[half, :])))
                            v_edge = np.abs(float(np.mean(block[:, half-1])) - float(np.mean(block[:, half])))

                            # Require sharp edges
                            if h_edge > 12 or v_edge > 12:
                                mosaic_blocks += 1

                if skin_blocks > 10:  # Need enough skin blocks to analyze
                    mosaic_ratio = mosaic_blocks / skin_blocks
                    if mosaic_ratio > best_mosaic_score:
                        best_mosaic_score = mosaic_ratio
                        best_details = f"block{block_size}:mosaic={mosaic_blocks}/{skin_blocks}={mosaic_ratio:.3f}"

            # Method 3: Detect grid pattern using Laplacian
            # Mosaic creates high-frequency grid-like edges
            laplacian = cv2.Laplacian(gray, cv2.CV_64F)

            # Focus on skin regions only
            skin_laplacian = laplacian.copy()
            skin_laplacian[skin_mask == 0] = 0

            # Calculate local variance of Laplacian in skin regions
            if np.sum(skin_mask > 0) > 1000:  # Enough skin pixels
                skin_lap_var = np.var(skin_laplacian[skin_mask > 0])
                # High variance in skin region = potential mosaic edges
                if skin_lap_var > 500:
                    lap_score = min(1.0, skin_lap_var / 2000)
                    if lap_score > best_mosaic_score * 0.5:  # Boost score if laplacian also high
                        best_mosaic_score = max(best_mosaic_score, best_mosaic_score + lap_score * 0.3)
                        best_details += f", lap_var={skin_lap_var:.0f}"

            # Threshold for mosaic detection
            # Balanced to catch real mosaic while avoiding false positives
            MOSAIC_THRESHOLD = 0.005  # 0.5% of skin blocks showing mosaic pattern

            is_mosaic = best_mosaic_score > MOSAIC_THRESHOLD

            if verbose:
                if is_mosaic:
                    print(f"[MOSAIC] DETECTED! score={best_mosaic_score:.3f}, {best_details}", file=sys.stderr)
                elif best_mosaic_score > 0.01:
                    print(f"[MOSAIC] Low score={best_mosaic_score:.3f}, {best_details}", file=sys.stderr)

            return is_mosaic, float(best_mosaic_score), best_details

        except Exception as e:
            if verbose:
                print(f"[MOSAIC] Error: {e}", file=sys.stderr)
            return False, 0.0, f"error: {e}"

    def _detect_pov(self, img: np.ndarray, face_data: list, verbose: bool = False) -> tuple:
        """
        Detect POV (Point of View) adult composition.
        Returns (is_pov_detected: bool, pov_score: float, details: str)

        POV characteristics:
        - Very large face (>15% of image)
        - Face centered horizontally
        - Skin-tone mass in bottom portion extending toward face
        - V-shape composition (dark sides, light center bottom)
        """
        try:
            img_h, img_w = img.shape[:2]
            img_area = img_h * img_w

            # Need face data to detect POV
            if not face_data or len(face_data) == 0:
                return False, 0.0, "no face"

            # Get largest face
            largest_face = max(face_data, key=lambda f: f[2] * f[3])
            fx, fy, fw, fh = largest_face
            face_area = fw * fh
            face_ratio = face_area / img_area

            # Check 1: Face must be large (>15% of image)
            if face_ratio < 0.15:
                return False, 0.0, f"face too small ({face_ratio:.2%})"

            # Check 2: Face should be roughly centered horizontally
            face_center_x = fx + fw / 2
            center_offset = abs(face_center_x - img_w / 2) / (img_w / 2)
            if center_offset > 0.4:  # Allow 40% offset from center
                return False, 0.0, f"face not centered ({center_offset:.2f})"

            # Check 3: Face should be in upper portion of image (stricter for POV)
            face_center_y = fy + fh / 2
            face_y_ratio = face_center_y / img_h
            if face_y_ratio > 0.50:  # Face center should be in upper 50% for true POV
                return False, 0.0, f"face not in upper portion ({face_y_ratio:.2f})"

            # Check 4: Detect skin-tone in bottom of image
            hsv = cv2.cvtColor(img, cv2.COLOR_BGR2HSV)

            # Skin tone detection
            lower_skin = np.array([0, 40, 80], dtype=np.uint8)
            upper_skin = np.array([25, 170, 255], dtype=np.uint8)
            skin_mask = cv2.inRange(hsv, lower_skin, upper_skin)

            # Also check pink/red tones
            lower_skin2 = np.array([170, 40, 80], dtype=np.uint8)
            upper_skin2 = np.array([180, 170, 255], dtype=np.uint8)
            skin_mask2 = cv2.inRange(hsv, lower_skin2, upper_skin2)
            skin_mask = cv2.bitwise_or(skin_mask, skin_mask2)

            # Analyze bottom third of image (below face)
            bottom_start = int(img_h * 0.6)
            bottom_region = skin_mask[bottom_start:, :]
            bottom_area = bottom_region.shape[0] * bottom_region.shape[1]

            # Calculate skin ratio in bottom region
            bottom_skin_pixels = np.sum(bottom_region > 0)
            bottom_skin_ratio = bottom_skin_pixels / bottom_area if bottom_area > 0 else 0

            # KEY CHECK: Skin must be at the VERY bottom edge (last 10% of image)
            # This distinguishes POV (body extending from bottom) from portrait (hands near face)
            bottom_edge_start = int(img_h * 0.9)
            bottom_edge_region = skin_mask[bottom_edge_start:, :]
            bottom_edge_area = bottom_edge_region.shape[0] * bottom_edge_region.shape[1]
            bottom_edge_skin_ratio = np.sum(bottom_edge_region > 0) / bottom_edge_area if bottom_edge_area > 0 else 0

            # Check 5: Skin should extend from bottom toward center (V-shape)
            # Divide bottom into left, center, right thirds
            third_w = img_w // 3
            left_region = skin_mask[bottom_start:, :third_w]
            center_region = skin_mask[bottom_start:, third_w:2*third_w]
            right_region = skin_mask[bottom_start:, 2*third_w:]

            left_skin = np.sum(left_region > 0) / (left_region.size + 1)
            center_skin = np.sum(center_region > 0) / (center_region.size + 1)
            right_skin = np.sum(right_region > 0) / (right_region.size + 1)

            # V-shape: center should have more skin than sides
            # Or elongated shape extending upward
            v_shape_score = 0.0
            if center_skin > 0.1:  # Significant skin in center
                if center_skin > left_skin and center_skin > right_skin:
                    v_shape_score = center_skin
                elif center_skin > 0.15:  # Or just high center skin
                    v_shape_score = center_skin * 0.8

            # Calculate POV score
            pov_score = 0.0

            # Large face contributes
            if face_ratio > 0.20:
                pov_score += 0.3
            elif face_ratio > 0.15:
                pov_score += 0.2

            # Bottom skin contributes
            if bottom_skin_ratio > 0.15:
                pov_score += 0.3
            elif bottom_skin_ratio > 0.08:
                pov_score += 0.2

            # V-shape contributes
            if v_shape_score > 0.15:
                pov_score += 0.3
            elif v_shape_score > 0.08:
                pov_score += 0.2

            # Face in upper portion contributes
            if face_y_ratio < 0.4:
                pov_score += 0.2

            # Threshold for POV detection (stricter)
            POV_THRESHOLD = 0.7

            # Additional requirement: need significant skin at VERY BOTTOM EDGE
            # This catches POV (body from bottom) while ignoring portraits (hands near face)
            # Threshold 50% = need most of bottom edge to be skin (body/legs visible)
            is_pov = bool(pov_score >= POV_THRESHOLD and
                         bottom_skin_ratio > 0.20 and
                         bottom_edge_skin_ratio > 0.50)  # Must have body at very bottom

            details = (f"face={face_ratio:.1%}, y={face_y_ratio:.2f}, "
                      f"bottom_skin={bottom_skin_ratio:.1%}, edge_skin={bottom_edge_skin_ratio:.1%}, v_shape={v_shape_score:.2f}")

            if verbose:
                if is_pov:
                    print(f"[POV] DETECTED! score={pov_score:.2f}, {details}", file=sys.stderr)
                elif pov_score > 0.3:
                    print(f"[POV] Possible score={pov_score:.2f}, {details}", file=sys.stderr)

            return is_pov, float(pov_score), details

        except Exception as e:
            if verbose:
                print(f"[POV] Error: {e}", file=sys.stderr)
            return False, 0.0, f"error: {e}"


# ═══════════════════════════════════════════════════════════════════════════════
# Batch Processing
# ═══════════════════════════════════════════════════════════════════════════════

def get_image_files(input_path: str) -> List[str]:
    """Get all image files from input path (file or directory)"""
    input_path = Path(input_path)

    if input_path.is_file():
        return [str(input_path)]

    if input_path.is_dir():
        files = set()  # Use set to avoid duplicates on case-insensitive systems
        for ext in IMAGE_EXTENSIONS:
            files.update(str(f) for f in input_path.glob(f"*{ext}"))
            files.update(str(f) for f in input_path.glob(f"*{ext.upper()}"))
        return sorted(files)

    return []


def classify_batch(input_path: str, verbose: bool = False, skip_mosaic: bool = False, skip_pov: bool = False, skip_dedup: bool = False, dedup_threshold: int = PHASH_THRESHOLD) -> Dict[str, Any]:
    """
    Classify all images in input path
    Returns BatchResult as dict

    Args:
        input_path: Path to folder or image file
        verbose: If True, print detailed per-image classification log
        skip_mosaic: If True, skip slow mosaic detection
        skip_pov: If True, skip slow POV detection
        skip_dedup: If True, skip image deduplication
        dedup_threshold: Hamming distance threshold for dedup (0=identical, 8=default)
    """
    start_time = time.time()

    # Get image files
    image_files = get_image_files(input_path)
    if not image_files:
        return {
            "results": {},
            "stats": {
                "total_images": 0,
                "super_safe_count": 0,
                "safe_count": 0,
                "nsfw_count": 0,
                "error_count": 0,
                "mosaic_count": 0,
                "pov_count": 0,
                "avg_nsfw_score": 0.0,
                "avg_face_score": 0.0,
                "processing_time_sec": 0.0
            },
            "output_path": input_path
        }

    # Deduplicate images first (remove similar frames)
    original_count = len(image_files)
    if not skip_dedup:
        image_files = deduplicate_images(image_files, threshold=dedup_threshold, verbose=verbose)
        dedup_removed = original_count - len(image_files)
    else:
        dedup_removed = 0
        print("[CONFIG] Skipping deduplication", file=sys.stderr)

    # Load classifier
    classifier = NSFWClassifier()
    classifier.skip_mosaic = skip_mosaic
    classifier.skip_pov = skip_pov
    classifier.load()

    if skip_mosaic or skip_pov:
        skipped = []
        if skip_mosaic:
            skipped.append("mosaic")
        if skip_pov:
            skipped.append("POV")
        print(f"[CONFIG] Skipping slow detections: {', '.join(skipped)}", file=sys.stderr)

    if verbose:
        print(f"\n[CONFIG] Thresholds: SUPER_SAFE<{SUPER_SAFE_THRESHOLD}, "
              f"NSFW>={NSFW_THRESHOLD}, MIN_FACE>{MIN_FACE_SCORE}", file=sys.stderr)
        print(f"[START] Processing {len(image_files)} images...\n", file=sys.stderr)

    # Process each image
    results = {}
    total_nsfw_score = 0.0
    total_face_score = 0.0
    super_safe_count = 0
    safe_count = 0
    nsfw_count = 0
    error_count = 0
    mosaic_count = 0
    pov_count = 0

    for i, image_path in enumerate(image_files):
        result = classifier.classify(image_path, verbose=verbose)
        filename = result["filename"]
        results[filename] = result

        if result["error"]:
            error_count += 1
        elif result["is_super_safe"]:
            super_safe_count += 1
        elif result["is_safe"]:
            safe_count += 1
        else:
            nsfw_count += 1

        # Count mosaic detections
        if result.get("mosaic_detected", False):
            mosaic_count += 1

        # Count POV detections
        if result.get("pov_detected", False):
            pov_count += 1

        total_nsfw_score += result["nsfw_score"]
        total_face_score += result["face_score"]

        # Progress to stderr (every 10 images, only if not verbose)
        if not verbose and (i + 1) % 10 == 0:
            print(f"[PROGRESS] {i + 1}/{len(image_files)} images processed", file=sys.stderr)

    processing_time = time.time() - start_time

    # Calculate stats
    total_images = len(image_files)
    avg_nsfw_score = total_nsfw_score / total_images if total_images > 0 else 0.0
    avg_face_score = total_face_score / total_images if total_images > 0 else 0.0

    return {
        "results": results,
        "stats": {
            "total_images": total_images,
            "original_images": original_count,
            "duplicates_removed": dedup_removed,
            "super_safe_count": super_safe_count,
            "safe_count": safe_count,
            "nsfw_count": nsfw_count,
            "error_count": error_count,
            "mosaic_count": mosaic_count,
            "pov_count": pov_count,
            "avg_nsfw_score": round(avg_nsfw_score, 4),
            "avg_face_score": round(avg_face_score, 4),
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
    parser.add_argument("--super-safe-threshold", type=float, default=0.15, help="Super safe threshold (default: 0.15)")
    parser.add_argument("--min-face-score", type=float, default=0.1, help="Minimum face score for super_safe (default: 0.1)")
    parser.add_argument("--verbose", "-v", action="store_true", help="Print detailed per-image classification log")
    parser.add_argument("--skip-mosaic", action="store_true", help="Skip slow mosaic detection")
    parser.add_argument("--skip-pov", action="store_true", help="Skip slow POV detection")
    parser.add_argument("--skip-dedup", action="store_true", help="Skip image deduplication")
    parser.add_argument("--dedup-threshold", type=int, default=8, help="Dedup hamming distance threshold (default: 8, lower=stricter)")

    args = parser.parse_args()

    # Update thresholds if specified
    global NSFW_THRESHOLD, SUPER_SAFE_THRESHOLD, MIN_FACE_SCORE
    NSFW_THRESHOLD = args.threshold
    SUPER_SAFE_THRESHOLD = args.super_safe_threshold
    MIN_FACE_SCORE = args.min_face_score

    # Validate input
    if not os.path.exists(args.input):
        print(json.dumps({"error": f"Input path does not exist: {args.input}"}))
        sys.exit(1)

    # Run classification
    try:
        result = classify_batch(
            args.input,
            verbose=args.verbose,
            skip_mosaic=args.skip_mosaic,
            skip_pov=args.skip_pov,
            skip_dedup=args.skip_dedup,
            dedup_threshold=args.dedup_threshold
        )

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
        print(f"  Original Images: {stats.get('original_images', stats['total_images'])}", file=sys.stderr)
        print(f"  Duplicates Removed: {stats.get('duplicates_removed', 0)}", file=sys.stderr)
        print(f"  Unique Images: {stats['total_images']}", file=sys.stderr)
        print(f"  Super Safe: {stats['super_safe_count']} (nsfw<{SUPER_SAFE_THRESHOLD} + face>{MIN_FACE_SCORE} + clean)", file=sys.stderr)
        print(f"  Safe: {stats['safe_count']} (nsfw<{NSFW_THRESHOLD} or POV)", file=sys.stderr)
        print(f"  NSFW: {stats['nsfw_count']} (nsfw>={NSFW_THRESHOLD} or mosaic)", file=sys.stderr)
        print(f"  Mosaic Detected: {stats.get('mosaic_count', 0)} (censored content)", file=sys.stderr)
        print(f"  POV Detected: {stats.get('pov_count', 0)} (suggestive composition)", file=sys.stderr)
        print(f"  Errors: {stats['error_count']}", file=sys.stderr)
        print(f"  Avg NSFW Score: {stats['avg_nsfw_score']}", file=sys.stderr)
        print(f"  Avg Face Score: {stats['avg_face_score']}", file=sys.stderr)
        print(f"  Time: {stats['processing_time_sec']}s", file=sys.stderr)

        # List problematic super_safe images (for debugging)
        if args.verbose:
            print(f"\n[SUPER_SAFE IMAGES]", file=sys.stderr)
            for filename, r in result["results"].items():
                if r.get("is_super_safe"):
                    print(f"  {filename}: nsfw={r['nsfw_score']:.4f} face={r['face_score']:.4f}", file=sys.stderr)

    except Exception as e:
        print(json.dumps({"error": str(e)}))
        sys.exit(1)


if __name__ == "__main__":
    main()
