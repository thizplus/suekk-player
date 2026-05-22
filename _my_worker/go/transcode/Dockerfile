# Transcode Worker Dockerfile
# NOTE: For GPU (NVENC) support, run on host instead of Docker

FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /transcode ./cmd/main.go

# Runtime with FFmpeg
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata ffmpeg

WORKDIR /app

COPY --from=builder /transcode .

# Default: CPU mode
ENV USE_NVENC=false

CMD ["./transcode"]
