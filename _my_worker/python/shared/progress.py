"""
Progress publisher for Python workers.
Publishes progress updates via NATS pub/sub.
"""

import json
import logging
import time
import threading
from dataclasses import dataclass, asdict
from datetime import datetime
from typing import Any, Optional, Dict

import nats

logger = logging.getLogger(__name__)


class ProgressThrottler:
    """
    Controls the rate of progress updates.

    ส่ง update เมื่อ:
    1. เวลาผ่านไป >= interval (default 2 วินาที)
    2. Progress เปลี่ยนไป >= percent_step (default 5%)
    3. Status เป็น completed หรือ failed (ส่งเสมอ)
    """

    def __init__(self, interval: float = 2.0, percent_step: int = 5):
        self.interval = interval
        self.percent_step = percent_step
        self._last_sent: Dict[str, float] = {}
        self._last_percent: Dict[str, int] = {}
        self._lock = threading.Lock()

    def should_send(self, job_id: str, progress: float, status: str) -> bool:
        """Returns True if this update should be sent"""
        # Always send terminal states
        if status in ("completed", "failed"):
            self.cleanup(job_id)
            return True

        with self._lock:
            current_time = time.time()
            current_percent = int(progress)

            # Always send first update
            if job_id not in self._last_sent or progress == 0:
                self._last_sent[job_id] = current_time
                self._last_percent[job_id] = current_percent
                return True

            last_sent = self._last_sent[job_id]
            last_percent = self._last_percent.get(job_id, 0)

            # Check if crossed percent threshold
            crossed_threshold = (current_percent // self.percent_step) > (last_percent // self.percent_step)

            # Check if time elapsed
            time_elapsed = (current_time - last_sent) >= self.interval

            if crossed_threshold or time_elapsed:
                self._last_sent[job_id] = current_time
                self._last_percent[job_id] = current_percent
                return True

            return False

    def cleanup(self, job_id: str):
        """Remove tracking data for completed job"""
        with self._lock:
            self._last_sent.pop(job_id, None)
            self._last_percent.pop(job_id, None)


@dataclass
class ProgressUpdate:
    """Progress update message"""
    job_id: str
    job_type: str
    entity_type: str
    entity_id: str
    entity_code: str
    worker_id: str
    status: str  # processing, completed, failed
    progress: float  # 0-100
    stage: str
    message: str
    updated_at: str = ""

    # Optional fields
    video_id: Optional[str] = None
    video_code: Optional[str] = None
    subtitle_id: Optional[str] = None
    current_language: Optional[str] = None
    duration_sec: Optional[float] = None
    output: Optional[dict] = None
    error: Optional[str] = None
    error_details: Optional[dict] = None

    def __post_init__(self):
        if not self.updated_at:
            self.updated_at = datetime.utcnow().isoformat() + "Z"

    def to_dict(self) -> dict:
        """Convert to dictionary, excluding None values"""
        result = {
            "job_id": self.job_id,
            "job_type": self.job_type,
            "entity_type": self.entity_type,
            "entity_id": self.entity_id,
            "entity_code": self.entity_code,
            "worker_id": self.worker_id,
            "status": self.status,
            "progress": self.progress,
            "stage": self.stage,
            "message": self.message,
            "updated_at": self.updated_at,
        }

        if self.video_id is not None:
            result["video_id"] = self.video_id
        if self.video_code is not None:
            result["video_code"] = self.video_code
        if self.subtitle_id is not None:
            result["subtitle_id"] = self.subtitle_id
        if self.current_language is not None:
            result["current_language"] = self.current_language
        if self.duration_sec is not None:
            result["duration_sec"] = self.duration_sec
        if self.output is not None:
            result["output"] = self.output
        if self.error is not None:
            result["error"] = self.error
        if self.error_details is not None:
            result["error_details"] = self.error_details

        return result


class ProgressPublisher:
    """Publishes progress updates via NATS with throttling"""

    def __init__(self, nc: nats.NATS, worker_id: str, throttler: Optional[ProgressThrottler] = None):
        self.nc = nc
        self.worker_id = worker_id
        self.throttler = throttler or ProgressThrottler()

    async def publish(self, update: ProgressUpdate):
        """Publish a progress update with retry for completed/failed status"""
        # Ensure worker_id is set
        update.worker_id = self.worker_id

        # Serialize
        data = json.dumps(update.to_dict()).encode()

        # Subject format: progress.{entity_type}.{entity_id}
        subject = f"progress.{update.entity_type}.{update.entity_id}"

        # Retry for critical messages (completed/failed)
        max_retries = 3 if update.status in ("completed", "failed") else 1
        for attempt in range(max_retries):
            try:
                await self.nc.publish(subject, data)
                await self.nc.flush()
                if update.status in ("completed", "failed"):
                    logger.info(f"Progress published: {subject} - {update.status}")
                else:
                    logger.debug(f"Progress published: {subject} - {update.status} {update.progress}%")
                return
            except Exception as e:
                logger.error(f"Progress publish failed (attempt {attempt+1}/{max_retries}): {e}")
                if attempt < max_retries - 1:
                    import asyncio
                    await asyncio.sleep(1)

        logger.error(f"Progress publish FAILED after {max_retries} attempts: {subject} - {update.status}")

    async def publish_processing(
        self,
        job_id: str,
        job_type: str,
        entity_type: str,
        entity_id: str,
        entity_code: str,
        stage: str,
        progress: float,
        message: str,
    ):
        """Convenience method for publishing processing updates (with throttling)"""
        # Throttle check
        if not self.throttler.should_send(job_id, progress, "processing"):
            return  # Skip this update

        update = ProgressUpdate(
            job_id=job_id,
            job_type=job_type,
            entity_type=entity_type,
            entity_id=entity_id,
            entity_code=entity_code,
            worker_id=self.worker_id,
            status="processing",
            progress=progress,
            stage=stage,
            message=message,
        )
        await self.publish(update)

    async def publish_completed(
        self,
        job_id: str,
        job_type: str,
        entity_type: str,
        entity_id: str,
        entity_code: str,
        duration_sec: float,
        output: Optional[dict] = None,
    ):
        """Convenience method for publishing completion (always sends, bypasses throttle)"""
        self.throttler.cleanup(job_id)
        update = ProgressUpdate(
            job_id=job_id,
            job_type=job_type,
            entity_type=entity_type,
            entity_id=entity_id,
            entity_code=entity_code,
            worker_id=self.worker_id,
            status="completed",
            progress=100.0,
            stage="completed",
            message="สำเร็จ",
            duration_sec=duration_sec,
            output=output,
        )
        await self.publish(update)

    async def publish_failed(
        self,
        job_id: str,
        job_type: str,
        entity_type: str,
        entity_id: str,
        entity_code: str,
        error: str,
        stage: str = "failed",
        error_code: str = "",
        is_retryable: bool = True,
    ):
        """Convenience method for publishing failure (always sends, bypasses throttle)"""
        self.throttler.cleanup(job_id)
        update = ProgressUpdate(
            job_id=job_id,
            job_type=job_type,
            entity_type=entity_type,
            entity_id=entity_id,
            entity_code=entity_code,
            worker_id=self.worker_id,
            status="failed",
            progress=0.0,
            stage="failed",
            message="ล้มเหลว",
            error=error,
            error_details={
                "code": error_code or "UNKNOWN_ERROR",
                "stage": stage,
                "is_retryable": is_retryable,
            },
        )
        await self.publish(update)
