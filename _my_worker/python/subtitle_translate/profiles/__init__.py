"""
Profile Registry — factory สำหรับเลือก LanguageProfile ตาม language pair.

เพิ่มภาษาใหม่ = สร้าง profile + dict แล้ว register ที่นี่
"""

import logging
from typing import Dict, Tuple, Type

from shared.entities import LanguageCode
from .base import LanguageProfile
from .generic import GenericProfile

logger = logging.getLogger(__name__)

_REGISTRY: Dict[Tuple[str, str], Type[LanguageProfile]] = {}


def register(source: str, target: str, cls: Type[LanguageProfile]):
    """Register a profile class for a language pair"""
    _REGISTRY[(source, target)] = cls


def get_profile(source_lang: LanguageCode, target_lang: LanguageCode) -> LanguageProfile:
    """Get profile for a language pair — fallback to GenericProfile"""
    key = (source_lang.code, target_lang.code)
    cls = _REGISTRY.get(key)
    if cls:
        logger.debug(f"Using profile: {cls.__name__} for {key}")
        return cls()
    logger.info(f"No profile for {key}, using GenericProfile")
    return GenericProfile(source_lang, target_lang)


# === Auto-register profiles ===
from .ja_th import JaThProfile
from .ja_en import JaEnProfile

register("ja", "th", JaThProfile)
register("ja", "en", JaEnProfile)

# เพิ่มภาษาใหม่แค่ import + register:
# from .ja_zh import JaZhProfile
# register("ja", "zh", JaZhProfile)
