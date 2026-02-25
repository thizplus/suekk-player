"""
Sync video titles from api.subth.com to api.suekk.com

Match: suekk.code == code extracted from subth.embedUrl
Update: suekk.description = subth.translations.en (or title)

Usage:
    python sync_titles.py              # Sync all
    python sync_titles.py --dry-run    # Preview only
    python sync_titles.py --category jav  # Filter by category
"""

import requests
import re
import time
import argparse

# API Credentials
SUBTH_EMAIL = "admin@subth.com"
SUBTH_PASSWORD = "Log2Window$P@ssWord"

SUEKK_EMAIL = "info@thizplus.com"
SUEKK_PASSWORD = "Log2Window$P@ssWord"

# API URLs
SUBTH_API = "https://api.subth.com/api/v1"
SUEKK_API = "https://api.suekk.com/api/v1"


def login(api_url: str, email: str, password: str) -> str:
    """Login and return token"""
    resp = requests.post(
        f"{api_url}/auth/login",
        json={"email": email, "password": password}
    )
    resp.raise_for_status()
    data = resp.json()
    if not data.get("success"):
        raise Exception(f"Login failed: {data}")
    return data["data"]["token"]


def extract_code_from_thumbnail(thumbnail: str) -> str:
    """Extract JAV code from thumbnail path: /thumbnails/FNS-126.jpg -> FNS-126"""
    if not thumbnail:
        return ""
    # Match /thumbnails/CODE.jpg or /thumbnails/CODE.webp
    match = re.search(r'/thumbnails/([^/]+)\.(jpg|webp|png)', thumbnail)
    return match.group(1) if match else ""


def get_all_videos_subth(token: str, category: str = None) -> dict:
    """Get all videos from subth.com, return dict of jav_code -> title_en"""
    videos = {}
    page = 1
    limit = 100

    # Map suekk category names to subth category names
    category_map = {
        "jav": "censored-jav",
        "censored-jav": "censored-jav",
        "western": "western-av",
        "western-av": "western-av",
        "chinese": "chinese-av",
        "chinese-av": "chinese-av",
    }
    subth_category = category_map.get(category, category) if category else None

    while True:
        params = {"page": page, "limit": limit, "lang": "en"}
        if subth_category:
            params["category"] = subth_category

        resp = requests.get(
            f"{SUBTH_API}/videos",
            params=params,
            headers={"Authorization": f"Bearer {token}"}
        )
        resp.raise_for_status()
        data = resp.json()

        if not data.get("success"):
            raise Exception(f"Failed to get videos: {data}")

        for video in data["data"]:
            # Extract JAV code from thumbnail path
            thumbnail = video.get("thumbnail", "")
            jav_code = extract_code_from_thumbnail(thumbnail)

            if jav_code:
                # Get English title (lang=en returns EN title)
                title_en = video.get("title", "")
                videos[jav_code] = {
                    "title": title_en,
                    "thumbnail": thumbnail,
                }

        # Check pagination
        meta = data.get("meta", {})
        cat_str = f" [{subth_category}]" if subth_category else ""
        print(f"[subth{cat_str}] Page {page}/{meta.get('totalPages', '?')}: {len(data['data'])} videos (total: {len(videos)})")

        if not meta.get("hasNext"):
            break
        page += 1
        time.sleep(0.1)  # Rate limit

    print(f"[subth] Total: {len(videos)} videos with JAV codes")
    return videos


def get_all_videos_suekk(token: str, category: str = None) -> list:
    """Get all videos from suekk.com"""
    videos = []
    page = 1
    limit = 100

    while True:
        params = {"page": page, "limit": limit}
        if category:
            params["category"] = category

        resp = requests.get(
            f"{SUEKK_API}/videos",
            params=params,
            headers={"Authorization": f"Bearer {token}"}
        )
        resp.raise_for_status()
        data = resp.json()

        if not data.get("success"):
            raise Exception(f"Failed to get videos: {data}")

        videos.extend(data["data"])

        # Check pagination
        meta = data.get("meta", {})
        cat_str = f" [{category}]" if category else ""
        print(f"[suekk{cat_str}] Page {page}/{meta.get('totalPages', '?')}: {len(data['data'])} videos")

        if not meta.get("hasNext"):
            break
        page += 1
        time.sleep(0.1)  # Rate limit

    print(f"[suekk] Total: {len(videos)} videos")
    return videos


def update_video_description(token: str, video_id: str, description: str) -> bool:
    """Update video description in suekk.com"""
    resp = requests.put(
        f"{SUEKK_API}/videos/{video_id}",
        json={"description": description},
        headers={"Authorization": f"Bearer {token}"}
    )
    return resp.status_code == 200


def main():
    parser = argparse.ArgumentParser(description="Sync video titles from subth.com to suekk.com")
    parser.add_argument("--dry-run", action="store_true", help="Preview changes without updating")
    parser.add_argument("--category", type=str, default=None, help="Filter by category (e.g., jav)")
    parser.add_argument("--force", action="store_true", help="Update even if description exists")
    args = parser.parse_args()

    print("=" * 60)
    print("Sync Video Titles: subth.com -> suekk.com")
    if args.dry_run:
        print(">>> DRY RUN - No changes will be made <<<")
    if args.category:
        print(f">>> Filtering by category: {args.category} <<<")
    if args.force:
        print(">>> Force mode: will overwrite existing descriptions <<<")
    print("=" * 60)

    # Login
    print("\n[1] Logging in...")
    subth_token = login(SUBTH_API, SUBTH_EMAIL, SUBTH_PASSWORD)
    print("    subth.com: OK")

    suekk_token = login(SUEKK_API, SUEKK_EMAIL, SUEKK_PASSWORD)
    print("    suekk.com: OK")

    # Get videos from subth.com
    print("\n[2] Getting videos from subth.com...")
    subth_videos = get_all_videos_subth(subth_token, args.category)

    # Get videos from suekk.com
    print("\n[3] Getting videos from suekk.com...")
    suekk_videos = get_all_videos_suekk(suekk_token, args.category)

    # Match and update
    print("\n[4] Matching and updating...")
    updated = 0
    skipped = 0
    not_found = 0
    would_update = 0

    for video in suekk_videos:
        video_id = video["id"]
        jav_code = video.get("title", "")  # suekk.title contains JAV code like "FNS-126"
        current_desc = video.get("description", "") or ""

        # Skip if already has description (unless --force)
        if current_desc.strip() and not args.force:
            skipped += 1
            continue

        # Find EN title from subth.com using JAV code
        subth_info = subth_videos.get(jav_code)
        if not subth_info:
            not_found += 1
            continue

        title_en = subth_info.get("title", "")
        if not title_en:
            not_found += 1
            continue

        if args.dry_run:
            would_update += 1
            # Safe print for Windows console
            safe_title = title_en[:60].encode('ascii', 'replace').decode('ascii')
            print(f"    [DRY] {jav_code}: {safe_title}")
        else:
            # Update description
            if update_video_description(suekk_token, video_id, title_en):
                updated += 1
                # Safe print for Windows console
                safe_title = title_en[:50].encode('ascii', 'replace').decode('ascii')
                print(f"    Updated: {jav_code} -> {safe_title}...")
            else:
                print(f"    Failed: {jav_code}")

            time.sleep(0.05)  # Rate limit

    print("\n" + "=" * 60)
    print("Summary:")
    if args.dry_run:
        print(f"  Would update: {would_update}")
    else:
        print(f"  Updated: {updated}")
    print(f"  Skipped (has desc): {skipped}")
    print(f"  Not found in subth: {not_found}")
    print("=" * 60)


if __name__ == "__main__":
    main()
