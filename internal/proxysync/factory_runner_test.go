package proxysync

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"hermit/internal/store"
)

type fakeBuilder struct {
	name  string
	match bool
}

func (f fakeBuilder) Name() string { return f.name }

func (f fakeBuilder) Match(_ store.Repository) bool { return f.match }

func (f fakeBuilder) Build(repo store.Repository, _ FactoryDeps) (RepoSyncer, error) {
	return fakeRepoSyncer{
		stats: RepoStats{Repository: repo.Name, Skills: 1, Versions: 2, Cached: 2, Failed: 0},
	}, nil
}

type fakeRepoSyncer struct {
	stats RepoStats
	err   error
}

func (f fakeRepoSyncer) Sync(_ context.Context, _ int) (RepoStats, error) {
	return f.stats, f.err
}

type fakeRepoLister struct {
	repos []store.Repository
	err   error
}

func (f fakeRepoLister) ListProxyRepositories(context.Context) ([]store.Repository, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.repos, nil
}

func TestAbstractFactory_PicksMatchingBuilder(t *testing.T) {
	t.Parallel()

	upstream := "https://x.example"
	repo := store.Repository{
		Name:        "proxy",
		Type:        store.RepoTypeProxy,
		UpstreamURL: &upstream,
	}
	factory := NewAbstractFactory(
		FactoryDeps{VersionCacher: &recordCacher{}},
		fakeBuilder{name: "nope", match: false},
		fakeBuilder{name: "yes", match: true},
	)

	syncer, err := factory.NewRepoSyncer(repo)
	if err != nil {
		t.Fatalf("NewRepoSyncer() error = %v", err)
	}
	stats, err := syncer.Sync(context.Background(), 10)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if stats.Repository != "proxy" {
		t.Fatalf("stats.Repository = %q, want proxy", stats.Repository)
	}
}

type fakeFactory struct {
	syncers map[string]RepoSyncer
	err     error
}

func (f fakeFactory) NewRepoSyncer(repo store.Repository) (RepoSyncer, error) {
	if f.err != nil {
		return nil, f.err
	}
	if s, ok := f.syncers[repo.Name]; ok {
		return s, nil
	}
	return nil, errors.New("missing")
}

func TestRunner_AggregatesStats(t *testing.T) {
	t.Parallel()

	upstream := "https://x.example"
	repos := []store.Repository{
		{Name: "proxy-a", Type: store.RepoTypeProxy, UpstreamURL: &upstream},
		{Name: "proxy-b", Type: store.RepoTypeProxy, UpstreamURL: &upstream},
	}

	runner := NewRunner(
		fakeRepoLister{repos: repos},
		fakeFactory{
			syncers: map[string]RepoSyncer{
				"proxy-a": fakeRepoSyncer{stats: RepoStats{Repository: "proxy-a", Skills: 2, Versions: 3, Cached: 3, Failed: 0}},
				"proxy-b": fakeRepoSyncer{stats: RepoStats{Repository: "proxy-b", Skills: 1, Versions: 2, Cached: 1, Failed: 1}},
			},
		},
	)

	got, err := runner.Run(context.Background(), 100)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got.Repositories != 2 || got.Skills != 3 || got.Versions != 5 || got.Cached != 4 || got.Failed != 1 {
		t.Fatalf("summary = %#v", got)
	}

	wantNames := []string{"proxy-a", "proxy-b"}
	gotNames := []string{got.ByRepository[0].Repository, got.ByRepository[1].Repository}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("ByRepository names = %#v, want %#v", gotNames, wantNames)
	}
}
