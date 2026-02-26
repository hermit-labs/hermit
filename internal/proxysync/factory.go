package proxysync

import (
	"fmt"
	"net/http"
	"strings"

	"hermit/internal/store"
)

type FactoryDeps struct {
	HTTPClient      *http.Client
	VersionCacher   VersionCacher
	SyncConcurrency int
}

type Builder interface {
	Name() string
	Match(store.Repository) bool
	Build(store.Repository, FactoryDeps) (RepoSyncer, error)
}

type AbstractFactory struct {
	deps     FactoryDeps
	builders []Builder
}

func NewAbstractFactory(deps FactoryDeps, builders ...Builder) *AbstractFactory {
	if len(builders) == 0 {
		builders = []Builder{NewClawHubBuilder()}
	}
	return &AbstractFactory{
		deps:     deps,
		builders: builders,
	}
}

func (f *AbstractFactory) NewRepoSyncer(repo store.Repository) (RepoSyncer, error) {
	if repo.Type != store.RepoTypeProxy {
		return nil, fmt.Errorf("repository %q is not proxy", repo.Name)
	}
	if repo.UpstreamURL == nil || strings.TrimSpace(*repo.UpstreamURL) == "" {
		return nil, fmt.Errorf("repository %q missing upstream URL", repo.Name)
	}
	if f.deps.VersionCacher == nil {
		return nil, fmt.Errorf("factory dependency VersionCacher is nil")
	}

	for _, b := range f.builders {
		if !b.Match(repo) {
			continue
		}
		return b.Build(repo, f.deps)
	}
	return nil, fmt.Errorf("no syncer builder matched repo %q", repo.Name)
}
