"""SQLite Tracker Database"""
from __future__ import annotations
import os
import aiosqlite
from loguru import logger

SCHEMA = """
CREATE TABLE IF NOT EXISTS series (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_site TEXT NOT NULL,
    source_id INTEGER NOT NULL,
    slug TEXT NOT NULL,
    title TEXT,
    thai_title TEXT DEFAULT '',
    thumbnail_url TEXT DEFAULT '',
    categories TEXT DEFAULT '[]',
    year INTEGER DEFAULT 0,
    rating REAL DEFAULT 0.0,
    total_episodes INTEGER DEFAULT 0,
    is_completed INTEGER DEFAULT 0,
    description TEXT DEFAULT '',
    poster_url TEXT DEFAULT '',
    audio_type TEXT DEFAULT '',
    trailer_url TEXT DEFAULT '',
    last_checked_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source_site, source_id)
);

CREATE TABLE IF NOT EXISTS episodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    series_id INTEGER NOT NULL REFERENCES series(id),
    episode_number INTEGER NOT NULL,
    video_url TEXT DEFAULT '',
    video_type TEXT DEFAULT '',
    status TEXT DEFAULT 'new',
    suekk_video_id TEXT DEFAULT '',
    suekk_video_code TEXT DEFAULT '',
    download_path TEXT DEFAULT '',
    file_size INTEGER DEFAULT 0,
    error_message TEXT DEFAULT '',
    retry_count INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(series_id, episode_number)
);

CREATE TABLE IF NOT EXISTS scrape_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_site TEXT,
    category TEXT,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME,
    new_series INTEGER DEFAULT 0,
    new_episodes INTEGER DEFAULT 0,
    errors INTEGER DEFAULT 0,
    status TEXT DEFAULT 'running'
);
"""


class Database:
    def __init__(self, db_path: str):
        self.db_path = db_path
        self._db: aiosqlite.Connection | None = None

    async def connect(self):
        os.makedirs(os.path.dirname(self.db_path), exist_ok=True)
        self._db = await aiosqlite.connect(self.db_path)
        self._db.row_factory = aiosqlite.Row
        await self._db.executescript(SCHEMA)
        await self._db.commit()
        logger.info(f"Database connected: {self.db_path}")

    async def close(self):
        if self._db:
            await self._db.close()

    async def execute(self, sql: str, params: tuple = ()) -> aiosqlite.Cursor:
        cursor = await self._db.execute(sql, params)
        await self._db.commit()
        return cursor

    async def fetchone(self, sql: str, params: tuple = ()) -> dict | None:
        cursor = await self._db.execute(sql, params)
        row = await cursor.fetchone()
        return dict(row) if row else None

    async def fetchall(self, sql: str, params: tuple = ()) -> list[dict]:
        cursor = await self._db.execute(sql, params)
        rows = await cursor.fetchall()
        return [dict(r) for r in rows]
