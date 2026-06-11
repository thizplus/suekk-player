"""
Import series metadata จาก SQLite → SUEKK API + Upload poster ไป E2

Usage:
    python import_to_suekk.py                 # Import ทั้งหมด
    python import_to_suekk.py --dry-run       # ทดสอบ ไม่เขียนจริง
    python import_to_suekk.py --limit 5       # Import แค่ 5 เรื่อง
"""
import os
import re
import sys
import json
import asyncio
import argparse
import tempfile
import aiohttp
import boto3
import sqlite3
from loguru import logger

# Setup logger
logger.remove()
logger.add(sys.stdout, format="<green>{time:HH:mm:ss}</green> | <level>{level:<7}</level> | {message}", level="INFO")

from dotenv import load_dotenv
load_dotenv()

# ═══════════════════════════════════════════
# Config
# ═══════════════════════════════════════════

SUEKK_API_URL = os.getenv("SUEKK_API_URL", "https://api.suekk.com")
SUEKK_TOKEN = os.getenv("SUEKK_TOKEN", "")
DB_PATH = os.getenv("DB_PATH", "./data/tracker.db")

# S3 (IDrive E2) — ใช้สำหรับ upload poster
S3_ENDPOINT = os.getenv("S3_ENDPOINT", "")
S3_ACCESS_KEY = os.getenv("S3_ACCESS_KEY", "")
S3_SECRET_KEY = os.getenv("S3_SECRET_KEY", "")
S3_BUCKET = os.getenv("S3_BUCKET", "suekk")
S3_REGION = os.getenv("S3_REGION", "us-east-1")

# Series category ID สำหรับ ซีรี่ย์เกาหลี
KOREAN_CATEGORY_ID = "92b65cfa-d664-46c6-9582-db09eb965544"


# ═══════════════════════════════════════════
# Title Cleaner
# ═══════════════════════════════════════════

def clean_title(raw: str) -> tuple[str, str]:
    """แยก title เป็น EN + TH และตัด junk ออก"""
    clean = raw.strip()
    clean = re.sub(r"^ดูซีรี่ย์\s*", "", clean)
    clean = re.sub(r"\s*EP\s*\d+[\s-]*\d*\s*จบ\s*", " ", clean, flags=re.IGNORECASE).strip()
    clean = re.sub(r"\s*\(\d{4}\)\s*", " ", clean).strip()

    junk_end = [
        r"\s+ซับไทย[\s\S]*$",
        r"\s+พากย์ไทย[\s\S]*$",
        r"\s+[Ss]erie[s]?[-\s]?[Dd]ays?[.\-\s]*(COM|com|HD)?[\s\S]*$",
        r"\s+SeriesDAY[\s\S]*$",
        r"\s+serie-days\.com[\s\S]*$",
    ]
    for pat in junk_end:
        clean = re.sub(pat, "", clean, flags=re.IGNORECASE).strip()

    thai_match = re.search(r"[\u0E00-\u0E7F]", clean)
    if thai_match:
        idx = thai_match.start()
        en = clean[:idx].strip()
        th = clean[idx:].strip()
        for pat in junk_end:
            th = re.sub(pat, "", th, flags=re.IGNORECASE).strip()
    else:
        en = clean
        th = ""

    return en, th


def extract_youtube_id(url: str) -> str:
    if not url:
        return ""
    match = re.search(r"[?&]v=([a-zA-Z0-9_-]{11})", url)
    if match:
        return match.group(1)
    match = re.search(r"youtu\.be/([a-zA-Z0-9_-]{11})", url)
    if match:
        return match.group(1)
    return ""


# ═══════════════════════════════════════════
# S3 Poster Upload
# ═══════════════════════════════════════════

def create_s3_client():
    return boto3.client(
        "s3",
        endpoint_url=S3_ENDPOINT,
        aws_access_key_id=S3_ACCESS_KEY,
        aws_secret_access_key=S3_SECRET_KEY,
        region_name=S3_REGION,
    )


