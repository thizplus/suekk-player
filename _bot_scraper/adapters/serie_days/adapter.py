"""
serie-days.com Adapter

Hybrid approach:
- Listing + Metadata: wp-json API (aiohttp — เร็ว)
- Episode list: HTML parse (aiohttp + BeautifulSoup)
- Video URL: Playwright (intercept network — ชัวร์)
"""
from __future__ import annotations
import re
import json
import asyncio
import aiohttp
from bs4 import BeautifulSoup
from loguru import logger

from domain.ports import ScraperPort
from domain.models import SeriesInfo, SeriesDetail, EpisodeInfo, VideoSource
from config import SiteConfig
from infrastructure.browser import get_browser


class SerieDaysAdapter(ScraperPort):
    def __init__(self, config: SiteConfig):
        self.base_url = config.base_url.rstrip("/")
        self.preferred_server = config.preferred_server
        self._session: aiohttp.ClientSession | None = None

    async def _get_session(self) -> aiohttp.ClientSession:
        if self._session is None or self._session.closed:
            self._session = aiohttp.ClientSession(
                headers={"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/125.0.0.0"},
                timeout=aiohttp.ClientTimeout(total=30),
            )
        return self._session

    def get_site_name(self) -> str:
        return "serie_days"

    # ─── Phase 1: Listing via wp-json API (เร็ว ไม่ต้อง browser) ───

    async def get_category_map(self) -> dict[str, int]:
        """ดึง category mapping: name -> id"""
        session = await self._get_session()
        url = f"{self.base_url}/wp-json/wp/v2/categories"
        params = {"per_page": 100}

        try:
            async with session.get(url, params=params) as resp:
                if resp.status != 200:
                    logger.error(f"Failed to fetch categories: {resp.status}")
                    return {}
                cats = await resp.json()
                mapping = {c["name"]: c["id"] for c in cats}
                logger.info(f"Loaded {len(mapping)} categories")
                return mapping
        except Exception as e:
            logger.error(f"Error fetching categories: {e}")
            return {}

    async def get_total_pages(self, category_id: int) -> int:
        """ดึงจำนวนหน้าทั้งหมดจาก wp-json header X-WP-TotalPages"""
        session = await self._get_session()
        url = f"{self.base_url}/wp-json/wp/v2/posts"
        params = {"per_page": 24, "page": 1, "categories": category_id}

        try:
            async with session.get(url, params=params) as resp:
                return int(resp.headers.get("X-WP-TotalPages", 1))
        except Exception:
            return 1

    async def get_series_list(self, category_id: int, page: int = 1) -> list[SeriesInfo]:
        """ดึงรายการซีรี่ย์จาก wp-json API"""
        session = await self._get_session()
        url = f"{self.base_url}/wp-json/wp/v2/posts"
        params = {
            "per_page": 24,
            "page": page,
            "categories": category_id,
            "_embed": "",
        }

        try:
            async with session.get(url, params=params) as resp:
                if resp.status != 200:
                    logger.warning(f"wp-json returned {resp.status} for cat={category_id} page={page}")
                    return []
                posts = await resp.json()
        except Exception as e:
            logger.error(f"Error fetching series list: {e}")
            return []

        results = []
        for post in posts:
            try:
                info = self._parse_wp_post(post)
                results.append(info)
            except Exception as e:
                logger.warning(f"Failed to parse post {post.get('id')}: {e}")

        logger.info(f"Fetched {len(results)} series from page {page}")
        return results

    def _parse_wp_post(self, post: dict) -> SeriesInfo:
        """Parse wp-json post เป็น SeriesInfo"""
        title_raw = post.get("title", {}).get("rendered", "")

        # Extract year จาก title: "Name (2026)" -> 2026
        year_match = re.search(r"\((\d{4})\)", title_raw)
        year = int(year_match.group(1)) if year_match else 0

        # Thai title: ถ้า title มีภาษาไทย แยกออกมา
        # Pattern: "English Title (2026) ไทยTitle"
        thai = ""
        parts = re.split(r"\(\d{4}\)\s*", title_raw, maxsplit=1)
        if len(parts) > 1 and parts[1].strip():
            thai = parts[1].strip()

        # Thumbnail จาก _embedded
        thumb = ""
        embedded = post.get("_embedded", {})
        media_list = embedded.get("wp:featuredmedia", [])
        if media_list:
            thumb = media_list[0].get("source_url", "")

        # Categories จาก _embedded
        categories = []
        terms_list = embedded.get("wp:term", [])
        for terms in terms_list:
            for term in terms:
                if term.get("taxonomy") == "category":
                    categories.append(term["name"])

        return SeriesInfo(
            source_site="serie_days",
            source_id=post["id"],
            slug=post["slug"],
            title=title_raw,
            thai_title=thai,
            thumbnail_url=thumb,
            categories=categories,
            year=year,
            published_at=post.get("date", ""),
        )

    # ─── Phase 2: Detail page (HTML parse — ไม่ต้อง browser) ───

    async def get_series_detail(self, slug: str) -> SeriesDetail:
        """Fetch detail page + parse episodes, nonce, metadata"""
        session = await self._get_session()
        url = f"{self.base_url}/{slug}/"

        try:
            async with session.get(url) as resp:
                if resp.status != 200:
                    logger.error(f"Detail page {slug} returned {resp.status}")
                    return None
                html = await resp.text()
        except Exception as e:
            logger.error(f"Error fetching detail {slug}: {e}")
            return None

        return self._parse_detail_html(html, slug)

    def _parse_detail_html(self, html: str, slug: str) -> SeriesDetail:
        """Parse detail page HTML"""
        soup = BeautifulSoup(html, "html.parser")

        # Title
        title_el = soup.select_one("h1.entry-title, .entry-title, title")
        title = title_el.get_text(strip=True) if title_el else slug

        # Year
        year_match = re.search(r"\((\d{4})\)", title)
        year = int(year_match.group(1)) if year_match else 0

        # Thai title
        thai = ""
        parts = re.split(r"\(\d{4}\)\s*", title, maxsplit=1)
        if len(parts) > 1 and parts[1].strip():
            thai = parts[1].strip()

        # Post ID จาก halim_cfg
        post_id = 0
        cfg_match = re.search(r'"post_id"\s*:\s*(\d+)', html)
        if cfg_match:
            post_id = int(cfg_match.group(1))

        # Nonce
        nonce = ""
        nonce_match = re.search(r'"nonce"\s*:\s*"([^"]+)"', html)
        if nonce_match:
            nonce = nonce_match.group(1)

        # Episodes: ดึงจาก <select> dropdown ที่มี option "ตอนที่ X"
        # โครงสร้างเว็บ:
        #   <select> <option value="/slug-ep-1/">ตอนที่ 1</option> ... </select>
        #   halim-btn / data-episode = server buttons (ตัวเล่นหลัก/สำรอง) ไม่ใช่ episode!
        episodes = []
        seen_eps = set()

        # Pattern 1 (หลัก): <select> dropdown with episode options
        for select in soup.select("select"):
            for option in select.select("option"):
                text = option.get_text(strip=True)
                value = option.get("value", "")
                ep_num = self._extract_episode_number(text)
                # ยังเช็คจาก URL pattern: /slug-ep-3/ -> 3
                if not ep_num and value:
                    url_match = re.search(r"-ep-(\d+)", value)
                    if url_match:
                        ep_num = int(url_match.group(1))
                if ep_num and ep_num not in seen_eps:
                    seen_eps.add(ep_num)
                    episodes.append(EpisodeInfo(episode_number=ep_num))

        # Pattern 2 (fallback): หา link ที่มี pattern /slug-ep-X/
        if not episodes:
            for a in soup.select("a[href]"):
                href = a.get("href", "")
                url_match = re.search(r"-ep-(\d+)", href)
                if url_match:
                    ep_num = int(url_match.group(1))
                    if ep_num not in seen_eps:
                        seen_eps.add(ep_num)
                        episodes.append(EpisodeInfo(episode_number=ep_num))

        episodes.sort(key=lambda e: e.episode_number)

        # ─── Metadata จาก OG tags + .col-in1 ───

        # Poster (OG image — full size, ดีกว่า thumbnail)
        poster_url = ""
        og_image = soup.select_one('meta[property="og:image"]')
        if og_image:
            poster_url = og_image.get("content", "")
            # Fix old domain: seriesday.com -> serie-days.com
            poster_url = poster_url.replace("www.seriesday.com", "www.serie-days.com")

        # OG title (สะอาดกว่า page title)
        og_title = ""
        og_title_el = soup.select_one('meta[property="og:title"]')
        if og_title_el:
            og_title = og_title_el.get("content", "")

        # Description จาก OG
        description = ""
        og_desc = soup.select_one('meta[property="og:description"]')
        if og_desc:
            description = og_desc.get("content", "").strip()

        # Metadata block: .col-in1 (ปี, เสียง, คะแนน, quality)
        rating = 0.0
        audio_type = ""
        quality = ""

        col_in1 = soup.select_one(".col-in1")
        if col_in1:
            # คะแนน IMDB จาก .score
            score_el = col_in1.select_one(".score")
            if score_el:
                score_match = re.search(r"(\d+\.?\d*)", score_el.get_text())
                if score_match:
                    rating = float(score_match.group(1))

            # เสียง: ดูจาก #Lang_select (ตัวเลือกจริง) ไม่ใช่ .text-content3 (แค่ default)
            # ถ้ามี "Thai" ใน options → audio = "Thai"
            # ถ้ามีแค่ "Sound Track" → audio = "Sound Track"
            pass  # audio_type จะ parse ข้างล่างจาก Lang_select

            # Quality (HD/SD/etc.)
            for span in col_in1.select("span"):
                t = span.get_text(strip=True)
                if t in ("HD", "FHD", "SD", "CAM"):
                    quality = t
                    break

        # Year จาก .col-in1 .year (อาจแม่นกว่า title)
        year_el = soup.select_one(".col-in1 .year a")
        if year_el:
            y = re.search(r"(\d{4})", year_el.get_text())
            if y:
                year = int(y.group(1))

        # Trailer YouTube ID — อยู่ใน <link preload> thumbnail
        # Pattern: img.youtube.com/vi/{VIDEO_ID}/0.jpg
        trailer_url = ""
        yt_ids = re.findall(r"img\.youtube\.com/vi/([a-zA-Z0-9_-]+)/", html)
        if yt_ids:
            trailer_url = f"https://www.youtube.com/watch?v={yt_ids[0]}"

        # Description/เรื่องย่อ — อยู่ใน div.col-in4 > p
        if not description:
            col_in4 = soup.select_one(".col-in4")
            if col_in4:
                paragraphs = []
                for p in col_in4.select("p"):
                    text = p.get_text(strip=True)
                    if len(text) > 30:
                        paragraphs.append(text)
                if paragraphs:
                    description = "\n\n".join(paragraphs)

        # Servers: ดูจาก halim-btn data-episode (server buttons)
        servers = set()
        for btn in soup.select("[data-episode]"):
            # data-episode ในที่นี้คือ server number (1=หลัก, 2=สำรอง1, ...)
            # ยกเว้น 1000 (download link)
            ep_val = int(btn.get("data-episode", 0))
            if 0 < ep_val < 100:
                servers.add(ep_val)

        # Language select: Thai / Sound Track — ใช้เป็น audio_type จริง
        lang_select = soup.select_one("#Lang_select, select[id*=Lang]")
        languages = []
        if lang_select:
            for opt in lang_select.select("option"):
                languages.append(opt.get_text(strip=True))

        # audio_type: ถ้ามี Thai → "Thai", ถ้ามีแค่ Sound Track → "Sound Track"
        if languages:
            if any("Thai" in l or "ไทย" in l for l in languages):
                audio_type = "Thai"
            else:
                audio_type = languages[0]
        elif not audio_type:
            # fallback จาก .text-content3 ถ้าไม่มี Lang_select
            audio_el = col_in1.select_one(".text-content3") if col_in1 else None
            if audio_el:
                audio_type = re.sub(r"^เสียง\s*:?\s*", "", audio_el.get_text(strip=True)).strip()

        # Completed check
        is_completed = "จบแล้ว" in html or "Complete" in html

        logger.debug(f"Parsed {slug}: {len(episodes)} episodes, post_id={post_id}, nonce={'yes' if nonce else 'no'}")

        return SeriesDetail(
            source_site="serie_days",
            source_id=post_id,
            slug=slug,
            title=og_title or title,
            thai_title=thai,
            year=year,
            rating=rating,
            quality=quality,
            description=description,
            episodes=episodes,
            total_episodes=len(episodes),
            is_completed=is_completed,
            nonce=nonce,
            poster_url=poster_url,
            audio_type=audio_type,
            trailer_url=trailer_url,
            og_title=og_title,
        )

    def _extract_episode_number(self, text: str) -> int | None:
        """ดึงเลขตอนจาก text เช่น "ตอนที่ 5" -> 5, "EP.5" -> 5"""
        patterns = [
            r"ตอนที่\s*(\d+)",
            r"EP\.?\s*(\d+)",
            r"Episode\s*(\d+)",
            r"^(\d+)$",
        ]
        for pattern in patterns:
            match = re.search(pattern, text.strip(), re.IGNORECASE)
            if match:
                return int(match.group(1))
        return None

    # ─── Phase 3: Video URL (Playwright — intercept network) ───
    # ใช้ browser แยก (light blocking) เพราะ player ต้องการ JS เต็ม

    async def get_episode_video_url(
        self, episode_page_url: str, prefer_thai: bool = True,
    ) -> VideoSource | None:
        """
        เปิดหน้า episode → เลือกเสียงไทย (ถ้ามี) → จับ m3u8 URL

        Logic:
        1. เปิดหน้า episode (e.g. /reverse-ep-1/)
        2. เช็ค #Lang_select → มี "Thai" ไหม? → select Thai
        3. รอ player iframe load
        4. กด play ใน iframe
        5. Intercept .m3u8 URL → return
        """
        from playwright.async_api import async_playwright

        pw = await async_playwright().start()
        browser = await pw.chromium.launch(headless=True)
        context = await browser.new_context(
            viewport={"width": 1280, "height": 720},
            user_agent=(
                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
                "AppleWebKit/537.36 Chrome/125.0.0.0 Safari/537.36"
            ),
        )

        # Light blocking: แค่ images + fonts (ปล่อย JS ทั้งหมดเพื่อให้ player ทำงาน)
        async def light_block(route):
            if route.request.resource_type in ("image", "font"):
                await route.abort()
            else:
                await route.continue_()

        await context.route("**/*", light_block)
        page = await context.new_page()

        captured_urls: list[str] = []

        def on_request(request):
            url = request.url.lower()
            if any(x in url for x in [".m3u8", "playlist.m3u8", "master.m3u8"]):
                if request.url not in captured_urls:
                    captured_urls.append(request.url)
            elif ".mp4" in url and "thumb" not in url and "poster" not in url:
                if request.url not in captured_urls:
                    captured_urls.append(request.url)

        page.on("request", on_request)

        selected_lang = ""

        try:
            logger.info(f"Playwright: opening {episode_page_url}")
            await page.goto(episode_page_url, wait_until="networkidle", timeout=30000)

            # ─── เลือกเสียง: Thai ก่อน, ถ้าไม่มีใช้ตัวแรก ───
            lang_select = page.locator("#Lang_select")
            if await lang_select.count() > 0:
                options = await lang_select.locator("option").all()
                lang_names = []
                for opt in options:
                    lang_names.append((await opt.text_content()).strip())

                if prefer_thai and "Thai" in lang_names:
                    await page.select_option("#Lang_select", "Thai")
                    selected_lang = "Thai"
                    logger.info("Selected language: Thai")
                else:
                    selected_lang = lang_names[0] if lang_names else "unknown"
                    logger.info(f"Thai not available, using: {selected_lang}")

                await page.wait_for_timeout(2000)
            else:
                selected_lang = "default"

            # ─── กด play ใน player iframe ───
            for frame in page.frames:
                if frame == page.main_frame:
                    continue
                if frame.url and "about:blank" not in frame.url:
                    try:
                        play_btn = frame.locator(
                            ".vjs-big-play-button, .jw-icon-display, "
                            "[class*=play-btn], [class*=play-button]"
                        ).first
                        if await play_btn.count() > 0:
                            await play_btn.click(timeout=5000)
                            logger.debug("Clicked play button")
                            await page.wait_for_timeout(3000)
                            break
                    except Exception:
                        pass

            # ─── รอจับ m3u8 (max 8 วินาที) ───
            for _ in range(16):
                if captured_urls:
                    break
                await page.wait_for_timeout(500)

            if not captured_urls:
                logger.warning(f"No video URL captured for {episode_page_url}")
                return None

            # เลือก m3u8 ตัวแรก (master playlist)
            best_url = captured_urls[0]
            vtype = "m3u8" if ".m3u8" in best_url.lower() else "mp4"

            logger.info(f"Captured: {vtype} | {selected_lang} | {best_url[:80]}...")

            return VideoSource(
                url=best_url,
                video_type=vtype,
                quality=selected_lang,  # เก็บว่าได้เสียงอะไร
                server=1,
                headers={"Referer": episode_page_url},
            )

        except Exception as e:
            logger.error(f"Playwright error: {e}")
            return None
        finally:
            await page.close()
            await context.close()
            await browser.close()
            await pw.stop()

    async def close(self):
        if self._session and not self._session.closed:
            await self._session.close()
