"""
Sync video categories from api.subth.com to api.suekk.com

Match: suekk.title (code) == subth.thumbnail (extract code from path)
Update: suekk.categoryId = mapped category from subth
"""

import requests
import re
import time

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


def get_categories(api_url: str, token: str) -> dict:
    """Get all categories, return dict of id -> name and name -> id"""
    resp = requests.get(
        f"{api_url}/categories",
        headers={"Authorization": f"Bearer {token}"}
    )
    resp.raise_for_status()
    data = resp.json()

    if not data.get("success"):
        raise Exception(f"Failed to get categories: {data}")

    # Handle different API formats
    raw_data = data.get("data", [])
    if isinstance(raw_data, dict):
        # suekk format: {"data": {"categories": [...]}}
        categories = raw_data.get("categories", [])
    else:
        # subth format: {"data": [...]}
        categories = raw_data

    id_to_name = {cat["id"]: cat["name"] for cat in categories}
    name_to_id = {cat["name"]: cat["id"] for cat in categories}

    return {"id_to_name": id_to_name, "name_to_id": name_to_id, "list": categories}


def create_category(api_url: str, token: str, name: str) -> str:
    """Create a category and return its ID"""
    # Generate slug from name: "Western AV" -> "western-av"
    slug = name.lower().replace(" ", "-")

    resp = requests.post(
        f"{api_url}/categories",
        json={"name": name, "slug": slug},
        headers={"Authorization": f"Bearer {token}"}
    )
    resp.raise_for_status()
    data = resp.json()

    if not data.get("success"):
        raise Exception(f"Failed to create category: {data}")

    return data["data"]["id"]


def get_all_videos_subth(token: str) -> dict:
    """Get all videos from subth.com, return dict of code -> category_name"""
    videos = {}
    page = 1
    limit = 100

    while True:
        resp = requests.get(
            f"{SUBTH_API}/videos",
            params={"page": page, "limit": limit},
            headers={"Authorization": f"Bearer {token}"}
        )
        resp.raise_for_status()
        data = resp.json()

        if not data.get("success"):
            raise Exception(f"Failed to get videos: {data}")

        for video in data["data"]:
            # Extract code from thumbnail: /thumbnails/CODE.jpg -> CODE
            thumbnail = video.get("thumbnail", "")
            match = re.search(r'/thumbnails/([^/]+)\.jpg', thumbnail)
            if match:
                code = match.group(1)
                # subth uses "category" as string name, not ID
                category_name = video.get("category")
                if category_name:
                    videos[code] = category_name

        # Check pagination
        meta = data.get("meta", {})
        print(f"[subth] Page {page}/{meta.get('totalPages', '?')}: {len(data['data'])} videos (with category: {len(videos)})")

        if not meta.get("hasNext"):
            break
        page += 1
        time.sleep(0.1)

    print(f"[subth] Total: {len(videos)} videos with categories")
    return videos


def get_all_videos_suekk(token: str) -> list:
    """Get all videos from suekk.com"""
    videos = []
    page = 1
    limit = 100

    while True:
        resp = requests.get(
            f"{SUEKK_API}/videos",
            params={"page": page, "limit": limit},
            headers={"Authorization": f"Bearer {token}"}
        )
        resp.raise_for_status()
        data = resp.json()

        if not data.get("success"):
            raise Exception(f"Failed to get videos: {data}")

        videos.extend(data["data"])

        # Check pagination
        meta = data.get("meta", {})
        print(f"[suekk] Page {page}/{meta.get('totalPages', '?')}: {len(data['data'])} videos")

        if not meta.get("hasNext"):
            break
        page += 1
        time.sleep(0.1)

    print(f"[suekk] Total: {len(videos)} videos")
    return videos


def update_video_category(token: str, video_id: str, category_id: str) -> bool:
    """Update video category in suekk.com"""
    resp = requests.put(
        f"{SUEKK_API}/videos/{video_id}",
        json={"categoryId": category_id},
        headers={"Authorization": f"Bearer {token}"}
    )
    return resp.status_code == 200


def main():
    print("=" * 60)
    print("Sync Video Categories: subth.com -> suekk.com")
    print("=" * 60)

    # Login
    print("\n[1] Logging in...")
    subth_token = login(SUBTH_API, SUBTH_EMAIL, SUBTH_PASSWORD)
    print("    subth.com: OK")

    suekk_token = login(SUEKK_API, SUEKK_EMAIL, SUEKK_PASSWORD)
    print("    suekk.com: OK")

    # Get categories from both APIs
    print("\n[2] Getting categories...")
    subth_cats = get_categories(SUBTH_API, subth_token)
    print(f"    subth.com: {len(subth_cats['list'])} categories")
    for cat in subth_cats['list']:
        print(f"      - {cat['name']} ({cat['id']})")

    suekk_cats = get_categories(SUEKK_API, suekk_token)
    print(f"    suekk.com: {len(suekk_cats['list'])} categories")
    for cat in suekk_cats['list']:
        print(f"      - {cat['name']} ({cat['id']})")

    # Ensure all subth categories exist in suekk
    print("\n[3] Syncing categories to suekk...")

    for cat_name in subth_cats['id_to_name'].values():
        if cat_name in suekk_cats['name_to_id']:
            print(f"    Exists: {cat_name}")
        else:
            # Create missing category in suekk.com
            print(f"    Creating: {cat_name}...")
            try:
                new_id = create_category(SUEKK_API, suekk_token, cat_name)
                suekk_cats['name_to_id'][cat_name] = new_id
                suekk_cats['id_to_name'][new_id] = cat_name
                print(f"    Created: {cat_name} ({new_id})")
            except Exception as e:
                print(f"    Failed to create {cat_name}: {e}")

    # Get videos from subth.com
    print("\n[4] Getting videos from subth.com...")
    subth_videos = get_all_videos_subth(subth_token)

    # Get videos from suekk.com
    print("\n[5] Getting videos from suekk.com...")
    suekk_videos = get_all_videos_suekk(suekk_token)

    # Match and update
    print("\n[6] Matching and updating categories...")
    updated = 0
    skipped_has_category = 0
    skipped_unmapped = 0
    not_found = 0

    for video in suekk_videos:
        video_id = video["id"]
        code = video.get("title", "")  # title contains the code
        current_category = video.get("categoryId") or (video.get("category", {}).get("id") if video.get("category") else None)

        # Skip if already has category
        if current_category:
            skipped_has_category += 1
            continue

        # Find category name from subth.com
        subth_category_name = subth_videos.get(code)
        if not subth_category_name:
            not_found += 1
            continue

        # Get suekk category ID by name
        suekk_category_id = suekk_cats['name_to_id'].get(subth_category_name)
        if not suekk_category_id:
            skipped_unmapped += 1
            print(f"    Skipped (no mapping): {code} -> {subth_category_name}")
            continue

        # Update category
        if update_video_category(suekk_token, video_id, suekk_category_id):
            updated += 1
            print(f"    Updated: {code} -> {subth_category_name}")
        else:
            print(f"    Failed: {code}")

        time.sleep(0.05)

    print("\n" + "=" * 60)
    print(f"Summary:")
    print(f"  Updated: {updated}")
    print(f"  Skipped (has category): {skipped_has_category}")
    print(f"  Skipped (unmapped category): {skipped_unmapped}")
    print(f"  Not found in subth: {not_found}")
    print("=" * 60)


if __name__ == "__main__":
    main()
