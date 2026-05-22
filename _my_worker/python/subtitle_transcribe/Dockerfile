# Subtitle Transcribe Worker Dockerfile
FROM python:3.11-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt /app/
RUN pip install --no-cache-dir -r requirements.txt

COPY shared/ /app/shared/
COPY subtitle_transcribe/ /app/subtitle_transcribe/

WORKDIR /app/subtitle_transcribe

CMD ["python", "main.py"]
