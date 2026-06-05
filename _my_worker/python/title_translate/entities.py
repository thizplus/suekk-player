"""
Domain entities for title translation service.
"""
from dataclasses import dataclass
from typing import Optional, Any, List
from enum import Enum


class TranslationType(str, Enum):
    TAG = "tag"
    CAST = "cast"
    CATEGORY = "category"
    VIDEO_TITLE = "video_title"


@dataclass
class Tag:
    id: Any
    name: str
    name_th: Optional[str] = None


@dataclass
class Cast:
    id: Any
    name: str
    name_th: Optional[str] = None


@dataclass
class Category:
    id: Any
    name: str
    name_th: Optional[str] = None


@dataclass
class VideoTitle:
    video_id: str
    title_en: str
    title_th: Optional[str] = None


@dataclass
class TranslationResult:
    original: str
    translated: str
    success: bool = True
    error: Optional[str] = None


@dataclass
class BatchTranslationResult:
    total: int
    success: int
    failed: int
    results: List[TranslationResult]
