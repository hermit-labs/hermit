package proxysync

import (
	"context"
	"time"

	"hermit/internal/store"
)

type RepoStats struct {
	Repository string
	Skills     int
	Versions   int
	Cached     int
	Failed     int
	Skipped    int
}

type Summary struct {
	Repositories int
	Skills       int
	Versions     int
	Cached       int
	Failed       int
	Skipped      int
	ByRepository []RepoStats
}

type RepositoryLister interface {
	ListProxyRepositories(context.Context) ([]store.Repository, error)
}

type VersionCacher interface {
	SyncProxyVersion(context.Context, store.Repository, string, string) error
}

type ProxySkillMetaCacher interface {
	SyncProxySkillMeta(context.Context, store.Repository, string, string, *string, map[string]string) error
}

type ProxyVersionMetaCacher interface {
	SyncProxyVersionMeta(context.Context, store.Repository, string, string, *time.Time, *string, *string) error
}

type VersionChecker interface {
	HasProxyVersion(ctx context.Context, repo store.Repository, slug, version string) bool
}

type RepoSyncer interface {
	Sync(context.Context, int) (RepoStats, error)
}

type RepoSyncerFactory interface {
	NewRepoSyncer(store.Repository) (RepoSyncer, error)
}
