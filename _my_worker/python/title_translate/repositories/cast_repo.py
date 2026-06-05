"""
Cast Repository - Database operations for casts
Uses cast_translations table for Thai translations
"""
from typing import List, Optional

from title_translate.database import get_db_cursor
from title_translate.entities import Cast



class CastRepository(ICastRepository):
    """Cast repository implementation"""

    def get_untranslated(self, limit: int) -> List[Cast]:
        """Get casts without Thai translation"""
        with get_db_cursor() as cursor:
            cursor.execute("""
                SELECT c.id, c.name
                FROM casts c
                WHERE NOT EXISTS (
                    SELECT 1 FROM cast_translations ct
                    WHERE ct.cast_id = c.id AND ct.lang = 'th'
                )
                ORDER BY c.name
                LIMIT %s
            """, (limit,))
            rows = cursor.fetchall()

        return [
            Cast(id=r['id'], name=r['name'], name_th=None)
            for r in rows
        ]

    def get_translation(self, cast_id: str) -> Optional[str]:
        """Get Thai translation for a cast"""
        with get_db_cursor() as cursor:
            cursor.execute("""
                SELECT name FROM cast_translations
                WHERE cast_id = %s AND lang = 'th'
            """, (cast_id,))
            row = cursor.fetchone()
            return row['name'] if row else None

    def update_translation(self, cast_id: str, name_th: str) -> bool:
        """Insert or update Thai translation for a cast"""
        try:
            with get_db_cursor() as cursor:
                cursor.execute("""
                    INSERT INTO cast_translations (cast_id, lang, name)
                    VALUES (%s, 'th', %s)
                    ON CONFLICT (cast_id, lang)
                    DO UPDATE SET name = EXCLUDED.name
                """, (cast_id, name_th))
            return True
        except Exception as e:
            print(f"[ERROR] Failed to update cast translation {cast_id}: {e}")
            return False

    def get_all_for_retranslate(self, limit: int) -> List[Cast]:
        """Get all casts for re-translation"""
        with get_db_cursor() as cursor:
            cursor.execute("""
                SELECT c.id, c.name, ct.name as name_th
                FROM casts c
                LEFT JOIN cast_translations ct ON c.id = ct.cast_id AND ct.lang = 'th'
                ORDER BY c.name
                LIMIT %s
            """, (limit,))
            rows = cursor.fetchall()

        return [
            Cast(id=r['id'], name=r['name'], name_th=r['name_th'])
            for r in rows
        ]

    def get_by_name(self, name: str) -> Optional[Cast]:
        """Get cast by English name"""
        with get_db_cursor() as cursor:
            cursor.execute("""
                SELECT c.id, c.name, ct.name as name_th
                FROM casts c
                LEFT JOIN cast_translations ct ON c.id = ct.cast_id AND ct.lang = 'th'
                WHERE c.name = %s
            """, (name,))
            row = cursor.fetchone()

            if row:
                return Cast(id=row['id'], name=row['name'], name_th=row['name_th'])
            return None

    def swap_name_order(self, name: str) -> str:
        """
        Swap name from 'Last First' to 'First Last'
        DB stores: "Natsushiro Maya" (Japanese order)
        Display:   "Maya Natsushiro" (Western order)
        """
        parts = name.split()
        if len(parts) == 2:
            return f"{parts[1]} {parts[0]}"
        return name
