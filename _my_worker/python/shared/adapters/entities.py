"""
DEPRECATED: Entities moved to shared/entities.py
This file re-exports for backward compatibility. Import from shared.entities instead.
"""

from shared.entities import (
    SubtitleLine,
    LanguageCode,
    VoiceSegment,
    TranscriptionResult,
    SpeakerSegment,
    DiarizationResult,
)

__all__ = [
    "SubtitleLine",
    "LanguageCode",
    "VoiceSegment",
    "TranscriptionResult",
    "SpeakerSegment",
    "DiarizationResult",
]