async def download_and_upload_poster(
    session: aiohttp.ClientSession,
    s3_client,
    poster_url: str,
    series_code: str,
) -> str:
    """Download poster → upload ไป E2 → return S3 path"""
    if not poster_url:
        return ""

    try:
        # Download
        async with session.get(poster_url, timeout=aiohttp.ClientTimeout(total=30)) as resp:
            if resp.status != 200:
                logger.warning(f"Poster download failed: {resp.status} {poster_url[:60]}")
                return ""
            data = await resp.read()

        # Detect extension
        ext = "jpg"
        if poster_url.lower().endswith(".png"):
            ext = "png"
        elif poster_url.lower().endswith(".webp"):
            ext = "webp"

        s3_path = f"series/{series_code}/poster.{ext}"
        content_type = f"image/{ext}"
        if ext == "jpg":
            content_type = "image/jpeg"

        # Upload to S3
        s3_client.put_object(
            Bucket=S3_BUCKET,
            Key=s3_path,
            Body=data,
            ContentType=content_type,
        )

        return s3_path

    except Exception as e:
        logger.error(f"Poster upload error: {e}")
        return ""


# ═══════════════════════════════════════════
# SUEKK API Client
# ═══════════════════════════════════════════

async def suekk_api(session, method, path, data=None):
    """เรียก SUEKK API"""
    url = f"{SUEKK_API_URL}{path}"
    headers = {"Authorization": f"Bearer {SUEKK_TOKEN}"}

    if method == "POST":
        async with session.post(url, json=data, headers=headers) as resp:
            body = await resp.json()
            return resp.status, body
    elif method == "PUT":
        async with session.put(url, json=data, headers=headers) as resp:
            body = await resp.json()
            return resp.status, body
    elif method == "PATCH":
        async with session.patch(url, json=data, headers=headers) as resp:
            body = await resp.json()
            return resp.status, body
    elif method == "GET":
        async with session.get(url, headers=headers) as resp:
            body = await resp.json()
            return resp.status, body

    return 0, {}


# ═══════════════════════════════════════════
# Import Logic
# ═══════════════════════════════════════════

