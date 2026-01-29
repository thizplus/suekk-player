# _api/Dockerfile
# ═══════════════════════════════════════════════════════════
# API Server - Go Fiber
# ❌ ห้ามติดตั้ง FFmpeg ที่นี่!
# ═══════════════════════════════════════════════════════════

FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies (NO FFmpeg!)
RUN apk add --no-cache git ca-certificates

# Use Go module proxy for faster downloads
ENV GOPROXY=https://proxy.golang.org,direct

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api ./cmd/api

# ═══════════════════════════════════════════════════════════
# Production image - Minimal, NO FFmpeg!
# ═══════════════════════════════════════════════════════════
FROM alpine:3.19

# Basic dependencies only (NO FFmpeg!)
RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /app

COPY --from=builder /app/api .

# Create logs directory and non-root user
RUN mkdir -p /app/logs && \
    adduser -D -u 1000 api && \
    chown -R api:api /app

USER api

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./api"]