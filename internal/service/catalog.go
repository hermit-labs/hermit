package service

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"hermit/internal/store"

	"github.com/google/uuid"
)

func (s *Service) SearchSkills(ctx context.Context, repo store.Repository, query string, limit int) ([]store.SkillSearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if repo.Type != store.RepoTypeGroup {
		return s.store.SearchSkills(ctx, repo.ID, query, limit)
	}

	members, err := s.store.ListGroupMembers(ctx, repo.ID)
	if err != nil {
		return nil, err
	}
	merged := make(map[string]store.SkillSearchResult)
	for _, member := range members {
		items, err := s.store.SearchSkills(ctx, member.ID, query, limit)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if item.Slug == nil {
				continue
			}
			slug := *item.Slug
			if _, exists := merged[slug]; !exists {
				merged[slug] = item
			}
		}
	}

	out := make([]store.SkillSearchResult, 0, len(merged))
	for _, item := range merged {
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			ti := time.Time{}
			tj := time.Time{}
			if out[i].UpdatedAt != nil {
				ti = *out[i].UpdatedAt
			}
			if out[j].UpdatedAt != nil {
				tj = *out[j].UpdatedAt
			}
			return ti.After(tj)
		}
		return out[i].Score > out[j].Score
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Service) ListSkills(
	ctx context.Context,
	repo store.Repository,
	limit int,
	offset int,
	sortBy string,
) ([]store.SkillListItem, error) {
	if limit <= 0 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}
	if repo.Type != store.RepoTypeGroup {
		return s.store.ListSkills(ctx, repo.ID, limit, offset, sortBy)
	}

	members, err := s.store.ListGroupMembers(ctx, repo.ID)
	if err != nil {
		return nil, err
	}

	mergeLimit := limit + offset + 200
	merged := make(map[string]store.SkillListItem)
	for _, member := range members {
		items, err := s.store.ListSkills(ctx, member.ID, mergeLimit, 0, sortBy)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if _, exists := merged[item.Slug]; !exists {
				merged[item.Slug] = item
			}
		}
	}

	out := make([]store.SkillListItem, 0, len(merged))
	for _, item := range merged {
		out = append(out, item)
	}
	sortSkillItems(out, sortBy)

	if offset >= len(out) {
		return []store.SkillListItem{}, nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], nil
}

func (s *Service) GetSkill(ctx context.Context, repo store.Repository, slug string) (SkillView, error) {
	targetRepo, err := s.findSkillRepository(ctx, repo, slug)
	if err != nil {
		return SkillView{}, err
	}

	skill, err := s.store.GetSkill(ctx, targetRepo.ID, slug)
	if err != nil {
		if store.IsNotFound(err) {
			return SkillView{}, ErrNotFound
		}
		return SkillView{}, err
	}

	var latest *store.SkillVersionSummary
	latestVersion, err := s.store.GetLatestVersionForSkill(ctx, skill.ID)
	if err == nil {
		latest = &store.SkillVersionSummary{
			Version:   latestVersion.Version,
			CreatedAt: latestVersion.CreatedAt,
			Changelog: latestVersion.Changelog,
		}
	} else if !store.IsNotFound(err) {
		return SkillView{}, err
	}

	return SkillView{
		Skill:         skill,
		LatestVersion: latest,
	}, nil
}

func (s *Service) ListSkillVersions(
	ctx context.Context,
	repo store.Repository,
	slug string,
	limit int,
	offset int,
) ([]store.SkillVersion, error) {
	targetRepo, err := s.findSkillRepository(ctx, repo, slug)
	if err != nil {
		return nil, err
	}
	return s.store.ListSkillVersions(ctx, targetRepo.ID, slug, limit, offset)
}

func (s *Service) GetSkillVersion(
	ctx context.Context,
	repo store.Repository,
	slug string,
	version string,
) (SkillVersionView, error) {
	targetRepo, err := s.findSkillRepository(ctx, repo, slug)
	if err != nil {
		return SkillVersionView{}, err
	}
	skill, sv, err := s.store.GetSkillVersion(ctx, targetRepo.ID, slug, version)
	if err != nil {
		if store.IsNotFound(err) {
			return SkillVersionView{}, ErrNotFound
		}
		return SkillVersionView{}, err
	}
	return SkillVersionView{Skill: skill, Version: sv}, nil
}

func (s *Service) ResolveSkillVersion(
	ctx context.Context,
	repo store.Repository,
	slug string,
	hash string,
) (ResolveView, error) {
	targetRepo, err := s.findSkillRepository(ctx, repo, slug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ResolveView{}, nil
		}
		return ResolveView{}, err
	}
	skill, err := s.store.GetSkill(ctx, targetRepo.ID, slug)
	if err != nil {
		if store.IsNotFound(err) {
			return ResolveView{}, nil
		}
		return ResolveView{}, err
	}

	latest, err := s.store.GetLatestVersionForSkill(ctx, skill.ID)
	if err != nil && !store.IsNotFound(err) {
		return ResolveView{}, err
	}
	var latestVersion *string
	if err == nil {
		latestVersion = &latest.Version
	}

	hash = strings.TrimSpace(hash)
	if hash == "" {
		return ResolveView{
			MatchVersion:  nil,
			LatestVersion: latestVersion,
		}, nil
	}

	match, err := s.store.ResolveVersionByHash(ctx, targetRepo.ID, slug, hash)
	if err != nil {
		return ResolveView{}, err
	}
	return ResolveView{
		MatchVersion:  match,
		LatestVersion: latestVersion,
	}, nil
}

