#!/bin/bash
# Clear all data for testing - PostgreSQL, MinIO, NATS, Worker logs

set -e

echo "========================================="
echo "  Clearing ALL data for fresh testing"
echo "========================================="

cd "$(dirname "$0")/.."

# 1. Stop services that need volume removal
echo ""
echo "[1/5] Stopping NATS and MinIO..."
docker-compose stop nats minio minio-setup 2>/dev/null || true

# 2. Remove containers to release volumes
echo "[2/5] Removing containers..."
docker-compose rm -f nats minio minio-setup 2>/dev/null || true

# 3. Remove volumes
echo "[3/5] Removing volumes..."
docker volume rm suekk_stream_nats-data 2>/dev/null || true
docker volume rm suekk_stream_minio-data 2>/dev/null || true

# 4. Clear PostgreSQL
echo "[4/5] Clearing PostgreSQL..."
docker exec suekk_stream-postgres-1 psql -U postgres -d suekk_stream -c "TRUNCATE videos CASCADE;" 2>/dev/null || echo "PostgreSQL not running or already clear"

# 5. Clear worker logs
echo "[5/5] Clearing worker logs..."
rm -rf _worker/logs/* 2>/dev/null || true

# 6. Restart services
echo ""
echo "Restarting services..."
docker-compose up -d nats minio minio-setup
sleep 3
docker-compose restart api

echo ""
echo "========================================="
echo "  All data cleared successfully!"
echo "========================================="
echo ""
echo "Next steps:"
echo "  1. Restart worker: cd _worker && go run ./cmd/worker"
echo "  2. Upload videos"
echo "  3. Queue pending videos"
