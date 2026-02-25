"""
Test Role Analysis - ทดสอบการวิเคราะห์ตัวละครโดยไม่ต้องแปล

Usage:
    python test_role_analysis.py <video_code>
    python test_role_analysis.py he33mfmx
"""

import sys
import os
import json
import re

# Add subtitle project to path
sys.path.insert(0, r"D:\Admin\Desktop\MY PROJECT\___SUEKK_STREAM\_subtitle")

from pathlib import Path
from infrastructure.s3_client import get_s3_client
from infrastructure.gemini_client import GeminiClient
from services.speaker_service import SpeakerService
import requests

# API Config
SUEKK_API = "https://api.suekk.com/api/v1"
SUEKK_EMAIL = "info@thizplus.com"
SUEKK_PASSWORD = "Log2Window$P@ssWord"


def login_suekk() -> str:
    """Login to suekk.com and return token"""
    resp = requests.post(
        f"{SUEKK_API}/auth/login",
        json={"email": SUEKK_EMAIL, "password": SUEKK_PASSWORD}
    )
    resp.raise_for_status()
    return resp.json()["data"]["token"]


def get_video_info(token: str, video_code: str) -> dict:
    """Get video info from suekk.com"""
    resp = requests.get(
        f"{SUEKK_API}/videos",
        params={"search": video_code, "limit": 1},
        headers={"Authorization": f"Bearer {token}"}
    )
    resp.raise_for_status()
    data = resp.json()
    if data.get("data"):
        return data["data"][0]
    return None


def parse_timestamp(ts: str) -> int:
    """Parse SRT timestamp to milliseconds"""
    match = re.match(r"(\d{2}):(\d{2}):(\d{2}),(\d{3})", ts)
    if match:
        h, m, s, ms = map(int, match.groups())
        return h * 3600000 + m * 60000 + s * 1000 + ms
    return 0


def parse_srt(content: str) -> list:
    """Parse SRT content to Subtitle objects"""
    from domain.models import Subtitle

    segments = []
    blocks = re.split(r"\n\n+", content.strip())

    for block in blocks:
        lines = block.strip().split("\n")
        if len(lines) >= 3:
            timestamp_match = re.match(
                r"(\d{2}:\d{2}:\d{2},\d{3}) --> (\d{2}:\d{2}:\d{2},\d{3})",
                lines[1],
            )
            if timestamp_match:
                start_ms = parse_timestamp(timestamp_match.group(1))
                end_ms = parse_timestamp(timestamp_match.group(2))
                text = "\n".join(lines[2:])
                segments.append(Subtitle(start_ms=start_ms, end_ms=end_ms, text=text))

    return segments


