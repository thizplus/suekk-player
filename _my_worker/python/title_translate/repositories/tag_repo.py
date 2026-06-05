"""
Tag Repository - Database operations for tags
Uses tag_translations table for Thai translations
"""
from typing import List, Optional

from title_translate.database import get_db_cursor
from title_translate.entities import Tag



class TagRepository(ITagRepository):
    """Tag repository implementation"""

    def get_untranslated(self, limit: int) -> List[Tag]:
        """Get tags without Thai translation"""
        with get_db_cursor() as cursor:
            cursor.execute("""
                SELECT t.id, t.name
                FROM tags t
                WHERE NOT EXISTS (
                    SELECT 1 FROM tag_translations tt
                    WHERE tt.tag_id = t.id AND tt.lang = 'th'
                )
                ORDER BY t.name
                LIMIT %s
            """, (limit,))
            rows = cursor.fetchall()

        return [
            Tag(id=r['id'], name=r['name'], name_th=None)
            for r in rows
        ]

    def get_translation(self, tag_id: str) -> Optional[str]:
        """Get Thai translation for a tag"""
        with get_db_cursor() as cursor:
            cursor.execute("""
                SELECT name FROM tag_translations
                WHERE tag_id = %s AND lang = 'th'
            """, (tag_id,))
            row = cursor.fetchone()
            return row['name'] if row else None

    def update_translation(self, tag_id: str, name_th: str) -> bool:
        """Insert or update Thai translation for a tag"""
        try:
            with get_db_cursor() as cursor:
                cursor.execute("""
                    INSERT INTO tag_translations (tag_id, lang, name)
                    VALUES (%s, 'th', %s)
                    ON CONFLICT (tag_id, lang)
                    DO UPDATE SET name = EXCLUDED.name
                """, (tag_id, name_th))
            return True
        except Exception as e:
            print(f"[ERROR] Failed to update tag translation {tag_id}: {e}")
            return False

    def get_all_for_retranslate(self, limit: int) -> List[Tag]:
        """Get all tags for re-translation"""
        with get_db_cursor() as cursor:
            cursor.execute("""
                SELECT t.id, t.name, tt.name as name_th
                FROM tags t
                LEFT JOIN tag_translations tt ON t.id = tt.tag_id AND tt.lang = 'th'
                ORDER BY t.name
                LIMIT %s
            """, (limit,))
            rows = cursor.fetchall()

        return [
            Tag(id=r['id'], name=r['name'], name_th=r['name_th'])
            for r in rows
        ]

    def get_by_name(self, name: str) -> Optional[Tag]:
        """Get tag by English name"""
        with get_db_cursor() as cursor:
            cursor.execute("""
                SELECT t.id, t.name, tt.name as name_th
                FROM tags t
                LEFT JOIN tag_translations tt ON t.id = tt.tag_id AND tt.lang = 'th'
                WHERE t.name = %s
            """, (name,))
            row = cursor.fetchone()

            if row:
                return Tag(id=row['id'], name=row['name'], name_th=row['name_th'])
            return None
