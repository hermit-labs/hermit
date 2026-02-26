package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"hermit/internal/storage"
	"hermit/internal/store"
)

func (s *Service) BootstrapDefaults(ctx context.Context, ownerSubject string) error {
	hosted, err := s.store.EnsureRepository(ctx, s.defaults.HostedRepo, store.RepoTypeHosted, nil)
	if err != nil {
		return err
	}
	group, err := s.store.EnsureRepository(ctx, s.defaults.GroupRepo, store.RepoTypeGroup, nil)
	if err != nil {
		return err
	}

	if err := s.store.UpsertRepoMember(ctx, hosted.ID, ownerSubject, store.RoleAdmin); err != nil {
		return err
	}
	if err := s.store.UpsertRepoMember(ctx, group.ID, ownerSubject, store.RoleAdmin); err != nil {
		return err
	}
	if err := s.store.UpsertRepoMember(ctx, group.ID, "*", store.RoleRead); err != nil {
		return err
	}
	if err := s.store.AddGroupMember(ctx, group.ID, hosted.ID, 10); err != nil {
		return err
	}

	if len(s.defaults.ProxyUpstreams) == 0 {
		return nil
	}

	for i, rawUpstream := range s.defaults.ProxyUpstreams {
		upstream := strings.TrimSpace(rawUpstream)
		if upstream == "" {
			continue
		}

		repoName := strings.TrimSpace(s.defaults.ProxyRepo)
		if repoName == "" {
			repoName = "proxy"
		}
		if i > 0 {
			repoName = fmt.Sprintf("%s-%d", repoName, i+1)
		}

		proxy, err := s.store.EnsureRepository(ctx, repoName, store.RepoTypeProxy, &upstream)
		if err != nil {
			return err
		}
		if err := s.store.UpsertRepoMember(ctx, proxy.ID, ownerSubject, store.RoleAdmin); err != nil {
			return err
		}
		if err := s.store.UpsertRepoMember(ctx, proxy.ID, "*", store.RoleRead); err != nil {
			return err
		}
		if err := s.store.AddGroupMember(ctx, group.ID, proxy.ID, 20+i); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) BlobStore() *storage.BlobStore {
	return s.blobs
}

func (s *Service) GetReadRepository(ctx context.Context) (store.Repository, error) {
	return s.GetRepository(ctx, s.defaults.GroupRepo)
}

func (s *Service) GetPublishRepository(ctx context.Context) (store.Repository, error) {
	return s.GetRepository(ctx, s.defaults.HostedRepo)
}

func (s *Service) GetRepository(ctx context.Context, repoName string) (store.Repository, error) {
	repo, err := s.store.GetRepositoryByName(ctx, repoName)
	if err != nil {
		if store.IsNotFound(err) {
			return store.Repository{}, ErrNotFound
		}
		return store.Repository{}, err
	}
	if !repo.Enabled {
		return store.Repository{}, ErrNotFound
	}
	return repo, nil
}

func (s *Service) HasRepoPermission(ctx context.Context, repo store.Repository, subject string, requiredRole string, isAdmin bool) (bool, error) {
	if isAdmin {
		return true, nil
	}
	role, err := s.store.GetRepoRole(ctx, repo.ID, subject)
	if err != nil {
		if store.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return roleAllows(role, requiredRole), nil
}

func (s *Service) CreateToken(ctx context.Context, subject string) (string, error) {
	if strings.TrimSpace(subject) == "" {
		return "", fmt.Errorf("%w: subject required", ErrInvalidInput)
	}

	rawToken, err := generateToken(32)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(rawToken))
	hash := hex.EncodeToString(sum[:])

	if err := s.store.CreateToken(ctx, subject, hash); err != nil {
		if errors.Is(err, store.ErrConflict) {
			return "", ErrConflict
		}
		return "", err
	}
	return rawToken, nil
}