async def import_series(dry_run=False, limit=0):
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row

    query = "SELECT * FROM series ORDER BY year DESC, rating DESC"
    if limit > 0:
        query += f" LIMIT {limit}"
    series_list = conn.execute(query).fetchall()

    logger.info(f"Importing {len(series_list)} series (dry_run={dry_run})")

    s3_client = None
    if not dry_run and S3_ENDPOINT:
        s3_client = create_s3_client()
        logger.info("S3 client created")

    async with aiohttp.ClientSession(headers={"User-Agent": "SuekkBot/1.0"}) as session:

        stats = {"created": 0, "updated": 0, "posters": 0, "episodes": 0, "errors": 0}

        for i, s in enumerate(series_list):
            try:
                # Clean title
                en_title, th_title = clean_title(s["title"])
                if not en_title:
                    en_title = s["title"][:100]

                # YouTube ID
                yt_id = extract_youtube_id(s["trailer_url"] or "")

                # Categories จากเว็บต้นทาง → platforms/genres
                cats = []
                try:
                    cats = json.loads(s["categories"]) if s["categories"] else []
                except:
                    pass

                platforms = []
                genres = []
                platform_keywords = ["NETFLIX", "HBO", "IQIYI", "VIU", "DISNEY", "APPLE TV", "AMAZON", "WeTV"]
                genre_keywords = ["Drama", "Comedy", "Action", "Romance", "Thriller", "Fantasy", "Horror", "Mystery", "Sci-Fi"]

                for cat in cats:
                    cat_upper = cat.upper()
                    if any(pk.upper() in cat_upper for pk in platform_keywords):
                        platforms.append(cat.strip())
                    for gk in genre_keywords:
                        if gk.lower() in cat.lower():
                            genres.append(gk)

                platforms = list(set(platforms))
                genres = list(set(genres))

                # Series code
                slug = s["slug"]
                code_base = re.sub(r"[^a-zA-Z0-9]", "", slug.upper())
                year = s["year"] or 0
                code = f"{code_base}{year}" if year else code_base
                if len(code) > 50:
                    code = code[:50]

                # Prepare data
                series_data = {
                    "title": en_title,
                    "thaiTitle": th_title,
                    "slug": slug,
                    "description": s["description"] or "",
                    "year": year,
                    "rating": float(s["rating"]) if s["rating"] and s["rating"] < 100 else 0,
                    "quality": "HD",
                    "audioType": s["audio_type"] or "",
                    "trailerYoutubeId": yt_id,
                    "totalEpisodes": s["total_episodes"] or 0,
                    "isCompleted": bool(s["is_completed"]),
                    "categoryId": KOREAN_CATEGORY_ID,
                    "platforms": platforms,
                    "genres": genres,
                    "sourceSite": "serie_days",
                    "sourceId": s["source_id"] or 0,
                    "sourceUrl": f"https://www.serie-days.com/{slug}/",
                }

                progress = f"[{i+1}/{len(series_list)}]"

                if dry_run:
                    logger.info(f"{progress} DRY: {en_title} ({th_title}) | {year} | EP:{s['total_episodes']} | platforms:{platforms}")
                    stats["created"] += 1
                    continue

                # 1. Upsert series
                status, body = await suekk_api(session, "POST", "/api/v1/series/upsert", series_data)
                if status not in (200, 201):
                    logger.error(f"{progress} API error {status}: {body.get('error', {}).get('message', 'unknown')}")
                    stats["errors"] += 1
                    continue

                series_id = body.get("data", {}).get("id", "")
                is_new = status == 201
                if is_new:
                    stats["created"] += 1
                else:
                    stats["updated"] += 1

                # 2. Upload poster
                poster_url = s["poster_url"] or ""
                if poster_url and s3_client:
                    poster_path = await download_and_upload_poster(session, s3_client, poster_url, code)
                    if poster_path:
                        await suekk_api(session, "PUT", f"/api/v1/series/{series_id}", {
                            "posterPath": poster_path,
                        })
                        stats["posters"] += 1

                # 3. Add episodes
                eps = conn.execute(
                    "SELECT episode_number FROM episodes WHERE series_id=? ORDER BY episode_number",
                    (s["id"],),
                ).fetchall()

                if eps:
                    ep_data = {
                        "episodes": [
                            {"episodeNumber": e["episode_number"], "sourceUrl": f"https://www.serie-days.com/{slug}-ep-{e['episode_number']}/"}
                            for e in eps
                        ]
                    }
                    ep_status, ep_body = await suekk_api(session, "POST", f"/api/v1/series/{series_id}/episodes", ep_data)
                    new_eps = ep_body.get("data", {}).get("newEpisodes", 0)
                    stats["episodes"] += new_eps

                logger.info(
                    f"{progress} {'NEW' if is_new else 'UPD'}: {en_title[:40]} | "
                    f"EP:{len(eps)} | poster:{'✓' if poster_url else '✗'}"
                )

                # Rate limit
                await asyncio.sleep(0.1)

            except Exception as e:
                logger.error(f"Error importing {s['slug']}: {e}")
                stats["errors"] += 1

        logger.info(f"\nDone! Created:{stats['created']} Updated:{stats['updated']} "
                     f"Posters:{stats['posters']} Episodes:{stats['episodes']} Errors:{stats['errors']}")

    conn.close()


def main():
    parser = argparse.ArgumentParser(description="Import series to SUEKK")
    parser.add_argument("--dry-run", action="store_true", help="ทดสอบ ไม่เขียนจริง")
    parser.add_argument("--limit", type=int, default=0, help="จำกัดจำนวน")
    args = parser.parse_args()

    if not SUEKK_TOKEN and not args.dry_run:
        print("ERROR: SUEKK_TOKEN not set in .env")
        sys.exit(1)

    asyncio.run(import_series(dry_run=args.dry_run, limit=args.limit))


if __name__ == "__main__":
    main()
