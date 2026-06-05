"""
Title Translation Service — FastAPI Entry Point

Usage:
    cd _my_worker/python
    python -m title_translate.main

Uses LLMPort (Port/Adapter) — เปลี่ยน LLM provider ได้จาก env:
    TITLE_LLM_PROVIDER=gemini   (default)
    TITLE_LLM_PROVIDER=openai   (backup)
"""
import uvicorn
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from title_translate.config import settings
from title_translate.routes import klon8, translate, query

app = FastAPI(
    title="Title Translation API",
    description="Translation service using LLMPort (Gemini/OpenAI/etc.)",
    version="2.0.0",
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Routes
app.include_router(klon8.router, prefix="/api/klon8", tags=["klon8"])
app.include_router(translate.router, prefix="/api/translate", tags=["translate"])
app.include_router(query.router, prefix="/api/query", tags=["query"])


@app.get("/")
async def root():
    from title_translate.dependencies import get_llm
    llm = get_llm()
    return {
        "service": "Title Translation API",
        "version": "2.0.0",
        "llm_provider": llm.get_provider_name(),
        "llm_model": llm.get_model_name(),
    }


@app.get("/health")
async def health():
    return {"status": "healthy"}


if __name__ == "__main__":
    uvicorn.run(
        "title_translate.main:app",
        host=settings.host,
        port=settings.port,
        reload=settings.debug,
    )
