package proxysync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"hermit/internal/store"
)

type recordCacher struct {
	mu    sync.Mutex
	calls []string
	fail  map[string]error
}

func (r *recordCacher) SyncProxyVersion(_ context.Context, _ store.Repository, slug, version string) error {
	key := slug + "@" + version
	r.mu.Lock()
	r.calls = append(r.calls, key)
	err, ok := r.fail[key]
	r.mu.Unlock()
	if ok {
		return err
	}
	return nil
}

func (r *recordCacher) Calls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

type metaRecordCacher struct {
	recordCacher
	metaMu           sync.Mutex
	metaCalls        []metaCall
	versionMetaCalls []versionMetaCall
}

type metaCall struct {
	slug        string
	displayName string
	summary     *string
	tags        map[string]string
}

type versionMetaCall struct {
	slug            string
	version         string
	createdAt       *time.Time
	changelog       *string
	changelogSource *string
}

func (m *metaRecordCacher) SyncProxySkillMeta(
	_ context.Context,
	_ store.Repository,
	slug string,
	displayName string,
	summary *string,
	tags map[string]string,
) error {
	var summaryCopy *string
	if summary != nil {
		v := *summary
		summaryCopy = &v
	}
	tagsCopy := map[string]string{}
	for k, v := range tags {
		tagsCopy[k] = v
	}

	m.metaMu.Lock()
	defer m.metaMu.Unlock()
	m.metaCalls = append(m.metaCalls, metaCall{
		slug:        slug,
		displayName: displayName,
		summary:     summaryCopy,
		tags:        tagsCopy,
	})
	return nil
}

func (m *metaRecordCacher) MetaCalls() []metaCall {
	m.metaMu.Lock()
	defer m.metaMu.Unlock()
	out := make([]metaCall, 0, len(m.metaCalls))
	for _, c := range m.metaCalls {
		var summaryCopy *string
		if c.summary != nil {
			v := *c.summary
			summaryCopy = &v
		}
		tagsCopy := map[string]string{}
		for k, v := range c.tags {
			tagsCopy[k] = v
		}
		out = append(out, metaCall{
			slug:        c.slug,
			displayName: c.displayName,
			summary:     summaryCopy,
			tags:        tagsCopy,
		})
	}
	return out
}

func (m *metaRecordCacher) SyncProxyVersionMeta(
	_ context.Context,
	_ store.Repository,
	slug string,
	version string,
	createdAt *time.Time,
	changelog *string,
	changelogSource *string,
) error {
	var createdAtCopy *time.Time
	if createdAt != nil {
		v := createdAt.UTC()
		createdAtCopy = &v
	}
	var changelogCopy *string
	if changelog != nil {
		v := *changelog
		changelogCopy = &v
	}
	var changelogSourceCopy *string
	if changelogSource != nil {
		v := *changelogSource
		changelogSourceCopy = &v
	}

	m.metaMu.Lock()
	defer m.metaMu.Unlock()
	m.versionMetaCalls = append(m.versionMetaCalls, versionMetaCall{
		slug:            slug,
		version:         version,
		createdAt:       createdAtCopy,
		changelog:       changelogCopy,
		changelogSource: changelogSourceCopy,
	})
	return nil
}

func (m *metaRecordCacher) VersionMetaCalls() []versionMetaCall {
	m.metaMu.Lock()
	defer m.metaMu.Unlock()
	out := make([]versionMetaCall, 0, len(m.versionMetaCalls))
	for _, c := range m.versionMetaCalls {
		var createdAtCopy *time.Time
		if c.createdAt != nil {
			v := c.createdAt.UTC()
			createdAtCopy = &v
		}
		var changelogCopy *string
		if c.changelog != nil {
			v := *c.changelog
			changelogCopy = &v
		}
		var changelogSourceCopy *string
		if c.changelogSource != nil {
			v := *c.changelogSource
			changelogSourceCopy = &v
		}
		out = append(out, versionMetaCall{
			slug:            c.slug,
			version:         c.version,
			createdAt:       createdAtCopy,
			changelog:       changelogCopy,
			changelogSource: changelogSourceCopy,
		})
	}
	return out
}

