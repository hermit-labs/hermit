<p align="center">
  <img src="assets/hermit-crab.svg" width="128" alt="hermit" />
</p>

<h1 align="center">hermit</h1>

<p align="center">
  Private, self-hosted skill registry server — ClawHub-compatible, with Repository Trio semantics.
</p>

---

## Overview

hermit is a lightweight registry that lets teams **publish, proxy, and discover** AI agent skills behind their own infrastructure. It speaks the same API as [ClawHub](https://clawhub.ai), so any compatible CLI or client works out of the box.

### Repository Trio

Every hermit instance bootstraps three repository types that work together:

| Type | Role |
|------|------|
| **Hosted** | Private publish target — your first-party skills live here. |
| **Proxy** | Lazy-caching mirror of one or more upstreams (e.g. ClawHub). Fetches on first hit, then serves from local cache. |
| **Group** | Unified read endpoint that merges Hosted + Proxy. All public API reads go through Group. |

### Tech Stack

- **Language**: Go 1.24 + [Echo](https://echo.labstack.com/) v4
- **Database**: PostgreSQL 16
- **Storage**: Local filesystem blob store

---

## Getting Started

### 1. Start Postgres

```bash
docker compose up -d postgres
```

The init script `docker/init/001_init.sql` is auto-applied on first database creation.

### 2. Configure Environment

```bash
cp .env.example .env
# edit .env as needed
```

The server auto-loads `.env` from the project root at startup. Key variables:

| Variable | Description |
|----------|-------------|
| `ADMIN_TOKEN` | Admin bearer token (bootstrap + token minting) |
| `CORS_ALLOWED_ORIGINS` | Allowed browser origins, comma-separated |
| `PROXY_UPSTREAM_URL` | Single upstream URL for proxy repo |
| `PROXY_UPSTREAM_URLS` | Multiple upstreams (comma / newline / semicolon separated). Creates `proxy`, `proxy-2`, … and attaches all to `group` by priority |
| `DEFAULT_HOSTED_REPO` / `DEFAULT_PROXY_REPO` / `DEFAULT_GROUP_REPO` | Default repository names |
| `PROXY_SYNC_ENABLED` | Enable background upstream catalog sync |
| `PROXY_SYNC_INTERVAL` | Sync interval (default `30m`) |
| `PROXY_SYNC_DELAY` | Startup delay before first sync |
| `PROXY_SYNC_PAGE_SIZE` | Page size for upstream crawling |
| `PROXY_SYNC_CONCURRENCY` | Max concurrent version warmups per skill |

See [`.env.example`](.env.example) for the full list.

### 3. Run Server

```bash
go run ./cmd/server
```

Default listen address: `http://localhost:8080`.

### 4. Run Web Frontend (optional)

```bash
cd ../web
npm install
npm run dev
```

Frontend runs on `http://localhost:5173` and proxies `/api` requests to the backend.

---

## API Reference

### Public Endpoints

```
GET  /api/v1/search?q=...                          Search skills
GET  /api/v1/skills?limit=&cursor=&sort=            List skills
GET  /api/v1/skills/{slug}                          Skill detail
GET  /api/v1/skills/{slug}/versions?limit=&cursor=  List versions
GET  /api/v1/skills/{slug}/versions/{version}       Version detail
GET  /api/v1/skills/{slug}/file?path=&version=&tag= Read text file
GET  /api/v1/resolve?slug=&hash=                    Resolve by hash
GET  /api/v1/download?slug=&version=&tag=           Download archive
```

### Authenticated Endpoints

> Requires `Authorization: Bearer <token>` header.

```
POST   /api/v1/skills                  Publish a skill
DELETE /api/v1/skills/{slug}           Soft-delete a skill
POST   /api/v1/skills/{slug}/undelete  Restore a soft-deleted skill
GET    /api/v1/whoami                  Token introspection
```

### Internal (Admin Only)

```
POST /api/internal/tokens              Mint a user token
```

---

## Rate Limits

Rate limiting is auth-aware — anonymous requests are bucketed by IP, authenticated requests by user.

| Scope | Read | Write |
|-------|------|-------|
| Anonymous (per IP) | 120 req/min | 30 req/min |
| Authenticated (per key) | 600 req/min | 120 req/min |

**Response headers:**

- `X-RateLimit-Limit` / `X-RateLimit-Remaining` / `X-RateLimit-Reset` (epoch seconds)
- `RateLimit-Limit` / `RateLimit-Remaining` / `RateLimit-Reset` (delay seconds)
- `Retry-After` on `429 Too Many Requests`

The proxy sync client also respects upstream rate limits — preferring `Retry-After`, then `RateLimit-Reset`, then `X-RateLimit-Reset`, with jittered retries on `429`.

---

## Quick Usage

### Check identity

```bash
curl -s http://localhost:8080/api/v1/whoami \
  -H "Authorization: Bearer dev-admin-token"
```

### Publish a skill

```bash
mkdir -p /tmp/demo-skill
cat > /tmp/demo-skill/SKILL.md <<'EOF'
# Demo Skill
Hello from hermit.
EOF

curl -s http://localhost:8080/api/v1/skills \
  -H "Authorization: Bearer dev-admin-token" \
  -F 'payload={"slug":"demo-skill","displayName":"Demo Skill","version":"1.0.0","changelog":"init","tags":["latest"]}' \
  -F "files=@/tmp/demo-skill/SKILL.md;filename=SKILL.md"
```

### Download a skill

```bash
curl -L -o /tmp/demo-skill.zip \
  "http://localhost:8080/api/v1/download?slug=demo-skill&version=1.0.0"
```

### Read a file

```bash
curl -s \
  "http://localhost:8080/api/v1/skills/demo-skill/file?path=SKILL.md&version=1.0.0"
```

---

## Design Notes

- **Immutable versions** — publishing the same `slug + version` returns `409 Conflict`.
- **Proxy lazy cache** — upstream skills are fetched on first request and cached locally; a negative cache prevents repeated misses.
- **Group as single entrance** — Hosted and Proxy are internal; the public API always reads through the Group repository.
- **Background sync** — when `PROXY_SYNC_ENABLED=true`, a worker proactively crawls the upstream catalog into the proxy cache instead of waiting for the first download hit.
