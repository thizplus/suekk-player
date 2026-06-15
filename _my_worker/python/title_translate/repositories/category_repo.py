"""
Category Repository - Database operations for categories
Uses category_translations table for Thai translations
"""
from typing import List, Optional

from title_translate.database import get_db_cursor
from title_translate.entities import Category



class CategoryRepository:
    """Category repository implementation"""

    def get_untranslated(self, limit: int) -> List[Category]:
        """Get categories without Thai translation"""
        with get_db_cursor() as cursor:
            cursor.execute("""
                SELECT c.id, c.name
                FROM categories c
                WHERE NOT EXISTS (
                    SELECT 1 FROM category_translations ct
                    WHERE ct.category_id = c.id AND ct.lang = 'th'
                )
                ORDER BY c.name
                LIMIT %s
            """, (limit,))
            rows = cursor.fetchall()

        return [
            Category(id=r['id'], name=r['name'], name_th=None)
            for r in rows
        ]

    def get_translation(self, category_id: str) -> Optional[str]:
        """Get Thai translation for a category"""
        with get_db_cursor() as cursor:
            cursor.execute("""
                SELECT name FROM category_translations
                WHERE category_id = %s AND lang = 'th'
            """, (category_id,))
            row = cursor.fetchone()
            return row['name'] if row else None

    def update_translation(self, category_id: str, name_th: str) -> bool:
        """Insert or update Thai translation for a category"""
        try:
            with get_db_cursor() as cursor:
                cursor.execute("""
                    INSERT INTO category_translations (category_id, lang, name)
                    VALUES (%s, 'th', %s)
                    ON CONFLICT (category_id, lang)
                    DO UPDATE SET name = EXCLUDED.name
                """, (category_id, name_th))
            return True
        except Exception as e:
            print(f"[ERROR] Failed to update category translation {category_id}: {e}")
            return False

    def get_by_name(self, name: str) -> Optional[Category]:
        """Get category by English name"""
        with get_db_cursor() as cursor:
            cursor.execute("""
                SELECT c.id, c.name, ct.name as name_th
                FROM categories c
                LEFT JOIN category_translations ct ON c.id = ct.category_id AND ct.lang = 'th'
                WHERE c.name = %s
            """, (name,))
            row = cursor.fetchone()

            if row:
                return Category(id=row['id'], name=row['name'], name_th=row['name_th'])
            return None
