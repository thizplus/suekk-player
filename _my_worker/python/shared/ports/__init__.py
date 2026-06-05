"""
Port interfaces for AI adapters.

Ports define WHAT the system needs (interface).
Adapters define HOW it's done (implementation).

Usage:
    from shared.ports import LLMPort, STTPort, VADPort, AudioPort
"""

from .llm_port import LLMPort, LLMResponse
from .stt_port import STTPort
from .vad_port import VADPort
from .audio_port import AudioPort

__all__ = [
    "LLMPort", "LLMResponse",
    "STTPort",
    "VADPort",
    "AudioPort",
]
