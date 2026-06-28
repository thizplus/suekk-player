"""
Batch Translate - แปลชื่อ video จาก SubTH ที่ยังไม่มีภาษาไทย

Usage:
    cd _my_worker/python
    python batch_translate.py                    # แปล 50 รายการ (default)
    python batch_translate.py --limit 100        # แปล 100 รายการ
    python batch_translate.py --category uncensored-jav
    python batch_translate.py --dry-run          # ทดสอบ ไม่ save

ต้องรัน title_translate service ก่อน:
    python -m title_translate.main
"""

import argparse
import json
import sys
import time

import requests

# === Config ===
SUBTH_API = "https://api.subth.com/api/v1"
TRANSLATE_API = "http://localhost:8003/api/klon8/translate-only"

# โหลด credentials จาก .env
from pathlib import Path
from dotenv import load_dotenv
import os

load_dotenv(Path(__file__).parent.parent / ".env", override=False)

EMAIL = os.getenv("API_EMAIL", "admin@subth.com")
PASSWORD = os.getenv("API_PASSWORD", "")


def login() -> str:
    """Login SubTH API แล้ว return token"""
    resp = requests.post(f"{SUBTH_API}/auth/login", json={
        "email": EMAIL,
        "password": PASSWORD,
    }, timeout=10)
    resp.raise_for_status()
    token = resp.json()["data"]["token"]
    print(f"[LOGIN] OK - {EMAIL}")
    return token


def get_pending_videos(token: str, limit: int, category: str = "") -> list:
    """ดึง video ที่ยังไม่มีชื่อไทย"""
    params = {"missing_th": "true", "limit": limit}
    if category:
        params["category"] = category

    resp = requests.get(f"{SUBTH_API}/videos", params=params,
                        headers={"Authorization": f"Bearer {token}"}, timeout=15)
    resp.raise_for_status()
    data = resp.json()
    videos = data.get("data", [])
    total = data.get("meta", {}).get("total", len(videos))
    print(f"[QUERY] พบ {total} รายการรอแปล (ดึงมา {len(videos)})")
    return videos


def translate_one(video: dict) -> dict | None:
    """แปล 1 video ผ่าน title_translate service"""
    title = video.get("title", "")
    casts = video.get("casts", [])
    tags = video.get("tags", [])

    payload = {
        "title_en": title,
        "tags": [t["name"] for t in tags] if tags else [],
        "cast_name": casts[0]["name"] if casts else "",
        "source_type": "jav",
    }

    resp = requests.post(TRANSLATE_API, json=payload, timeout=30)
    resp.raise_for_status()
    result = resp.json()

    if result.get("success"):
        return result["data"]
    return None


def save_title(token: str, video_id: str, title_th: str) -> bool:
    """Save ชื่อไทยกลับ SubTH API"""
    resp = requests.put(
        f"{SUBTH_API}/videos/{video_id}",
        json={"titles": {"th": title_th}},
        headers={"Authorization": f"Bearer {token}"},
        timeout=10,
    )
    return resp.status_code == 200


def main():
    parser = argparse.ArgumentParser(description="Batch translate video titles for SubTH")
    parser.add_argument("--limit", type=int, default=50, help="จำนวนที่จะแปล (default: 50)")
    parser.add_argument("--category", type=str, default="", help="filter by category slug")
    parser.add_argument("--dry-run", action="store_true", help="ทดสอบ ไม่ save กลับ API")
    parser.add_argument("--delay", type=float, default=0.5, help="delay ระหว่างแต่ละรายการ (วินาที)")
    args = parser.parse_args()

    # 1. Login
    token = login()

    # 2. ดึง video รอแปล
    videos = get_pending_videos(token, args.limit, args.category)
    if not videos:
        print("[DONE] ไม่มี video รอแปล")
        return

    # 3. แปลทีละรายการ
    success = 0
    failed = 0

    for i, v in enumerate(videos, 1):
        vid = v.get("id", "")
        title_en = v.get("title", "")
        prefix = f"[{i}/{len(videos)}]"

        try:
            result = translate_one(v)
            if result:
                title_th = result["title_th"]
                if args.dry_run:
                    print(f"{prefix} [DRY] {title_th}")
                else:
                    if save_title(token, vid, title_th):
                        print(f"{prefix} [OK] {title_th}")
                        success += 1
                    else:
                        print(f"{prefix} [SAVE_FAIL] {title_en[:60]}")
                        failed += 1
            else:
                print(f"{prefix} [SKIP] translate failed: {title_en[:60]}")
                failed += 1
        except Exception as e:
            print(f"{prefix} [ERROR] {e} | {title_en[:60]}")
            failed += 1

        if i < len(videos):
            time.sleep(args.delay)

    # 4. Summary
    print(f"\n{'='*50}")
    print(f"[SUMMARY] Total: {len(videos)} | Success: {success} | Failed: {failed}")
    if args.dry_run:
        print("[DRY RUN] ไม่ได้ save จริง")


if __name__ == "__main__":
    main()
