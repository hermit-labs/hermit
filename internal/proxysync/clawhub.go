package proxysync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"hermit/internal/store"
)

type clawHubBuilder struct{}

func NewClawHubBuilder() Builder {
	return clawHubBuilder{}
}

func (clawHubBuilder) Name() string {
	return "clawhub"
}

func (clawHubBuilder) Match(repo store.Repository) bool {
	return repo.Type == store.RepoTypeProxy && repo.UpstreamURL != nil && strings.TrimSpace(*repo.UpstreamURL) != ""
}

func (clawHubBuilder) Build(repo store.Repository, deps FactoryDeps) (RepoSyncer, error) {
	client := deps.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	concurrency := deps.SyncConcurrency
	if concurrency <= 0 {
		concurrency = 4
	}
	return &clawHubSyncer{
		repo:                repo,
		client:              client,
		cache:               deps.VersionCacher,
		maxRateLimitRetries: 3,
		syncConcurrency:     concurrency,
		sleepFn:             sleepWithContext,
		jitterFn:            addJitter,
	}, nil
}

type clawHubSyncer struct {
	repo                store.Repository
	client              *http.Client
	cache               VersionCacher
	maxRateLimitRetries int
	syncConcurrency     int
	sleepFn             func(context.Context, time.Duration) error
	jitterFn            func(time.Duration) time.Duration
}

type upstreamSkillsListResponse struct {
	Items []struct {
		Slug          string         `json:"slug"`
		DisplayName   string         `json:"displayName"`
		Summary       *string        `json:"summary"`
		Tags          map[string]any `json:"tags"`
		LatestVersion *struct {
			Version         string  `json:"version"`
			CreatedAt       *int64  `json:"createdAt"`
			Changelog       *string `json:"changelog"`
			ChangelogSource *string `json:"changelogSource"`
		} `json:"latestVersion"`
	} `json:"items"`
	NextCursor *string `json:"nextCursor"`
}

type upstreamVersion struct {
	Version         string  `json:"version"`
	CreatedAt       *int64  `json:"createdAt"`
	Changelog       *string `json:"changelog"`
	ChangelogSource *string `json:"changelogSource"`
}

type syncVersion struct {
	version         string
	createdAt       *time.Time
	changelog       *string
	changelogSource *string
}

type upstreamVersionsListResponse struct {
	Items      []upstreamVersion `json:"items"`
	NextCursor *string           `json:"nextCursor"`
}

func (s *clawHubSyncer) Sync(ctx context.Context, pageSize int) (RepoStats, error) {
	if pageSize <= 0 {
		pageSize = 100
	}
	stats := RepoStats{Repository: s.repo.Name}

	cursor := ""
	for {
		if err := ctx.Err(); err != nil {
			return stats, err
		}

		page, err := s.fetchSkillsPage(ctx, pageSize, cursor)
		if err != nil {
			return stats, err
		}

		for _, item := range page.Items {
			if err := ctx.Err(); err != nil {
				return stats, err
			}

			slug := normalizeSlug(item.Slug)
			if slug == "" {
				continue
			}
			stats.Skills++

			latest := syncVersion{}
			if item.LatestVersion != nil {
				latest = syncVersion{
					version:         strings.TrimSpace(item.LatestVersion.Version),
					createdAt:       unixMillisToTime(item.LatestVersion.CreatedAt),
					changelog:       trimOptionalString(item.LatestVersion.Changelog, true),
					changelogSource: trimOptionalString(item.LatestVersion.ChangelogSource, false),
				}
			}

			versions, err := s.fetchAllVersions(ctx, slug, pageSize)
			if err != nil {
				if latest.version == "" {
					stats.Failed++
					continue
				}
				versions = []syncVersion{latest}
			}

			versions = normalizeVersions(versions, latest)
			stats.Versions += len(versions)
			cached, failed := s.syncVersions(ctx, slug, versions)
			stats.Cached += cached
			stats.Failed += failed

			if metaCacher, ok := s.cache.(ProxySkillMetaCacher); ok {
				if err := metaCacher.SyncProxySkillMeta(
					ctx,
					s.repo,
					slug,
					strings.TrimSpace(item.DisplayName),
					normalizeSummary(item.Summary),
					normalizeTagPatch(item.Tags),
				); err != nil {
					stats.Failed++
				}
			}
		}

		if page.NextCursor == nil || strings.TrimSpace(*page.NextCursor) == "" {
			break
		}
		cursor = strings.TrimSpace(*page.NextCursor)
	}
	return stats, nil
}

