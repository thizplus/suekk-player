"""
AudioAdapter - Audio processing using Demucs and FFmpeg
"""

import logging
import shutil
import subprocess
from pathlib import Path
from typing import Optional

from ..ports.audio_port import AudioPort

logger = logging.getLogger(__name__)



class AudioAdapter(AudioPort):
    """
    Audio processing adapter using Demucs and FFmpeg.

    Provides:
    - Voice separation (Demucs)
    - Audio extraction (FFmpeg)
    - Segment cutting (FFmpeg)

    Usage:
        audio = AudioAdapter()
        audio.separate_vocals(audio_path, output_path)
        audio.cut_segment(audio_path, output_path, start, end)
    """

    def __init__(self, temp_dir: Optional[Path] = None):
        """
        Initialize audio adapter.

        Args:
            temp_dir: Temporary directory for processing
        """
        self._temp_dir = temp_dir or Path("/tmp/audio")
        self._temp_dir.mkdir(parents=True, exist_ok=True)

    def separate_vocals(
        self,
        audio_path: Path,
        output_path: Path,
        model: str = "htdemucs",
    ) -> bool:
        """
        Separate vocals from background using Demucs.

        Args:
            audio_path: Input audio file
            output_path: Output vocals file (WAV, 16kHz mono)
            model: Demucs model to use

        Returns:
            True if successful
        """
        try:
            # Create temp output dir
            demucs_out = self._temp_dir / "demucs_output"
            demucs_out.mkdir(exist_ok=True)

            # Run demucs
            cmd = [
                "demucs",
                "-d", "cuda",
                "-n", model,
                "--two-stems", "vocals",
                "-o", str(demucs_out),
                str(audio_path),
            ]

            logger.info(f"Running Demucs: {' '.join(cmd)}")

            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=1800,  # 30 min — GPU ใช้ ~3-5 นาที แต่เผื่อ audio ยาวมาก
            )

            if result.returncode != 0:
                logger.error(f"Demucs failed: {result.stderr}")
                return False

            # Find output vocals file
            stem_name = audio_path.stem
            vocals_path = demucs_out / model / stem_name / "vocals.wav"

            if not vocals_path.exists():
                logger.error(f"Demucs output not found: {vocals_path}")
                return False

            # Convert to 16kHz mono
            return self._convert_audio(vocals_path, output_path, 16000, 1)

        except subprocess.TimeoutExpired:
            logger.error("Demucs timeout")
            return False
        except Exception as e:
            logger.error(f"Demucs error: {e}")
            return False

    def extract_audio(
        self,
        video_path: Path,
        output_path: Path,
        sample_rate: int = 16000,
        channels: int = 1,
    ) -> bool:
        """
        Extract audio from video file.

        Args:
            video_path: Input video file
            output_path: Output audio file
            sample_rate: Target sample rate
            channels: Number of channels

        Returns:
            True if successful
        """
        return self._convert_audio(video_path, output_path, sample_rate, channels)

    def cut_segment(
        self,
        audio_path: Path,
        output_path: Path,
        start_sec: float,
        end_sec: float,
    ) -> bool:
        """
        Cut a segment from audio file.

        Args:
            audio_path: Input audio file
            output_path: Output segment file
            start_sec: Start time in seconds
            end_sec: End time in seconds

        Returns:
            True if successful
        """
        try:
            duration = end_sec - start_sec

            cmd = [
                "ffmpeg",
                "-y",
                "-ss", str(start_sec),
                "-i", str(audio_path),
                "-t", str(duration),
                "-c:a", "pcm_s16le",
                "-ar", "16000",
                "-ac", "1",
                str(output_path),
            ]

            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=60,
            )

            if result.returncode != 0:
                logger.error(f"FFmpeg cut failed: {result.stderr}")
                return False

            return output_path.exists()

        except Exception as e:
            logger.error(f"Cut segment error: {e}")
            return False

    def get_duration(self, audio_path: Path) -> Optional[float]:
        """
        Get duration of audio file in seconds.

        Args:
            audio_path: Path to audio file

        Returns:
            Duration in seconds, or None if failed
        """
        try:
            cmd = [
                "ffprobe",
                "-v", "error",
                "-show_entries", "format=duration",
                "-of", "default=noprint_wrappers=1:nokey=1",
                str(audio_path),
            ]

            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=30,
            )

            if result.returncode == 0:
                return float(result.stdout.strip())
            return None

        except Exception as e:
            logger.error(f"Get duration error: {e}")
            return None

    def _convert_audio(
        self,
        input_path: Path,
        output_path: Path,
        sample_rate: int,
        channels: int,
    ) -> bool:
        """Convert audio to specified format"""
        try:
            cmd = [
                "ffmpeg",
                "-y",
                "-i", str(input_path),
                "-ar", str(sample_rate),
                "-ac", str(channels),
                "-c:a", "pcm_s16le",
                str(output_path),
            ]

            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=300,
            )

            if result.returncode != 0:
                logger.error(f"FFmpeg convert failed: {result.stderr}")
                return False

            return output_path.exists()

        except Exception as e:
            logger.error(f"Convert audio error: {e}")
            return False

    def is_healthy(self) -> bool:
        """Check if FFmpeg and Demucs are available"""
        ffmpeg_ok = shutil.which("ffmpeg") is not None
        demucs_ok = shutil.which("demucs") is not None

        if not ffmpeg_ok:
            logger.warning("FFmpeg not found")
        if not demucs_ok:
            logger.warning("Demucs not found")

        return ffmpeg_ok and demucs_ok