func TestClawHubSyncer_SyncsPagedSkillsAndVersions(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/registry/api/v1/skills", func(w http.ResponseWriter, req *http.Request) {
		cursor := req.URL.Query().Get("cursor")
		if req.URL.Query().Get("limit") != "50" {
			t.Fatalf("skills limit = %q, want 50", req.URL.Query().Get("limit"))
		}

		switch cursor {
		case "":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{"slug": "alpha", "latestVersion": map[string]any{"version": "2.0.0"}},
				},
				"nextCursor": "page2",
			})
		case "page2":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{"slug": "beta", "latestVersion": map[string]any{"version": "0.2.0"}},
				},
				"nextCursor": nil,
			})
		default:
			http.Error(w, "bad cursor", http.StatusBadRequest)
		}
	})
	mux.HandleFunc("/registry/api/v1/skills/alpha/versions", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"version": "2.0.0"},
				{"version": "1.0.0"},
			},
			"nextCursor": nil,
		})
	})
	mux.HandleFunc("/registry/api/v1/skills/beta/versions", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"version": "0.2.0"},
			},
			"nextCursor": nil,
		})
	})

	s := httptest.NewServer(mux)
	defer s.Close()

	upstream := s.URL + "/registry"
	repo := store.Repository{
		Name:        "proxy",
		Type:        store.RepoTypeProxy,
		UpstreamURL: &upstream,
		Enabled:     true,
	}
	cacher := &recordCacher{}
	factory := NewAbstractFactory(
		FactoryDeps{
			HTTPClient:    s.Client(),
			VersionCacher: cacher,
		},
		NewClawHubBuilder(),
	)
	syncer, err := factory.NewRepoSyncer(repo)
	if err != nil {
		t.Fatalf("NewRepoSyncer() error = %v", err)
	}

	stats, err := syncer.Sync(context.Background(), 50)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	wantCalls := []string{"alpha@1.0.0", "alpha@2.0.0", "beta@0.2.0"}
	gotCalls := sortedStrings(cacher.Calls())
	if !reflect.DeepEqual(gotCalls, sortedStrings(wantCalls)) {
		t.Fatalf("calls = %#v, want %#v", gotCalls, wantCalls)
	}
	if stats.Repository != "proxy" || stats.Skills != 2 || stats.Versions != 3 || stats.Cached != 3 || stats.Failed != 0 {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestClawHubSyncer_DedupAndFailureCount(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/skills", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"slug": "delta", "latestVersion": map[string]any{"version": "3.0.0"}},
			},
			"nextCursor": nil,
		})
	})
	mux.HandleFunc("/api/v1/skills/delta/versions", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"version": "3.0.0"},
				{"version": "2.0.0"},
				{"version": "3.0.0"},
			},
			"nextCursor": nil,
		})
	})

	s := httptest.NewServer(mux)
	defer s.Close()
	upstream := s.URL
	repo := store.Repository{
		Name:        "proxy",
		Type:        store.RepoTypeProxy,
		UpstreamURL: &upstream,
		Enabled:     true,
	}
	cacher := &recordCacher{
		fail: map[string]error{
			"delta@2.0.0": context.DeadlineExceeded,
		},
	}
	factory := NewAbstractFactory(
		FactoryDeps{
			HTTPClient:    s.Client(),
			VersionCacher: cacher,
		},
		NewClawHubBuilder(),
	)
	syncer, err := factory.NewRepoSyncer(repo)
	if err != nil {
		t.Fatalf("NewRepoSyncer() error = %v", err)
	}

	stats, err := syncer.Sync(context.Background(), 20)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	wantCalls := []string{"delta@2.0.0", "delta@3.0.0"}
	gotCalls := sortedStrings(cacher.Calls())
	if !reflect.DeepEqual(gotCalls, sortedStrings(wantCalls)) {
		t.Fatalf("calls = %#v, want %#v", gotCalls, wantCalls)
	}
	if stats.Skills != 1 || stats.Versions != 2 || stats.Cached != 1 || stats.Failed != 1 {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestClawHubSyncer_PassesSkillMetadataToMetaCacher(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/skills", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{
					"slug":        "alpha",
					"displayName": "Alpha Tool",
					"summary":     "metadata synced",
					"tags": map[string]any{
						"stable": "1.2.0",
						"qa":     "1.3.0-rc1",
						"bad":    123,
					},
					"latestVersion": map[string]any{"version": "1.2.0"},
				},
			},
			"nextCursor": nil,
		})
	})
	mux.HandleFunc("/api/v1/skills/alpha/versions", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":      []map[string]any{{"version": "1.2.0"}},
			"nextCursor": nil,
		})
	})

	s := httptest.NewServer(mux)
	defer s.Close()
	upstream := s.URL
	repo := store.Repository{
		Name:        "proxy",
		Type:        store.RepoTypeProxy,
		UpstreamURL: &upstream,
		Enabled:     true,
	}

	cacher := &metaRecordCacher{}
	factory := NewAbstractFactory(
		FactoryDeps{
			HTTPClient:    s.Client(),
			VersionCacher: cacher,
		},
		NewClawHubBuilder(),
	)
	syncer, err := factory.NewRepoSyncer(repo)
	if err != nil {
		t.Fatalf("NewRepoSyncer() error = %v", err)
	}

	stats, err := syncer.Sync(context.Background(), 20)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if stats.Cached != 1 {
		t.Fatalf("stats.Cached = %d, want 1", stats.Cached)
	}

	metaCalls := cacher.MetaCalls()
	if len(metaCalls) != 1 {
		t.Fatalf("meta calls len = %d, want 1", len(metaCalls))
	}
	got := metaCalls[0]
	if got.slug != "alpha" {
		t.Fatalf("meta slug = %q, want alpha", got.slug)
	}
	if got.displayName != "Alpha Tool" {
		t.Fatalf("meta displayName = %q, want %q", got.displayName, "Alpha Tool")
	}
	if got.summary == nil || *got.summary != "metadata synced" {
		t.Fatalf("meta summary = %#v, want metadata synced", got.summary)
	}
	wantTags := map[string]string{"stable": "1.2.0", "qa": "1.3.0-rc1"}
	if !reflect.DeepEqual(got.tags, wantTags) {
		t.Fatalf("meta tags = %#v, want %#v", got.tags, wantTags)
	}
}

