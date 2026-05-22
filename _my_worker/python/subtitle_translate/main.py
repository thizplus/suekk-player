#!/usr/bin/env python3
"""
Subtitle Translate Worker - Entry Point
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

from handler import SubtitleTranslateHandler

logging.basicConfig(
    level=logging.INFO,
    format='{"time":"%(asctime)s","level":"%(levelname)s","component":"%(name)s","msg":"%(message)s"}',
)
logger = logging.getLogger("subtitle_translate")


async def main():
    logger.info("=" * 40)
    logger.info("  Subtitle Translate Worker Starting")
    logger.info("=" * 40)

    config = load_config("subtitle_translate")

    errors = validate_config(config)
    if errors:
        for error in errors:
            logger.error(f"Config error: {error}")
        sys.exit(1)

    logger.info(f"Config: worker_id={config.worker_id}")

    storage = S3Client(config)
    consumer = NATSConsumer(config)
    await consumer.connect()

    progress = ProgressPublisher(consumer.connection(), config.worker_id)

    handler = SubtitleTranslateHandler(config, storage, progress)

    logger.info("Worker Ready - Waiting for Jobs")

    try:
        await consumer.start(handler.handle)
    finally:
        await consumer.close()


if __name__ == "__main__":
    asyncio.run(main())
