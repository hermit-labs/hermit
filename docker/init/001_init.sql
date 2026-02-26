CREATE EXTENSION IF NOT EXISTS pgcrypto;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'repo_type') THEN
    CREATE TYPE repo_type AS ENUM ('hosted', 'proxy', 'group');
  END IF;
END$$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'repo_role') THEN
    CREATE TYPE repo_role AS ENUM ('read', 'push', 'admin');
  END IF;
END$$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'proxy_cache_status') THEN
    CREATE TYPE proxy_cache_status AS ENUM ('cached', 'not_found', 'error');
  END IF;
END$$;

CREATE TABLE IF NOT EXISTS repositories (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL UNIQUE,
  type repo_type NOT NULL,
  upstream_url TEXT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT repositories_type_upstream_ck CHECK (
    (type = 'proxy' AND upstream_url IS NOT NULL)
    OR (type IN ('hosted', 'group') AND upstream_url IS NULL)
  )
);

CREATE TABLE IF NOT EXISTS repo_members (
  repo_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  subject TEXT NOT NULL,
  role repo_role NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (repo_id, subject)
);

CREATE TABLE IF NOT EXISTS group_members (
  group_repo_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  member_repo_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  priority INTEGER NOT NULL DEFAULT 100,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (group_repo_id, member_repo_id),
  CONSTRAINT group_members_self_ck CHECK (group_repo_id <> member_repo_id)
);

CREATE TABLE IF NOT EXISTS packages (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  repo_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '',
  summary TEXT NULL,
  tags JSONB NOT NULL DEFAULT '{}'::jsonb,
  downloads BIGINT NOT NULL DEFAULT 0,
  stars BIGINT NOT NULL DEFAULT 0,
  installs_current BIGINT NOT NULL DEFAULT 0,
  installs_all_time BIGINT NOT NULL DEFAULT 0,
  trending_score DOUBLE PRECISION NOT NULL DEFAULT 0,
  created_by TEXT NOT NULL DEFAULT '',
  deleted_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (repo_id, name)
);

CREATE INDEX IF NOT EXISTS idx_packages_repo_updated ON packages(repo_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_packages_repo_name ON packages(repo_id, name);
CREATE INDEX IF NOT EXISTS idx_packages_repo_deleted ON packages(repo_id, deleted_at);

CREATE TABLE IF NOT EXISTS versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  package_id UUID NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
  version TEXT NOT NULL,
  digest TEXT NOT NULL,
  size_bytes BIGINT NOT NULL,
  changelog TEXT NOT NULL DEFAULT '',
  changelog_source TEXT NULL,
  files JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_by TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (package_id, version)
);

CREATE INDEX IF NOT EXISTS idx_versions_package_created ON versions(package_id, created_at DESC);

CREATE TABLE IF NOT EXISTS assets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  version_id UUID NOT NULL REFERENCES versions(id) ON DELETE CASCADE,
  path TEXT NOT NULL,
  blob_path TEXT NOT NULL,
  size_bytes BIGINT NOT NULL,
  digest TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (version_id, path)
);

CREATE TABLE IF NOT EXISTS proxy_cache (
  repo_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  package_name TEXT NOT NULL,
  version TEXT NOT NULL,
  status proxy_cache_status NOT NULL,
  etag TEXT NULL,
  expires_at TIMESTAMPTZ NULL,
  last_error TEXT NULL,
  last_checked TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (repo_id, package_name, version)
);

CREATE TABLE IF NOT EXISTS api_tokens (
  token_hash TEXT PRIMARY KEY,
  subject TEXT NOT NULL UNIQUE,
  disabled BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
