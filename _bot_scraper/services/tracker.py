"""Tracker: ป้องกันซ้ำ + ตรวจตอนใหม่ + track download status"""
from __future__ import annotations
import json
from datetime import datetime
from loguru import logger

from infrastructure.database import Database
from domain.models import SeriesInfo, SeriesDetail, EpisodeInfo


class Tracker:
    def __init__(self, db: Database):
        self.db = db

    # ─── Series ───

    async def get_series(self, source_site: str, source_id: int) -> dict | None:
        return await self.db.fetchone(
            "SELECT * FROM series WHERE source_site=? AND source_id=?",
            (source_site, source_id),
        )

    async def get_series_by_slug(self, source_site: str, slug: str) -> dict | None:
        return await self.db.fetchone(
            "SELECT * FROM series WHERE source_site=? AND slug=?",
            (source_site, slug),
        )

    async def save_series(self, detail: SeriesDetail) -> int:
        """Insert or update series"""
        existing = await self.get_series(detail.source_site, detail.source_id)

        if existing:
            await self.db.execute(
                """UPDATE series SET
                    title=?, thai_title=?, thumbnail_url=?, categories=?,
                    total_episodes=?, is_completed=?, description=?,
                    poster_url=?, audio_type=?, trailer_url=?,
                    rating=?, last_checked_at=?, updated_at=?
                WHERE id=?""",
                (
                    detail.title, detail.thai_title, detail.thumbnail_url,
                    json.dumps(detail.categories, ensure_ascii=False),
                    detail.total_episodes, int(detail.is_completed),
                    detail.description, detail.poster_url, detail.audio_type,
                    detail.trailer_url, detail.rating,
                    datetime.now().isoformat(), datetime.now().isoformat(),
                    existing["id"],
                ),
            )
            return existing["id"]
        else:
            cursor = await self.db.execute(
                """INSERT INTO series
                    (source_site, source_id, slug, title, thai_title, thumbnail_url,
                     categories, year, rating, total_episodes, is_completed,
                     description, poster_url, audio_type, trailer_url, last_checked_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)""",
                (
                    detail.source_site, detail.source_id, detail.slug,
                    detail.title, detail.thai_title, detail.thumbnail_url,
                    json.dumps(detail.categories, ensure_ascii=False),
                    detail.year, detail.rating, detail.total_episodes,
                    int(detail.is_completed), detail.description,
                    detail.poster_url, detail.audio_type, detail.trailer_url,
                    datetime.now().isoformat(),
                ),
            )
            return cursor.lastrowid

    # ─── Episodes ───

    async def get_episode_count(self, series_id: int) -> int:
        row = await self.db.fetchone(
            "SELECT COUNT(*) as cnt FROM episodes WHERE series_id=?",
            (series_id,),
        )
        return row["cnt"] if row else 0

    async def get_known_episode_numbers(self, series_id: int) -> set[int]:
        rows = await self.db.fetchall(
            "SELECT episode_number FROM episodes WHERE series_id=?",
            (series_id,),
        )
        return {r["episode_number"] for r in rows}

    async def save_episodes(self, series_id: int, episodes: list[EpisodeInfo]) -> list[int]:
        """Insert new episodes (skip existing)"""
        known = await self.get_known_episode_numbers(series_id)
        new_ids = []

        for ep in episodes:
            if ep.episode_number in known:
                continue
            cursor = await self.db.execute(
                "INSERT INTO episodes (series_id, episode_number) VALUES (?, ?)",
                (series_id, ep.episode_number),
            )
            new_ids.append(cursor.lastrowid)
            logger.debug(f"New episode tracked: series_id={series_id} ep={ep.episode_number}")

        return new_ids

    async def get_pending_episodes(self) -> list[dict]:
        """Episodes ที่ status = 'new' รอ download"""
        return await self.db.fetchall(
            """SELECT e.*, s.slug, s.title as series_title, s.source_site
            FROM episodes e
            JOIN series s ON e.series_id = s.id
            WHERE e.status = 'new'
            ORDER BY e.created_at ASC
            LIMIT 10""",
        )

    async def update_episode_status(self, episode_id: int, status: str, **kwargs):
        """อัพเดท status + optional fields"""
        sets = ["status=?", "updated_at=?"]
        params = [status, datetime.now().isoformat()]

        for key in ("video_url", "video_type", "download_path", "file_size",
                     "suekk_video_id", "suekk_video_code", "error_message", "retry_count"):
            if key in kwargs:
                sets.append(f"{key}=?")
                params.append(kwargs[key])

        params.append(episode_id)
        await self.db.execute(
            f"UPDATE episodes SET {', '.join(sets)} WHERE id=?",
            tuple(params),
        )

    # ─── Scrape Runs ───

    async def start_run(self, source_site: str, category: str) -> int:
        cursor = await self.db.execute(
            "INSERT INTO scrape_runs (source_site, category) VALUES (?, ?)",
            (source_site, category),
        )
        return cursor.lastrowid

    async def finish_run(self, run_id: int, new_series: int, new_episodes: int, errors: int):
        await self.db.execute(
            """UPDATE scrape_runs SET
                finished_at=?, new_series=?, new_episodes=?, errors=?, status='completed'
            WHERE id=?""",
            (datetime.now().isoformat(), new_series, new_episodes, errors, run_id),
        )

    # ─── Stats ───

    async def get_stats(self) -> dict:
        series = await self.db.fetchone("SELECT COUNT(*) as cnt FROM series")
        episodes = await self.db.fetchone("SELECT COUNT(*) as cnt FROM episodes")
        pending = await self.db.fetchone("SELECT COUNT(*) as cnt FROM episodes WHERE status='new'")
        uploaded = await self.db.fetchone("SELECT COUNT(*) as cnt FROM episodes WHERE status='uploaded'")
        failed = await self.db.fetchone("SELECT COUNT(*) as cnt FROM episodes WHERE status='failed'")

        return {
            "total_series": series["cnt"],
            "total_episodes": episodes["cnt"],
            "pending": pending["cnt"],
            "uploaded": uploaded["cnt"],
            "failed": failed["cnt"],
        }
