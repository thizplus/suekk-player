#!/usr/bin/env python3
"""
Image Selector - Select best cover and gallery images
Based on NSFW filtering, face detection, and aesthetic scoring
With Smart Blur fallback for NSFW images

Usage:
    python image_selector.py --input gallery_urls.json --output selected.json
    python image_selector.py --url https://example.com/image.jpg  # Score single image
"""
import os
import sys
import json
import argparse
import time
from typing import List, Optional, Tuple, Dict
from dataclasses import dataclass, asdict, field
from io import BytesIO
from pathlib import Path

import torch
import numpy as np
from PIL import Image, ImageFilter
import requests
from tqdm import tqdm
import cv2


@dataclass
class ImageScore:
    """Score for a single image"""
    url: str
    filename: str
    nsfw_score: float  # 0-1, lower is safer
    face_score: float  # 0-1, higher means clearer face
    aesthetic_score: float  # 0-1, higher is better
    combined_score: float  # weighted combination
    is_safe: bool  # passes NSFW threshold
    is_blurred: bool = False  # was this image blurred?
    blurred_path: Optional[str] = None  # local path to blurred image


@dataclass
class SelectionResult:
    """Result of image selection"""
    cover: Optional[dict]
    gallery: List[dict]
    total_images: int
    safe_images: int
    blurred_images: int
    processing_time: float


