"""
Check category video count in api.subth.com
Find discrepancy between reported count and actual videos
"""

import requests

# API Credentials
SUBTH_EMAIL = "admin@subth.com"
SUBTH_PASSWORD = "Log2Window$P@ssWord"
SUBTH_API = "https://api.subth.com/api/v1"


def login():
    resp = requests.post(
        f"{SUBTH_API}/auth/login",
        json={"email": SUBTH_EMAIL, "password": SUBTH_PASSWORD}
    )
    resp.raise_for_status()
    data = resp.json()
    if not data.get("success"):
        raise Exception(f"Login failed: {data}")
    return data["data"]["token"]


def get_categories(token):
    resp = requests.get(
        f"{SUBTH_API}/categories",
        headers={"Authorization": f"Bearer {token}"}
    )
    resp.raise_for_status()
    return resp.json().get("data", [])


def count_videos_in_category(token, category_name):
    """Count actual videos with this category name"""
    videos = []
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
            break

        for video in data["data"]:
            if video.get("category") == category_name:
                videos.append({
                    "id": video.get("id"),
                    "title": video.get("title", ""),
                    "thumbnail": video.get("thumbnail", "")
                })

        meta = data.get("meta", {})
        print(f"  Scanning page {page}/{meta.get('totalPages', '?')}...", end="\r")

        if not meta.get("hasNext"):
            break
        page += 1

    print()
    return videos


def main():
    print("=" * 60)
    print("Check Category Video Count")
    print("=" * 60)

    token = login()
    print("Logged in to api.subth.com")

    categories = get_categories(token)
    print(f"\nCategories found: {len(categories)}")

    for cat in categories:
        name = cat.get("name", "Unknown")
        reported_count = cat.get("videoCount", 0)
        cat_id = cat.get("id")

        print(f"\n[{name}]")
        print(f"  Reported count: {reported_count}")

        # Count actual videos
        actual_videos = count_videos_in_category(token, name)
        actual_count = len(actual_videos)

        print(f"  Actual count: {actual_count}")

        if reported_count != actual_count:
            print(f"  ⚠️  MISMATCH! Difference: {reported_count - actual_count}")

            if actual_count < 10:
                print(f"  Videos in this category:")
                for v in actual_videos:
                    print(f"    - {v['title']}")

    print("\n" + "=" * 60)


if __name__ == "__main__":
    main()
