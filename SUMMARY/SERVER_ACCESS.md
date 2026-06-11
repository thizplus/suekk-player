# Server Access — SUEKK Stream (Hetzner)

## Server Info

| Item | Value |
|------|-------|
| Provider | Hetzner Cloud |
| Project | SUEKK PLAYER |
| IP | 5.223.46.39 |
| OS | Ubuntu 24.04.3 LTS |
| Location | Singapore (sin) |
| Hostname | ubuntu-16gb-sin-1 |
| RAM | 16 GB |
| Disk | 150 GB (used ~21%) |

## SSH Access

```bash
ssh -i ~/.ssh/id_ed25519_suekk root@5.223.46.39
```

| Item | Value |
|------|-------|
| SSH Key | `~/.ssh/id_ed25519_suekk` |
| User | root |
| Port | 22 (default) |

## Project Location

```
/opt/suekk-stream/              — project root (git clone from github.com/thizplus/suekk-player)
/opt/suekk-stream/docker-compose.yml  — main docker-compose (API + Frontend + NATS + DB)
/opt/suekk-stream/_gofiber_starter/   — Go API source
/opt/suekk-stream/_vite_starter/      — React frontend source
/opt/suekk-stream/_my_worker/         — Worker source (subtitle Python workers run on local PC)
```

## Docker Services

| Container | Image | Port | Domain | Status |
|-----------|-------|------|--------|--------|
| suekk-api | Go Fiber | 8080 | api.suekk.com | healthy |
| suekk-frontend | React + Nginx | 3000 | player.suekk.com | healthy |
| suekk-nats | nats:2.10-alpine | 4222, 8222 | internal | healthy |
| suekk-postgres | postgres:13-alpine | 5432 | internal | healthy |
| suekk-pgbouncer | pgbouncer:latest | 6432 | internal | healthy |
| suekk-redis | redis:7-alpine | 6379 | internal | healthy |

## Public Endpoints

| Service | URL |
|---------|-----|
| API | https://api.suekk.com |
| Frontend (Admin) | https://player.suekk.com |
| CDN | https://cdn.suekk.com |
| NATS Monitor | http://5.223.46.39:8222 |

## Common Commands

```bash
# SSH
ssh -i ~/.ssh/id_ed25519_suekk root@5.223.46.39

# Docker status
docker ps --format 'table {{.Names}}\t{{.Status}}'

# Logs
docker logs suekk-api --tail 50 -f
docker logs suekk-frontend --tail 50 -f
docker logs suekk-nats --tail 50 -f

# Health check
curl http://localhost:8080/health

# NATS connections
curl http://localhost:8222/connz
curl http://localhost:8222/jsz?streams=true

# Database
docker exec -it suekk-postgres psql -U postgres -d suekk_stream
```

## Deploy Workflow

```bash
# 1. Local: commit + push
cd "D:/Admin/Desktop/MY PROJECT/___SUEKK_STREAM"
git add . && git commit -m "message" && git push

# 2. Server: pull + rebuild
ssh -i ~/.ssh/id_ed25519_suekk root@5.223.46.39
cd /opt/suekk-stream && git pull && docker compose up -d --build api frontend

# Or one-liner from local:
ssh -i ~/.ssh/id_ed25519_suekk root@5.223.46.39 "cd /opt/suekk-stream && git pull && docker compose up -d --build api frontend"
```

## Host Services (Not in Docker)

| Service | Runs On | Why |
|---------|---------|-----|
| Transcode Worker (Go) | Local PC | Needs GPU (NVENC) |
| Subtitle Workers (Python) | Local PC | Needs GPU (CUDA/Whisper) |

Workers connect to server via:
- NATS: `nats://5.223.46.39:4222`
- API: `https://api.suekk.com`

## SSH Config (Add to ~/.ssh/config)

```
Host suekk
    HostName 5.223.46.39
    User root
    IdentityFile ~/.ssh/id_ed25519_suekk
```

After adding, connect with just: `ssh suekk`
