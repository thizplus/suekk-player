"""
LLM Port — Interface for Large Language Models.

ทำแค่ "ส่ง prompt รับ text" ไม่มี business logic (prompt building / response parsing)
Business logic อยู่ใน handler ของแต่ละ service

Implementations:
    - GeminiLLM (shared/adapters/gemini_llm.py)
    - OpenAILLM (shared/adapters/openai_llm.py) — future
    - LocalLLM  (shared/adapters/local_llm.py)  — future (Ollama/vLLM)
"""

from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import Optional


@dataclass
class LLMResponse:
    """Response from LLM"""
    text: str
    success: bool
    error: Optional[str] = None
    model: Optional[str] = None


class LLMPort(ABC):
    """
    Interface for LLM providers.

    เปลี่ยน provider ได้โดยไม่แก้ business logic:
        llm = create_llm("gemini")   # or "openai", "local"
        response = llm.generate(prompt)
    """

    @abstractmethod
    def generate(self, prompt: str) -> LLMResponse:
        """Send prompt to LLM and get response text"""
        pass

    @abstractmethod
    def generate_unsafe(self, prompt: str) -> LLMResponse:
        """Send prompt with safety filters disabled (for adult content)"""
        pass

    @abstractmethod
    def get_model_name(self) -> str:
        """Get current model name"""
        pass

    @abstractmethod
    def get_provider_name(self) -> str:
        """Get provider name (gemini, openai, local)"""
        pass
