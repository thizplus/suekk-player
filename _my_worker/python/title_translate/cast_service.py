"""
Cast Service — จัดการ cast ผ่าน SubTH API + ทับศัพท์ด้วย LLMPort

Port จาก _suekk_bot/python_translate/app/services/klon8_service.py
เปลี่ยนจาก google.generativeai → LLMPort
"""
import logging
from typing import Optional, Dict

import requests

from title_translate.config import settings
from title_translate.dependencies import get_llm
from title_translate.prompts import transliterate_name

logger = logging.getLogger(__name__)


class CastService:
    """จัดการ cast: search/create/update via SubTH API + ทับศัพท์"""

    def __init__(self):
        self._api_token: Optional[str] = None

    # === Auth ===

    def _login_api(self) -> Optional[str]:
        if not settings.api_email or not settings.api_password:
            logger.warning("API_EMAIL or API_PASSWORD not configured")
            return None
        try:
            url = f"{settings.api_subth_url}/auth/login"
            resp = requests.post(url, json={
                "email": settings.api_email,
                "password": settings.api_password,
            }, timeout=10)
            if resp.status_code == 200:
                self._api_token = resp.json().get('data', {}).get('token')
                if self._api_token:
                    logger.info("SubTH API login OK")
                    return self._api_token
            logger.warning("SubTH API login failed: %s", resp.status_code)
        except Exception as e:
            logger.warning("SubTH API login error: %s", e)
        return None

    def _get_token(self) -> Optional[str]:
        if self._api_token:
            return self._api_token
        return self._login_api()

    def _refresh_if_401(self, status_code: int) -> bool:
        if status_code == 401:
            self._api_token = None
            return self._login_api() is not None
        return False

    # === Cast API ===

    def _search_cast(self, name: str) -> Optional[Dict]:
        try:
            url = f"{settings.api_subth_url}/casts/search"
            resp = requests.get(url, params={"q": name, "lang": "en", "limit": 1}, timeout=10)
            if resp.status_code == 200:
                casts = resp.json().get('data', [])
                if casts:
                    cast_id = casts[0].get('id')
                    if cast_id:
                        return self._get_cast_detail(cast_id)
        except Exception as e:
            logger.warning("Search cast error: %s", e)
        return None

    def _get_cast_detail(self, cast_id: str) -> Optional[Dict]:
        try:
            resp = requests.get(f"{settings.api_subth_url}/casts/{cast_id}", timeout=10)
            if resp.status_code == 200:
                return resp.json().get('data')
        except Exception as e:
            logger.warning("Get cast detail error: %s", e)
        return None

    def _create_cast(self, name: str, translations: Dict[str, str] = None, _retry=True) -> Optional[Dict]:
        token = self._get_token()
        if not token:
            return None
        try:
            payload = {"name": name}
            if translations:
                payload["translations"] = translations
            resp = requests.post(
                f"{settings.api_subth_url}/casts",
                json=payload,
                headers={"Authorization": f"Bearer {token}"},
                timeout=10,
            )
            if resp.status_code == 201:
                logger.info("Created cast: %s", name)
                return resp.json().get('data')
            elif resp.status_code == 409:
                return self._search_cast(name)
            elif resp.status_code == 401 and _retry:
                if self._refresh_if_401(401):
                    return self._create_cast(name, translations, _retry=False)
            else:
                logger.warning("Create cast failed: %s", resp.status_code)
        except Exception as e:
            logger.warning("Create cast error: %s", e)
        return None

    def _update_cast(self, cast_id: str, translations: Dict[str, str], _retry=True) -> bool:
        token = self._get_token()
        if not token:
            return False
        try:
            resp = requests.put(
                f"{settings.api_subth_url}/casts/{cast_id}",
                json={"translations": translations},
                headers={"Authorization": f"Bearer {token}"},
                timeout=10,
            )
            if resp.status_code == 200:
                return True
            elif resp.status_code == 401 and _retry:
                if self._refresh_if_401(401):
                    return self._update_cast(cast_id, translations, _retry=False)
        except Exception as e:
            logger.warning("Update cast error: %s", e)
        return False

    # === Core ===

    @staticmethod
    def _swap_name_order(name: str) -> str:
        """สลับ 'Mikami Yua' → 'Yua Mikami' (JP name order → Western)"""
        if not name:
            return name
        parts = name.strip().split()
        if len(parts) == 2:
            return f"{parts[1]} {parts[0]}"
        return name

    def ensure_cast_with_translation(self, cast_name_en: str) -> str:
        """
        ตรวจสอบ + สร้าง cast + ทับศัพท์ผ่าน SubTH API

        Flow:
        1. Search cast ใน SubTH API
        2. ถ้าไม่มี → ทับศัพท์ + create
        3. ถ้ามีแต่ไม่มี TH → ทับศัพท์ + update
        4. Return: "Yua Mikami (ยัว มิคามิ)"
        """
        if not cast_name_en:
            return ""

        cast_name_original = cast_name_en.strip()
        cast_th = None
        llm = get_llm()

        try:
            # 1. Search cast via API
            cast_data = self._search_cast(cast_name_original)

            if cast_data:
                translations = cast_data.get('translations', {})
                cast_th = translations.get('th')

                if not cast_th:
                    # 2. ทับศัพท์ + สลับ + update
                    cast_th_raw = transliterate_name(llm, cast_name_original)
                    if cast_th_raw:
                        cast_th = self._swap_name_order(cast_th_raw)
                        cast_id = cast_data.get('id')
                        if cast_id:
                            new_translations = {**translations, 'th': cast_th}
                            self._update_cast(str(cast_id), new_translations)
                            logger.info("Updated TH: %s → %s", cast_name_original, cast_th)
            else:
                # 3. Cast ไม่มี → ทับศัพท์ + create
                cast_th_raw = transliterate_name(llm, cast_name_original)
                if cast_th_raw:
                    cast_th = self._swap_name_order(cast_th_raw)

                translations = {"th": cast_th} if cast_th else None
                self._create_cast(cast_name_original, translations)

        except Exception as e:
            logger.warning("ensure_cast error: %s", e)
            # Fallback: ทับศัพท์อย่างเดียว ไม่ save
            cast_th_raw = transliterate_name(llm, cast_name_original)
            cast_th = self._swap_name_order(cast_th_raw) if cast_th_raw else None

        # Return format: "Yua Mikami (ยัว มิคามิ)"
        cast_en_swapped = self._swap_name_order(cast_name_original)

        if cast_th:
            return f"{cast_en_swapped} ({cast_th})"
        return cast_en_swapped


# Singleton
_cast_service: Optional[CastService] = None


def get_cast_service() -> CastService:
    global _cast_service
    if _cast_service is None:
        _cast_service = CastService()
    return _cast_service
