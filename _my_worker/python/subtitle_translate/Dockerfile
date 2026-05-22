# Subtitle Translate Worker Dockerfile
FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt /app/
RUN pip install --no-cache-dir -r requirements.txt

COPY shared/ /app/shared/
COPY subtitle_translate/ /app/subtitle_translate/

WORKDIR /app/subtitle_translate

CMD ["python", "main.py"]