func (s *clawHubSyncer) syncVersions(ctx context.Context, slug string, versions []syncVersion) (cached int, failed int) {
	if len(versions) == 0 {
		return 0, 0
	}
	metaCacher, hasMetaCacher := s.cache.(ProxyVersionMetaCacher)

	workers := s.syncConcurrency
	if workers <= 0 {
		workers = 1
	}
	if workers > len(versions) {
		workers = len(versions)
	}

	if workers == 1 {
		for _, item := range versions {
			if err := s.cache.SyncProxyVersion(ctx, s.repo, slug, item.version); err != nil {
				failed++
				continue
			}
			if hasMetaCacher {
				if err := metaCacher.SyncProxyVersionMeta(
					ctx,
					s.repo,
					slug,
					item.version,
					item.createdAt,
					item.changelog,
					item.changelogSource,
				); err != nil {
					failed++
				}
			}
			cached++
		}
		return cached, failed
	}

	jobs := make(chan syncVersion)
	var cachedCount int64
	var failedCount int64
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for item := range jobs {
			if err := s.cache.SyncProxyVersion(ctx, s.repo, slug, item.version); err != nil {
				atomic.AddInt64(&failedCount, 1)
				continue
			}
			if hasMetaCacher {
				if err := metaCacher.SyncProxyVersionMeta(
					ctx,
					s.repo,
					slug,
					item.version,
					item.createdAt,
					item.changelog,
					item.changelogSource,
				); err != nil {
					atomic.AddInt64(&failedCount, 1)
				}
			}
			atomic.AddInt64(&cachedCount, 1)
		}
	}

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go worker()
	}

	for _, item := range versions {
		jobs <- item
	}
	close(jobs)
	wg.Wait()

	return int(cachedCount), int(failedCount)
}

func (s *clawHubSyncer) fetchSkillsPage(ctx context.Context, limit int, cursor string) (upstreamSkillsListResponse, error) {
	params := url.Values{}
	params.Set("limit", fmt.Sprintf("%d", limit))
	if strings.TrimSpace(cursor) != "" {
		params.Set("cursor", cursor)
	}
	u, err := buildUpstreamAPIURL(*s.repo.UpstreamURL, "/api/v1/skills", params)
	if err != nil {
		return upstreamSkillsListResponse{}, err
	}
	var resp upstreamSkillsListResponse
	if err := s.getJSON(ctx, u, &resp); err != nil {
		return upstreamSkillsListResponse{}, err
	}
	return resp, nil
}

func (s *clawHubSyncer) fetchAllVersions(ctx context.Context, slug string, limit int) ([]syncVersion, error) {
	cursor := ""
	var versions []syncVersion
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		page, err := s.fetchVersionsPage(ctx, slug, limit, cursor)
		if err != nil {
			return nil, err
		}
		for _, item := range page.Items {
			versions = append(versions, toSyncVersion(item))
		}
		if page.NextCursor == nil || strings.TrimSpace(*page.NextCursor) == "" {
			break
		}
		cursor = strings.TrimSpace(*page.NextCursor)
	}
	return versions, nil
}

func (s *clawHubSyncer) fetchVersionsPage(ctx context.Context, slug string, limit int, cursor string) (upstreamVersionsListResponse, error) {
	params := url.Values{}
	params.Set("limit", fmt.Sprintf("%d", limit))
	if strings.TrimSpace(cursor) != "" {
		params.Set("cursor", cursor)
	}
	u, err := buildUpstreamAPIURL(*s.repo.UpstreamURL, "/api/v1/skills/"+url.PathEscape(slug)+"/versions", params)
	if err != nil {
		return upstreamVersionsListResponse{}, err
	}
	var resp upstreamVersionsListResponse
	if err := s.getJSON(ctx, u, &resp); err != nil {
		return upstreamVersionsListResponse{}, err
	}
	return resp, nil
}

func (s *clawHubSyncer) getJSON(ctx context.Context, requestURL string, out any) error {
	retries := s.maxRateLimitRetries
	if retries < 0 {
		retries = 0
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			delay := retryDelayFromHeadersWithJitter(resp.Header, time.Now().UTC(), s.jitterFn)
			_ = resp.Body.Close()
			if attempt >= retries {
				return fmt.Errorf("upstream status 429 after %d retries", retries)
			}
			if delay > 0 && s.sleepFn != nil {
				if err := s.sleepFn(ctx, delay); err != nil {
					return err
				}
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			_ = resp.Body.Close()
			return fmt.Errorf("upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode upstream response: %w", err)
		}
		return nil
	}
}

