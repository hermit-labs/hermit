package service

import (
	"context"
	"fmt"
	"strings"

	"hermit/internal/store"

	"github.com/google/uuid"
)

type DashboardStats struct {
	TotalSkills    int64           `json:"totalSkills"`
	TotalVersions  int64           `json:"totalVersions"`
	TotalDownloads int64           `json:"totalDownloads"`
	TotalStars     int64           `json:"totalStars"`
	TotalInstalls  int64           `json:"totalInstalls"`
	Repositories   []RepoStatsView `json:"repositories"`
}

type RepoStatsView struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	UpstreamURL *string `json:"upstreamUrl"`
	Enabled     bool    `json:"enabled"`
	SkillCount  int64   `json:"skillCount"`
}

func (s *Service) GetDashboardStats(ctx context.Context) (DashboardStats, error) {
	repoStats, err := s.store.GetRepoStats(ctx)
	if err != nil {
		return DashboardStats{}, err
	}

	repoIDs := make([]uuid.UUID, 0, len(repoStats))
	repoViews := make([]RepoStatsView, 0, len(repoStats))
	for _, rs := range repoStats {
		repoIDs = append(repoIDs, rs.Repository.ID)
		repoViews = append(repoViews, RepoStatsView{
			ID:          rs.Repository.ID.String(),
			Name:        rs.Repository.Name,
			Type:        rs.Repository.Type,
			UpstreamURL: rs.Repository.UpstreamURL,
			Enabled:     rs.Repository.Enabled,
			SkillCount:  rs.SkillCount,
		})
	}

	var stats DashboardStats
	if len(repoIDs) > 0 {
		dbStats, err := s.store.GetDashboardStats(ctx, repoIDs)
		if err != nil {
			return DashboardStats{}, err
		}
		stats.TotalSkills = dbStats.TotalSkills
		stats.TotalVersions = dbStats.TotalVersions
		stats.TotalDownloads = dbStats.TotalDownloads
		stats.TotalStars = dbStats.TotalStars
		stats.TotalInstalls = dbStats.TotalInstalls
	}
	stats.Repositories = repoViews

	return stats, nil
}

type SyncSourceView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	UpstreamURL string `json:"upstreamUrl"`
	Enabled     bool   `json:"enabled"`
	SkillCount  int64  `json:"skillCount"`
}

func (s *Service) ListSyncSources(ctx context.Context) ([]SyncSourceView, error) {
	repoStats, err := s.store.GetRepoStats(ctx)
	if err != nil {
		return nil, err
	}

	var sources []SyncSourceView
	for _, rs := range repoStats {
		if rs.Repository.Type != store.RepoTypeProxy {
			continue
		}
		upstream := ""
		if rs.Repository.UpstreamURL != nil {
			upstream = *rs.Repository.UpstreamURL
		}
		sources = append(sources, SyncSourceView{
			ID:          rs.Repository.ID.String(),
			Name:        rs.Repository.Name,
			UpstreamURL: upstream,
			Enabled:     rs.Repository.Enabled,
			SkillCount:  rs.SkillCount,
		})
	}
	return sources, nil
}

func (s *Service) AddSyncSource(ctx context.Context, name, upstreamURL string) (SyncSourceView, error) {
	name = strings.TrimSpace(name)
	upstreamURL = strings.TrimSpace(upstreamURL)
	if name == "" {
		return SyncSourceView{}, fmt.Errorf("%w: name required", ErrInvalidInput)
	}
	if upstreamURL == "" {
		return SyncSourceView{}, fmt.Errorf("%w: upstream URL required", ErrInvalidInput)
	}

	repo, err := s.store.CreateRepository(ctx, name, store.RepoTypeProxy, &upstreamURL)
	if err != nil {
		if err == store.ErrConflict {
			return SyncSourceView{}, fmt.Errorf("%w: repository name already exists", ErrConflict)
		}
		return SyncSourceView{}, err
	}

	if err := s.store.UpsertRepoMember(ctx, repo.ID, "*", store.RoleRead); err != nil {
		return SyncSourceView{}, err
	}

	group, err := s.store.GetRepositoryByName(ctx, s.defaults.GroupRepo)
	if err == nil {
		_ = s.store.AddGroupMember(ctx, group.ID, repo.ID, 50)
	}

	return SyncSourceView{
		ID:          repo.ID.String(),
		Name:        repo.Name,
		UpstreamURL: upstreamURL,
		Enabled:     repo.Enabled,
		SkillCount:  0,
	}, nil
}

