#!/usr/bin/env python3
"""
Subtitle Detect Worker - Entry Point

Detects the primary language in audio using Whisper.
"""

import asyncio
import logging
import sys
from pathlib import Path

# Add shared module to path
sys.path.insert(0, str(Path(__file__).parent.parent))

from shared.config import load_config, validate_config
from shared.nats_consumer import NATSConsumer
from shared.progress import ProgressPublisher
from shared.storage import S3Client
from shared.job_publisher import JobPublisher

from handler import SubtitleDetectHandler

# Setup logging (console + file)
log_format = '{"time":"%(asctime)s","level":"%(levelname)s","component":"%(name)s","msg":"%(message)s"}'
log_dir = Path(__file__).parent.parent.parent / "logs"
log_dir.mkdir(parents=True, exist_ok=True)

logging.basicConfig(
    level=logging.INFO,
    format=log_format,
    datefmt='%Y-%m-%dT%H:%M:%S',
    handlers=[
        logging.StreamHandler(),
        logging.FileHandler(log_dir / "subtitle_detect.log", encoding="utf-8"),
    ],
)
logger = logging.getLogger("subtitle_detect")


async def main():
    logger.info("=" * 40)
    logger.info("  Subtitle Detect Worker Starting")
    logger.info("=" * 40)

    # Load config
    config = load_config("subtitle_detect")

    # Validate
    errors = validate_config(config)
    if errors:
        for error in errors:
            logger.error(f"Config error: {error}")
        sys.exit(1)

    logger.info(f"Config loaded:")
    logger.info(f"  worker_id: {config.worker_id}")
    logger.info(f"  nats_url: {config.nats_url}")
    logger.info(f"  stream: {config.stream_name}")
    logger.info(f"  subject: {config.subject}")
    logger.info(f"  whisper_model: {config.whisper_model}")
    logger.info(f"  whisper_device: {config.whisper_device}")

    # Create S3 client
    storage = S3Client(config)

    # Create NATS consumer
    consumer = NATSConsumer(config)
    await consumer.connect()

    # Create progress publisher
    progress = ProgressPublisher(consumer.connection(), config.worker_id)

    # Create job publisher
    job_publisher = JobPublisher(consumer.nc.jetstream())

    # Create handler
    handler = SubtitleDetectHandler(config, storage, progress, job_publisher)

    # Start consuming
    logger.info("=" * 40)
    logger.info("  Worker Ready - Waiting for Jobs")
    logger.info("=" * 40)

    try:
        await consumer.start(handler.handle)
    except KeyboardInterrupt:
        logger.info("Shutting down...")
    finally:
        await consumer.close()

    logger.info("=" * 40)
    logger.info("  Worker Stopped")
    logger.info("=" * 40)


if __name__ == "__main__":
    asyncio.run(main())
