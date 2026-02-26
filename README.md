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

**Server**

- Go 1.24 + [Echo](https://echo.labstack.com/) v4
- PostgreSQL 16
- Local filesystem blob store

**Web Frontend**

- React 19 + TypeScript
- [TanStack Router](https://tanstack.com/router) + [TanStack Query](https://tanstack.com/query)
- Tailwind CSS 4 + [daisyUI](https://daisyui.com/) 5
- Vite 7

---

## Getting Started

```bash
docker compose up -d
```

This starts both **PostgreSQL** and the **hermit server** (with the web frontend bundled in). The database schema is auto-applied on first run.

Once ready, open `http://localhost:8080` in your browser.

Default credentials:

| | Value |
|---|---|
| Admin UI login | `admin` / `changeme123` |
| Admin API token | `dev-admin-token` |

### Configuration

Override any setting via environment variables in `docker-compose.yml` under the `server` service, or mount a `.env` file. Key variables:

| Variable | Description |
|----------|-------------|
| `ADMIN_TOKEN` | Admin bearer token (bootstrap + token minting) |
| `ADMIN_USERNAME` / `ADMIN_PASSWORD` | Initial admin account (created on first startup) |
| `CORS_ALLOWED_ORIGINS` | Allowed browser origins, comma-separated |
| `PROXY_UPSTREAM_URL` | Single upstream URL for proxy repo |
| `PROXY_UPSTREAM_URLS` | Multiple upstreams (comma / newline / semicolon separated). Creates `proxy`, `proxy-2`, … and attaches all to `group` by priority |
| `DEFAULT_HOSTED_REPO` / `DEFAULT_PROXY_REPO` / `DEFAULT_GROUP_REPO` | Default repository names |
| `PROXY_SYNC_PAGE_SIZE` | Page size for manual sync crawling |
| `PROXY_SYNC_CONCURRENCY` | Max concurrent version warmups per skill |
| `STORAGE_BACKEND` | `local` (default) or `s3` for S3/MinIO |

See [`.env.example`](.env.example) for the full list.

### Development (without Docker)

To run the server and frontend separately for local development:

```bash
# 1. Start Postgres only
docker compose up -d postgres

# 2. Copy and edit environment config
cp .env.example .env

# 3. Run the Go server
go run ./cmd/server

# 4. Run the frontend dev server (in another terminal)
cd web && pnpm install && pnpm dev
```

The frontend dev server runs on `http://localhost:5173` and proxies `/api` requests to the backend.

---

## Web Frontend

The bundled SPA provides a full-featured UI for browsing, publishing, and managing skills.

### Pages

| Route | Description |
|-------|-------------|
| `/` | Skill catalog with search, sort, and pagination |
| `/skills/:slug` | Skill detail — files, version compare, version history |
| `/search?q=...` | Full-text search |
| `/publish` | Publish a new skill version (drag-and-drop file upload) |
| `/admin` | Admin dashboard (stats, skills, sync, RBAC, users, auth, tokens) |

### Skill Detail Tabs

- **Files** — file explorer with syntax-highlighted preview for source files and rendered Markdown for `.md` files.
- **Compare** — side-by-side unified diff between any two versions. Defaults to latest vs previous; shows added / removed / modified files with line-level changes.
- **Versions** — chronological version history with changelogs.

### Admin Dashboard

- **Stats** — total skills, versions, downloads, stars, installs, and per-repository breakdown.
- **Skills** — manage all skills (soft-delete / restore).
- **Sync** — configure proxy sync sources, trigger manual sync, view sync status and config.
- **RBAC** — assign per-repository roles (`admin` / `developer` / `viewer`).
- **Users** — create, update, disable local users and reset passwords.
- **Auth** — configure authentication providers (local + LDAP).
- **Tokens** — mint admin tokens for any user.

### Theming

Light (`hermit-light`) and dark (`hermit-dark`) themes with automatic system preference detection and manual toggle.

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

### Auth Endpoints (No Token Required)

```
GET  /api/v1/auth/providers                         List auth providers
POST /api/v1/auth/login                             Local login
POST /api/v1/auth/ldap                              LDAP login
```

### Authenticated Endpoints

> Requires `Authorization: Bearer <token>` header.

```
GET    /api/v1/whoami                   Token introspection
POST   /api/v1/skills                   Publish a skill
DELETE /api/v1/skills/{slug}            Soft-delete a skill
POST   /api/v1/skills/{slug}/undelete   Restore a soft-deleted skill
GET    /api/v1/tokens                   List personal access tokens
POST   /api/v1/tokens                   Create personal access token
DELETE /api/v1/tokens/{tokenId}         Revoke personal access token
```

### Internal (Admin Only)

```
POST   /api/internal/tokens                              Mint a user token
GET    /api/internal/stats                               Dashboard stats
GET    /api/internal/sync-sources                        List sync sources
POST   /api/internal/sync-sources                        Add sync source
DELETE /api/internal/sync-sources/{id}                   Remove sync source
PATCH  /api/internal/sync-sources/{id}                   Toggle sync source
POST   /api/internal/sync                                Trigger sync
GET    /api/internal/sync/status                         Sync status
GET    /api/internal/sync/config                         Get sync config
PUT    /api/internal/sync/config                         Update sync config
GET    /api/internal/rbac/members                        List all RBAC members
GET    /api/internal/rbac/repos/{id}/members             List repo members
POST   /api/internal/rbac/repos/{id}/members             Assign member role
DELETE /api/internal/rbac/repos/{id}/members/{subject}   Remove member
GET    /api/internal/users                               List users
POST   /api/internal/users                               Create user
PATCH  /api/internal/users/{id}                          Update user
POST   /api/internal/users/{id}/reset-password           Reset password
DELETE /api/internal/users/{id}                          Delete user
GET    /api/internal/auth-configs                        List auth configs
GET    /api/internal/auth-configs/{type}                 Get auth config
PUT    /api/internal/auth-configs/{type}                 Save auth config
DELETE /api/internal/auth-configs/{type}                 Delete auth config
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
- **Manual sync** — admins can trigger a one-off upstream catalog sync via the "Sync Now" button; page size and concurrency are configurable in the Admin UI.
- **Client-side version diff** — the Compare tab fetches file content for two versions and computes unified diffs in the browser using the `diff` library, requiring no server-side diff endpoint.
