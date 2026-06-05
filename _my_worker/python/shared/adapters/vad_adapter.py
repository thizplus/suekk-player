"""
SileroVADAdapter - Voice Activity Detection using Silero VAD
"""

import logging
from pathlib import Path
from typing import List, Optional

from .entities import VoiceSegment
from ..ports.vad_port import VADPort

logger = logging.getLogger(__name__)


class SileroVADAdapter(VADPort):
    """
    Silero VAD adapter for detecting voice activity in audio.

    Usage:
        vad = SileroVADAdapter()
        segments = vad.detect_voice(audio_path)
    """

    def __init__(self):
        """Initialize Silero VAD adapter"""
        self._model = None
        self._utils = None

    def _load_model(self):
        """Lazy load Silero VAD model"""
        if self._model is None:
            import torch

            logger.info("Loading Silero VAD model...")

            model, utils = torch.hub.load(
                repo_or_dir='snakers4/silero-vad',
                model='silero_vad',
                force_reload=False,
                onnx=False,
            )

            self._model = model
            self._utils = utils

            logger.info("Silero VAD model loaded")

        return self._model, self._utils

    def detect_voice(
        self,
        audio_path: Path,
        threshold: float = 0.5,
        min_speech_duration_ms: int = 250,
        min_silence_duration_ms: int = 100,
        sampling_rate: int = 16000,
    ) -> List[VoiceSegment]:
        """
        Detect voice segments in audio file.

        Args:
            audio_path: Path to audio file
            threshold: VAD threshold (0.0-1.0)
            min_speech_duration_ms: Minimum speech duration
            min_silence_duration_ms: Minimum silence duration
            sampling_rate: Expected sample rate

        Returns:
            List of VoiceSegment objects
        """
        model, utils = self._load_model()

        # Get utility functions
        get_speech_timestamps, _, read_audio, _, _ = utils

        # Read audio
        wav = read_audio(str(audio_path), sampling_rate=sampling_rate)

        # Get speech timestamps
        speech_timestamps = get_speech_timestamps(
            wav,
            model,
            threshold=threshold,
            min_speech_duration_ms=min_speech_duration_ms,
            min_silence_duration_ms=min_silence_duration_ms,
            sampling_rate=sampling_rate,
        )

        # Convert to VoiceSegment
        segments = []
        for ts in speech_timestamps:
            start_sec = ts['start'] / sampling_rate
            end_sec = ts['end'] / sampling_rate

            segments.append(VoiceSegment(
                start_sec=start_sec,
                end_sec=end_sec,
                confidence=1.0,
            ))

        logger.info(f"VAD detected {len(segments)} voice segments")
        return segments

    def find_gaps(
        self,
        vad_segments: List[VoiceSegment],
        subtitles: List,
        min_gap_duration: float = 0.5,
    ) -> List[dict]:
        """
        Find speech gaps not covered by subtitles.

        A gap is where VAD detected speech but no subtitle covers it.

        Args:
            vad_segments: VAD voice segments
            subtitles: Subtitle lines (with start_sec, end_sec)
            min_gap_duration: Minimum gap duration in seconds

        Returns:
            List of gap dicts with start, end
        """
        gaps = []

        for vad in vad_segments:
            vad_start = vad.start_sec
            vad_end = vad.end_sec

            # Check if covered by any subtitle
            covered = False
            for sub in subtitles:
                sub_start = sub.start_sec if hasattr(sub, 'start_sec') else sub.get('start', 0)
                sub_end = sub.end_sec if hasattr(sub, 'end_sec') else sub.get('end', 0)

                # Check overlap (any intersection)
                if sub_start <= vad_end and sub_end >= vad_start:
                    covered = True
                    break

            if not covered and (vad_end - vad_start) >= min_gap_duration:
                gaps.append({
                    'start': vad_start,
                    'end': vad_end,
                })

        logger.info(f"Found {len(gaps)} uncovered gaps")
        return gaps

    def is_healthy(self) -> bool:
        """Check if Silero VAD is available"""
        try:
            self._load_model()
            return True
        except Exception as e:
            logger.warning(f"Silero VAD health check failed: {e}")
            return False

    def release(self) -> None:
        """Release model from memory"""
        if self._model is not None:
            import torch

            del self._model
            del self._utils
            self._model = None
            self._utils = None

            if torch.cuda.is_available():
                torch.cuda.empty_cache()

            logger.info("Silero VAD model released")
