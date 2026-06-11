"""สร้าง HTML แสดงข้อมูลทั้งหมดที่ scrape ได้"""
import sqlite3
import json
import html as html_escape

DB_PATH = "data/tracker.db"
OUTPUT = "data/viewer.html"


def generate():
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row

    series_list = conn.execute("""
        SELECT * FROM series ORDER BY created_at DESC
    """).fetchall()

    # Stats
    total = len(series_list)
    total_eps = conn.execute("SELECT COUNT(*) FROM episodes").fetchone()[0]
    has_trailer = sum(1 for s in series_list if s["trailer_url"])
    has_desc = sum(1 for s in series_list if s["description"])
    has_poster = sum(1 for s in series_list if s["poster_url"])

    rows_html = ""
    for s in series_list:
        # Poster thumbnail
        poster = s["poster_url"] or ""
        poster_img = f'<img src="{poster}" width="60" loading="lazy" style="border-radius:4px">' if poster else "-"

        # Trailer link
        trailer = s["trailer_url"] or ""
        yt_id = ""
        if "youtube.com/watch?v=" in trailer:
            yt_id = trailer.split("v=")[1][:11]
        trailer_html = f'<a href="{trailer}" target="_blank">{yt_id}</a>' if trailer else '<span style="color:#666">-</span>'

        # Description (truncate)
        desc = html_escape.escape(s["description"] or "")
        desc_short = desc[:80] + "..." if len(desc) > 80 else desc
        desc_html = f'<span title="{desc}">{desc_short}</span>' if desc else '<span style="color:#666">-</span>'

        # Categories
        try:
            cats = json.loads(s["categories"]) if s["categories"] else []
        except:
            cats = []
        cats_html = ", ".join(cats[:3]) if cats else "-"

        # Episodes for this series
        eps = conn.execute(
            "SELECT episode_number FROM episodes WHERE series_id=? ORDER BY episode_number",
            (s["id"],)
        ).fetchall()
        ep_nums = [e["episode_number"] for e in eps]
        ep_range = f"EP.{ep_nums[0]}-{ep_nums[-1]}" if ep_nums else "-"
        ep_count = len(ep_nums)

        # Audio badge
        audio = s["audio_type"] or "-"
        audio_class = "badge-thai" if "Thai" in audio or "ไทย" in audio else "badge-st"

        # Status
        completed = "จบ" if s["is_completed"] else "ออกอยู่"
        status_class = "badge-done" if s["is_completed"] else "badge-airing"

        rating = s["rating"] if s["rating"] and s["rating"] < 100 else "-"

        rows_html += f"""
        <tr>
            <td>{poster_img}</td>
            <td>
                <strong>{html_escape.escape(s["title"] or "")}</strong>
                <div class="sub">{html_escape.escape(s["thai_title"] or "")}</div>
            </td>
            <td>{s["year"] or "-"}</td>
            <td>{rating}</td>
            <td><span class="{audio_class}">{audio}</span></td>
            <td><span class="{status_class}">{completed}</span> {ep_count} ตอน<br><small>{ep_range}</small></td>
            <td>{trailer_html}</td>
            <td class="desc-col">{desc_html}</td>
            <td>{cats_html}</td>
        </tr>"""

    page = f"""<!DOCTYPE html>
<html lang="th">
<head>
<meta charset="UTF-8">
<title>Bot Scraper - Data Viewer ({total} series)</title>
<style>
    * {{ margin: 0; padding: 0; box-sizing: border-box; }}
    body {{ font-family: 'Segoe UI', sans-serif; background: #0f0f0f; color: #e0e0e0; padding: 20px; }}
    h1 {{ color: #e50914; margin-bottom: 10px; }}
    .stats {{ display: flex; gap: 20px; margin-bottom: 20px; flex-wrap: wrap; }}
    .stat {{ background: #1a1a1a; padding: 12px 20px; border-radius: 8px; border: 1px solid #333; }}
    .stat .num {{ font-size: 24px; font-weight: bold; color: #fff; }}
    .stat .label {{ font-size: 12px; color: #888; }}
    table {{ width: 100%; border-collapse: collapse; font-size: 13px; }}
    th {{ background: #1a1a1a; color: #aaa; padding: 10px 8px; text-align: left; position: sticky; top: 0; z-index: 1; }}
    td {{ padding: 8px; border-bottom: 1px solid #222; vertical-align: top; }}
    tr:hover {{ background: #1a1a1a; }}
    img {{ display: block; }}
    .sub {{ font-size: 11px; color: #888; margin-top: 2px; }}
    .badge-thai {{ background: #0ea5e9; color: #fff; padding: 2px 8px; border-radius: 4px; font-size: 11px; }}
    .badge-st {{ background: #666; color: #fff; padding: 2px 8px; border-radius: 4px; font-size: 11px; }}
    .badge-done {{ background: #22c55e; color: #000; padding: 2px 6px; border-radius: 4px; font-size: 11px; }}
    .badge-airing {{ background: #f59e0b; color: #000; padding: 2px 6px; border-radius: 4px; font-size: 11px; }}
    a {{ color: #e50914; text-decoration: none; }}
    a:hover {{ text-decoration: underline; }}
    .desc-col {{ max-width: 250px; font-size: 11px; color: #999; }}
    small {{ color: #666; }}
    #search {{ width: 300px; padding: 8px 12px; border-radius: 6px; border: 1px solid #444; background: #1a1a1a; color: #fff; margin-bottom: 15px; font-size: 14px; }}
</style>
</head>
<body>
<h1>Bot Scraper - Data Viewer</h1>
<div class="stats">
    <div class="stat"><div class="num">{total}</div><div class="label">Series</div></div>
    <div class="stat"><div class="num">{total_eps}</div><div class="label">Episodes</div></div>
    <div class="stat"><div class="num">{has_poster}</div><div class="label">Has Poster</div></div>
    <div class="stat"><div class="num">{has_trailer}</div><div class="label">Has Trailer</div></div>
    <div class="stat"><div class="num">{has_desc}</div><div class="label">Has Description</div></div>
</div>

<input type="text" id="search" placeholder="Search series..." oninput="filterTable()">

<table id="dataTable">
<thead>
<tr>
    <th>Poster</th>
    <th>Title</th>
    <th>Year</th>
    <th>Rating</th>
    <th>Audio</th>
    <th>Episodes</th>
    <th>Trailer</th>
    <th>Description</th>
    <th>Categories</th>
</tr>
</thead>
<tbody>
{rows_html}
</tbody>
</table>

<script>
function filterTable() {{
    const q = document.getElementById('search').value.toLowerCase();
    const rows = document.querySelectorAll('#dataTable tbody tr');
    rows.forEach(row => {{
        const text = row.textContent.toLowerCase();
        row.style.display = text.includes(q) ? '' : 'none';
    }});
}}
</script>
</body>
</html>"""

    with open(OUTPUT, "w", encoding="utf-8") as f:
        f.write(page)

    print(f"Generated {OUTPUT}")
    print(f"  {total} series, {total_eps} episodes")
    print(f"  Poster: {has_poster} | Trailer: {has_trailer} | Desc: {has_desc}")


if __name__ == "__main__":
    generate()
