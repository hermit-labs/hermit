package service

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"time"

	"hermit/internal/store"
)

func (s *Service) ListProxyRepositories(ctx context.Context) ([]store.Repository, error) {
	group, err := s.GetReadRepository(ctx)
	if err != nil {
		return nil, err
	}
	members, err := s.store.ListGroupMembers(ctx, group.ID)
	if err != nil {
		return nil, err
	}

	proxies := make([]store.Repository, 0, len(members))
	for _, member := range members {
		if member.Type != store.RepoTypeProxy || !member.Enabled {
			continue
		}
		if member.UpstreamURL == nil || strings.TrimSpace(*member.UpstreamURL) == "" {
			continue
		}
		proxies = append(proxies, member)
	}
	return proxies, nil
}

func (s *Service) SyncProxyVersion(ctx context.Context, repo store.Repository, slug, version string) error {
	if err := s.syncProxyVersion(ctx, repo, slug, version); err != nil {
		return err
	}
	return s.ensureProxyVersionFiles(ctx, repo, slug, version)
}

func (s *Service) SyncProxyVersionMeta(
	ctx context.Context,
	repo store.Repository,
	slug string,
	version string,
	createdAt *time.Time,
	changelog *string,
	changelogSource *string,
) error {
	slug = normalizeSlug(slug)
	version = strings.TrimSpace(version)
	if slug == "" || version == "" {
		return nil
	}

	if createdAt != nil {
		t := createdAt.UTC()
		createdAt = &t
	}

	if changelog != nil {
		trimmed := strings.TrimSpace(*changelog)
		changelog = &trimmed
	}

	if changelogSource != nil {
		trimmed := strings.TrimSpace(*changelogSource)
		if trimmed == "" {
			changelogSource = nil
		} else {
			changelogSource = &trimmed
		}
	}
	if createdAt == nil && changelog == nil && changelogSource == nil {
		return nil
	}

	err := s.store.UpdateVersionMeta(ctx, repo.ID, slug, version, createdAt, changelog, changelogSource)
	if store.IsNotFound(err) {
		return nil
	}
	return err
}

func (s *Service) SyncProxySkillMeta(
	ctx context.Context,
	repo store.Repository,
	slug string,
	displayName string,
	summary *string,
	tagPatch map[string]string,
) error {
	slug = normalizeSlug(slug)
	if slug == "" {
		return nil
	}

	skill, err := s.store.GetSkill(ctx, repo.ID, slug)
	if err != nil {
		if store.IsNotFound(err) {
			return nil
		}
		return err
	}

	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = slug
	}

	if summary != nil {
		trimmed := strings.TrimSpace(*summary)
		if trimmed == "" {
			summary = nil
		} else {
			summary = &trimmed
		}
	}

	tagPatchJSON := json.RawMessage(`{}`)
	if len(tagPatch) > 0 {
		raw, err := json.Marshal(tagPatch)
		if err != nil {
			return err
		}
		tagPatchJSON = raw
	}

	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.store.UpdatePackageMetaTx(ctx, tx, skill.ID, displayName, summary, tagPatchJSON); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) ensureProxyVersionFiles(
	ctx context.Context,
	repo store.Repository,
	slug string,
	version string,
) error {
	_, sv, err := s.store.GetSkillVersion(ctx, repo.ID, slug, version)
	if err != nil {
		if store.IsNotFound(err) {
			return nil
		}
		return err
	}
	if hasFileDescriptors(sv.Files) {
		return nil
	}

	artifact, err := s.store.GetArtifact(ctx, repo.ID, slug, version)
	if err != nil {
		if store.IsNotFound(err) {
			return nil
		}
		return err
	}

	blobFile, info, err := s.blobs.Open(artifact.BlobPath)
	if err != nil {
		return err
	}
	descriptors, err := describeZipArchiveFiles(blobFile, info.Size())
	closeErr := blobFile.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}

	filesJSON, err := json.Marshal(descriptors)
	if err != nil {
		return err
	}
	if err := s.store.UpdateVersionFiles(ctx, sv.ID, filesJSON); err != nil {
		if store.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

func hasFileDescriptors(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("[]")) {
		return false
	}
	var arr []any
	if err := json.Unmarshal(trimmed, &arr); err != nil {
		return false
	}
	return len(arr) > 0
}