func TestClawHubSyncer_PassesVersionMetadataToMetaCacher(t *testing.T) {
	t.Parallel()

	const createdAtMillis int64 = 1772079265941

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/skills", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{
					"slug":          "claw-clash",
					"displayName":   "claw-clash",
					"latestVersion": map[string]any{"version": "1.7.0"},
				},
			},
			"nextCursor": nil,
		})
	})
	mux.HandleFunc("/api/v1/skills/claw-clash/versions", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{
					"version":         "1.7.0",
					"createdAt":       createdAtMillis,
					"changelog":       " Security fixes ",
					"changelogSource": " user ",
				},
			},
			"nextCursor": nil,
		})
	})

	s := httptest.NewServer(mux)
	defer s.Close()
	upstream := s.URL
	repo := store.Repository{
		Name:        "proxy",
		Type:        store.RepoTypeProxy,
		UpstreamURL: &upstream,
		Enabled:     true,
	}

	cacher := &metaRecordCacher{}
	factory := NewAbstractFactory(
		FactoryDeps{
			HTTPClient:    s.Client(),
			VersionCacher: cacher,
		},
		NewClawHubBuilder(),
	)
	syncer, err := factory.NewRepoSyncer(repo)
	if err != nil {
		t.Fatalf("NewRepoSyncer() error = %v", err)
	}

	stats, err := syncer.Sync(context.Background(), 20)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if stats.Cached != 1 || stats.Failed != 0 {
		t.Fatalf("stats = %#v", stats)
	}

	versionMetaCalls := cacher.VersionMetaCalls()
	if len(versionMetaCalls) != 1 {
		t.Fatalf("version meta calls len = %d, want 1", len(versionMetaCalls))
	}
	got := versionMetaCalls[0]
	if got.slug != "claw-clash" || got.version != "1.7.0" {
		t.Fatalf("version meta call = %#v", got)
	}
	wantCreatedAt := time.UnixMilli(createdAtMillis).UTC()
	if got.createdAt == nil || !got.createdAt.Equal(wantCreatedAt) {
		t.Fatalf("createdAt = %#v, want %s", got.createdAt, wantCreatedAt)
	}
	if got.changelog == nil || *got.changelog != "Security fixes" {
		t.Fatalf("changelog = %#v, want %q", got.changelog, "Security fixes")
	}
	if got.changelogSource == nil || *got.changelogSource != "user" {
		t.Fatalf("changelogSource = %#v, want %q", got.changelogSource, "user")
	}
}

