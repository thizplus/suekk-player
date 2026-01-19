@echo off
echo ========================================
echo   SUEKK STREAM - Docker Commands
echo ========================================
echo.
echo 1. Start all services
echo 2. Stop all services
echo 3. Restart API
echo 4. Restart Worker
echo 5. Restart Frontend
echo 6. View API logs
echo 7. View Worker logs
echo 8. Clear database (videos)
echo 9. Clear ALL data (DANGER!)
echo 0. Exit
echo.

set /p choice="Enter choice: "

if "%choice%"=="1" docker-compose up -d
if "%choice%"=="2" docker-compose down
if "%choice%"=="3" docker-compose restart api
if "%choice%"=="4" docker-compose restart worker
if "%choice%"=="5" docker-compose restart frontend
if "%choice%"=="6" docker-compose logs -f api
if "%choice%"=="7" docker-compose logs -f worker
if "%choice%"=="8" docker-compose exec postgres psql -U postgres -d suekk_stream -c "TRUNCATE videos CASCADE;"
if "%choice%"=="9" (
    echo WARNING: This will delete ALL data!
    set /p confirm="Type 'yes' to confirm: "
    if "%confirm%"=="yes" docker-compose down -v
)
if "%choice%"=="0" exit

pause
