from __future__ import annotations
from abc import ABC, abstractmethod

from domain.models import SeriesInfo, SeriesDetail, VideoSource


class ScraperPort(ABC):
    """Interface ที่ทุก site adapter ต้อง implement"""

    @abstractmethod
    async def get_series_list(self, category_id: int, page: int = 1) -> list[SeriesInfo]:
        """ดึงรายการซีรี่ย์จาก category (ใช้ API — เร็ว)"""
        ...

    @abstractmethod
    async def get_series_detail(self, slug: str) -> SeriesDetail:
        """ดึงข้อมูล series + episode list (parse HTML)"""
        ...

    @abstractmethod
    async def get_episode_video_url(
        self, page_url: str, episode: int, server: int = 1
    ) -> VideoSource | None:
        """ดึง video URL ของตอนนั้นๆ (ใช้ Playwright)"""
        ...

    @abstractmethod
    async def get_category_map(self) -> dict[str, int]:
        """ดึง mapping: category_name -> category_id"""
        ...

    @abstractmethod
    def get_site_name(self) -> str:
        ...

    @abstractmethod
    async def close(self):
        ...
