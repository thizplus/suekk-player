"""Loguru setup"""
import sys
from loguru import logger

# ลบ default handler
logger.remove()

# Console: สี + เวลา
logger.add(
    sys.stdout,
    format="<green>{time:HH:mm:ss}</green> | <level>{level:<7}</level> | <cyan>{name}</cyan> | {message}",
    level="INFO",
    colorize=True,
)

# File: rotate 10MB
logger.add(
    "data/bot_scraper.log",
    format="{time:YYYY-MM-DD HH:mm:ss} | {level:<7} | {name} | {message}",
    level="DEBUG",
    rotation="10 MB",
    retention="7 days",
    encoding="utf-8",
)
