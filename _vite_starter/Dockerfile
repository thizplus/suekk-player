# =============================================================================
# SUEKK Frontend - Production Dockerfile
# Multi-stage build: Node (build) -> Nginx (serve)
# =============================================================================

# Stage 1: Builder
FROM node:20-alpine AS builder
WORKDIR /app

# Install dependencies first (for better caching)
COPY package*.json ./
RUN npm ci

# Copy source code
COPY . .

# Build-time environment variables
ARG VITE_API_URL
ARG VITE_STREAM_URL
ARG VITE_WS_URL
ARG VITE_APP_TITLE="Suekk Stream"

ENV VITE_API_URL=$VITE_API_URL
ENV VITE_STREAM_URL=$VITE_STREAM_URL
ENV VITE_WS_URL=$VITE_WS_URL
ENV VITE_APP_TITLE=$VITE_APP_TITLE

# Build static files
RUN npm run build

# =============================================================================
# Stage 2: Production (Nginx)
# =============================================================================
FROM nginx:alpine

# Install wget for healthcheck
RUN apk --no-cache add wget

# Copy built files from builder
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy nginx configuration
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:80/ || exit 1

CMD ["nginx", "-g", "daemon off;"]
