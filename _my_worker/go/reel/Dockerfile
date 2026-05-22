# Reel Worker Dockerfile
FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /reel ./cmd/main.go

# Runtime with FFmpeg
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata ffmpeg

WORKDIR /app

COPY --from=builder /reel .

CMD ["./reel"]