def main():
    if len(sys.argv) < 2:
        print("Usage: python test_role_analysis.py <video_code>")
        print("Example: python test_role_analysis.py he33mfmx")
        sys.exit(1)

    video_code = sys.argv[1]
    print("=" * 60)
    print(f"Testing Role Analysis for: {video_code}")
    print("=" * 60)

    # 1. Login and get video info
    print("\n[1] Getting video info...")
    token = login_suekk()
    video = get_video_info(token, video_code)

    if not video:
        print(f"    Video not found: {video_code}")
        sys.exit(1)

    context = video.get("description", "")
    print(f"    Title: {video.get('title')}")
    print(f"    Description: {context[:100]}..." if len(context) > 100 else f"    Description: {context}")

    # 2. Download JP.srt and speakers.json from S3
    print("\n[2] Downloading files from S3...")
    s3 = get_s3_client()
    temp_dir = Path(r"D:\Admin\Desktop\MY PROJECT\___SUEKK_STREAM\_subtitle\temp")
    temp_dir.mkdir(exist_ok=True)

    # Download JP.srt
    jp_srt_path = temp_dir / f"{video_code}_ja.srt"
    s3_srt_path = f"subtitles/{video_code}/ja.srt"
    if s3.download_file(s3_srt_path, jp_srt_path):
        print(f"    Downloaded: {s3_srt_path}")
    else:
        print(f"    Failed to download: {s3_srt_path}")
        sys.exit(1)

    # Download speakers.json
    speakers_path = temp_dir / f"{video_code}_speakers.json"
    s3_speakers_path = f"subtitles/{video_code}/speakers.json"
    has_speakers = s3.download_file(s3_speakers_path, speakers_path)
    if has_speakers:
        print(f"    Downloaded: {s3_speakers_path}")
    else:
        print(f"    No speakers.json found (will skip speaker tagging)")

    # 3. Parse JP.srt
    print("\n[3] Parsing JP.srt...")
    srt_content = jp_srt_path.read_text(encoding="utf-8")
    segments = parse_srt(srt_content)
    print(f"    Total segments: {len(segments)}")

    # Show first 10 lines (safe print for Windows)
    print("\n    First 10 lines:")
    for i, seg in enumerate(segments[:10]):
        safe_text = seg.text[:50].encode('ascii', 'replace').decode('ascii')
        print(f"      [{i}] {safe_text}...")

    # 4. Load speaker info and tag subtitles
    print("\n[4] Loading speaker info...")
    speaker_service = SpeakerService(use_gpu=False)

    if has_speakers:
        speaker_info = speaker_service.load_speaker_info(speakers_path)
        if speaker_info:
            print(f"    Original speakers: {list(speaker_info.speakers.keys())}")
            for spk, info in speaker_info.speakers.items():
                gender = info.get('gender', '?')
                print(f"      {spk}: {gender}")

            # Tag subtitles
            tagged_segments = speaker_service.tag_subtitles(segments, speaker_info)

            # === NEW: Merge similar speakers (reduce over-segmentation) ===
            print("\n[4.1] Merging over-segmented speakers...")
            merged_subtitles, speaker_merge_map, merged_speaker_info = speaker_service.merge_similar_speakers(
                tagged_segments,
                speaker_info,
                max_speakers=4  # Limit to 4 speakers
            )
            print(f"    Merge map: {speaker_merge_map}")
            print(f"    After merge: {list(merged_speaker_info.speakers.keys())}")

            # Use merged data for role detection
            tagged_segments = merged_subtitles
            speaker_info = merged_speaker_info

            # Detect roles with keyword voting
            role_info = speaker_service.detect_speaker_roles(tagged_segments, speaker_info)
            merged_speakers = role_info.get('merged_speakers', {})
            roles = role_info.get('roles', {})
            keyword_scores = role_info.get('keyword_scores', {})

            print(f"\n    Merged speakers: {merged_speakers}")
            print(f"\n    Keyword scores:")
            for speaker, scores in keyword_scores.items():
                non_zero = {k: v for k, v in scores.items() if v > 0}
                if non_zero:
                    print(f"      {speaker}: {non_zero}")
        else:
            print("    Failed to load speaker info")
            tagged_segments = segments
            merged_speakers = {}
            roles = {}
    else:
        tagged_segments = segments
        merged_speakers = {}
        roles = {}

    # 5. Run Gemini role analysis
    print("\n[5] Running Gemini role analysis...")
    print(f"    Context: {context[:80]}..." if context else "    Context: (none)")

    gemini = GeminiClient()
    analyzed_roles, scenario = speaker_service.analyze_roles_with_llm(
        tagged_segments,
        merged_speakers,
        roles,
        gemini,
        context=context
    )

    # 6. Show results - write to file to avoid encoding issues
    result_file = temp_dir / f"{video_code}_role_analysis.txt"
    with open(result_file, 'w', encoding='utf-8') as f:
        f.write("=" * 60 + "\n")
        f.write("ROLE ANALYSIS RESULTS\n")
        f.write("=" * 60 + "\n")
        f.write(f"[CONTEXT] {context}\n")
        f.write(f"[SCENARIO] {scenario}\n")
        f.write("-" * 40 + "\n")

        for letter, info in sorted(analyzed_roles.items()):
            role = info.get('role', '?')
            pronoun = info.get('pronoun', '?')
            # Find original speaker
            orig_speaker = [k for k, v in merged_speakers.items() if v == letter]
            orig_str = f"({orig_speaker[0]})" if orig_speaker else "(unknown)"
            f.write(f"  {letter} {orig_str} = {role} (สรรพนาม: {pronoun})\n")

        f.write("=" * 60 + "\n")

    print(f"\nResults saved to: {result_file}")
    print("\n" + "=" * 60)
    print("ROLE ANALYSIS RESULTS (see file for Thai text)")
    print("=" * 60)

    # Cleanup
    jp_srt_path.unlink(missing_ok=True)
    speakers_path.unlink(missing_ok=True)

    print("\nDone!")


if __name__ == "__main__":
    main()
