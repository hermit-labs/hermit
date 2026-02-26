package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"hermit/internal/store"

	"github.com/google/uuid"
)

func (s *Service) DownloadArtifact(
	ctx context.Context,
	repo store.Repository,
	slug string,
	version string,
	tag string,
	countDownload bool,
) (store.Artifact, error) {
	slug = normalizeSlug(slug)
	if slug == "" {
		return store.Artifact{}, fmt.Errorf("%w: slug required", ErrInvalidInput)
	}

	artifactVersion := strings.TrimSpace(version)
	if artifactVersion == "" && strings.TrimSpace(tag) != "" {
		resolved, err := s.resolveTagVersionInRepo(ctx, repo, slug, tag, map[uuid.UUID]struct{}{})
		if err != nil {
			return store.Artifact{}, err
		}
		if resolved != nil {
			artifactVersion = *resolved
		}
	}
	if artifactVersion == "" {
		latest, err := s.resolveLatestArtifactInRepo(ctx, repo, slug, map[uuid.UUID]struct{}{})
		if err != nil {
			return store.Artifact{}, err
		}
		artifactVersion = latest.Version
	}

	artifact, err := s.resolveInRepo(ctx, repo, slug, artifactVersion, map[uuid.UUID]struct{}{})
	if err != nil {
		return store.Artifact{}, err
	}
	if countDownload {
		_ = s.store.IncrementSkillDownloads(ctx, artifact.RepoID, slug)
	}
	return artifact, nil
}

func (s *Service) resolveHosted(ctx context.Context, repo store.Repository, slug, version string) (store.Artifact, error) {
	a, err := s.store.GetArtifact(ctx, repo.ID, slug, version)
	if err != nil {
		if store.IsNotFound(err) {
			return store.Artifact{}, ErrNotFound
		}
		return store.Artifact{}, err
	}
	return a, nil
}

func (s *Service) resolveProxy(ctx context.Context, repo store.Repository, slug, version string) (store.Artifact, error) {
	artifact, err := s.resolveHosted(ctx, repo, slug, version)
	if err == nil {
		return artifact, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return store.Artifact{}, err
	}

	cacheEntry, cacheErr := s.store.GetProxyCache(ctx, repo.ID, slug, version)
	if cacheErr == nil && cacheEntry.Status == proxyCacheStatusNotFound && cacheEntry.ExpiresAt != nil && cacheEntry.ExpiresAt.After(time.Now().UTC()) {
		return store.Artifact{}, ErrNotFound
	}
	if cacheErr != nil && !store.IsNotFound(cacheErr) {
		return store.Artifact{}, cacheErr
	}

	sfKey := fmt.Sprintf("%s:%s:%s", repo.ID.String(), slug, version)
	v, err, _ := s.fetchGroup.Do(sfKey, func() (any, error) {
		if current, currentErr := s.resolveHosted(ctx, repo, slug, version); currentErr == nil {
			return current, nil
		}
		return s.fetchAndCacheProxy(ctx, repo, slug, version, cacheEntry.ETag)
	})
	if err != nil {
		return store.Artifact{}, err
	}
	return v.(store.Artifact), nil
}

