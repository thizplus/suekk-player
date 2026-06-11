"""ScrapeService: ตรวจซีรี่ย์ใหม่ + ตอนใหม่"""
from __future__ import annotations
import asyncio
from loguru import logger

from domain.ports import ScraperPort
from services.tracker import Tracker


class ScrapeService:
    def __init__(self, tracker: Tracker):
        self.tracker = tracker

    async def check_category(
        self,
        scraper: ScraperPort,
        category_name: str,
        category_id: int,
        max_pages: int = 0,
    ) -> dict:
        """
        เช็ค category หนึ่ง หาซีรี่ย์ใหม่ + ตอนใหม่
        ใช้แค่ aiohttp (ไม่ต้อง browser) — เร็วมาก
        max_pages=0 หมายถึงดึงทุกหน้า
        """
        site = scraper.get_site_name()
        run_id = await self.tracker.start_run(site, category_name)

        # ถ้า max_pages=0 ดึงจำนวนหน้าจริงจาก API
        if max_pages <= 0:
            total_pages = await scraper.get_total_pages(category_id)
            logger.info(f"[{site}] {category_name}: {total_pages} pages total")
        else:
            total_pages = max_pages

        stats = {"new_series": 0, "new_episodes": 0, "errors": 0}

        try:
            for page in range(1, total_pages + 1):
                logger.info(f"[{site}] Checking {category_name} page {page}/{total_pages}")

                series_list = await scraper.get_series_list(category_id, page=page)
                if not series_list:
                    break

                for series_info in series_list:
                    try:
                        await self._process_series(scraper, series_info, stats)
                    except Exception as e:
                        logger.error(f"Error processing {series_info.slug}: {e}")
                        stats["errors"] += 1

                # Delay ระหว่าง page (ไม่ให้ request ถี่เกินไป)
                if page < max_pages:
                    await asyncio.sleep(1)

        except Exception as e:
            logger.error(f"Category check failed: {e}")
            stats["errors"] += 1

        await self.tracker.finish_run(run_id, **stats)
        logger.info(
            f"[{site}] {category_name} done: "
            f"{stats['new_series']} new series, "
            f"{stats['new_episodes']} new episodes, "
            f"{stats['errors']} errors"
        )
        return stats

    async def _process_series(self, scraper: ScraperPort, series_info, stats: dict):
        """Process ซีรี่ย์หนึ่ง: เช็คว่ามีใน DB ไหม + มีตอนใหม่ไหม"""
        site = scraper.get_site_name()
        existing = await self.tracker.get_series(site, series_info.source_id)

        # ดึง detail page เพื่อรู้จำนวน episodes จริง
        detail = await scraper.get_series_detail(series_info.slug)
        if not detail:
            logger.warning(f"Could not get detail for {series_info.slug}")
            return

        # บางทีอาจจะไม่ได้ post_id จาก detail — ใช้จาก listing
        if detail.source_id == 0:
            detail.source_id = series_info.source_id
        # เอา thumbnail จาก listing (wp-json มี URL เต็ม)
        if not detail.thumbnail_url and series_info.thumbnail_url:
            detail.thumbnail_url = series_info.thumbnail_url
        # categories จาก listing
        if not detail.categories and series_info.categories:
            detail.categories = series_info.categories

        if existing is None:
            # ── ซีรี่ย์ใหม่ทั้งหมด ──
            series_id = await self.tracker.save_series(detail)
            if detail.episodes:
                new_ep_ids = await self.tracker.save_episodes(series_id, detail.episodes)
                stats["new_episodes"] += len(new_ep_ids)
            stats["new_series"] += 1

            logger.info(
                f"NEW SERIES: {detail.title} "
                f"({len(detail.episodes)} episodes)"
            )
        else:
            # ── ซีรี่ย์เก่า — เช็คตอนใหม่ ──
            series_id = existing["id"]
            known_count = await self.tracker.get_episode_count(series_id)

            if len(detail.episodes) > known_count:
                # มีตอนใหม่!
                new_ep_ids = await self.tracker.save_episodes(series_id, detail.episodes)
                if new_ep_ids:
                    stats["new_episodes"] += len(new_ep_ids)
                    logger.info(
                        f"NEW EPISODES: {detail.title} "
                        f"+{len(new_ep_ids)} episodes "
                        f"(was {known_count}, now {len(detail.episodes)})"
                    )

            # Update series info
            await self.tracker.save_series(detail)

        # Delay ระหว่าง series (ป้องกัน rate limit)
        await asyncio.sleep(0.5)