func (s *Service) ReadSkillFile(
	ctx context.Context,
	repo store.Repository,
	slug string,
	version string,
	tag string,
	filePath string,
) ([]byte, error) {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, fmt.Errorf("%w: path required", ErrInvalidInput)
	}

	artifact, err := s.DownloadArtifact(ctx, repo, slug, version, tag, false)
	if err != nil {
		return nil, err
	}
	f, info, err := s.blobs.Open(artifact.BlobPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	zipReader, err := zip.NewReader(f, info.Size())
	if err != nil {
		return nil, fmt.Errorf("%w: artifact is not a zip", ErrInvalidInput)
	}

	target := sanitizeArchivePath(filePath)
	if target == "" {
		return nil, fmt.Errorf("%w: invalid path", ErrInvalidInput)
	}
	for _, zf := range zipReader.File {
		if sanitizeArchivePath(zf.Name) != target {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, ErrNotFound
}

func (s *Service) DeleteSkill(ctx context.Context, repo store.Repository, slug string, deleted bool) error {
	slug = normalizeSlug(slug)
	if slug == "" {
		return fmt.Errorf("%w: slug required", ErrInvalidInput)
	}
	if err := s.store.SetSkillDeleted(ctx, repo.ID, slug, deleted); err != nil {
		if store.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s *Service) findSkillRepository(ctx context.Context, repo store.Repository, slug string) (store.Repository, error) {
	if repo.Type != store.RepoTypeGroup {
		if _, err := s.store.GetSkill(ctx, repo.ID, slug); err != nil {
			if store.IsNotFound(err) {
				return store.Repository{}, ErrNotFound
			}
			return store.Repository{}, err
		}
		return repo, nil
	}
	members, err := s.store.ListGroupMembers(ctx, repo.ID)
	if err != nil {
		return store.Repository{}, err
	}
	for _, member := range members {
		if _, err := s.store.GetSkill(ctx, member.ID, slug); err == nil {
			return member, nil
		}
	}
	return store.Repository{}, ErrNotFound
}

func (s *Service) resolveInRepo(
	ctx context.Context,
	repo store.Repository,
	slug string,
	version string,
	visited map[uuid.UUID]struct{},
) (store.Artifact, error) {
	if !repo.Enabled {
		return store.Artifact{}, ErrNotFound
	}

	switch repo.Type {
	case store.RepoTypeHosted:
		return s.resolveHosted(ctx, repo, slug, version)
	case store.RepoTypeProxy:
		return s.resolveProxy(ctx, repo, slug, version)
	case store.RepoTypeGroup:
		if _, seen := visited[repo.ID]; seen {
			return store.Artifact{}, ErrNotFound
		}
		visited[repo.ID] = struct{}{}

		members, err := s.store.ListGroupMembers(ctx, repo.ID)
		if err != nil {
			return store.Artifact{}, err
		}
		for _, member := range members {
			artifact, err := s.resolveInRepo(ctx, member, slug, version, visited)
			if err == nil {
				return artifact, nil
			}
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return store.Artifact{}, err
		}
		return store.Artifact{}, ErrNotFound
	default:
		return store.Artifact{}, fmt.Errorf("%w: unknown repository type", ErrInvalidInput)
	}
}

func (s *Service) resolveLatestArtifactInRepo(
	ctx context.Context,
	repo store.Repository,
	slug string,
	visited map[uuid.UUID]struct{},
) (store.Artifact, error) {
	if !repo.Enabled {
		return store.Artifact{}, ErrNotFound
	}
	switch repo.Type {
	case store.RepoTypeHosted, store.RepoTypeProxy:
		a, err := s.store.GetLatestArtifact(ctx, repo.ID, slug)
		if err != nil {
			if store.IsNotFound(err) {
				return store.Artifact{}, ErrNotFound
			}
			return store.Artifact{}, err
		}
		return a, nil
	case store.RepoTypeGroup:
		if _, seen := visited[repo.ID]; seen {
			return store.Artifact{}, ErrNotFound
		}
		visited[repo.ID] = struct{}{}

		members, err := s.store.ListGroupMembers(ctx, repo.ID)
		if err != nil {
			return store.Artifact{}, err
		}
		for _, member := range members {
			a, err := s.resolveLatestArtifactInRepo(ctx, member, slug, visited)
			if err == nil {
				return a, nil
			}
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return store.Artifact{}, err
		}
		return store.Artifact{}, ErrNotFound
	default:
		return store.Artifact{}, ErrNotFound
	}
}

func (s *Service) resolveTagVersionInRepo(
	ctx context.Context,
	repo store.Repository,
	slug string,
	tag string,
	visited map[uuid.UUID]struct{},
) (*string, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, nil
	}
	switch repo.Type {
	case store.RepoTypeHosted, store.RepoTypeProxy:
		return s.store.ResolveVersionByTag(ctx, repo.ID, slug, tag)
	case store.RepoTypeGroup:
		if _, seen := visited[repo.ID]; seen {
			return nil, nil
		}
		visited[repo.ID] = struct{}{}

		members, err := s.store.ListGroupMembers(ctx, repo.ID)
		if err != nil {
			return nil, err
		}
		for _, member := range members {
			ver, err := s.resolveTagVersionInRepo(ctx, member, slug, tag, visited)
			if err != nil {
				return nil, err
			}
			if ver != nil {
				return ver, nil
			}
		}
		return nil, nil
	default:
		return nil, nil
	}
}
