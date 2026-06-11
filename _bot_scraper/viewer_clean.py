"""สร้าง HTML แสดงข้อมูลที่จะเอาเข้า SUEKK — title แยก EN/TH สะอาด"""
import sqlite3
import json
import re
import html as html_escape

DB_PATH = "data/tracker.db"
OUTPUT = "data/viewer_clean.html"


def clean_title(raw: str) -> tuple[str, str]:
    """แยก title เป็น EN + TH และตัด junk ออก"""
    clean = raw.strip()

    # 1. ตัด prefix
    clean = re.sub(r"^ดูซีรี่ย์\s*", "", clean)

    # 2. ตัด EP X-XX จบ (อยู่ตรงกลาง)
    clean = re.sub(r"\s*EP\s*\d+[\s-]*\d*\s*จบ\s*", " ", clean, flags=re.IGNORECASE).strip()

    # 3. ตัด (20XX)
    clean = re.sub(r"\s*\(\d{4}\)\s*", " ", clean).strip()

    # 4. ตัด junk ท้าย
    junk_end = [
        r"\s+ซับไทย[\s\S]*$",
        r"\s+พากย์ไทย[\s\S]*$",
        r"\s+[Ss]erie[s]?[-\s]?[Dd]ays?[.\-\s]*(COM|com)?[\s\S]*$",
        r"\s+SeriesDAY[\s\S]*$",
        r"\s+serie-days\.com[\s\S]*$",
    ]
    for pat in junk_end:
        clean = re.sub(pat, "", clean, flags=re.IGNORECASE).strip()

    # 5. แยก EN / TH
    thai_match = re.search(r"[\u0E00-\u0E7F]", clean)
    if thai_match:
        idx = thai_match.start()
        en = clean[:idx].strip()
        th = clean[idx:].strip()
        for pat in junk_end:
            th = re.sub(pat, "", th, flags=re.IGNORECASE).strip()
    else:
        en = clean
        th = ""

    return en, th


