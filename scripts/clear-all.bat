@echo off
REM Clear all data for testing - PostgreSQL, MinIO, NATS, Worker logs

echo =========================================
echo   Clearing ALL data for fresh testing
echo =========================================

cd /d "%~dp0.."

echo.
echo [1/5] Stopping NATS and MinIO...
docker-compose stop nats minio minio-setup 2>nul

echo [2/5] Removing containers...
docker-compose rm -f nats minio minio-setup 2>nul

echo [3/5] Removing volumes...
docker volume rm suekk_stream_nats-data 2>nul
docker volume rm suekk_stream_minio-data 2>nul

echo [4/5] Clearing PostgreSQL...
docker exec suekk_stream-postgres-1 psql -U postgres -d suekk_stream -c "TRUNCATE videos CASCADE;" 2>nul

echo [5/5] Clearing worker logs...
if exist "_worker\logs" rd /s /q "_worker\logs" 2>nul
mkdir "_worker\logs" 2>nul

echo.
echo Restarting services...
docker-compose up -d nats minio minio-setup
timeout /t 3 /nobreak >nul
docker-compose restart api

echo.
echo =========================================
echo   All data cleared successfully!
echo =========================================
echo.
echo Next steps:
echo   1. Restart worker: cd _worker ^&^& go run ./cmd/worker
echo   2. Upload videos
echo   3. Queue pending videos