func buildUpstreamAPIURL(baseURL string, apiPath string, params url.Values) (string, error) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	basePath := strings.TrimSuffix(u.Path, "/")
	u.Path = basePath + "/" + strings.TrimPrefix(apiPath, "/")
	if params != nil {
		u.RawQuery = params.Encode()
	}
	return u.String(), nil
}

func normalizeVersions(versions []syncVersion, latest syncVersion) []syncVersion {
	seen := make(map[string]struct{}, len(versions))
	out := make([]syncVersion, 0, len(versions)+1)
	for _, item := range versions {
		v := strings.TrimSpace(item.version)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		item.version = v
		out = append(out, item)
	}

	latestVersion := strings.TrimSpace(latest.version)
	if latestVersion == "" {
		return out
	}
	latest.version = latestVersion
	if _, ok := seen[latestVersion]; !ok {
		return append(out, latest)
	}

	withoutLatest := make([]syncVersion, 0, len(out))
	latestEntry := latest
	for _, item := range out {
		if item.version == latestVersion {
			latestEntry = mergeSyncVersionMeta(item, latest)
			continue
		}
		withoutLatest = append(withoutLatest, item)
	}
	return append(withoutLatest, latestEntry)
}

func mergeSyncVersionMeta(primary syncVersion, fallback syncVersion) syncVersion {
	out := primary
	if out.createdAt == nil {
		out.createdAt = fallback.createdAt
	}
	if out.changelog == nil {
		out.changelog = fallback.changelog
	}
	if out.changelogSource == nil {
		out.changelogSource = fallback.changelogSource
	}
	return out
}

func toSyncVersion(item upstreamVersion) syncVersion {
	return syncVersion{
		version:         strings.TrimSpace(item.Version),
		createdAt:       unixMillisToTime(item.CreatedAt),
		changelog:       trimOptionalString(item.Changelog, true),
		changelogSource: trimOptionalString(item.ChangelogSource, false),
	}
}

func unixMillisToTime(ms *int64) *time.Time {
	if ms == nil || *ms <= 0 {
		return nil
	}
	t := time.UnixMilli(*ms).UTC()
	return &t
}

func trimOptionalString(raw *string, keepEmpty bool) *string {
	if raw == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	if !keepEmpty && trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeSlug(slug string) string {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return ""
	}
	if strings.Contains(slug, "/") || strings.Contains(slug, "\\") || strings.Contains(slug, "..") {
		return ""
	}
	return slug
}

func normalizeSummary(summary *string) *string {
	if summary == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*summary)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeTagPatch(tags map[string]any) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	out := make(map[string]string, len(tags))
	for rawKey, rawValue := range tags {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			continue
		}
		value, ok := rawValue.(string)
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func retryDelayFromHeaders(header http.Header, now time.Time) time.Duration {
	return retryDelayFromHeadersWithJitter(header, now, addJitter)
}

func retryDelayFromHeadersWithJitter(
	header http.Header,
	now time.Time,
	jitterFn func(time.Duration) time.Duration,
) time.Duration {
	delay := parseRetryAfter(header.Get("Retry-After"), now)
	if delay <= 0 {
		delay = parseDelaySecondsHeader(header.Get("RateLimit-Reset"))
	}
	if delay <= 0 {
		epochSeconds := parseEpochSeconds(header.Get("X-RateLimit-Reset"))
		if epochSeconds > 0 {
			until := time.Unix(epochSeconds, 0).UTC().Sub(now)
			if until > 0 {
				delay = until
			}
		}
	}
	if delay <= 0 {
		delay = time.Second
	}
	if jitterFn == nil {
		return delay
	}
	return jitterFn(delay)
}

func parseRetryAfter(raw string, now time.Time) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if secs, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if secs < 0 {
			secs = 0
		}
		return time.Duration(secs) * time.Second
	}

	if ts, err := time.Parse(time.RFC1123, raw); err == nil {
		if ts.Before(now) {
			return 0
		}
		return ts.Sub(now)
	}
	if ts, err := time.Parse(time.RFC1123Z, raw); err == nil {
		if ts.Before(now) {
			return 0
		}
		return ts.Sub(now)
	}
	return 0
}

func parseDelaySecondsHeader(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	secs, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

func parseEpochSeconds(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	secs, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || secs <= 0 {
		return 0
	}
	return secs
}

func addJitter(base time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	jitterMax := 250 * time.Millisecond
	return base + time.Duration(rand.Int63n(int64(jitterMax)+1))
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
