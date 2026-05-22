"""
Shared adapters for subtitle workers.

Provides adapters for:
- Whisper (STT)
- Gemini (LLM)
- Silero VAD
- Demucs (Audio processing)

Provides domain entities for:
- SubtitleLine - Single subtitle line with timing
- LanguageCode - Language code wrapper
- VoiceSegment - VAD segment
- TranscriptionResult - Result of transcription
- SpeakerSegment - Speaker diarization segment
- DiarizationResult - Speaker diarization result
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
from .gemini_adapter import GeminiAdapter
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
    # Adapters
    "WhisperAdapter",
    "GeminiAdapter",
    "SileroVADAdapter",
    "AudioAdapter",
]
