"""
Batch Translation API routes (tags, casts, categories, titles)
Uses LLMPort — เปลี่ยน provider ได้โดยไม่แก้ route logic
"""
from fastapi import APIRouter, Query, HTTPException
from pydantic import BaseModel
from typing import List, Optional

from title_translate.prompts import (
    translate_tags_batch, translate_casts_batch,
    translate_categories_batch, translate_titles_batch,
)
from title_translate.repositories import TagRepository, CastRepository, CategoryRepository, VideoTitleRepository
from title_translate.dependencies import get_llm

router = APIRouter()


class TranslationItem(BaseModel):
    original: str
    translated: str
    success: bool
    error: Optional[str] = None


class TranslationResponse(BaseModel):
    success: bool
    data: dict
    error: Optional[str] = None


@router.post("/tags/pending", response_model=TranslationResponse)
async def translate_pending_tags(limit: int = Query(default=50, ge=1, le=500)):
    """Translate tags that don't have Thai translation"""
    try:
        llm = get_llm()
        repo = TagRepository()
        tags = repo.get_untranslated(limit)

        if not tags:
            return TranslationResponse(success=True, data={"total": 0, "message": "No pending tags"})

        names = [t.name for t in tags]
        results = translate_tags_batch(llm, names)

        saved = 0
        for tag, result in zip(tags, results):
            if result.success and result.translated:
                repo.update_translation(str(tag.id), result.translated)
                saved += 1

        return TranslationResponse(success=True, data={
            "total": len(tags), "success": saved, "failed": len(tags) - saved,
        })
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/casts/pending", response_model=TranslationResponse)
async def translate_pending_casts(limit: int = Query(default=50, ge=1, le=500)):
    """Translate casts that don't have Thai translation"""
    try:
        llm = get_llm()
        repo = CastRepository()
        casts = repo.get_untranslated(limit)

        if not casts:
            return TranslationResponse(success=True, data={"total": 0, "message": "No pending casts"})

        names = [c.name for c in casts]
        results = translate_casts_batch(llm, names)

        saved = 0
        for cast, result in zip(casts, results):
            if result.success and result.translated:
                repo.update_translation(str(cast.id), result.translated)
                saved += 1

        return TranslationResponse(success=True, data={
            "total": len(casts), "success": saved, "failed": len(casts) - saved,
        })
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/categories/pending", response_model=TranslationResponse)
async def translate_pending_categories(limit: int = Query(default=50, ge=1, le=500)):
    """Translate categories that don't have Thai translation"""
    try:
        llm = get_llm()
        repo = CategoryRepository()
        categories = repo.get_untranslated(limit)

        if not categories:
            return TranslationResponse(success=True, data={"total": 0, "message": "No pending categories"})

        names = [c.name for c in categories]
        results = translate_categories_batch(llm, names)

        saved = 0
        for cat, result in zip(categories, results):
            if result.success and result.translated:
                repo.update_translation(str(cat.id), result.translated)
                saved += 1

        return TranslationResponse(success=True, data={
            "total": len(categories), "success": saved, "failed": len(categories) - saved,
        })
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/sync-all", response_model=TranslationResponse)
async def sync_all_pending(limit: int = Query(default=100, ge=1, le=500)):
    """Sync all pending translations (casts, tags, categories)"""
    try:
        llm = get_llm()

        cast_repo = CastRepository()
        tag_repo = TagRepository()
        cat_repo = CategoryRepository()

        # Casts
        casts = cast_repo.get_untranslated(limit)
        cast_saved = 0
        if casts:
            results = translate_casts_batch(llm, [c.name for c in casts])
            for cast, result in zip(casts, results):
                if result.success and result.translated:
                    cast_repo.update_translation(str(cast.id), result.translated)
                    cast_saved += 1

        # Tags
        tags = tag_repo.get_untranslated(limit)
        tag_saved = 0
        if tags:
            results = translate_tags_batch(llm, [t.name for t in tags])
            for tag, result in zip(tags, results):
                if result.success and result.translated:
                    tag_repo.update_translation(str(tag.id), result.translated)
                    tag_saved += 1

        # Categories
        cats = cat_repo.get_untranslated(limit)
        cat_saved = 0
        if cats:
            results = translate_categories_batch(llm, [c.name for c in cats])
            for cat, result in zip(cats, results):
                if result.success and result.translated:
                    cat_repo.update_translation(str(cat.id), result.translated)
                    cat_saved += 1

        return TranslationResponse(success=True, data={
            "casts": {"total": len(casts), "success": cast_saved},
            "tags": {"total": len(tags), "success": tag_saved},
            "categories": {"total": len(cats), "success": cat_saved},
        })
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


class DirectTranslateRequest(BaseModel):
    texts: List[str]


@router.post("/direct/tags", response_model=TranslationResponse)
async def translate_tags_direct(request: DirectTranslateRequest):
    """Translate tags directly (no DB)"""
    try:
        llm = get_llm()
        results = translate_tags_batch(llm, request.texts)
        return TranslationResponse(success=True, data={
            "total": len(results),
            "results": [{"original": r.original, "translated": r.translated, "success": r.success} for r in results],
        })
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/direct/casts", response_model=TranslationResponse)
async def translate_casts_direct(request: DirectTranslateRequest):
    """Translate cast names directly (no DB)"""
    try:
        llm = get_llm()
        results = translate_casts_batch(llm, request.texts)
        return TranslationResponse(success=True, data={
            "total": len(results),
            "results": [{"original": r.original, "translated": r.translated, "success": r.success} for r in results],
        })
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
