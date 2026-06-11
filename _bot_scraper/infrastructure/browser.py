"""
Playwright Browser Manager — Optimized for Speed

Techniques ที่ใช้:
1. Block images, CSS, fonts, media (ไม่ต้อง render สวย แค่จับ video URL)
2. Block ad/tracking scripts (Google Analytics, Facebook, ads)
3. Reuse browser instance (ไม่ต้องเปิดใหม่ทุกครั้ง)
4. Minimal viewport (เล็กลง = render เร็วขึ้น)
5. Disable animations, smooth scrolling
6. Route abort สำหรับ resource ที่ไม่ต้องการ
"""
from __future__ import annotations
import asyncio
from loguru import logger
from playwright.async_api import async_playwright, Browser, BrowserContext, Page

# Resource types ที่ block (ไม่ต้องใช้ตอน scrape)
BLOCKED_RESOURCE_TYPES = {
    "image",
    "media",
    "font",
    "stylesheet",
    "texttrack",
    "eventsource",
    "websocket",
    "manifest",
}

# URL patterns ที่ block (ads, tracking, analytics)
BLOCKED_URL_PATTERNS = [
    # Ads
    "googleads", "googlesyndication", "doubleclick",
    "adservice", "adsense", "adnxs",
    "ads.", "/ads/", "adserver",
    "popads", "popcash", "popunder",
    "juicyads", "exoclick", "trafficjunky",
    # Analytics & Tracking
    "google-analytics", "googletagmanager",
    "facebook.net", "fbevents", "fbcdn",
    "hotjar", "clarity.ms", "mixpanel",
    "segment.io", "amplitude",
    # Social widgets
    "platform.twitter", "connect.facebook",
    "disqus", "addthis", "sharethis",
    # อื่นๆ ที่ไม่เกี่ยว
    "recaptcha", "gstatic.com/recaptcha",
    "cdn-cgi",  # Cloudflare challenge scripts (ระวัง อาจต้อง whitelist บางอัน)
]


class BrowserManager:
    """Singleton Playwright browser — reuse ไม่ต้องเปิดใหม่"""

    def __init__(self):
        self._playwright = None
        self._browser: Browser | None = None
        self._context: BrowserContext | None = None

    async def start(self):
        if self._browser:
            return

        logger.info("Starting Playwright browser (headless, optimized)...")
        self._playwright = await async_playwright().start()

        self._browser = await self._playwright.chromium.launch(
            headless=True,
            args=[
                "--disable-gpu",
                "--disable-dev-shm-usage",
                "--disable-setuid-sandbox",
                "--no-sandbox",
                "--disable-extensions",
                "--disable-background-networking",
                "--disable-sync",
                "--disable-translate",
                "--disable-default-apps",
                "--mute-audio",
                "--no-first-run",
                "--disable-component-update",
            ],
        )

        self._context = await self._browser.new_context(
            viewport={"width": 1280, "height": 720},
            user_agent=(
                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
                "AppleWebKit/537.36 (KHTML, like Gecko) "
                "Chrome/125.0.0.0 Safari/537.36"
            ),
            java_script_enabled=True,  # ต้องเปิด JS สำหรับ AJAX player
            locale="th-TH",
            timezone_id="Asia/Bangkok",
        )

        # Block resources ระดับ context (ทุก page ที่เปิดจะ block เลย)
        await self._context.route("**/*", self._route_handler)

        logger.info("Browser started successfully")

    async def _route_handler(self, route):
        """Block unnecessary resources for speed"""
        request = route.request

        # Block by resource type
        if request.resource_type in BLOCKED_RESOURCE_TYPES:
            await route.abort()
            return

        # Block by URL pattern
        url_lower = request.url.lower()
        for pattern in BLOCKED_URL_PATTERNS:
            if pattern in url_lower:
                await route.abort()
                return

        # อื่นๆ ปล่อยผ่าน
        await route.continue_()

    async def new_page(self) -> Page:
        """สร้าง page ใหม่ (ใช้ context ที่ block resources แล้ว)"""
        if not self._context:
            await self.start()
        page = await self._context.new_page()

        # Inject script ปิด animation + dialog
        await page.add_init_script("""
            // ปิด animation ทั้งหมด
            const style = document.createElement('style');
            style.textContent = '*, *::before, *::after { animation-duration: 0s !important; transition-duration: 0s !important; }';
            document.head?.appendChild(style);

            // Auto-dismiss dialogs
            window.alert = () => {};
            window.confirm = () => true;
            window.prompt = () => '';

            // Block popups
            window.open = () => null;
        """)

        return page

    async def close(self):
        if self._context:
            await self._context.close()
            self._context = None
        if self._browser:
            await self._browser.close()
            self._browser = None
        if self._playwright:
            await self._playwright.stop()
            self._playwright = None
        logger.info("Browser closed")


# Singleton instance
_browser_manager: BrowserManager | None = None


async def get_browser() -> BrowserManager:
    global _browser_manager
    if _browser_manager is None:
        _browser_manager = BrowserManager()
        await _browser_manager.start()
    return _browser_manager
