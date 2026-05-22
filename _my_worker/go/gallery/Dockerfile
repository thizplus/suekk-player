# Gallery Worker Dockerfile
# Includes Python for NSFW classification

FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /gallery ./cmd/main.go

# Runtime with FFmpeg and Python
FROM python:3.11-alpine

RUN apk --no-cache add ca-certificates tzdata ffmpeg

# Install Python dependencies for classifier
COPY ../python/requirements.txt /tmp/
RUN pip install --no-cache-dir nudenet opencv-python-headless pillow

WORKDIR /app

COPY --from=builder /gallery .

# Copy classifier script
COPY ../python/classify_batch.py /app/classify_batch.py

ENV CLASSIFIER_SCRIPT=/app/classify_batch.py

CMD ["./gallery"]
