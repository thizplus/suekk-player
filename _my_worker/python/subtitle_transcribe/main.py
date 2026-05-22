#!/usr/bin/env python3
"""
Subtitle Transcribe Worker - Entry Point
"""

import asyncio
import logging
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))

from shared.config import load_config, validate_config
from shared.nats_consumer import NATSConsumer
from shared.progress import ProgressPublisher
from shared.storage import S3Client

from handler import SubtitleTranscribeHandler

log_format = '{"time":"%(asctime)s","level":"%(levelname)s","component":"%(name)s","msg":"%(message)s"}'
log_dir = Path(__file__).parent.parent.parent / "logs"
log_dir.mkdir(parents=True, exist_ok=True)

logging.basicConfig(
    level=logging.INFO,
    format=log_format,
    datefmt='%Y-%m-%dT%H:%M:%S',
    handlers=[
        logging.StreamHandler(),
        logging.FileHandler(log_dir / "subtitle_transcribe.log", encoding="utf-8"),
    ],
)
logger = logging.getLogger("subtitle_transcribe")


async def main():
    logger.info("=" * 40)
    logger.info("  Subtitle Transcribe Worker Starting")
    logger.info("=" * 40)

    config = load_config("subtitle_transcribe")

    errors = validate_config(config)
    if errors:
        for error in errors:
            logger.error(f"Config error: {error}")
        sys.exit(1)

    logger.info(f"Config: worker_id={config.worker_id}, model={config.whisper_model}")

    storage = S3Client(config)
    consumer = NATSConsumer(config)
    await consumer.connect()

    progress = ProgressPublisher(consumer.connection(), config.worker_id)

    handler = SubtitleTranscribeHandler(config, storage, progress)

    logger.info("Worker Ready - Waiting for Jobs")

    try:
        await consumer.start(handler.handle)
    finally:
        await consumer.close()


if __name__ == "__main__":
    asyncio.run(main())
