"""
Query translation routes — translate Thai search queries to English.
Uses LLMPort.
"""
from fastapi import APIRouter, HTTPException
from pydantic import BaseModel
from typing import Optional

from title_translate.prompts import translate_query
from title_translate.dependencies import get_llm

router = APIRouter()


class QueryTranslateRequest(BaseModel):
    query: str
    source_lang: str = "th"
    target_lang: str = "en"


class QueryTranslateResponse(BaseModel):
    success: bool
    data: dict
    error: Optional[str] = None


@router.post("/translate", response_model=QueryTranslateResponse)
async def translate_query_endpoint(request: QueryTranslateRequest):
    """Translate search query from Thai to English"""
    try:
        if request.source_lang == "en":
            return QueryTranslateResponse(success=True, data={
                "original": request.query, "translated": request.query,
                "source_lang": "en", "target_lang": "en",
            })

        llm = get_llm()
        translated = translate_query(llm, request.query)

        return QueryTranslateResponse(success=True, data={
            "original": request.query, "translated": translated,
            "source_lang": request.source_lang, "target_lang": request.target_lang,
        })
    except Exception as e:
        return QueryTranslateResponse(success=False, data={
            "original": request.query, "translated": request.query,
        }, error=str(e))


@router.get("/detect-lang")
async def detect_language(query: str):
    """Detect if query is Thai or English"""
    has_thai = any('\u0e00' <= char <= '\u0e7f' for char in query)
    return {"query": query, "lang": "th" if has_thai else "en", "has_thai_chars": has_thai}
