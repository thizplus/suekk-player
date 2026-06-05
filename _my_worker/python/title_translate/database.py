"""
Database connection for title_translate service.
Connects to subth database (separate from SUEKK).
"""
from contextlib import contextmanager
from typing import Generator

import psycopg2
from psycopg2.extras import RealDictCursor

from title_translate.config import settings


@contextmanager
def get_db_connection():
    """Get database connection as context manager"""
    conn = psycopg2.connect(settings.database_url)
    try:
        yield conn
    finally:
        conn.close()


@contextmanager
def get_db_cursor() -> Generator:
    """Get database cursor with RealDictCursor"""
    with get_db_connection() as conn:
        cursor = conn.cursor(cursor_factory=RealDictCursor)
        try:
            yield cursor
            conn.commit()
        except Exception:
            conn.rollback()
            raise
        finally:
            cursor.close()
