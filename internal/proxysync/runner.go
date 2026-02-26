package proxysync

import (
	"context"
	"errors"
	"fmt"
)

type Runner struct {
	repoLister RepositoryLister
	factory    RepoSyncerFactory
}

func NewRunner(repoLister RepositoryLister, factory RepoSyncerFactory) *Runner {
	return &Runner{
		repoLister: repoLister,
		factory:    factory,
	}
}

func (r *Runner) Run(ctx context.Context, pageSize int) (Summary, error) {
	if r.repoLister == nil {
		return Summary{}, fmt.Errorf("repo lister is nil")
	}
	if r.factory == nil {
		return Summary{}, fmt.Errorf("syncer factory is nil")
	}
	if pageSize <= 0 {
		pageSize = 100
	}

	repos, err := r.repoLister.ListProxyRepositories(ctx)
	if err != nil {
		return Summary{}, err
	}

	var (
		summary Summary
		joined  error
	)
	for _, repo := range repos {
		if err := ctx.Err(); err != nil {
			return summary, err
		}

		syncer, err := r.factory.NewRepoSyncer(repo)
		if err != nil {
			joined = errors.Join(joined, fmt.Errorf("%s: create syncer: %w", repo.Name, err))
			continue
		}

		stats, err := syncer.Sync(ctx, pageSize)
		if err != nil {
			joined = errors.Join(joined, fmt.Errorf("%s: %w", repo.Name, err))
		}

		summary.Repositories++
		summary.Skills += stats.Skills
		summary.Versions += stats.Versions
		summary.Cached += stats.Cached
		summary.Failed += stats.Failed
		summary.ByRepository = append(summary.ByRepository, stats)
	}
	return summary, joined
}
