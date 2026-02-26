<p align="center">
  <img src="assets/hermit-crab.svg" width="128" alt="hermit" />
</p>

<h1 align="center">Hermit</h1>

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

- Go 1.24 + [Echo](https://echo.labstack.com/) v4
- PostgreSQL 16
- Local filesystem or S3/MinIO blob store
- React 19 + TypeScript + Tailwind CSS 4 + [daisyUI](https://daisyui.com/) 5 (bundled SPA)

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

Override settings via environment variables in `docker-compose.yml` or by mounting a `.env` file. See [`.env.example`](.env.example) for the full list.

### Development (without Docker)

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

## Features

### Skill Catalog

- Full-text search across skill names, display names, and summaries.
- Paginated skill listing with cursor-based pagination.
- Sortable by: updated, downloads, stars, installs (current / all-time), trending.
- Skill detail with file explorer, syntax-highlighted source preview, and rendered Markdown.
- Side-by-side unified diff between any two versions (added / removed / modified files with line-level changes).
- Chronological version history with changelogs.

### Publishing

- Publish skill versions via multipart upload. Each version is an immutable zip archive; republishing the same `slug + version` returns `409 Conflict`.
- `SKILL.md` manifest required. Files are sorted, archived, and stored with per-file SHA-256 descriptors.
- Tag support (`latest` is always set; additional custom tags can be provided).

### Proxy & Sync

- **Lazy cache** — upstream skills are fetched on first download request and cached locally. A negative cache with configurable TTL prevents repeated misses.
- **Incremental sync** — admins can trigger a manual catalog sync. The skills list is always fetched from upstream, but for each skill whose latest version already exists locally, the per-version fetch and download loop is skipped. Only skills with new upstream versions trigger a full version sync. Metadata (display name, summary, tags) is always refreshed regardless. This reduces upstream API calls from O(N) to O(changed) on subsequent syncs.
- **Sync configuration** — page size and concurrency are configurable via the Admin UI or API. Sync sources can be added, removed, and toggled independently.
- **Rate limit aware** — the sync client respects upstream rate limits, preferring `Retry-After`, then `RateLimit-Reset`, then `X-RateLimit-Reset`, with jittered retries on `429`.
- **Structured logging** — detailed `[sync]` prefixed logs trace every stage: repo discovery, per-skill decisions (skipped / synced / failed), rate-limit retries, and final summary with skills / versions / cached / failed / skipped counts.

### Authentication

- **Local accounts** — username/password login with bcrypt hashing. Admins can create, update, disable users, and reset passwords.
- **LDAP** — configurable LDAP authentication with bind DN, user filter, group-based admin mapping, and optional StartTLS.
- **API tokens** — bearer token authentication. Users can self-service their personal access tokens; admins can mint tokens for any user.
- **Self-service** — authenticated users can change their own password via `/api/v1/account/change-password`.

### RBAC

Per-repository role assignment with three levels:

| Role | Permissions |
|------|-------------|
| `read` | Download and browse skills |
| `push` | Publish skills (includes read) |
| `admin` | Full control (includes push) |

Admin users bypass RBAC checks. Anonymous requests use the Group repository's public access.

### Rate Limiting

Rate limiting is auth-aware — anonymous requests are bucketed by IP, authenticated requests by user.

| Scope | Read | Write |
|-------|------|-------|
| Anonymous (per IP) | 120 req/min | 30 req/min |
| Authenticated (per key) | 600 req/min | 120 req/min |

Response headers: `X-RateLimit-Limit` / `X-RateLimit-Remaining` / `X-RateLimit-Reset` (epoch seconds), `RateLimit-Limit` / `RateLimit-Remaining` / `RateLimit-Reset` (delay seconds), `Retry-After` on `429 Too Many Requests`.

### Admin Dashboard

- **Stats** — total skills, versions, downloads, stars, installs, and per-repository breakdown.
- **Skills** — manage all skills (soft-delete / restore).
- **Sync** — configure sync sources, trigger incremental sync, view sync status and config.
- **RBAC** — assign per-repository roles.
- **Users** — create, update, disable local users and reset passwords.
- **Auth** — configure authentication providers (local + LDAP).
- **Tokens** — mint admin tokens for any user.
- **Theming** — light and dark themes with automatic system preference detection and manual toggle.

---

## API

hermit is fully compatible with the [ClawHub](https://clawhub.ai) API — any ClawHub-compatible CLI or client works without modification. The server also exposes a `/.well-known/clawhub.json` discovery document and a `/healthz` health-check endpoint.

### ClawHub CLI Examples

Point the CLI at your hermit instance via the `--registry` flag or `CLAWHUB_REGISTRY` environment variable:

```bash
# Option A: pass --registry on every command
clawhub --registry http://localhost:8080 login --token dev-admin-token

# Option B: set once via environment variable
export CLAWHUB_REGISTRY=http://localhost:8080
clawhub login --token dev-admin-token
```

```bash
# Search and install
clawhub search "code review"
clawhub install my-skill
clawhub install my-skill --version 1.2.0

# Manage installed skills
clawhub list
clawhub update --all
clawhub uninstall my-skill

# Publish
clawhub publish ./my-skill --version 1.0.0
```
