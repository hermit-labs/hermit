# hermit

Private ClawHub-compatible registry server with Repository Trio semantics:
- Hosted: private publish target.
- Proxy: upstream lazy cache.
- Group: single read endpoint (Hosted + Proxy).

Stack:
- Backend: Go + Echo
- DB: PostgreSQL
- Storage: local filesystem blob store

## 1) Start Postgres

```bash
docker compose up -d postgres
```

`docker/init/001_init.sql` is auto-applied on first DB initialization.

## 2) Configure env

```bash
cp .env.example .env
```

Key vars:
- `ADMIN_TOKEN`: admin bearer token.
- `CORS_ALLOWED_ORIGINS`: allowed browser origins for API requests.
- `PROXY_UPSTREAM_URL`: optional single upstream for proxy repo.
- `PROXY_UPSTREAM_URLS`: optional multiple upstreams (comma/newline/semicolon separated). If set, bootstrap creates `proxy`, `proxy-2`, `proxy-3`... and attaches all to `group` by priority.
- `DEFAULT_HOSTED_REPO` / `DEFAULT_PROXY_REPO` / `DEFAULT_GROUP_REPO`.
- `PROXY_SYNC_ENABLED`: enable background sync from upstream catalog into proxy cache.
- `PROXY_SYNC_INTERVAL`: periodic sync interval.
- `PROXY_SYNC_DELAY`: startup delay before first sync.
- `PROXY_SYNC_PAGE_SIZE`: page size for upstream `/api/v1/skills` and `/versions` crawling.
- `PROXY_SYNC_CONCURRENCY`: max concurrent version warmups per skill during sync.

## 3) Run server

```bash
go run ./cmd/server
```

Server default: `http://localhost:8080` (the server auto-loads `.env` from project root at startup).

## 4) Run web frontend (optional)

```bash
cd web
npm install
npm run dev
```

Frontend runs on `http://localhost:5173` and proxies `/api` to the backend.

See `web/README.md` for details on the React + TanStack frontend.

## API (ClawHub-style)

Public:
- `GET /api/v1/search?q=...`
- `GET /api/v1/skills?limit=&cursor=&sort=`
- `GET /api/v1/skills/{slug}`
- `GET /api/v1/skills/{slug}/versions?limit=&cursor=`
- `GET /api/v1/skills/{slug}/versions/{version}`
- `GET /api/v1/skills/{slug}/file?path=&version=&tag=`
- `GET /api/v1/resolve?slug=&hash=`
- `GET /api/v1/download?slug=&version=&tag=`

Auth required:
- `POST /api/v1/skills`
- `DELETE /api/v1/skills/{slug}`
- `POST /api/v1/skills/{slug}/undelete`
- `GET /api/v1/whoami`

Internal admin helper:
- `POST /api/internal/tokens` (admin only): mint user token.

## Rate Limits

Auth-aware enforcement:
- Anonymous: per IP bucket.
- Valid bearer token: per user bucket.
- Missing/invalid token: fallback to IP bucket.

Limits:
- Read: `120/min` per IP, `600/min` per key.
- Write: `30/min` per IP, `120/min` per key.

Headers:
- `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset` (epoch seconds)
- `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset` (delay seconds)
- `Retry-After` on `429`

The proxy sync client respects upstream rate limits:
- Prefer `Retry-After`
- Fallback to `RateLimit-Reset`
- Fallback to `X-RateLimit-Reset`
- Retries with jitter on `429`

## Quick usage

### 1) Whoami (admin token)

```bash
curl -s http://localhost:8080/api/v1/whoami \
  -H "Authorization: Bearer dev-admin-token"
```

### 2) Publish skill

```bash
mkdir -p /tmp/demo-skill
cat >/tmp/demo-skill/SKILL.md <<'EOF'
# Demo Skill
Hello from hermit.
EOF

curl -s http://localhost:8080/api/v1/skills \
  -H "Authorization: Bearer dev-admin-token" \
  -F 'payload={"slug":"demo-skill","displayName":"Demo Skill","version":"1.0.0","changelog":"init","tags":["latest"]}' \
  -F "files=@/tmp/demo-skill/SKILL.md;filename=SKILL.md"
```

### 3) Download

```bash
curl -L -o /tmp/demo-skill.zip \
  "http://localhost:8080/api/v1/download?slug=demo-skill&version=1.0.0"
```

### 4) Inspect text file

```bash
curl -s \
  "http://localhost:8080/api/v1/skills/demo-skill/file?path=SKILL.md&version=1.0.0"
```

## Notes

- Version is immutable: publishing same `slug + version` returns conflict.
- Proxy repository uses lazy cache + negative cache.
- Group repository is the single read entrance.
- Hosted and Proxy are internal implementation details; public API always reads through Group.
- If `PROXY_SYNC_ENABLED=true`, the worker proactively syncs skills/versions from upstream instead of waiting for first download hit.
