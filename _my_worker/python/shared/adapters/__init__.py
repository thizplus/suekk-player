"""
Shared adapters for workers.

Port/Adapter pattern:
- Ports (interfaces):  shared/ports/  — define WHAT the system needs
- Adapters (impl):     shared/adapters/ — define HOW it's done

Adapters implement Ports:
- WhisperAdapter  implements STTPort   (Speech-to-Text)
- GeminiLLM       implements LLMPort   (Large Language Model)
- SileroVADAdapter implements VADPort  (Voice Activity Detection)
- AudioAdapter    implements AudioPort (Audio Processing)

To swap models, use Factory:
    from shared.adapters.llm_factory import create_llm
    llm = create_llm("gemini")  # or "openai", "local"
"""

from .entities import (
    SubtitleLine,
    LanguageCode,
    VoiceSegment,
    TranscriptionResult,
    SpeakerSegment,
    DiarizationResult,
)
from .whisper_adapter import WhisperAdapter
from .gemini_adapter import GeminiAdapter  # Legacy — use LLMPort + create_llm() instead
from .vad_adapter import SileroVADAdapter
from .audio_adapter import AudioAdapter

__all__ = [
    # Entities
    "SubtitleLine",
    "LanguageCode",
    "VoiceSegment",
    "TranscriptionResult",
    "SpeakerSegment",
    "DiarizationResult",
    # Adapters (implement Ports)
    "WhisperAdapter",    # implements STTPort
    "GeminiAdapter",     # Legacy — use create_llm() instead
    "SileroVADAdapter",  # implements VADPort
    "AudioAdapter",      # implements AudioPort
]