func TestClawHubSyncer_UsesLatestMetadataWhenVersionsEndpointFails(t *testing.T) {
	t.Parallel()

	const createdAtMillis int64 = 1772079265941

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/skills", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{
					"slug":        "claw-clash",
					"displayName": "claw-clash",
					"latestVersion": map[string]any{
						"version":   "1.7.0",
						"createdAt": createdAtMillis,
						"changelog": " latest fallback ",
					},
				},
			},
			"nextCursor": nil,
		})
	})
	mux.HandleFunc("/api/v1/skills/claw-clash/versions", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	})

	s := httptest.NewServer(mux)
	defer s.Close()
	upstream := s.URL
	repo := store.Repository{
		Name:        "proxy",
		Type:        store.RepoTypeProxy,
		UpstreamURL: &upstream,
		Enabled:     true,
	}

	cacher := &metaRecordCacher{}
	factory := NewAbstractFactory(
		FactoryDeps{
			HTTPClient:    s.Client(),
			VersionCacher: cacher,
		},
		NewClawHubBuilder(),
	)
	syncer, err := factory.NewRepoSyncer(repo)
	if err != nil {
		t.Fatalf("NewRepoSyncer() error = %v", err)
	}

	stats, err := syncer.Sync(context.Background(), 20)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if stats.Cached != 1 || stats.Failed != 0 {
		t.Fatalf("stats = %#v, want cached=1 failed=0", stats)
	}

	versionMetaCalls := cacher.VersionMetaCalls()
	if len(versionMetaCalls) != 1 {
		t.Fatalf("version meta calls len = %d, want 1", len(versionMetaCalls))
	}
	got := versionMetaCalls[0]
	wantCreatedAt := time.UnixMilli(createdAtMillis).UTC()
	if got.createdAt == nil || !got.createdAt.Equal(wantCreatedAt) {
		t.Fatalf("createdAt = %#v, want %s", got.createdAt, wantCreatedAt)
	}
	if got.changelog == nil || *got.changelog != "latest fallback" {
		t.Fatalf("changelog = %#v, want %q", got.changelog, "latest fallback")
	}
}

func TestRetryDelayFromHeaders_Priority(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	h := http.Header{}
	h.Set("Retry-After", "5")
	h.Set("RateLimit-Reset", "10")
	h.Set("X-RateLimit-Reset", "1700001000")

	delay := retryDelayFromHeadersWithJitter(h, now, func(d time.Duration) time.Duration { return d })
	if delay != 5*time.Second {
		t.Fatalf("delay = %s, want 5s", delay)
	}

	h = http.Header{}
	h.Set("RateLimit-Reset", "7")
	delay = retryDelayFromHeadersWithJitter(h, now, func(d time.Duration) time.Duration { return d })
	if delay != 7*time.Second {
		t.Fatalf("delay = %s, want 7s", delay)
	}

	h = http.Header{}
	h.Set("X-RateLimit-Reset", "1700000012")
	delay = retryDelayFromHeadersWithJitter(h, now, func(d time.Duration) time.Duration { return d })
	if delay != 12*time.Second {
		t.Fatalf("delay = %s, want 12s", delay)
	}
}

