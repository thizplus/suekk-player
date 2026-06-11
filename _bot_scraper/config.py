from __future__ import annotations
import os
from dataclasses import dataclass, field
from dotenv import load_dotenv

load_dotenv()


@dataclass
class SiteConfig:
    name: str
    base_url: str
    enabled: bool = True
    check_interval: int = 30            # นาที
    categories: list[str] = field(default_factory=list)
    max_pages_per_check: int = 0        # 0 = ดึงทุกหน้า, >0 = จำกัดหน้า
    preferred_server: int = 1           # 1=ไทย, 2=soundtrack


@dataclass
class Config:
    # SUEKK
    suekk_api_url: str = ""
    suekk_token: str = ""

    # Storage
    download_path: str = "./data/downloads"
    db_path: str = "./data/tracker.db"

    # Telegram
    telegram_bot_token: str = ""
    telegram_chat_id: str = ""

    # Sites
    sites: list[SiteConfig] = field(default_factory=list)

    # General
    max_concurrent_downloads: int = 2
    auto_download: bool = True
    auto_upload: bool = False           # ยังไม่เปิด upload อัตโนมัติ


def load_config() -> Config:
    sites = []

    # serie-days
    sd_url = os.getenv("SERIE_DAYS_URL", "https://www.serie-days.com")
    sd_cats = os.getenv("SERIE_DAYS_CATEGORIES", "ซีรี่ย์เกาหลี")
    sd_interval = int(os.getenv("SERIE_DAYS_CHECK_INTERVAL", "30"))
    sd_server = int(os.getenv("SERIE_DAYS_PREFERRED_SERVER", "1"))

    sites.append(SiteConfig(
        name="serie_days",
        base_url=sd_url,
        categories=[c.strip() for c in sd_cats.split(",") if c.strip()],
        check_interval=sd_interval,
        preferred_server=sd_server,
    ))

    return Config(
        suekk_api_url=os.getenv("SUEKK_API_URL", ""),
        suekk_token=os.getenv("SUEKK_TOKEN", ""),
        download_path=os.getenv("DOWNLOAD_PATH", "./data/downloads"),
        db_path=os.getenv("DB_PATH", "./data/tracker.db"),
        telegram_bot_token=os.getenv("TELEGRAM_BOT_TOKEN", ""),
        telegram_chat_id=os.getenv("TELEGRAM_CHAT_ID", ""),
        sites=sites,
    )
