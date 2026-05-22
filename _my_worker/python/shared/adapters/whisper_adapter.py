"""
WhisperAdapter - Speech-to-text using faster-whisper

Supports multiple models:
- turbo: Main transcription (fast + quality)
- kotoba: Japanese gap re-transcription (less hallucination)
- large-v3: Other language gap re-transcription
"""

import re
import logging
from pathlib import Path
from typing import List, Tuple, Optional, Callable

from .entities import SubtitleLine, LanguageCode, TranscriptionResult
from ..config import WHISPER_MODELS, HALLUCINATION_PATTERNS

logger = logging.getLogger(__name__)


class WhisperAdapter:
    """
    Whisper Speech-to-Text adapter.

    Usage:
        # Main transcription
        adapter = WhisperAdapter(model="turbo", device="cuda")
        subtitles = adapter.transcribe(audio_path, LanguageCode("ja"))

        # Gap re-transcription (use kotoba for Japanese)
        gap_adapter = WhisperAdapter(model="kotoba", device="cuda")
        text, is_valid = gap_adapter.transcribe_segment(gap_audio, LanguageCode("ja"))
    """

    def __init__(
        self,
        model: str = "turbo",
        device: str = "cuda",
        vad_filter: bool = True
    ):
        """
        Initialize Whisper adapter.

        Args:
            model: Model key (turbo, kotoba, large-v3)
            device: Device to use (cuda, cpu)
            vad_filter: Enable VAD filtering
        """
        self._model_key = model
        self._device = device
        self._vad_filter = vad_filter
        self._model = None

        # Get model config
        self._config = WHISPER_MODELS.get(model, WHISPER_MODELS["turbo"])
        self._model_name = self._config["name"]
        self._compute_type = self._config["compute_type"]
        self._word_timestamps = self._config.get("word_timestamps", True)

        logger.info(f"WhisperAdapter initialized: model={model}, device={device}")

    def _load_model(self):
        """Lazy load model"""
        if self._model is None:
            from faster_whisper import WhisperModel

            logger.info(f"Loading Whisper model: {self._model_name}")

            try:
                self._model = WhisperModel(
                    self._model_name,
                    device=self._device,
                    compute_type=self._compute_type,
                )
                logger.info(f"Model loaded on {self._device}")
            except Exception as e:
                logger.warning(f"GPU failed ({e}), falling back to CPU")
                self._model = WhisperModel(
                    self._model_name,
                    device="cpu",
                    compute_type="int8",
                )

        return self._model

    def transcribe(
        self,
        audio_path: Path,
        language: LanguageCode,
        progress_callback: Optional[Callable[[int, int], None]] = None,
    ) -> List[SubtitleLine]:
        """
        Transcribe audio file to subtitles.

        Args:
            audio_path: Path to audio file
            language: Target language
            progress_callback: Optional progress callback(current, total)

        Returns:
            List of SubtitleLine objects
        """
        model = self._load_model()

        # Build options
        options = {
            "language": language.code,
            "vad_filter": self._vad_filter,
            "vad_parameters": {"min_silence_duration_ms": 500},
            "condition_on_previous_text": False,
            "word_timestamps": self._word_timestamps,
            "no_speech_threshold": 0.3,
            "temperature": 0,
        }

        logger.info(f"Transcribing: {audio_path} (lang={language.code})")

        segments, info = model.transcribe(str(audio_path), **options)

        subtitles = []
        for i, seg in enumerate(segments):
            text = seg.text.strip()

            # Skip hallucinations
            if self._is_hallucination(text):
                continue

            subtitles.append(SubtitleLine(
                index=i,
                text=text,
                start_ms=int(seg.start * 1000),
                end_ms=int(seg.end * 1000),
                confidence=getattr(seg, 'avg_logprob', None),
            ))

        logger.info(f"Transcription complete: {len(subtitles)} segments")
        return subtitles

    def transcribe_segment(
        self,
        audio_path: Path,
        language: LanguageCode,
        no_speech_threshold: float = 0.5,
    ) -> Tuple[str, bool]:
        """
        Transcribe a single audio segment (for gap re-transcription).

        Uses stricter filtering for short segments to avoid hallucination.

        Args:
            audio_path: Path to audio segment
            language: Language code
            no_speech_threshold: Filter if no_speech_prob > this

        Returns:
            Tuple of (text, is_valid)
        """
        model = self._load_model()

        segments, info = model.transcribe(
            str(audio_path),
            language=language.code,
            vad_filter=False,  # Don't filter - check confidence
            condition_on_previous_text=False,
        )

        seg_list = list(segments)

        if seg_list:
            text = " ".join([s.text.strip() for s in seg_list])
            no_speech_prob = sum(s.no_speech_prob for s in seg_list) / len(seg_list)
        else:
            text = ""
            no_speech_prob = 1.0

        # Filter by no_speech probability
        if no_speech_prob > no_speech_threshold:
            return "", False

        # Filter hallucinations
        if self._is_hallucination(text):
            return "", False

        return text, True

    def detect_language(self, audio_path: Path) -> Tuple[LanguageCode, float]:
        """
        Detect language from audio file.

        Args:
            audio_path: Path to audio file

        Returns:
            Tuple of (LanguageCode, confidence)
        """
        model = self._load_model()

        segments, info = model.transcribe(
            str(audio_path),
            language=None,  # Auto-detect
            vad_filter=True,
            vad_parameters={"min_silence_duration_ms": 500},
        )

        # Consume iterator
        _ = list(segments)

        lang_code = info.language
        confidence = info.language_probability

        logger.info(f"Detected language: {lang_code} ({confidence:.2%})")

        return LanguageCode.detect_or_default(lang_code), confidence

    def _is_hallucination(self, text: str) -> bool:
        """Check if text matches hallucination patterns"""
        if not text:
            return False

        for pattern in HALLUCINATION_PATTERNS:
            if re.search(pattern, text):
                logger.debug(f"Hallucination filtered: {text[:50]}")
                return True

        return False

    def release(self) -> None:
        """Release model from memory"""
        # Note: faster-whisper destructor can crash, keep model in memory
        logger.info("Whisper model kept in memory (skip release)")

    def is_healthy(self) -> bool:
        """Check if Whisper is available"""
        try:
            self._load_model()
            return True
        except Exception:
            return False

    def get_model_name(self) -> str:
        """Get model name"""
        return self._model_key