func TestClawHubSyncer_RetriesOn429(t *testing.T) {
	t.Parallel()

	var skillsCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/skills", func(w http.ResponseWriter, _ *http.Request) {
		skillsCalls++
		if skillsCalls == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "too many", http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"slug": "alpha", "latestVersion": map[string]any{"version": "1.0.0"}},
			},
			"nextCursor": nil,
		})
	})
	mux.HandleFunc("/api/v1/skills/alpha/versions", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":      []map[string]any{{"version": "1.0.0"}},
			"nextCursor": nil,
		})
	})

	s := httptest.NewServer(mux)
	defer s.Close()
	upstream := s.URL
	repo := store.Repository{
		Name:        "proxy",
		Type:        store.RepoTypeProxy,
		UpstreamURL: &upstream,
	}
	cacher := &recordCacher{}
	factory := NewAbstractFactory(
		FactoryDeps{
			HTTPClient:    s.Client(),
			VersionCacher: cacher,
		},
		NewClawHubBuilder(),
	)
	syncerIface, err := factory.NewRepoSyncer(repo)
	if err != nil {
		t.Fatalf("NewRepoSyncer() error = %v", err)
	}
	syncer := syncerIface.(*clawHubSyncer)
	syncer.jitterFn = func(d time.Duration) time.Duration { return d }
	syncer.sleepFn = func(context.Context, time.Duration) error { return nil }

	_, err = syncer.Sync(context.Background(), 10)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if skillsCalls < 2 {
		t.Fatalf("skills endpoint calls = %d, want >= 2", skillsCalls)
	}
}

type concurrencyProbeCacher struct {
	inFlight    atomic.Int64
	maxInFlight atomic.Int64
	ready       chan struct{}
	release     chan struct{}
	once        sync.Once
}

func newConcurrencyProbeCacher() *concurrencyProbeCacher {
	return &concurrencyProbeCacher{
		ready:   make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (c *concurrencyProbeCacher) SyncProxyVersion(context.Context, store.Repository, string, string) error {
	current := c.inFlight.Add(1)
	for {
		maxNow := c.maxInFlight.Load()
		if current <= maxNow {
			break
		}
		if c.maxInFlight.CompareAndSwap(maxNow, current) {
			break
		}
	}
	if current >= 2 {
		c.once.Do(func() { close(c.ready) })
	}
	<-c.release
	c.inFlight.Add(-1)
	return nil
}

func TestClawHubSyncer_SyncVersionsConcurrently(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/skills", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"slug": "alpha", "latestVersion": map[string]any{"version": "2.0.0"}},
			},
			"nextCursor": nil,
		})
	})
	mux.HandleFunc("/api/v1/skills/alpha/versions", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"version": "1.0.0"},
				{"version": "2.0.0"},
			},
			"nextCursor": nil,
		})
	})

	s := httptest.NewServer(mux)
	defer s.Close()
	upstream := s.URL
	repo := store.Repository{
		Name:        "proxy",
		Type:        store.RepoTypeProxy,
		UpstreamURL: &upstream,
		Enabled:     true,
	}

	probe := newConcurrencyProbeCacher()
	factory := NewAbstractFactory(
		FactoryDeps{
			HTTPClient:      s.Client(),
			VersionCacher:   probe,
			SyncConcurrency: 2,
		},
		NewClawHubBuilder(),
	)
	syncer, err := factory.NewRepoSyncer(repo)
	if err != nil {
		t.Fatalf("NewRepoSyncer() error = %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, syncErr := syncer.Sync(context.Background(), 10)
		done <- syncErr
	}()

	select {
	case <-probe.ready:
		close(probe.release)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for concurrent SyncProxyVersion calls")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Sync() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting sync to complete")
	}

	if probe.maxInFlight.Load() < 2 {
		t.Fatalf("maxInFlight = %d, want >= 2", probe.maxInFlight.Load())
	}
}

func sortedStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
