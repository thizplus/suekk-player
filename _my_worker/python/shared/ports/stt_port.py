"""
STT Port — Interface for Speech-to-Text models.

Implementations:
    - WhisperAdapter (shared/adapters/whisper_adapter.py) — faster-whisper
    - Future: whisper.cpp, Google STT, Azure STT, etc.
"""

from abc import ABC, abstractmethod
from pathlib import Path
from typing import List, Tuple, Optional, Callable

from shared.entities import SubtitleLine, LanguageCode


class STTPort(ABC):
    """
    Interface for Speech-to-Text providers.

    เปลี่ยน STT model ได้โดยไม่แก้ business logic:
        stt = WhisperAdapter(model="turbo", device="cuda")
        subtitles = stt.transcribe(audio_path, language)
    """

    @abstractmethod
    def transcribe(
        self,
        audio_path: Path,
        language: LanguageCode,
        progress_callback: Optional[Callable[[int, int], None]] = None,
    ) -> List[SubtitleLine]:
        """Transcribe audio file to subtitle lines"""
        pass

    @abstractmethod
    def transcribe_segment(
        self,
        audio_path: Path,
        language: LanguageCode,
        no_speech_threshold: float = 0.5,
    ) -> Tuple[str, bool]:
        """Transcribe a single short audio segment (for gap re-transcription)"""
        pass

    @abstractmethod
    def detect_language(self, audio_path: Path) -> Tuple[LanguageCode, float]:
        """Detect language from audio, returns (language, confidence)"""
        pass

    @abstractmethod
    def get_model_name(self) -> str:
        """Get current model name"""
        pass
