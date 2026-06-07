"""
Gemini LLM Adapter — implements LLMPort.

ใช้ google-genai SDK ใหม่ (ไม่ใช่ google-generativeai ที่ deprecated)
รองรับ thinking_budget=0 เพื่อปิด thinking tokens ($3.50/1M)
"""

import os
import logging
from typing import Optional

from shared.ports.llm_port import LLMPort, LLMResponse

logger = logging.getLogger(__name__)


class GeminiLLM(LLMPort):
    """
    Google Gemini implementation of LLMPort.
    ใช้ google.genai SDK (ใหม่) พร้อม thinking_budget=0

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

        if not self._api_key:
            logger.warning("GEMINI_API_KEY not set")

    def _get_client(self):
        """Lazy load genai client"""
        if self._client is None:
            from google import genai
            self._client = genai.Client(api_key=self._api_key)
            logger.info(f"Gemini client initialized: {self._model_name}")
        return self._client

    def generate(self, prompt: str) -> LLMResponse:
        """Send prompt to Gemini with default safety settings, thinking OFF"""
        client = self._get_client()
        try:
            import httpx
            response = client.models.generate_content(
                model=self._model_name,
                contents=prompt,
                config={
                    'thinking_config': {'thinking_budget': 0},
                    'temperature': 0.3,
                    'max_output_tokens': 2048,  # ป้องกัน hallucination ยาว
                    'http_options': {'timeout': 60000},
                },
            )
            if not response.text:
                return LLMResponse(text="", success=False, error="Empty response", model=self._model_name)
            return self._parse_response(response)
        except Exception as e:
            logger.error(f"Gemini generate failed: {e}")
            return LLMResponse(text="", success=False, error=str(e), model=self._model_name)

    def generate_unsafe(self, prompt: str) -> LLMResponse:
        """Send prompt with safety filters disabled, thinking OFF, timeout 60s"""
        client = self._get_client()
        try:
            response = client.models.generate_content(
                model=self._model_name,
                contents=prompt,
                config={
                    'thinking_config': {'thinking_budget': 0},
                    'temperature': 0.3,
                    'max_output_tokens': 2048,  # ป้องกัน hallucination ยาว
                    'safety_settings': [
                        {'category': 'HARM_CATEGORY_SEXUALLY_EXPLICIT', 'threshold': 'BLOCK_NONE'},
                        {'category': 'HARM_CATEGORY_HARASSMENT', 'threshold': 'BLOCK_NONE'},
                        {'category': 'HARM_CATEGORY_DANGEROUS_CONTENT', 'threshold': 'BLOCK_NONE'},
                        {'category': 'HARM_CATEGORY_HATE_SPEECH', 'threshold': 'BLOCK_NONE'},
                    ],
                    'http_options': {'timeout': 60000},
                },
            )
            if not response.text:
                logger.warning("Unsafe response empty, skipping this cluster")
                return LLMResponse(text="", success=False, error="Content blocked", model=self._model_name)
            return self._parse_response(response)
        except Exception as e:
            logger.warning(f"Unsafe generate failed: {e}")
            return LLMResponse(text="", success=False, error=str(e), model=self._model_name)

    def _parse_response(self, response) -> LLMResponse:
        """Parse response + log token usage"""
        usage = response.usage_metadata
        thinking_tokens = getattr(usage, 'thoughts_token_count', 0) or 0
        input_tokens = getattr(usage, 'prompt_token_count', 0) or 0
        output_tokens = getattr(usage, 'candidates_token_count', 0) or 0

        if thinking_tokens > 0:
            logger.warning(f"THINKING ON! tokens: input={input_tokens}, output={output_tokens}, thinking={thinking_tokens} ($$)")
        else:
            logger.info(f"Tokens: input={input_tokens}, output={output_tokens}, thinking=0")

        return LLMResponse(text=response.text.strip(), success=True, model=self._model_name)

    def get_model_name(self) -> str:
        return self._model_name

    def get_provider_name(self) -> str:
        return "gemini"
