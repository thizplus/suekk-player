"""
LLM Factory — สร้าง LLM adapter จาก provider name.

Usage:
    from shared.adapters.llm_factory import create_llm

    # ใช้ default จาก env LLM_PROVIDER
    llm = create_llm()

    # ระบุ provider
    llm = create_llm("gemini")
    llm = create_llm("openai")

    # แต่ละ service เลือกคนละตัว
    llm = create_llm(os.getenv("SUBTITLE_LLM_PROVIDER"))
    llm = create_llm(os.getenv("TITLE_LLM_PROVIDER"))
"""

import os
import logging
from typing import Optional

from shared.ports.llm_port import LLMPort

logger = logging.getLogger(__name__)


def create_llm(provider: Optional[str] = None, **kwargs) -> LLMPort:
    """
    สร้าง LLM adapter จาก provider name.

    Args:
        provider: "gemini", "openai", "local" (or from env LLM_PROVIDER)
        **kwargs: Override config (api_key, model)

    Returns:
        LLMPort implementation
    """
    provider = provider or os.getenv("LLM_PROVIDER", "gemini")

    if provider == "gemini":
        from shared.adapters.gemini_llm import GeminiLLM
        return GeminiLLM(
            api_key=kwargs.get("api_key") or os.getenv("GEMINI_API_KEY"),
            model=kwargs.get("model") or os.getenv("GEMINI_MODEL", "gemini-2.5-flash"),
        )
    # elif provider == "openai":
    #     from shared.adapters.openai_llm import OpenAILLM
    #     return OpenAILLM(
    #         api_key=kwargs.get("api_key") or os.getenv("OPENAI_API_KEY"),
    #         model=kwargs.get("model") or os.getenv("OPENAI_MODEL", "gpt-4o-mini"),
    #     )
    else:
        raise ValueError(f"Unknown LLM provider: {provider}. Supported: gemini")
