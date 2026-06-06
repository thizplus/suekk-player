@echo off
echo Starting all subtitle workers...

start "Subtitle Detect" cmd /k "cd /d %~dp0python\subtitle_detect && python main.py"
start "Subtitle Transcribe" cmd /k "cd /d %~dp0python\subtitle_transcribe && python main.py"
start "Subtitle Translate" cmd /k "cd /d %~dp0python\subtitle_translate && python main.py"

echo All subtitle workers started in separate windows.
