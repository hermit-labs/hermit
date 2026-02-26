package proxysync

import (
	"context"
	"errors"
	"fmt"
	"log"
)

type Runner struct {
	repoLister RepositoryLister
	factory    RepoSyncerFactory
	logger     *log.Logger
}

func NewRunner(repoLister RepositoryLister, factory RepoSyncerFactory, logger *log.Logger) *Runner {
	if logger == nil {
		logger = log.Default()
	}
	return &Runner{
		repoLister: repoLister,
		factory:    factory,
		logger:     logger,
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

	r.logger.Printf("[sync] starting sync run (pageSize=%d)", pageSize)

	repos, err := r.repoLister.ListProxyRepositories(ctx)
	if err != nil {
		r.logger.Printf("[sync] failed to list proxy repositories: %v", err)
		return Summary{}, err
	}
	r.logger.Printf("[sync] found %d proxy repositories", len(repos))

	var (
		summary Summary
		joined  error
	)
	for i, repo := range repos {
		if err := ctx.Err(); err != nil {
			r.logger.Printf("[sync] context cancelled, aborting")
			return summary, err
		}

		r.logger.Printf("[sync] [%d/%d] syncing repo %q", i+1, len(repos), repo.Name)

		syncer, err := r.factory.NewRepoSyncer(repo)
		if err != nil {
			r.logger.Printf("[sync] [%d/%d] repo %q: failed to create syncer: %v", i+1, len(repos), repo.Name, err)
			joined = errors.Join(joined, fmt.Errorf("%s: create syncer: %w", repo.Name, err))
			continue
		}

		stats, err := syncer.Sync(ctx, pageSize)
		if err != nil {
			r.logger.Printf("[sync] [%d/%d] repo %q: sync error: %v", i+1, len(repos), repo.Name, err)
			joined = errors.Join(joined, fmt.Errorf("%s: %w", repo.Name, err))
		}

		r.logger.Printf("[sync] [%d/%d] repo %q done: skills=%d versions=%d cached=%d failed=%d skipped=%d",
			i+1, len(repos), repo.Name, stats.Skills, stats.Versions, stats.Cached, stats.Failed, stats.Skipped)

		summary.Repositories++
		summary.Skills += stats.Skills
		summary.Versions += stats.Versions
		summary.Cached += stats.Cached
		summary.Failed += stats.Failed
		summary.Skipped += stats.Skipped
		summary.ByRepository = append(summary.ByRepository, stats)
	}

	r.logger.Printf("[sync] sync run complete: repos=%d skills=%d versions=%d cached=%d failed=%d skipped=%d",
		summary.Repositories, summary.Skills, summary.Versions, summary.Cached, summary.Failed, summary.Skipped)

	return summary, joined
}
