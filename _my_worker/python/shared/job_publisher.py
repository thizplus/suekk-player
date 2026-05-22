"""
Job publisher for publishing downstream jobs to other workers.
"""

import json
import logging
import uuid
from dataclasses import dataclass
from datetime import datetime
from typing import Any, Optional

import nats
from nats.js import JetStreamContext

logger = logging.getLogger(__name__)


@dataclass
class JobMeta:
    """Standard job metadata"""
    job_id: str
    job_type: str
    entity_type: str
    entity_id: str
    entity_code: str
    priority: int = 1
    retry_count: int = 0
    max_retries: int = 3
    timeout_sec: int = 600
    created_at: int = 0

    def to_dict(self) -> dict:
        return {
            "job_id": self.job_id,
            "job_type": self.job_type,
            "entity_type": self.entity_type,
            "entity_id": self.entity_id,
            "entity_code": self.entity_code,
            "priority": self.priority,
            "retry_count": self.retry_count,
            "max_retries": self.max_retries,
            "timeout_sec": self.timeout_sec,
            "created_at": self.created_at or int(datetime.utcnow().timestamp()),
        }


class JobPublisher:
    """Publishes jobs to other workers via JetStream"""

    def __init__(self, js: JetStreamContext):
        self.js = js

    async def publish_job(
        self,
        subject: str,
        meta: JobMeta,
        input_data: dict,
    ) -> str:
        """
        Publish a job to a subject.

        Args:
            subject: NATS subject (e.g., jobs.subtitle.transcribe)
            meta: Job metadata
            input_data: Job-specific input

        Returns:
            Published message sequence number
        """
        job = {
            "_meta": meta.to_dict(),
            "input": input_data,
        }

        data = json.dumps(job).encode()

        # Publish via JetStream
        ack = await self.js.publish(subject, data)

        logger.info(f"Published job: {subject} -> seq={ack.seq}")
        return str(ack.seq)

    async def publish_subtitle_transcribe(
        self,
        parent_meta: JobMeta,
        audio_path: str,
        language: str,
        output_path: str,
    ) -> str:
        """
        Publish a subtitle_transcribe job.

        Args:
            parent_meta: Metadata from parent job (for entity info)
            audio_path: S3 path to audio file
            language: Detected language code
            output_path: S3 path prefix for output

        Returns:
            Published message sequence number
        """
        new_meta = JobMeta(
            job_id=str(uuid.uuid4()),
            job_type="subtitle_transcribe",
            entity_type=parent_meta.entity_type,
            entity_id=parent_meta.entity_id,
            entity_code=parent_meta.entity_code,
            priority=parent_meta.priority,
            timeout_sec=1800,  # 30 minutes for transcription
        )

        input_data = {
            "audio_path": audio_path,
            "language": language,
            "output_path": output_path,
        }

        return await self.publish_job(
            "jobs.subtitle.transcribe",
            new_meta,
            input_data,
        )

    async def publish_subtitle_translate(
        self,
        parent_meta: JobMeta,
        srt_path: str,
        source_language: str,
        target_languages: list[str],
        output_path: str,
    ) -> str:
        """
        Publish a subtitle_translate job.

        Args:
            parent_meta: Metadata from parent job
            srt_path: S3 path to source SRT file
            source_language: Source language code
            target_languages: List of target language codes
            output_path: S3 path prefix for output

        Returns:
            Published message sequence number
        """
        new_meta = JobMeta(
            job_id=str(uuid.uuid4()),
            job_type="subtitle_translate",
            entity_type=parent_meta.entity_type,
            entity_id=parent_meta.entity_id,
            entity_code=parent_meta.entity_code,
            priority=parent_meta.priority,
            timeout_sec=600,  # 10 minutes for translation
        )

        input_data = {
            "srt_path": srt_path,
            "source_language": source_language,
            "target_languages": target_languages,
            "output_path": output_path,
        }

        return await self.publish_job(
            "jobs.subtitle.translate",
            new_meta,
            input_data,
        )