func (s *Service) fetchAndCacheProxy(
	ctx context.Context,
	repo store.Repository,
	slug string,
	version string,
	etag *string,
) (store.Artifact, error) {
	if repo.UpstreamURL == nil || strings.TrimSpace(*repo.UpstreamURL) == "" {
		return store.Artifact{}, fmt.Errorf("%w: proxy repository missing upstream URL", ErrInvalidInput)
	}

	upstreamURL, err := buildUpstreamDownloadURL(*repo.UpstreamURL, slug, version)
	if err != nil {
		return store.Artifact{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		return store.Artifact{}, err
	}
	if etag != nil && strings.TrimSpace(*etag) != "" {
		req.Header.Set("If-None-Match", strings.TrimSpace(*etag))
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		cacheErr := err.Error()
		expiresAt := time.Now().UTC().Add(1 * time.Minute)
		_ = s.store.UpsertProxyCache(ctx, repo.ID, slug, version, proxyCacheStatusError, nil, &expiresAt, &cacheErr)
		return store.Artifact{}, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		digest, sizeBytes, blobPath, err := s.blobs.PutStream(ctx, resp.Body)
		if err != nil {
			return store.Artifact{}, err
		}
		fileName := resolveFileName(resp, slug, version)

		blobFile, blobInfo, err := s.blobs.Open(blobPath)
		if err != nil {
			return store.Artifact{}, err
		}
		fileDescriptors, err := describeZipArchiveFiles(blobFile, blobInfo.Size())
		closeErr := blobFile.Close()
		if err != nil {
			return store.Artifact{}, err
		}
		if closeErr != nil {
			return store.Artifact{}, closeErr
		}

		tx, err := s.store.BeginTx(ctx)
		if err != nil {
			return store.Artifact{}, err
		}
		defer tx.Rollback(ctx)

		packageID, err := s.store.EnsurePackageTx(ctx, tx, repo.ID, slug, "proxy:"+repo.Name)
		if err != nil {
			return store.Artifact{}, err
		}
		filesJSON, err := json.Marshal(fileDescriptors)
		if err != nil {
			return store.Artifact{}, err
		}
		versionID, err := s.store.InsertVersionTx(
			ctx,
			tx,
			packageID,
			version,
			digest,
			sizeBytes,
			"",
			nil,
			filesJSON,
			"proxy:"+repo.Name,
		)
		if err != nil {
			if !errors.Is(err, store.ErrConflict) {
				return store.Artifact{}, err
			}
			if current, currentErr := s.resolveHosted(ctx, repo, slug, version); currentErr == nil {
				return current, nil
			}
			return store.Artifact{}, ErrConflict
		}
		if err := s.store.InsertAssetTx(ctx, tx, versionID, fileName, blobPath, sizeBytes, digest); err != nil {
			return store.Artifact{}, err
		}

		tagPatch := json.RawMessage(fmt.Sprintf(`{"latest":%q}`, version))
		if err := s.store.UpdatePackageMetaTx(ctx, tx, packageID, slug, nil, tagPatch); err != nil {
			return store.Artifact{}, err
		}

		if err := tx.Commit(ctx); err != nil {
			return store.Artifact{}, err
		}

		etagHeader := strings.TrimSpace(resp.Header.Get("ETag"))
		var etagPtr *string
		if etagHeader != "" {
			etagPtr = &etagHeader
		}
		_ = s.store.UpsertProxyCache(ctx, repo.ID, slug, version, proxyCacheStatusCached, etagPtr, nil, nil)

		return store.Artifact{
			RepoID:      repo.ID,
			RepoName:    repo.Name,
			PackageName: slug,
			Version:     version,
			FileName:    fileName,
			Digest:      digest,
			SizeBytes:   sizeBytes,
			BlobPath:    blobPath,
		}, nil
	case http.StatusNotModified:
		current, err := s.resolveHosted(ctx, repo, slug, version)
		if err == nil {
			return current, nil
		}
		return store.Artifact{}, ErrNotFound
	case http.StatusNotFound:
		expiresAt := time.Now().UTC().Add(s.proxyNegativeTTL)
		_ = s.store.UpsertProxyCache(ctx, repo.ID, slug, version, proxyCacheStatusNotFound, nil, &expiresAt, nil)
		return store.Artifact{}, ErrNotFound
	default:
		cacheErr := fmt.Sprintf("upstream status %d", resp.StatusCode)
		expiresAt := time.Now().UTC().Add(1 * time.Minute)
		_ = s.store.UpsertProxyCache(ctx, repo.ID, slug, version, proxyCacheStatusError, nil, &expiresAt, &cacheErr)
		return store.Artifact{}, fmt.Errorf("upstream status %d", resp.StatusCode)
	}
}

func buildUpstreamDownloadURL(baseURL, slug, version string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	u.Path = path.Join(u.Path, "api/v1/download")
	q := u.Query()
	q.Set("slug", slug)
	q.Set("version", version)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func resolveFileName(resp *http.Response, slug, version string) string {
	cd := strings.TrimSpace(resp.Header.Get("Content-Disposition"))
	if cd != "" {
		parts := strings.Split(cd, ";")
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if strings.HasPrefix(strings.ToLower(trimmed), "filename=") {
				name := strings.TrimPrefix(trimmed, "filename=")
				name = strings.Trim(name, "\"")
				if name != "" {
					return name
				}
			}
		}
	}

	if resp.Request != nil && resp.Request.URL != nil {
		base := path.Base(resp.Request.URL.Path)
		if base != "." && base != "/" && base != "" {
			return base
		}
	}
	return fmt.Sprintf("%s-%s.zip", slug, version)
}
