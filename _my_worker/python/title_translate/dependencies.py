"""
Shared dependencies for title_translate service.
LLM singleton — เปลี่ยน provider จาก env TITLE_LLM_PROVIDER
"""
import os
import logging

from shared.ports.llm_port import LLMPort
from shared.adapters.llm_factory import create_llm

logger = logging.getLogger(__name__)

_llm_instance = None


def get_llm() -> LLMPort:
    """Get LLM singleton for title translation"""
    global _llm_instance
    if _llm_instance is None:
        provider = os.getenv("TITLE_LLM_PROVIDER") or os.getenv("LLM_PROVIDER")
        _llm_instance = create_llm(provider)
        logger.info(f"Title LLM loaded: {_llm_instance.get_provider_name()} / {_llm_instance.get_model_name()}")
    return _llm_instance