class SmartBlur:
    """Smart blur NSFW regions using NudeNet"""

    # NudeNet labels ที่ต้อง blur (NudeNet v2/v3 format)
    BLUR_LABELS = [
        "EXPOSED_BREAST_F",
        "EXPOSED_GENITALIA_F",
        "EXPOSED_GENITALIA_M",
        "EXPOSED_BUTTOCKS",
        "EXPOSED_ANUS",
        "COVERED_BREAST_F",  # blur ด้วยเพื่อความปลอดภัย
    ]

    # Labels ที่ไม่ต้อง blur (NudeNet v2/v3 format)
    SAFE_LABELS = [
        "FACE_F",
        "FACE_M",
        "FEET_EXPOSED",
        "BELLY_EXPOSED",
        "ARMPITS_EXPOSED",
    ]

    def __init__(self, output_dir: str = "output/blurred"):
        self.output_dir = Path(output_dir)
        self.output_dir.mkdir(parents=True, exist_ok=True)
        self.detector = None
        self._loaded = False

    def load_model(self):
        """Load NudeNet detector"""
        if self._loaded:
            return

        try:
            from nudenet import NudeDetector
            self.detector = NudeDetector()
            print("[OK] NudeNet detector loaded")
            self._loaded = True
        except Exception as e:
            print(f"[WARN] Could not load NudeNet: {e}")

    def detect_nsfw_regions(self, image: Image.Image) -> List[dict]:
        """Detect NSFW regions in image"""
        if self.detector is None:
            return []

        try:
            # Convert PIL to numpy array
            img_array = np.array(image)
            # Convert RGB to BGR for OpenCV
            img_bgr = cv2.cvtColor(img_array, cv2.COLOR_RGB2BGR)

            # Detect using NudeNet
            detections = self.detector.detect(img_bgr)

            # Filter only regions that need blur
            nsfw_regions = []
            for det in detections:
                if det['class'] in self.BLUR_LABELS and det['score'] > 0.3:
                    # NudeNet returns [x, y, width, height], convert to [x1, y1, x2, y2]
                    box = det['box']
                    x1, y1, w, h = box[0], box[1], box[2], box[3]
                    nsfw_regions.append({
                        'label': det['class'],
                        'box': [x1, y1, x1 + w, y1 + h],  # [x1, y1, x2, y2]
                        'score': det['score']
                    })

            return nsfw_regions

        except Exception as e:
            print(f"[WARN] Detection failed: {e}")
            return []

    def blur_regions(self, image: Image.Image, regions: List[dict], blur_radius: int = 99, expand_percent: float = 0.60) -> Image.Image:
        """
        Blur specific regions in image with expanded bounding boxes

        Uses extreme blur settings to pass NSFW classifiers like Falconsai.

        Args:
            image: PIL Image
            regions: List of detected regions
            blur_radius: Gaussian blur radius (higher = more blur)
            expand_percent: Expand bounding box by this percentage (0.60 = 60%)
        """
        if not regions:
            return image

        # Convert to numpy for OpenCV operations
        img_array = np.array(image)
        img_h, img_w = img_array.shape[:2]

        for region in regions:
            box = region['box']
            # Box is already [x1, y1, x2, y2] from detect_nsfw_regions
            x1, y1, x2, y2 = int(box[0]), int(box[1]), int(box[2]), int(box[3])

            # Calculate expansion based on box size (40% larger for safety)
            box_w = x2 - x1
            box_h = y2 - y1
            expand_w = int(box_w * expand_percent)
            expand_h = int(box_h * expand_percent)

            # Expand bounding box
            x1 = max(0, x1 - expand_w)
            y1 = max(0, y1 - expand_h)
            x2 = min(img_w, x2 + expand_w)
            y2 = min(img_h, y2 + expand_h)

            if x2 <= x1 or y2 <= y1:
                continue

            # Extract region
            roi = img_array[y1:y2, x1:x2]

            # Apply extreme Gaussian blur (SEO-safe)
            # blur_size must be odd
            blur_size = blur_radius * 2 + 1
            blurred_roi = cv2.GaussianBlur(roi, (blur_size, blur_size), 0)

            # Apply blur 7 times for extreme effect
            for _ in range(6):
                blurred_roi = cv2.GaussianBlur(blurred_roi, (blur_size, blur_size), 0)

            # Add pixelation layer on top for extra safety
            roi_h, roi_w = blurred_roi.shape[:2]
            if roi_w > 10 and roi_h > 10:
                pixel_size = max(roi_w, roi_h) // 6  # Larger pixels
                if pixel_size > 1:
                    small = cv2.resize(blurred_roi, (max(1, roi_w // pixel_size), max(1, roi_h // pixel_size)))
                    blurred_roi = cv2.resize(small, (roi_w, roi_h), interpolation=cv2.INTER_NEAREST)

            # Apply strong desaturation to remove skin tones
            blurred_hsv = cv2.cvtColor(blurred_roi, cv2.COLOR_RGB2HSV)
            blurred_hsv[:, :, 1] = blurred_hsv[:, :, 1] * 0.2  # 80% desaturation
            blurred_roi = cv2.cvtColor(blurred_hsv, cv2.COLOR_HSV2RGB)

            # Add solid color overlay to completely hide skin tones
            overlay_color = np.array([128, 128, 128], dtype=np.uint8)  # Neutral gray
            overlay = np.full_like(blurred_roi, overlay_color)
            alpha = 0.55  # 55% opacity - very strong overlay
            blurred_roi = cv2.addWeighted(blurred_roi, 1 - alpha, overlay, alpha, 0)

            # Put blurred region back
            img_array[y1:y2, x1:x2] = blurred_roi

        return Image.fromarray(img_array)

    def process_image(self, image: Image.Image, filename: str) -> Tuple[bool, Optional[str]]:
        """
        Process image: detect and blur NSFW regions
        Returns: (was_blurred, output_path)
        """
        if not self._loaded:
            self.load_model()

        if self.detector is None:
            return False, None

        # Detect NSFW regions
        regions = self.detect_nsfw_regions(image)

        if not regions:
            return False, None

        # Blur the regions
        blurred_image = self.blur_regions(image, regions)

        # Save to output directory
        output_path = self.output_dir / f"{Path(filename).stem}_blurred.jpg"
        blurred_image.save(output_path, "JPEG", quality=90)

        return True, str(output_path)


class ImageSelector:
    """Service for selecting best images from gallery"""

    # NSFW threshold - images above this are filtered out (or blurred)
    NSFW_THRESHOLD = 0.3

    # Minimum face score to be considered for cover
    MIN_FACE_SCORE = 0.2

    # Weights for combined score
    WEIGHT_NSFW = 0.4  # Lower NSFW is better
    WEIGHT_FACE = 0.3  # Higher face score is better
    WEIGHT_AESTHETIC = 0.3  # Higher aesthetic is better

    # Gallery selection
    GALLERY_COUNT = 12

    def __init__(self, device: str = None, enable_blur: bool = True, output_dir: str = "output/blurred"):
        self.device = device or ("cuda" if torch.cuda.is_available() else "cpu")
        self.nsfw_model = None
        self.clip_model = None
        self.clip_preprocess = None
        self.face_cascade = None
        self._loaded = False

        # Smart blur
        self.enable_blur = enable_blur
        self.smart_blur = SmartBlur(output_dir) if enable_blur else None

        # Aesthetic prompts for CLIP scoring
        self.aesthetic_prompts = [
            "a high quality photo",
            "a professional photo",
            "a beautiful photo",
            "a well-lit photo",
            "a sharp clear photo",
        ]
        self.negative_prompts = [
            "a blurry photo",
            "a low quality photo",
            "a dark photo",
            "a grainy photo",
        ]
        self.aesthetic_embeddings = None
        self.negative_embeddings = None

        # Cache for loaded images (for blur processing)
        self._image_cache: Dict[str, Image.Image] = {}

    def load_models(self) -> None:
        """Load all required models"""
        if self._loaded:
            return

        print(f"[ImageSelector] Loading models on {self.device}...")

        # Load NSFW model (Falconsai/nsfw_image_detection)
        try:
            from transformers import pipeline
            self.nsfw_model = pipeline(
                "image-classification",
                model="Falconsai/nsfw_image_detection",
                device=0 if self.device == "cuda" else -1
            )
            print("[OK] NSFW model loaded")
        except Exception as e:
            print(f"[WARN] Could not load NSFW model: {e}")

        # Load OpenCV face cascade
        try:
            cascade_path = cv2.data.haarcascades + 'haarcascade_frontalface_default.xml'
            self.face_cascade = cv2.CascadeClassifier(cascade_path)
            print("[OK] Face cascade loaded")
        except Exception as e:
            print(f"[WARN] Could not load face cascade: {e}")

        # Load CLIP for aesthetic scoring
        try:
            import clip
            self.clip_model, self.clip_preprocess = clip.load("ViT-B/32", device=self.device)

            # Pre-compute aesthetic prompt embeddings
            with torch.no_grad():
                pos_tokens = clip.tokenize(self.aesthetic_prompts).to(self.device)
                self.aesthetic_embeddings = self.clip_model.encode_text(pos_tokens)
                self.aesthetic_embeddings = self.aesthetic_embeddings / self.aesthetic_embeddings.norm(dim=-1, keepdim=True)

                neg_tokens = clip.tokenize(self.negative_prompts).to(self.device)
                self.negative_embeddings = self.clip_model.encode_text(neg_tokens)
                self.negative_embeddings = self.negative_embeddings / self.negative_embeddings.norm(dim=-1, keepdim=True)

            print("[OK] CLIP model loaded")
        except Exception as e:
            print(f"[WARN] Could not load CLIP model: {e}")

        # Load Smart Blur (NudeNet)
        if self.smart_blur:
            self.smart_blur.load_model()

        self._loaded = True
        print("[OK] All models loaded")

    def _load_image(self, url: str) -> Optional[Image.Image]:
        """Load image from URL"""
        try:
            response = requests.get(url, timeout=15)
            response.raise_for_status()
            image = Image.open(BytesIO(response.content)).convert("RGB")
            # Cache for later blur processing
            self._image_cache[url] = image
            return image
        except Exception as e:
            return None

    def _score_nsfw(self, image: Image.Image) -> float:
        """Score image for NSFW content (0=safe, 1=explicit)"""
        if self.nsfw_model is None:
            return 0.0

        try:
            results = self.nsfw_model(image)
            for r in results:
                label = r["label"].lower()
                if label in ["nsfw", "porn", "sexy", "hentai"]:
                    return r["score"]
            return 0.0
        except Exception:
            return 0.5

    def _score_face(self, image: Image.Image) -> float:
        """Score image for face visibility (0=no face, 1=clear face)"""
        if self.face_cascade is None:
            return 0.3

        try:
            img_array = np.array(image)
            gray = cv2.cvtColor(img_array, cv2.COLOR_RGB2GRAY)

            faces = self.face_cascade.detectMultiScale(
                gray,
                scaleFactor=1.1,
                minNeighbors=5,
                minSize=(40, 40)
            )

            if len(faces) == 0:
                return 0.0

            # Score based on face size relative to image
            img_area = image.width * image.height
            max_face_ratio = 0.0

            for (x, y, w, h) in faces:
                face_ratio = (w * h) / img_area
                max_face_ratio = max(max_face_ratio, face_ratio)

            # Normalize: 3% of image = 0.5, 10% = 1.0
            score = min(1.0, max_face_ratio * 12)
            return score

        except Exception:
            return 0.3

    def _score_aesthetic(self, image: Image.Image) -> float:
        """Score image aesthetic quality using CLIP (0=low, 1=high)"""
        if self.clip_model is None:
            return 0.5

        try:
            with torch.no_grad():
                image_input = self.clip_preprocess(image).unsqueeze(0).to(self.device)
                image_embedding = self.clip_model.encode_image(image_input)
                image_embedding = image_embedding / image_embedding.norm(dim=-1, keepdim=True)

                pos_sim = (image_embedding @ self.aesthetic_embeddings.T).mean().item()
                neg_sim = (image_embedding @ self.negative_embeddings.T).mean().item()

                score = (pos_sim - neg_sim + 1) / 2
                return max(0.0, min(1.0, score))

        except Exception:
            return 0.5

    def score_image(self, url: str) -> Optional[ImageScore]:
        """Score a single image"""
        if not self._loaded:
            self.load_models()

        image = self._load_image(url)
        if image is None:
            return None

        filename = os.path.basename(url.split("?")[0])

        nsfw_score = self._score_nsfw(image)
        face_score = self._score_face(image)
        aesthetic_score = self._score_aesthetic(image)

        is_safe = nsfw_score < self.NSFW_THRESHOLD

        if is_safe:
            combined = (
                self.WEIGHT_NSFW * (1 - nsfw_score) +
                self.WEIGHT_FACE * face_score +
                self.WEIGHT_AESTHETIC * aesthetic_score
            )
        else:
            # สำหรับภาพ NSFW ให้ใช้ aesthetic + face score เป็น combined
            # เพื่อเลือกภาพที่สวยสำหรับ blur
            combined = (
                self.WEIGHT_FACE * face_score +
                self.WEIGHT_AESTHETIC * aesthetic_score
            ) * 0.5  # ลด weight ลงครึ่งหนึ่ง

        return ImageScore(
            url=url,
            filename=filename,
            nsfw_score=round(nsfw_score, 3),
            face_score=round(face_score, 3),
            aesthetic_score=round(aesthetic_score, 3),
            combined_score=round(combined, 3),
            is_safe=is_safe,
            is_blurred=False,
            blurred_path=None
        )

    def select_images(
        self,
        image_urls: List[str],
        gallery_count: int = None,
        show_progress: bool = True
    ) -> SelectionResult:
        """Select best cover and gallery images with smart blur fallback"""
        start_time = time.time()

        if not self._loaded:
            self.load_models()

        gallery_count = gallery_count or self.GALLERY_COUNT

        # Score all images
        scores: List[ImageScore] = []
        iterator = tqdm(image_urls, desc="Scoring images") if show_progress else image_urls

        for url in iterator:
            score = self.score_image(url)
            if score:
                scores.append(score)

        total_images = len(scores)
        safe_images = sum(1 for s in scores if s.is_safe)

        # แยกภาพ safe และ nsfw
        safe_scores = [s for s in scores if s.is_safe]
        nsfw_scores = [s for s in scores if not s.is_safe]

        # เรียง nsfw ตาม aesthetic + face score (เลือกภาพสวยมา blur)
        # เรียงตาม face_score เป็นหลัก (เห็นสีหน้าชัด) แล้วค่อย aesthetic
        nsfw_scores.sort(key=lambda s: (s.face_score * 2) + s.aesthetic_score, reverse=True)

        # Select cover: highest combined score with face (from safe images only)
        cover = None
        cover_candidates = [s for s in safe_scores if s.face_score >= self.MIN_FACE_SCORE]
        if cover_candidates:
            cover = max(cover_candidates, key=lambda s: s.combined_score)
        elif safe_scores:
            cover = max(safe_scores, key=lambda s: s.combined_score)

        # Select gallery: เลือกเฉพาะภาพที่มีคน (face_score > 0)
        # กรองภาพท้องฟ้า, scenery, หรือภาพที่ไม่มีใครอยู่
        gallery_candidates = [s for s in safe_scores if s.face_score > 0]

        # ถ้าไม่มีภาพที่มี face เลย → fallback ใช้ภาพ aesthetic score สูง
        if not gallery_candidates:
            print(f"[WARN] No images with face detected, using aesthetic fallback")
            gallery_candidates = sorted(safe_scores, key=lambda s: s.aesthetic_score, reverse=True)

        gallery = self._select_diverse_gallery(gallery_candidates, gallery_count, exclude=cover)

        # ถ้ายังไม่ครบ และ enable_blur → blur ภาพ NSFW เพิ่ม
        blurred_count = 0
        if self.enable_blur and self.smart_blur and len(gallery) < gallery_count:
            needed = gallery_count - len(gallery)
            print(f"\n[SmartBlur] Need {needed} more images, processing NSFW images...")

            for nsfw_img in nsfw_scores[:needed * 2]:  # Process more in case some fail
                if len(gallery) >= gallery_count:
                    break

                # Get cached image
                image = self._image_cache.get(nsfw_img.url)
                if image is None:
                    continue

                # Blur NSFW regions
                was_blurred, blurred_path = self.smart_blur.process_image(image, nsfw_img.filename)

                if was_blurred and blurred_path:
                    # Update score with blurred info
                    blurred_score = ImageScore(
                        url=nsfw_img.url,
                        filename=nsfw_img.filename,
                        nsfw_score=nsfw_img.nsfw_score,
                        face_score=nsfw_img.face_score,
                        aesthetic_score=nsfw_img.aesthetic_score,
                        combined_score=nsfw_img.combined_score,
                        is_safe=True,  # Now safe after blur
                        is_blurred=True,
                        blurred_path=blurred_path
                    )
                    gallery.append(blurred_score)
                    blurred_count += 1
                    print(f"  [OK] Blurred: {nsfw_img.filename} -> {blurred_path}")

        # Clear image cache to free memory
        self._image_cache.clear()

        processing_time = time.time() - start_time

        return SelectionResult(
            cover=asdict(cover) if cover else None,
            gallery=[asdict(s) for s in gallery],
            total_images=total_images,
            safe_images=safe_images,
            blurred_images=blurred_count,
            processing_time=round(processing_time, 2)
        )

    def _select_diverse_gallery(
        self,
        scores: List[ImageScore],
        count: int,
        exclude: Optional[ImageScore] = None
    ) -> List[ImageScore]:
        """Select diverse gallery images"""
        candidates = [s for s in scores if exclude is None or s.url != exclude.url]

        if len(candidates) <= count:
            return candidates

        # Pick every N-th image for diversity
        step = max(1, len(candidates) // count)
        selected = []

        for i in range(0, len(candidates), step):
            if len(selected) >= count:
                break
            selected.append(candidates[i])

        # Sort by combined score
        selected.sort(key=lambda s: s.combined_score, reverse=True)

        return selected[:count]


def main():
    parser = argparse.ArgumentParser(description="Image Selector - Select best cover and gallery images")
    parser.add_argument("--input", "-i", help="JSON file with list of image URLs")
    parser.add_argument("--output", "-o", help="Output JSON file for results")
    parser.add_argument("--url", "-u", help="Single image URL to score")
    parser.add_argument("--gallery-count", "-n", type=int, default=12, help="Number of gallery images")
    parser.add_argument("--device", "-d", default=None, help="Device (cuda/cpu)")
    parser.add_argument("--no-blur", action="store_true", help="Disable smart blur")
    parser.add_argument("--blur-output", default="output/blurred", help="Output directory for blurred images")

    args = parser.parse_args()

    selector = ImageSelector(
        device=args.device,
        enable_blur=not args.no_blur,
        output_dir=args.blur_output
    )

    if args.url:
        # Score single image
        selector.load_models()
        score = selector.score_image(args.url)
        if score:
            print(json.dumps(asdict(score), indent=2))
        else:
            print(f"Failed to score image: {args.url}")
            sys.exit(1)

    elif args.input:
        # Process list of images
        with open(args.input, "r") as f:
            data = json.load(f)

        if isinstance(data, list):
            image_urls = data
        elif isinstance(data, dict) and "urls" in data:
            image_urls = data["urls"]
        else:
            print("Invalid input format. Expected list or {urls: [...]}")
            sys.exit(1)

        result = selector.select_images(image_urls, gallery_count=args.gallery_count)

        output_data = {
            "cover": result.cover,
            "gallery": result.gallery,
            "stats": {
                "total_images": result.total_images,
                "safe_images": result.safe_images,
                "blurred_images": result.blurred_images,
                "processing_time": result.processing_time
            }
        }

        if args.output:
            with open(args.output, "w") as f:
                json.dump(output_data, f, indent=2)
            print(f"Results saved to {args.output}")
        else:
            print(json.dumps(output_data, indent=2))

    else:
        parser.print_help()


if __name__ == "__main__":
    main()
