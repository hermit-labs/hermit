package service

import (
	"context"
	"net/http"
	"time"

	"hermit/internal/storage"
	"hermit/internal/store"

	"golang.org/x/sync/singleflight"
)

const (
	proxyCacheStatusCached   = "cached"
	proxyCacheStatusNotFound = "not_found"
	proxyCacheStatusError    = "error"
)

type Defaults struct {
	HostedRepo     string
	GroupRepo      string
	ProxyRepo      string
	ProxyUpstreams []string
}

type PublishPayload struct {
	Slug        string
	DisplayName string
	Version     string
	Changelog   string
	Summary     *string
	Tags        []string
}

type PublishFileInput struct {
	Path        string
	ContentType string
	Bytes       []byte
}

type PublishResult struct {
	SkillID   string
	VersionID string
}

type SkillView struct {
	Skill         store.Skill
	LatestVersion *store.SkillVersionSummary
}

type SkillVersionView struct {
	Skill   store.Skill
	Version store.SkillVersion
}

type ResolveView struct {
	MatchVersion  *string
	LatestVersion *string
}

type Service struct {
	store            *store.Store
	blobs            storage.BlobStorage
	httpClient       *http.Client
	proxyNegativeTTL time.Duration
	defaults         Defaults
	fetchGroup       singleflight.Group
	syncProxyVersion func(context.Context, store.Repository, string, string) error
}

func New(
	st *store.Store,
	blobs storage.BlobStorage,
	proxyTimeout time.Duration,
	proxyNegativeTTL time.Duration,
	defaults Defaults,
) *Service {
	svc := &Service{
		store: st,
		blobs: blobs,
		httpClient: &http.Client{
			Timeout: proxyTimeout,
		},
		proxyNegativeTTL: proxyNegativeTTL,
		defaults:         defaults,
	}
	svc.syncProxyVersion = func(ctx context.Context, repo store.Repository, slug string, version string) error {
		_, err := svc.resolveProxy(ctx, repo, slug, version)
		return err
	}
	return svc
}
