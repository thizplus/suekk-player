"""
Title Translation API routes (กลอน 8 / CN / EN)
Uses LLMPort — เปลี่ยน provider ได้โดยไม่แก้ route logic
"""
from fastapi import APIRouter, Query, HTTPException
from pydantic import BaseModel
from typing import List, Optional, Dict

from title_translate.prompts import (
    translate_title_klon8, translate_title_cn, translate_title_en,
    extract_code,
)
from title_translate.dependencies import get_llm
from title_translate.cast_service import get_cast_service

router = APIRouter()


class TitleTranslateRequest(BaseModel):
    title_en: str
    cast_name: Optional[str] = None
    cast: Optional[List[str]] = None
    tags: Optional[List[str]] = None
    source_type: Optional[str] = "jav"  # "jav", "cn", "en"


class TitleResponse(BaseModel):
    success: bool
    data: dict
    error: Optional[str] = None


@router.post("/translate-only", response_model=TitleResponse)
async def translate_only(request: TitleTranslateRequest):
    """แปล title โดยไม่ต้องมี video_id (ไม่ save DB)"""
    try:
        llm = get_llm()
        code = extract_code(request.title_en)
        tags = request.tags or []

        # จัดการ cast: search/create/transliterate via SubTH API
        cast_service = get_cast_service()
        cast_name_formatted = ""
        if request.cast_name:
            cast_name_formatted = cast_service.ensure_cast_with_translation(request.cast_name)

        if request.source_type == "en":
            result = translate_title_en(llm, request.title_en, cast_name_formatted, request.cast)
        elif request.source_type == "cn":
            result = translate_title_cn(llm, code, request.title_en, tags, cast_name_formatted) if code else None
        else:
            result = translate_title_klon8(llm, code, request.title_en, tags, cast_name_formatted) if code else None

        if result:
            return TitleResponse(success=True, data={
                "title_en": request.title_en,
                "title_th": result,
                "source_type": request.source_type,
                "cast_formatted": cast_name_formatted,
            })
        return TitleResponse(success=False, data={"title_en": request.title_en}, error="Translation failed")

    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
