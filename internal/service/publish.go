package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"hermit/internal/store"
)

func (s *Service) PublishSkill(
	ctx context.Context,
	repo store.Repository,
	payload PublishPayload,
	files []PublishFileInput,
	actor string,
) (PublishResult, error) {
	if repo.Type != store.RepoTypeHosted {
		return PublishResult{}, fmt.Errorf("%w: publish only supports hosted repository", ErrInvalidInput)
	}

	slug := normalizeSlug(payload.Slug)
	if slug == "" {
		return PublishResult{}, fmt.Errorf("%w: slug required", ErrInvalidInput)
	}
	if strings.TrimSpace(payload.Version) == "" {
		return PublishResult{}, fmt.Errorf("%w: version required", ErrInvalidInput)
	}
	if len(files) == 0 {
		return PublishResult{}, fmt.Errorf("%w: at least one file is required", ErrInvalidInput)
	}

	displayName := strings.TrimSpace(payload.DisplayName)
	if displayName == "" {
		displayName = slug
	}

	sort.SliceStable(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	if !hasSkillManifest(files) {
		return PublishResult{}, fmt.Errorf("%w: SKILL.md required", ErrInvalidInput)
	}

	archiveBytes, fileDescriptors, err := buildZipArchive(files)
	if err != nil {
		return PublishResult{}, err
	}
	filesJSON, err := json.Marshal(fileDescriptors)
	if err != nil {
		return PublishResult{}, err
	}

	tagPatch := map[string]string{"latest": payload.Version}
	for _, t := range payload.Tags {
		tag := strings.TrimSpace(t)
		if tag == "" {
			continue
		}
		tagPatch[tag] = payload.Version
	}
	tagPatchJSON, err := json.Marshal(tagPatch)
	if err != nil {
		return PublishResult{}, err
	}

	digest, sizeBytes, blobPath, err := s.blobs.PutStream(ctx, bytes.NewReader(archiveBytes))
	if err != nil {
		return PublishResult{}, err
	}

	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		return PublishResult{}, err
	}
	defer tx.Rollback(ctx)

	packageID, err := s.store.EnsurePackageTx(ctx, tx, repo.ID, slug, actor)
	if err != nil {
		return PublishResult{}, err
	}
	versionID, err := s.store.InsertVersionTx(
		ctx,
		tx,
		packageID,
		payload.Version,
		digest,
		sizeBytes,
		payload.Changelog,
		nil,
		filesJSON,
		actor,
	)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			return PublishResult{}, ErrConflict
		}
		return PublishResult{}, err
	}

	archiveName := fmt.Sprintf("%s-%s.zip", slug, payload.Version)
	if err := s.store.InsertAssetTx(ctx, tx, versionID, archiveName, blobPath, sizeBytes, digest); err != nil {
		return PublishResult{}, err
	}
	if err := s.store.UpdatePackageMetaTx(ctx, tx, packageID, displayName, payload.Summary, tagPatchJSON); err != nil {
		return PublishResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PublishResult{}, err
	}

	return PublishResult{
		SkillID:   packageID.String(),
		VersionID: versionID.String(),
	}, nil
}
