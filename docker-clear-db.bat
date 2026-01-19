@echo off
echo ========================================
echo   Clear PostgreSQL Videos Table
echo ========================================
echo.

docker-compose exec postgres psql -U postgres -d suekk_stream -c "TRUNCATE videos CASCADE;"

echo.
echo Done! Videos table has been cleared.
pause
