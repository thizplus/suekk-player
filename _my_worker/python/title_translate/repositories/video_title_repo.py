"""
Video Title Repository - Database operations for video translations
"""
from typing import List

from title_translate.database import get_db_cursor, get_db_connection
from title_translate.entities import VideoTitle



class VideoTitleRepository(IVideoTitleRepository):
    """Video title repository implementation"""

    def get_untranslated(self, limit: int) -> List[VideoTitle]:
        """Get video titles without Thai translation"""
        with get_db_cursor() as cursor:
            # หา videos ที่มี title_en แต่ไม่มี title_th
            cursor.execute("""
                SELECT
                    vt_en.video_id,
                    vt_en.title as title_en
                FROM video_translations vt_en
                WHERE vt_en.lang = 'en'
                AND NOT EXISTS (
                    SELECT 1 FROM video_translations vt_th
                    WHERE vt_th.video_id = vt_en.video_id
                    AND vt_th.lang = 'th'
                    AND vt_th.title IS NOT NULL
                    AND vt_th.title != ''
                )
                ORDER BY vt_en.video_id
                LIMIT %s
            """, (limit,))
            rows = cursor.fetchall()

        return [
            VideoTitle(video_id=r['video_id'], title_en=r['title_en'])
            for r in rows
        ]

    def update_translation(self, video_id: str, title_th: str) -> bool:
        """Update Thai translation for a video title"""
        try:
            with get_db_cursor() as cursor:
                cursor.execute("""
                    UPDATE video_translations
                    SET title = %s
                    WHERE video_id = %s AND lang = 'th'
                """, (title_th, video_id))
            return True
        except Exception as e:
            print(f"[ERROR] Failed to update video title {video_id}: {e}")
            return False

    def create_translation(self, video_id: str, title_th: str) -> bool:
        """Create new Thai translation entry for a video"""
        try:
            with get_db_connection() as conn:
                cursor = conn.cursor()
                # ลอง INSERT ถ้า conflict ก็ UPDATE
                cursor.execute("""
                    INSERT INTO video_translations (video_id, lang, title)
                    VALUES (%s, 'th', %s)
                    ON CONFLICT (video_id, lang)
                    DO UPDATE SET title = EXCLUDED.title
                """, (video_id, title_th))
                conn.commit()
            return True
        except Exception as e:
            print(f"[ERROR] Failed to create video translation {video_id}: {e}")
            return False