def generate():
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row

    series_list = conn.execute("SELECT * FROM series ORDER BY year DESC, rating DESC").fetchall()
    total = len(series_list)
    total_eps = conn.execute("SELECT COUNT(*) FROM episodes").fetchone()[0]

    # Stats
    has_trailer = sum(1 for s in series_list if s["trailer_url"])
    has_desc = sum(1 for s in series_list if s["description"])
    has_poster = sum(1 for s in series_list if s["poster_url"])
    thai_count = sum(1 for s in series_list if s["audio_type"] == "Thai")

    rows_html = ""
    for s in series_list:
        en_title, th_title = clean_title(s["title"])
        if not th_title:
            th_title = s["thai_title"] or ""

        # Poster
        poster = s["poster_url"] or ""
        poster_img = f'<img src="{poster}" width="50" loading="lazy" style="border-radius:4px;aspect-ratio:2/3;object-fit:cover">' if poster else '<div style="width:50px;height:75px;background:#222;border-radius:4px"></div>'

        # Trailer
        trailer = s["trailer_url"] or ""
        yt_id = ""
        if "youtube.com/watch?v=" in trailer:
            yt_id = trailer.split("v=")[1][:11]
        trailer_html = f'<a href="{trailer}" target="_blank" title="YouTube">{yt_id}</a>' if yt_id else '<span style="color:#555">-</span>'

        # Description
        desc_raw = s["description"] or ""
        desc_clean = html_escape.escape(desc_raw)
        desc_short = desc_clean[:60] + "..." if len(desc_clean) > 60 else desc_clean

        # Audio badge
        audio = s["audio_type"] or "-"
        if "Thai" in audio or "ไทย" in audio:
            audio_badge = '<span class="badge-thai">พากย์ไทย</span>'
        else:
            audio_badge = '<span class="badge-st">ซับไทย</span>'

        # Rating
        rating = s["rating"] if s["rating"] and s["rating"] < 100 else 0
        rating_html = f'<span class="rating"><span class="star">★</span> {rating}</span>' if rating else '-'

        # Episodes
        eps = conn.execute(
            "SELECT COUNT(*) as cnt FROM episodes WHERE series_id=?", (s["id"],)
        ).fetchone()["cnt"]

        # Completed
        status = "จบ" if s["is_completed"] else "ออกอยู่"
        status_cls = "done" if s["is_completed"] else "airing"

        rows_html += f"""
        <tr>
            <td class="poster-col">{poster_img}</td>
            <td>
                <div class="en-title">{html_escape.escape(en_title)}</div>
                <div class="th-title">{html_escape.escape(th_title)}</div>
            </td>
            <td class="center">{s["year"] or "-"}</td>
            <td class="center">{rating_html}</td>
            <td class="center">{audio_badge}</td>
            <td class="center">{eps} <span class="ep-status {status_cls}">{status}</span></td>
            <td class="center">{trailer_html}</td>
            <td class="desc-col" title="{desc_clean}">{desc_short if desc_raw else '<span style=\"color:#555\">-</span>'}</td>
        </tr>"""

    page = f"""<!DOCTYPE html>
<html lang="th">
<head>
<meta charset="UTF-8">
<title>Series Data Preview ({total} series)</title>
<style>
    * {{ margin:0; padding:0; box-sizing:border-box; }}
    body {{ font-family:'Segoe UI',sans-serif; background:#0a0a0a; color:#ddd; padding:20px; font-size:13px; }}
    h1 {{ color:#e50914; margin-bottom:8px; font-size:22px; }}
    .subtitle {{ color:#888; font-size:13px; margin-bottom:16px; }}
    .stats {{ display:flex; gap:12px; margin-bottom:16px; flex-wrap:wrap; }}
    .stat {{ background:#151515; padding:10px 16px; border-radius:8px; border:1px solid #282828; text-align:center; }}
    .stat .num {{ font-size:20px; font-weight:bold; color:#fff; }}
    .stat .label {{ font-size:11px; color:#777; }}
    table {{ width:100%; border-collapse:collapse; }}
    th {{ background:#151515; color:#888; padding:8px 6px; text-align:left; position:sticky; top:0; z-index:1; font-size:11px; text-transform:uppercase; letter-spacing:0.5px; }}
    td {{ padding:6px; border-bottom:1px solid #1a1a1a; vertical-align:middle; }}
    tr:hover {{ background:#151515; }}
    .poster-col {{ width:55px; }}
    .en-title {{ font-weight:600; color:#fff; font-size:13px; }}
    .th-title {{ color:#888; font-size:11px; margin-top:1px; }}
    .center {{ text-align:center; }}
    .badge-thai {{ background:#0369a1; color:#fff; padding:1px 6px; border-radius:3px; font-size:10px; white-space:nowrap; }}
    .badge-st {{ background:#444; color:#ccc; padding:1px 6px; border-radius:3px; font-size:10px; white-space:nowrap; }}
    .rating {{ color:#fbbf24; font-size:12px; }}
    .star {{ font-size:10px; }}
    .ep-status {{ font-size:10px; margin-left:2px; }}
    .ep-status.done {{ color:#22c55e; }}
    .ep-status.airing {{ color:#f59e0b; }}
    .desc-col {{ max-width:200px; font-size:11px; color:#666; }}
    a {{ color:#e50914; text-decoration:none; font-size:11px; font-family:monospace; }}
    a:hover {{ text-decoration:underline; }}
    #search {{ width:280px; padding:7px 12px; border-radius:6px; border:1px solid #333; background:#151515; color:#fff; font-size:13px; margin-bottom:12px; }}
    #search:focus {{ border-color:#e50914; outline:none; }}
    .filter-row {{ display:flex; gap:8px; margin-bottom:12px; align-items:center; }}
    select {{ padding:6px 10px; border-radius:6px; border:1px solid #333; background:#151515; color:#ccc; font-size:12px; }}
</style>
</head>
<body>
<h1>Series Data Preview</h1>
<p class="subtitle">ข้อมูลที่จะเอาเข้า SUEKK Stream — title แยก EN/TH สะอาด</p>

<div class="stats">
    <div class="stat"><div class="num">{total}</div><div class="label">Series</div></div>
    <div class="stat"><div class="num">{total_eps}</div><div class="label">Episodes</div></div>
    <div class="stat"><div class="num">{thai_count}</div><div class="label">พากย์ไทย</div></div>
    <div class="stat"><div class="num">{total - thai_count}</div><div class="label">ซับไทย</div></div>
    <div class="stat"><div class="num">{has_poster}</div><div class="label">Poster</div></div>
    <div class="stat"><div class="num">{has_trailer}</div><div class="label">Trailer</div></div>
    <div class="stat"><div class="num">{has_desc}</div><div class="label">Description</div></div>
</div>

<div class="filter-row">
    <input type="text" id="search" placeholder="ค้นหา..." oninput="filterTable()">
    <select id="audioFilter" onchange="filterTable()">
        <option value="">ทั้งหมด</option>
        <option value="Thai">พากย์ไทย</option>
        <option value="Sound Track">ซับไทย</option>
    </select>
</div>

<table id="dataTable">
<thead>
<tr>
    <th>Poster</th>
    <th>Title (EN / TH)</th>
    <th>Year</th>
    <th>Rating</th>
    <th>Audio</th>
    <th>Episodes</th>
    <th>Trailer</th>
    <th>Description</th>
</tr>
</thead>
<tbody>
{rows_html}
</tbody>
</table>

<script>
function filterTable() {{
    const q = document.getElementById('search').value.toLowerCase();
    const audio = document.getElementById('audioFilter').value;
    const rows = document.querySelectorAll('#dataTable tbody tr');
    rows.forEach(row => {{
        const text = row.textContent.toLowerCase();
        const matchSearch = text.includes(q);
        const matchAudio = !audio || row.innerHTML.includes(audio === 'Thai' ? 'badge-thai' : 'badge-st');
        row.style.display = matchSearch && matchAudio ? '' : 'none';
    }});
}}
</script>
</body>
</html>"""

    with open(OUTPUT, "w", encoding="utf-8") as f:
        f.write(page)

    print(f"Generated {OUTPUT}")
    print(f"  {total} series, {total_eps} episodes")
    print(f"  Thai: {thai_count} | Poster: {has_poster} | Trailer: {has_trailer} | Desc: {has_desc}")


if __name__ == "__main__":
    generate()
