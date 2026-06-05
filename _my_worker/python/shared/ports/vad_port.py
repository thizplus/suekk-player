"""
VAD Port — Interface for Voice Activity Detection.

Implementations:
    - SileroVADAdapter (shared/adapters/vad_adapter.py) — Silero VAD
    - Future: WebRTC VAD, pyannote, etc.
"""

from abc import ABC, abstractmethod
from pathlib import Path
from typing import List

from shared.entities import VoiceSegment


class VADPort(ABC):
    """
    Interface for Voice Activity Detection providers.

    เปลี่ยน VAD model ได้โดยไม่แก้ business logic.
    """

    @abstractmethod
    def detect_voice(
        self,
        audio_path: Path,
        threshold: float = 0.5,
        min_speech_duration_ms: int = 250,
        min_silence_duration_ms: int = 100,
        sampling_rate: int = 16000,
    ) -> List[VoiceSegment]:
        """Detect voice segments in audio file"""
        pass

    @abstractmethod
    def find_gaps(
        self,
        vad_segments: List[VoiceSegment],
        subtitles: List,
        min_gap_duration: float = 0.5,
    ) -> List[dict]:
        """Find speech gaps not covered by subtitles"""
        pass
