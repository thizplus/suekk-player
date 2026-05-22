#!/bin/bash
# =====================================================
# HLS Integrity Checker
# ตรวจสอบว่า videos ที่ status=ready มีไฟล์ครบบน E2 หรือไม่
# =====================================================

# Config
BUCKET="suekk-01"
ALIAS="e2"

echo "=========================================="
echo "HLS Integrity Checker"
echo "=========================================="
echo ""

# ดึงรายการ video ที่ status=ready จาก database
echo "Fetching videos with status=ready..."
VIDEOS=$(docker exec suekk-postgres psql -U postgres -d suekk_stream -t -A -c "SELECT code FROM videos WHERE status = 'ready' ORDER BY created_at DESC;")

TOTAL=0
OK=0
BROKEN=0
BROKEN_LIST=""

for CODE in $VIDEOS; do
    TOTAL=$((TOTAL + 1))

    # ตรวจสอบว่ามี master.m3u8 หรือไม่
    MASTER=$(mcli ls ${ALIAS}/${BUCKET}/hls/${CODE}/master.m3u8 2>/dev/null | wc -l)

    if [ "$MASTER" -eq 0 ]; then
        echo "[$CODE] BROKEN - missing master.m3u8"
        BROKEN=$((BROKEN + 1))
        BROKEN_LIST="$BROKEN_LIST $CODE"
        continue
    fi

    # ตรวจสอบว่ามี segments ใน 480p หรือไม่
    SEGMENTS_480=$(mcli ls ${ALIAS}/${BUCKET}/hls/${CODE}/480p/ 2>/dev/null | grep -c "segment_")

    # ตรวจสอบว่ามี segments ใน 720p หรือไม่
    SEGMENTS_720=$(mcli ls ${ALIAS}/${BUCKET}/hls/${CODE}/720p/ 2>/dev/null | grep -c "segment_")

    if [ "$SEGMENTS_480" -eq 0 ] && [ "$SEGMENTS_720" -eq 0 ]; then
        echo "[$CODE] BROKEN - no segments found (480p: $SEGMENTS_480, 720p: $SEGMENTS_720)"
        BROKEN=$((BROKEN + 1))
        BROKEN_LIST="$BROKEN_LIST $CODE"
    else
        echo "[$CODE] OK (480p: $SEGMENTS_480 segments, 720p: $SEGMENTS_720 segments)"
        OK=$((OK + 1))
    fi
done

echo ""
echo "=========================================="
echo "Summary"
echo "=========================================="
echo "Total videos (ready): $TOTAL"
echo "OK: $OK"
echo "BROKEN: $BROKEN"
echo ""

if [ "$BROKEN" -gt 0 ]; then
    echo "Broken video codes:"
    echo "$BROKEN_LIST" | tr ' ' '\n' | grep -v '^$'
    echo ""
    echo "To fix: Re-transcode these videos or update status to 'failed'"
fi
