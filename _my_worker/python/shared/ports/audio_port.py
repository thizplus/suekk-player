"""
Audio Port — Interface for Audio Processing.

Implementations:
    - AudioAdapter (shared/adapters/audio_adapter.py) — Demucs + FFmpeg
    - Future: different voice separation models, etc.
"""

from abc import ABC, abstractmethod
from pathlib import Path
from typing import Optional


class AudioPort(ABC):
    """
    Interface for audio processing providers.

    เปลี่ยน audio processor ได้โดยไม่แก้ business logic:
        audio = create_audio_processor("demucs")
        audio.separate_vocals(input_path, output_path)
    """

    @abstractmethod
    def separate_vocals(
        self,
        audio_path: Path,
        output_path: Path,
        model: str = "htdemucs",
    ) -> bool:
        """Separate vocals from background audio"""
        pass

    @abstractmethod
    def cut_segment(
        self,
        audio_path: Path,
        output_path: Path,
        start_sec: float,
        end_sec: float,
    ) -> bool:
        """Cut a time segment from audio file"""
        pass

    @abstractmethod
    def get_duration(self, audio_path: Path) -> Optional[float]:
        """Get duration of audio file in seconds"""
        pass
