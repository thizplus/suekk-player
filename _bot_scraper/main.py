"""
Bot Scraper - Entry Point

Usage:
    python main.py              # รัน scheduler (auto-check ทุก X นาที)
    python main.py --once       # รันครั้งเดียว (check ทุก category แล้วจบ)
    python main.py --stats      # แสดง stats จาก DB
    python main.py --test-video # ทดสอบ Playwright จับ video URL
"""
from __future__ import annotations
import sys
import asyncio
import argparse
from loguru import logger

# Setup logger ก่อน import อื่น
import infrastructure.logger  # noqa: F401

from config import load_config
from infrastructure.database import Database
from infrastructure.browser import get_browser
from services.tracker import Tracker
from services.scrape_service import ScrapeService
from adapters.factory import create_scraper


async def run_once(config):
    """รัน check ครั้งเดียว ทุก site + ทุก category"""
    db = Database(config.db_path)
    await db.connect()
    tracker = Tracker(db)
    service = ScrapeService(tracker)

    for site_cfg in config.sites:
        if not site_cfg.enabled:
            continue

        scraper = create_scraper(site_cfg)
        logger.info(f"=== Checking {site_cfg.name} ===")

        # ดึง category map
        cat_map = await scraper.get_category_map()
        if not cat_map:
            logger.error(f"Failed to load categories for {site_cfg.name}")
            continue

        for cat_name in site_cfg.categories:
            cat_id = cat_map.get(cat_name)
            if cat_id is None:
                # ลอง search แบบ partial match
                for k, v in cat_map.items():
                    if cat_name.lower() in k.lower():
                        cat_id = v
                        logger.info(f"Matched category '{cat_name}' -> '{k}' (id={v})")
                        break

            if cat_id is None:
                logger.warning(f"Category '{cat_name}' not found. Available: {list(cat_map.keys())[:10]}")
                continue

            stats = await service.check_category(
                scraper, cat_name, cat_id,
                max_pages=site_cfg.max_pages_per_check,
            )

        await scraper.close()

    await db.close()
    logger.info("Done!")


async def run_scheduler(config):
    """รัน scheduler (auto-check ทุก X นาที)"""
    from apscheduler.schedulers.asyncio import AsyncIOScheduler

    db = Database(config.db_path)
    await db.connect()
    tracker = Tracker(db)
    service = ScrapeService(tracker)

    scrapers = {}
    cat_maps = {}

    for site_cfg in config.sites:
        if not site_cfg.enabled:
            continue
        scraper = create_scraper(site_cfg)
        scrapers[site_cfg.name] = (scraper, site_cfg)
        cat_maps[site_cfg.name] = await scraper.get_category_map()

    scheduler = AsyncIOScheduler()

    for site_name, (scraper, site_cfg) in scrapers.items():
        cat_map = cat_maps.get(site_name, {})

        for cat_name in site_cfg.categories:
            cat_id = None
            for k, v in cat_map.items():
                if cat_name.lower() in k.lower():
                    cat_id = v
                    break

            if cat_id is None:
                logger.warning(f"Skipping unknown category: {cat_name}")
                continue

            job_id = f"check_{site_name}_{cat_id}"
            scheduler.add_job(
                service.check_category,
                trigger="interval",
                minutes=site_cfg.check_interval,
                args=[scraper, cat_name, cat_id, site_cfg.max_pages_per_check],
                id=job_id,
                max_instances=1,
                next_run_time=None,  # ไม่รันทันที (รอ run_once ก่อน)
            )
            logger.info(f"Scheduled: {job_id} every {site_cfg.check_interval}m")

    # รันครั้งแรกทันที
    await run_once(config)

    # เริ่ม scheduler
    scheduler.start()
    logger.info(f"Scheduler started! Checking every {config.sites[0].check_interval} minutes")

    try:
        await asyncio.Event().wait()
    except (KeyboardInterrupt, SystemExit):
        logger.info("Shutting down...")
        scheduler.shutdown()
        for scraper, _ in scrapers.values():
            await scraper.close()
        await db.close()


async def show_stats(config):
    """แสดง stats จาก DB"""
    db = Database(config.db_path)
    await db.connect()
    tracker = Tracker(db)

    stats = await tracker.get_stats()
    print("\n=== Bot Scraper Stats ===")
    print(f"  Series:    {stats['total_series']}")
    print(f"  Episodes:  {stats['total_episodes']}")
    print(f"  Pending:   {stats['pending']}")
    print(f"  Uploaded:  {stats['uploaded']}")
    print(f"  Failed:    {stats['failed']}")

    # แสดง series ล่าสุด 5 ตัว
    recent = await db.fetchall(
        "SELECT title, total_episodes, is_completed, created_at FROM series ORDER BY created_at DESC LIMIT 5"
    )
    if recent:
        print("\n  Recent series:")
        for r in recent:
            status = "จบ" if r["is_completed"] else "ยังออก"
            print(f"    - {r['title']} ({r['total_episodes']} eps, {status})")

    print()
    await db.close()


async def test_video(config):
    """ทดสอบ Playwright จับ video URL"""
    site_cfg = config.sites[0]
    scraper = create_scraper(site_cfg)

    # ดึง series แรกจาก category แรก
    cat_map = await scraper.get_category_map()
    cat_name = site_cfg.categories[0]
    cat_id = None
    for k, v in cat_map.items():
        if cat_name.lower() in k.lower():
            cat_id = v
            break

    if not cat_id:
        print(f"Category '{cat_name}' not found")
        await scraper.close()
        return

    series_list = await scraper.get_series_list(cat_id, page=1)
    if not series_list:
        print("No series found")
        await scraper.close()
        return

    first = series_list[0]
    print(f"\nTesting with: {first.title}")
    print(f"URL: {site_cfg.base_url}/{first.slug}/")

    # ดึง detail
    detail = await scraper.get_series_detail(first.slug)
    if detail and detail.episodes:
        ep = detail.episodes[0]
        print(f"Episodes: {len(detail.episodes)}, testing EP.{ep.episode_number}")

        # ทดสอบ Playwright
        print("\nStarting Playwright (video URL capture)...")
        source = await scraper.get_episode_video_url(
            f"{site_cfg.base_url}/{first.slug}/",
            ep.episode_number,
            server=site_cfg.preferred_server,
        )

        if source:
            print(f"\nVideo URL captured!")
            print(f"  Type: {source.video_type}")
            print(f"  URL:  {source.url[:120]}...")
            print(f"  Server: {source.server}")
        else:
            print("\nFailed to capture video URL")
    else:
        print("No episodes found in detail page")

    # Cleanup
    browser = await get_browser()
    await browser.close()
    await scraper.close()


def main():
    parser = argparse.ArgumentParser(description="Bot Scraper for SUEKK Stream")
    parser.add_argument("--once", action="store_true", help="Run once and exit")
    parser.add_argument("--stats", action="store_true", help="Show stats")
    parser.add_argument("--test-video", action="store_true", help="Test Playwright video capture")
    args = parser.parse_args()

    config = load_config()

    if args.stats:
        asyncio.run(show_stats(config))
    elif args.test_video:
        asyncio.run(test_video(config))
    elif args.once:
        asyncio.run(run_once(config))
    else:
        asyncio.run(run_scheduler(config))


if __name__ == "__main__":
    main()
