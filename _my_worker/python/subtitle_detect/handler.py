"""
Handler for subtitle detection jobs.

Uses WhisperAdapter (faster-whisper) for language detection.
"""

import logging
import time
from pathlib import Path

from shared.config import Config
from shared.nats_consumer import Job, JobMeta
from shared.progress import ProgressPublisher
from shared.storage import S3Client
from shared.adapters import WhisperAdapter

logger = logging.getLogger(__name__)

# Lazy loaded adapter
_whisper_adapter = None


def get_whisper_adapter(model: str = "turbo", device: str = "cuda") -> WhisperAdapter:
    """Lazy load WhisperAdapter"""
    global _whisper_adapter
    if _whisper_adapter is None:
        logger.info(f"Loading WhisperAdapter: model={model}, device={device}")
        _whisper_adapter = WhisperAdapter(model=model, device=device)
        logger.info("WhisperAdapter loaded")
    return _whisper_adapter


# Stages
STAGE_INITIALIZING = "initializing"
STAGE_DOWNLOADING = "downloading"
STAGE_DETECTING = "detecting"
STAGE_COMPLETED = "completed"
STAGE_FAILED = "failed"


class SubtitleDetectHandler:
    """Handles subtitle detection jobs"""

    def __init__(
        self,
        config: Config,
        storage: S3Client,
        progress: ProgressPublisher,
    ):
        self.config = config
        self.storage = storage
        self.progress = progress

    async def handle(self, job: Job) -> dict:
        """
        Process a subtitle detection job.

        Input:
            audio_path: S3 path to audio.wav
            output_path: S3 path prefix for subtitles

        Output:
            language: Detected language code (e.g., "en", "th", "ja")
            confidence: Detection confidence
        """
        start_time = time.time()
        meta = job.meta
        input_data = job.input

        audio_path = input_data.get("audio_path", "")
        output_path = input_data.get("output_path", "")

        logger.info(f"Starting subtitle detection: {meta.job_id}")
        logger.info(f"  Audio: {audio_path}")
        logger.info(f"  Output: {output_path}")

        # Create temp directory
        job_dir = Path(self.config.temp_dir) / meta.job_id
        job_dir.mkdir(parents=True, exist_ok=True)

        try:
            # 1. Initialize
            await self._publish_progress(meta, STAGE_INITIALIZING, 0, "เริ่มตรวจจับภาษา")

            # 2. Download audio
            await self._publish_progress(meta, STAGE_DOWNLOADING, 10, "กำลังดาวน์โหลดเสียง...")

            local_audio = job_dir / "audio.wav"
            self.storage.download(audio_path, str(local_audio))

            logger.info(f"Audio downloaded: {local_audio}")

            # 3. Detect language
            await self._publish_progress(meta, STAGE_DETECTING, 30, "กำลังตรวจจับภาษา...")

            language, confidence = self._detect_language(str(local_audio))

            logger.info(f"Language detected: {language} (confidence: {confidence:.2f})")

            await self._publish_progress(meta, STAGE_DETECTING, 70, f"ตรวจพบภาษา: {language}")

            # 4. Complete
            duration_sec = time.time() - start_time
            output = {
                "language": language,
                "confidence": confidence,
            }

            await self.progress.publish_completed(
                job_id=meta.job_id,
                job_type=meta.job_type,
                entity_type=meta.entity_type,
                entity_id=meta.entity_id,
                entity_code=meta.entity_code,
                duration_sec=duration_sec,
                output=output,
            )

            logger.info(f"Subtitle detection completed in {duration_sec:.1f}s")
            return output

        except Exception as e:
            logger.error(f"Subtitle detection failed: {e}")

            await self.progress.publish_failed(
                job_id=meta.job_id,
                job_type=meta.job_type,
                entity_type=meta.entity_type,
                entity_id=meta.entity_id,
                entity_code=meta.entity_code,
                error=str(e),
                stage=STAGE_FAILED,
                error_code="SUBTITLE_DETECT_FAILED",
            )

            raise

        finally:
            # Cleanup
            self._cleanup(job_dir)

    def _detect_language(self, audio_path: str) -> tuple[str, float]:
        """
        Detect language from audio using faster-whisper.

        Returns:
            Tuple of (language_code, confidence)
        """
        try:
            adapter = get_whisper_adapter(
                model=self.config.whisper_model,
                device=self.config.whisper_device,
            )

            # Detect language using adapter
            lang_code, confidence = adapter.detect_language(Path(audio_path))

            return lang_code.code, confidence

        except Exception as e:
            logger.error(f"Language detection error: {e}")
            # Fallback to Japanese (most common in this system)
            return "ja", 0.0

    async def _publish_progress(
        self,
        meta: JobMeta,
        stage: str,
        progress: float,
        message: str,
    ):
        """Publish progress update"""
        await self.progress.publish_processing(
            job_id=meta.job_id,
            job_type=meta.job_type,
            entity_type=meta.entity_type,
            entity_id=meta.entity_id,
            entity_code=meta.entity_code,
            stage=stage,
            progress=progress,
            message=message,
        )

    def _cleanup(self, job_dir: Path):
        """Remove temporary files"""
        import shutil
        try:
            if job_dir.exists():
                shutil.rmtree(job_dir)
                logger.debug(f"Cleaned up: {job_dir}")
        except Exception as e:
            logger.warning(f"Cleanup failed: {e}")
