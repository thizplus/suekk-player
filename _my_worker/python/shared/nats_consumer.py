"""
NATS JetStream consumer for Python workers.
"""

import asyncio
import json
import logging
import signal
from dataclasses import dataclass
from typing import Any, Callable, Optional
from uuid import UUID

import nats
from nats.js import JetStreamContext
# Note: Stream/Consumer created by Backend - worker just connects to existing

from .config import Config

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

    @classmethod
    def from_dict(cls, data: dict) -> "JobMeta":
        return cls(
            job_id=data.get("job_id", ""),
            job_type=data.get("job_type", ""),
            entity_type=data.get("entity_type", ""),
            entity_id=data.get("entity_id", ""),
            entity_code=data.get("entity_code", ""),
            priority=data.get("priority", 1),
            retry_count=data.get("retry_count", 0),
            max_retries=data.get("max_retries", 3),
            timeout_sec=data.get("timeout_sec", 600),
            created_at=data.get("created_at", 0),
        )


@dataclass
class Job:
    """Generic job with metadata and input"""
    meta: JobMeta
    input: dict

    @classmethod
    def from_dict(cls, data: dict) -> "Job":
        return cls(
            meta=JobMeta.from_dict(data.get("_meta", {})),
            input=data.get("input", {}),
        )


# Type alias for job handler
JobHandler = Callable[[Job], Any]


class NATSConsumer:
    """NATS JetStream consumer for processing jobs"""

    def __init__(self, config: Config):
        self.config = config
        self.nc: Optional[nats.NATS] = None
        self.js: Optional[JetStreamContext] = None
        self.subscription = None
        self._running = False
        self._shutdown_event = asyncio.Event()

    async def connect(self):
        """Connect to NATS server"""
        logger.info(f"Connecting to NATS: {self.config.nats_url}")

        self.nc = await nats.connect(
            self.config.nats_url,
            name=self.config.worker_id,
            reconnect_time_wait=2,
            max_reconnect_attempts=-1,
            disconnected_cb=self._on_disconnect,
            reconnected_cb=self._on_reconnect,
            error_cb=self._on_error,
        )

        self.js = self.nc.jetstream()

        # GET existing consumer (created by Backend/API)
        # Per ___WORKER_INTERFACE_STANDARD.md Section 7.3: Workers should NOT create streams/consumers
        try:
            consumer_info = await self.js.consumer_info(
                self.config.stream_name,
                self.config.consumer_name
            )
            logger.info(f"Consumer ready: {self.config.consumer_name} on stream {self.config.stream_name}")
        except Exception as e:
            await self.nc.close()
            raise RuntimeError(
                f"Consumer '{self.config.consumer_name}' not found on stream '{self.config.stream_name}'. "
                f"Make sure Backend/API is running and has created the stream. Error: {e}"
            )

        logger.info("NATS connected")

    async def _on_disconnect(self):
        logger.warning("NATS disconnected")

    async def _on_reconnect(self):
        logger.info(f"NATS reconnected: {self.nc.connected_url}")

    async def _on_error(self, e):
        logger.error(f"NATS error: {e}")

    async def start(self, handler: JobHandler):
        """Start consuming messages"""
        if not self.nc or not self.js:
            await self.connect()

        logger.info(f"Starting consumer: stream={self.config.stream_name}, subject={self.config.subject}")

        # Subscribe to pull consumer
        self.subscription = await self.js.pull_subscribe(
            self.config.subject,
            durable=self.config.consumer_name,
        )

        self._running = True

        # Setup signal handlers (not supported on Windows)
        import sys
        if sys.platform != "win32":
            loop = asyncio.get_event_loop()
            for sig in (signal.SIGINT, signal.SIGTERM):
                loop.add_signal_handler(sig, self._handle_shutdown)

        logger.info("Worker ready - waiting for jobs")

        while self._running:
            try:
                # Fetch messages with timeout
                messages = await self.subscription.fetch(
                    batch=1,
                    timeout=5,
                )

                for msg in messages:
                    await self._process_message(msg, handler)

            except nats.errors.TimeoutError:
                # No messages available, continue polling
                continue
            except Exception as e:
                if self._running:
                    logger.error(f"Error fetching messages: {e}")
                    await asyncio.sleep(1)

        logger.info("Consumer stopped")

    async def _heartbeat(self, msg, stop_event: asyncio.Event, job_id: str):
        """
        Send InProgress heartbeat every 30 seconds per ___WORKER_INTERFACE_STANDARD.md Section 7.2.
        This extends the ack wait time so long-running jobs don't timeout.
        """
        while not stop_event.is_set():
            try:
                await asyncio.sleep(30)
                if not stop_event.is_set():
                    await msg.in_progress()
                    logger.debug(f"InProgress heartbeat sent: {job_id}")
            except asyncio.CancelledError:
                break
            except Exception as e:
                logger.warning(f"Failed to send InProgress heartbeat: {e}")

    async def _process_message(self, msg, handler: JobHandler):
        """Process a single message with heartbeat support"""
        heartbeat_task = None
        stop_heartbeat = asyncio.Event()

        try:
            # Parse job
            data = json.loads(msg.data.decode())
            job = Job.from_dict(data)

            logger.info(f"Processing job: {job.meta.job_id} ({job.meta.entity_code})")

            # Start heartbeat task for long-running jobs (per standard Section 7.2)
            heartbeat_task = asyncio.create_task(
                self._heartbeat(msg, stop_heartbeat, job.meta.job_id)
            )

            # Call handler
            result = handler(job)

            # Handle async handlers
            if asyncio.iscoroutine(result):
                result = await result

            # Stop heartbeat and acknowledge success
            stop_heartbeat.set()
            if heartbeat_task:
                heartbeat_task.cancel()
                try:
                    await heartbeat_task
                except asyncio.CancelledError:
                    pass

            await msg.ack()
            logger.info(f"Job completed: {job.meta.job_id}")

        except json.JSONDecodeError as e:
            logger.error(f"Invalid JSON in message: {e}")
            stop_heartbeat.set()
            if heartbeat_task:
                heartbeat_task.cancel()
            await msg.term()  # Don't retry invalid messages

        except Exception as e:
            logger.error(f"Job failed: {e}")

            # Stop heartbeat
            stop_heartbeat.set()
            if heartbeat_task:
                heartbeat_task.cancel()

            # Check retry count
            metadata = msg.metadata
            if metadata and metadata.num_delivered >= 3:
                logger.warning(f"Max retries reached, terminating job")
                await msg.term()
            else:
                # NAK with delay for retry
                await msg.nak(delay=30)

    def _handle_shutdown(self):
        """Handle shutdown signal"""
        logger.info("Shutdown signal received")
        self._running = False
        self._shutdown_event.set()

    async def close(self):
        """Close NATS connection"""
        self._running = False

        if self.subscription:
            await self.subscription.unsubscribe()

        if self.nc:
            await self.nc.close()

        logger.info("NATS connection closed")

    def connection(self) -> Optional[nats.NATS]:
        """Get the NATS connection for progress publisher"""
        return self.nc
