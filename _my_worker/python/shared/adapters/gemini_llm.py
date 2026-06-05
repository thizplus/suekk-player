"""
Gemini LLM Adapter — implements LLMPort.

ทำแค่ส่ง prompt และรับ text กลับ ไม่มี business logic
"""

import os
import logging
from typing import Optional

from shared.ports.llm_port import LLMPort, LLMResponse

logger = logging.getLogger(__name__)


class GeminiLLM(LLMPort):
    """
    Google Gemini implementation of LLMPort.

    Usage:
        llm = GeminiLLM(api_key="xxx", model="gemini-2.5-flash")
        response = llm.generate("Translate this...")
    """

    def __init__(
        self,
        api_key: Optional[str] = None,
        model: Optional[str] = None,
    ):
        self._api_key = api_key or os.getenv("GEMINI_API_KEY", "")
        self._model_name = model or os.getenv("GEMINI_MODEL", "gemini-2.5-flash")
        self._client = None
        self._client_unsafe = None

        if not self._api_key:
            logger.warning("GEMINI_API_KEY not set")

    def _get_client(self):
        """Lazy load Gemini client (with default safety)"""
        if self._client is None:
            import google.generativeai as genai
            genai.configure(api_key=self._api_key)
            self._client = genai.GenerativeModel(self._model_name)
            logger.info(f"Gemini client initialized: {self._model_name}")
        return self._client

    def _get_client_unsafe(self):
        """Lazy load Gemini client with safety filters disabled"""
        if self._client_unsafe is None:
            import google.generativeai as genai
            genai.configure(api_key=self._api_key)
            safety_settings = [
                {"category": "HARM_CATEGORY_SEXUALLY_EXPLICIT", "threshold": "BLOCK_NONE"},
                {"category": "HARM_CATEGORY_HARASSMENT", "threshold": "BLOCK_NONE"},
                {"category": "HARM_CATEGORY_DANGEROUS_CONTENT", "threshold": "BLOCK_NONE"},
                {"category": "HARM_CATEGORY_HATE_SPEECH", "threshold": "BLOCK_NONE"},
            ]
            self._client_unsafe = genai.GenerativeModel(
                self._model_name,
                safety_settings=safety_settings,
            )
            logger.info(f"Gemini client (unsafe) initialized: {self._model_name}")
        return self._client_unsafe

    def generate(self, prompt: str) -> LLMResponse:
        """Send prompt to Gemini with default safety settings"""
        client = self._get_client()
        try:
            response = client.generate_content(prompt)
            if not response.candidates:
                return LLMResponse(text="", success=False, error="Content blocked by safety filter", model=self._model_name)
            return LLMResponse(text=response.text.strip(), success=True, model=self._model_name)
        except Exception as e:
            logger.error(f"Gemini generate failed: {e}")
            return LLMResponse(text="", success=False, error=str(e), model=self._model_name)

    def generate_unsafe(self, prompt: str) -> LLMResponse:
        """Send prompt with safety filters disabled, fallback to normal if blocked"""
        # Try unsafe client first
        client = self._get_client_unsafe()
        try:
            response = client.generate_content(prompt)
            if not response.candidates:
                # Fallback to normal client
                logger.warning("Unsafe client blocked, falling back to normal client")
                return self.generate(prompt)
            return LLMResponse(text=response.text.strip(), success=True, model=self._model_name)
        except Exception as e:
            # Fallback to normal client on any error
            logger.warning(f"Unsafe client failed ({e}), falling back to normal client")
            return self.generate(prompt)

    def get_model_name(self) -> str:
        return self._model_name

    def get_provider_name(self) -> str:
        return "gemini"