func (s *Service) RemoveSyncSource(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("%w: invalid id", ErrInvalidInput)
	}
	if err := s.store.DeleteRepository(ctx, uid); err != nil {
		if store.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s *Service) ToggleSyncSource(ctx context.Context, id string, enabled bool) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("%w: invalid id", ErrInvalidInput)
	}
	if err := s.store.UpdateRepositoryEnabled(ctx, uid, enabled); err != nil {
		if store.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// --- RBAC Management ---

const (
	RoleAdmin     = "admin"
	RoleDeveloper = "developer"
	RoleViewer    = "viewer"
)

// mapRoleToDB converts user-facing role names to DB role enum values.
func mapRoleToDB(role string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case RoleAdmin:
		return store.RoleAdmin, nil
	case RoleDeveloper, store.RolePush, "dev":
		return store.RolePush, nil
	case RoleViewer, store.RoleRead, "view":
		return store.RoleRead, nil
	default:
		return "", fmt.Errorf("%w: invalid role %q (must be admin, developer, or viewer)", ErrInvalidInput, role)
	}
}

// mapRoleFromDB converts DB role enum values to user-facing role names.
func mapRoleFromDB(dbRole string) string {
	switch dbRole {
	case store.RoleAdmin:
		return RoleAdmin
	case store.RolePush:
		return RoleDeveloper
	case store.RoleRead:
		return RoleViewer
	default:
		return dbRole
	}
}

type RepoMemberView struct {
	RepoID   string `json:"repoId"`
	RepoName string `json:"repoName"`
	Subject  string `json:"subject"`
	Role     string `json:"role"`
}

func (s *Service) AssignRepoRole(ctx context.Context, repoID string, subject string, role string) error {
	uid, err := uuid.Parse(repoID)
	if err != nil {
		return fmt.Errorf("%w: invalid repo id", ErrInvalidInput)
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return fmt.Errorf("%w: subject required", ErrInvalidInput)
	}
	dbRole, err := mapRoleToDB(role)
	if err != nil {
		return err
	}

	return s.store.UpsertRepoMember(ctx, uid, subject, dbRole)
}

func (s *Service) RemoveRepoRole(ctx context.Context, repoID string, subject string) error {
	uid, err := uuid.Parse(repoID)
	if err != nil {
		return fmt.Errorf("%w: invalid repo id", ErrInvalidInput)
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return fmt.Errorf("%w: subject required", ErrInvalidInput)
	}
	if err := s.store.RemoveRepoMember(ctx, uid, subject); err != nil {
		if store.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s *Service) ListRepoMembers(ctx context.Context, repoID string) ([]RepoMemberView, error) {
	uid, err := uuid.Parse(repoID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid repo id", ErrInvalidInput)
	}
	members, err := s.store.ListRepoMembers(ctx, uid)
	if err != nil {
		return nil, err
	}

	views := make([]RepoMemberView, 0, len(members))
	for _, m := range members {
		views = append(views, RepoMemberView{
			RepoID:   m.RepoID.String(),
			RepoName: m.RepoName,
			Subject:  m.Subject,
			Role:     mapRoleFromDB(m.Role),
		})
	}
	return views, nil
}

func (s *Service) ListAllRepoMembers(ctx context.Context) ([]RepoMemberView, error) {
	members, err := s.store.ListAllRepoMembers(ctx)
	if err != nil {
		return nil, err
	}

	views := make([]RepoMemberView, 0, len(members))
	for _, m := range members {
		views = append(views, RepoMemberView{
			RepoID:   m.RepoID.String(),
			RepoName: m.RepoName,
			Subject:  m.Subject,
			Role:     mapRoleFromDB(m.Role),
		})
	}
	return views, nil
}
