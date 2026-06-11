from __future__ import annotations
from domain.ports import ScraperPort
from config import SiteConfig
from adapters.serie_days.adapter import SerieDaysAdapter


def create_scraper(config: SiteConfig) -> ScraperPort:
    adapters = {
        "serie_days": SerieDaysAdapter,
    }
    cls = adapters.get(config.name)
    if cls is None:
        raise ValueError(f"Unknown site adapter: {config.name}")
    return cls(config)
