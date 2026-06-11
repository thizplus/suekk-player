from __future__ import annotations
from dataclasses import dataclass, field


@dataclass
class SeriesInfo:
    """ข้อมูลจาก listing page (wp-json)"""
    source_site: str            # "serie_days"
    source_id: int              # WordPress post ID
    slug: str                   # "the-legend-of-kitchen-soldier"
    title: str                  # "The Legend Of Kitchen Soldier (2026)"
    thai_title: str = ""        # "บันทึกครัวค่ายทหาร"
    thumbnail_url: str = ""
    categories: list[str] = field(default_factory=list)
    year: int = 0
    rating: float = 0.0
    quality: str = ""           # "HD"
    published_at: str = ""      # ISO datetime


@dataclass
class EpisodeInfo:
    """ข้อมูลตอนจาก detail page"""
    episode_number: int
    servers: list[int] = field(default_factory=lambda: [1])


@dataclass
class SeriesDetail(SeriesInfo):
    """ข้อมูลเต็มจาก detail page (HTML parse + OG meta)"""
    description: str = ""
    genres: list[str] = field(default_factory=list)
    episodes: list[EpisodeInfo] = field(default_factory=list)
    total_episodes: int = 0
    is_completed: bool = False
    nonce: str = ""             # สำหรับ AJAX call
    # metadata เพิ่มเติมจาก detail page
    poster_url: str = ""        # OG image (full size, ไม่ใช่ thumbnail)
    audio_type: str = ""        # "Sound Track", "Thai", "พากย์ไทย"
    trailer_url: str = ""       # YouTube trailer URL (ถ้ามี)
    og_title: str = ""          # OG title (สะอาดกว่า page title)


@dataclass
class VideoSource:
    """ผลลัพธ์จากการ resolve video URL"""
    url: str
    video_type: str = ""        # "m3u8" | "mp4"
    quality: str = ""
    server: int = 1
    headers: dict = field(default_factory=dict)
