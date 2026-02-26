package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RepoTypeHosted = "hosted"
	RepoTypeProxy  = "proxy"
	RepoTypeGroup  = "group"

	RoleRead  = "read"
	RolePush  = "push"
	RoleAdmin = "admin"
)

var ErrConflict = errors.New("conflict")

type Repository struct {
	ID          uuid.UUID
	Name        string
	Type        string
	UpstreamURL *string
	Enabled     bool
}

type Artifact struct {
	RepoID      uuid.UUID
	RepoName    string
	PackageName string
	Version     string
	FileName    string
	Digest      string
	SizeBytes   int64
	BlobPath    string
}

type ProxyCacheEntry struct {
	Status      string
	ETag        *string
	ExpiresAt   *time.Time
	LastChecked time.Time
	LastError   *string
}

type Skill struct {
	ID              uuid.UUID
	Slug            string
	DisplayName     string
	Summary         *string
	Tags            json.RawMessage
	Downloads       int64
	Stars           int64
	InstallsCurrent int64
	InstallsAllTime int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type SkillVersion struct {
	ID              uuid.UUID
	PackageID       uuid.UUID
	Version         string
	Digest          string
	SizeBytes       int64
	Changelog       string
	ChangelogSource *string
	Files           json.RawMessage
	CreatedAt       time.Time
}

type SkillListItem struct {
	Skill
	LatestVersion *SkillVersionSummary
}

type SkillVersionSummary struct {
	Version   string
	CreatedAt time.Time
	Changelog string
}

type SkillSearchResult struct {
	Slug        *string
	DisplayName *string
	Summary     *string
	Version     *string
	Score       float64
	UpdatedAt   *time.Time
}

type Store struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return s.db.Begin(ctx)
}

func (s *Store) GetRepositoryByName(ctx context.Context, name string) (Repository, error) {
	var repo Repository
	err := s.db.QueryRow(ctx, `
		SELECT id, name, type::text, upstream_url, enabled
		FROM repositories
		WHERE name = $1
	`, name).Scan(&repo.ID, &repo.Name, &repo.Type, &repo.UpstreamURL, &repo.Enabled)
	if err != nil {
		return Repository{}, err
	}
	return repo, nil
}

func (s *Store) CreateRepository(ctx context.Context, name string, repoType string, upstreamURL *string) (Repository, error) {
	var repo Repository
	err := s.db.QueryRow(ctx, `
		INSERT INTO repositories (name, type, upstream_url)
		VALUES ($1, $2::repo_type, $3)
		RETURNING id, name, type::text, upstream_url, enabled
	`, name, repoType, upstreamURL).Scan(&repo.ID, &repo.Name, &repo.Type, &repo.UpstreamURL, &repo.Enabled)
	if err != nil {
		if isUniqueViolation(err) {
			return Repository{}, ErrConflict
		}
		return Repository{}, err
	}
	return repo, nil
}

func (s *Store) EnsureRepository(ctx context.Context, name string, repoType string, upstreamURL *string) (Repository, error) {
	repo, err := s.GetRepositoryByName(ctx, name)
	if err == nil {
		if repo.Type != repoType {
			return Repository{}, fmt.Errorf("repository %q already exists with type %q", name, repo.Type)
		}
		return repo, nil
	}
	if !IsNotFound(err) {
		return Repository{}, err
	}
	return s.CreateRepository(ctx, name, repoType, upstreamURL)
}

func (s *Store) UpsertRepoMember(ctx context.Context, repoID uuid.UUID, subject string, role string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO repo_members (repo_id, subject, role)
		VALUES ($1, $2, $3::repo_role)
		ON CONFLICT (repo_id, subject)
		DO UPDATE SET role = EXCLUDED.role
	`, repoID, subject, role)
	return err
}

func (s *Store) AddGroupMember(ctx context.Context, groupRepoID uuid.UUID, memberRepoID uuid.UUID, priority int) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO group_members (group_repo_id, member_repo_id, priority)
		VALUES ($1, $2, $3)
		ON CONFLICT (group_repo_id, member_repo_id)
		DO UPDATE SET priority = EXCLUDED.priority
	`, groupRepoID, memberRepoID, priority)
	return err
}

func (s *Store) GetRepoRole(ctx context.Context, repoID uuid.UUID, subject string) (string, error) {
	var role string
	err := s.db.QueryRow(ctx, `
		SELECT role::text
		FROM repo_members
		WHERE repo_id = $1
		  AND subject IN ($2, '*')
		ORDER BY CASE WHEN subject = $2 THEN 0 ELSE 1 END
		LIMIT 1
	`, repoID, subject).Scan(&role)
	if err != nil {
		return "", err
	}
	return role, nil
}

func (s *Store) ListGroupMembers(ctx context.Context, groupRepoID uuid.UUID) ([]Repository, error) {
	rows, err := s.db.Query(ctx, `
		SELECT r.id, r.name, r.type::text, r.upstream_url, r.enabled
		FROM group_members gm
		JOIN repositories r ON r.id = gm.member_repo_id
		WHERE gm.group_repo_id = $1
		ORDER BY gm.priority ASC, r.name ASC
	`, groupRepoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []Repository
	for rows.Next() {
		var repo Repository
		if err := rows.Scan(&repo.ID, &repo.Name, &repo.Type, &repo.UpstreamURL, &repo.Enabled); err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return repos, nil
}

// ListAccessibleGroupMembers returns group member repos that the given subject
// can access. A member is accessible if:
//   - the subject has any role on it, OR
//   - the wildcard subject '*' has a role on it (public repo).
//
// If allAccess is true, all members are returned (admin shortcut).
func (s *Store) ListAccessibleGroupMembers(ctx context.Context, groupRepoID uuid.UUID, subject string, allAccess bool) ([]Repository, error) {
	if allAccess || subject == "" {
		return s.ListGroupMembers(ctx, groupRepoID)
	}

	rows, err := s.db.Query(ctx, `
		SELECT DISTINCT r.id, r.name, r.type::text, r.upstream_url, r.enabled, gm.priority
		FROM group_members gm
		JOIN repositories r ON r.id = gm.member_repo_id
		WHERE gm.group_repo_id = $1
		  AND (
			EXISTS (SELECT 1 FROM repo_members rm WHERE rm.repo_id = r.id AND rm.subject = $2)
			OR EXISTS (SELECT 1 FROM repo_members rm WHERE rm.repo_id = r.id AND rm.subject = '*')
		  )
		ORDER BY gm.priority ASC, r.name ASC
	`, groupRepoID, subject)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []Repository
	for rows.Next() {
		var repo Repository
		var priority int
		if err := rows.Scan(&repo.ID, &repo.Name, &repo.Type, &repo.UpstreamURL, &repo.Enabled, &priority); err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}
	return repos, rows.Err()
}

type RepoMember struct {
	RepoID    uuid.UUID
	RepoName  string
	Subject   string
	Role      string
	CreatedAt time.Time
}

func (s *Store) ListRepoMembers(ctx context.Context, repoID uuid.UUID) ([]RepoMember, error) {
	rows, err := s.db.Query(ctx, `
		SELECT rm.repo_id, r.name, rm.subject, rm.role::text, rm.created_at
		FROM repo_members rm
		JOIN repositories r ON r.id = rm.repo_id
		WHERE rm.repo_id = $1
		ORDER BY rm.subject
	`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []RepoMember
	for rows.Next() {
		var m RepoMember
		if err := rows.Scan(&m.RepoID, &m.RepoName, &m.Subject, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *Store) ListAllRepoMembers(ctx context.Context) ([]RepoMember, error) {
	rows, err := s.db.Query(ctx, `
		SELECT rm.repo_id, r.name, rm.subject, rm.role::text, rm.created_at
		FROM repo_members rm
		JOIN repositories r ON r.id = rm.repo_id
		ORDER BY r.name, rm.subject
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []RepoMember
	for rows.Next() {
		var m RepoMember
		if err := rows.Scan(&m.RepoID, &m.RepoName, &m.Subject, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *Store) RemoveRepoMember(ctx context.Context, repoID uuid.UUID, subject string) error {
	ct, err := s.db.Exec(ctx, `
		DELETE FROM repo_members
		WHERE repo_id = $1 AND subject = $2
	`, repoID, subject)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// HasAnyRepoAccess checks if the subject has any role on the given repo,
// either directly or via the '*' wildcard.
func (s *Store) HasAnyRepoAccess(ctx context.Context, repoID uuid.UUID, subject string) (bool, error) {
	var n int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM repo_members
		WHERE repo_id = $1
		  AND subject IN ($2, '*')
	`, repoID, subject).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Store) EnsurePackageTx(
	ctx context.Context,
	tx pgx.Tx,
	repoID uuid.UUID,
	slug string,
	createdBy string,
) (uuid.UUID, error) {
	var packageID uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO packages (repo_id, name, display_name, created_by)
		VALUES ($1, $2, $2, $3)
		ON CONFLICT (repo_id, name)
		DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, repoID, slug, createdBy).Scan(&packageID)
	return packageID, err
}

func (s *Store) UpdatePackageMetaTx(
	ctx context.Context,
	tx pgx.Tx,
	packageID uuid.UUID,
	displayName string,
	summary *string,
	tagPatch json.RawMessage,
) error {
	if displayName == "" {
		displayName = ""
	}
	if len(tagPatch) == 0 {
		tagPatch = json.RawMessage(`{}`)
	}

	_, err := tx.Exec(ctx, `
		UPDATE packages
		SET
			display_name = CASE
				WHEN $2 = '' THEN display_name
				ELSE $2
			END,
			summary = COALESCE($3, summary),
			tags = COALESCE(tags, '{}'::jsonb) || $4::jsonb,
			deleted_at = NULL,
			updated_at = now()
		WHERE id = $1
	`, packageID, displayName, summary, tagPatch)
	return err
}

func (s *Store) InsertVersionTx(
	ctx context.Context,
	tx pgx.Tx,
	packageID uuid.UUID,
	version string,
	digest string,
	sizeBytes int64,
	changelog string,
	changelogSource *string,
	files json.RawMessage,
	createdBy string,
) (uuid.UUID, error) {
	if len(files) == 0 {
		files = json.RawMessage(`[]`)
	}

	var versionID uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO versions (package_id, version, digest, size_bytes, changelog, changelog_source, files, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)
		RETURNING id
	`, packageID, version, digest, sizeBytes, changelog, changelogSource, files, createdBy).Scan(&versionID)
	if err != nil {
		if isUniqueViolation(err) {
			return uuid.Nil, ErrConflict
		}
		return uuid.Nil, err
	}
	return versionID, nil
}

func (s *Store) InsertAssetTx(
	ctx context.Context,
	tx pgx.Tx,
	versionID uuid.UUID,
	fileName string,
	blobPath string,
	sizeBytes int64,
	digest string,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO assets (version_id, path, blob_path, size_bytes, digest)
		VALUES ($1, $2, $3, $4, $5)
	`, versionID, fileName, blobPath, sizeBytes, digest)
	return err
}

func (s *Store) UpdateVersionFiles(ctx context.Context, versionID uuid.UUID, files json.RawMessage) error {
	if len(files) == 0 {
		files = json.RawMessage(`[]`)
	}
	tag, err := s.db.Exec(ctx, `
		UPDATE versions
		SET files = $2::jsonb
		WHERE id = $1
	`, versionID, files)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateVersionMeta(
	ctx context.Context,
	repoID uuid.UUID,
	slug string,
	version string,
	createdAt *time.Time,
	changelog *string,
	changelogSource *string,
) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE versions v
		SET
			created_at = COALESCE($4::timestamptz, v.created_at),
			changelog = COALESCE($5::text, v.changelog),
			changelog_source = COALESCE($6::text, v.changelog_source)
		FROM packages p
		WHERE p.id = v.package_id
		  AND p.repo_id = $1
		  AND p.name = $2
		  AND p.deleted_at IS NULL
		  AND v.version = $3
	`, repoID, slug, version, createdAt, changelog, changelogSource)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) GetArtifact(ctx context.Context, repoID uuid.UUID, slug string, version string) (Artifact, error) {
	var a Artifact
	err := s.db.QueryRow(ctx, `
		SELECT r.id, r.name, p.name, v.version, a.path, a.digest, a.size_bytes, a.blob_path
		FROM repositories r
		JOIN packages p ON p.repo_id = r.id
		JOIN versions v ON v.package_id = p.id
		JOIN assets a ON a.version_id = v.id
		WHERE r.id = $1
		  AND p.name = $2
		  AND p.deleted_at IS NULL
		  AND v.version = $3
		ORDER BY a.created_at
		LIMIT 1
	`, repoID, slug, version).Scan(
		&a.RepoID,
		&a.RepoName,
		&a.PackageName,
		&a.Version,
		&a.FileName,
		&a.Digest,
		&a.SizeBytes,
		&a.BlobPath,
	)
	if err != nil {
		return Artifact{}, err
	}
	return a, nil
}

func (s *Store) GetLatestArtifact(ctx context.Context, repoID uuid.UUID, slug string) (Artifact, error) {
	var a Artifact
	err := s.db.QueryRow(ctx, `
		SELECT r.id, r.name, p.name, v.version, a.path, a.digest, a.size_bytes, a.blob_path
		FROM repositories r
		JOIN packages p ON p.repo_id = r.id
		JOIN versions v ON v.package_id = p.id
		JOIN assets a ON a.version_id = v.id
		WHERE r.id = $1
		  AND p.name = $2
		  AND p.deleted_at IS NULL
		ORDER BY v.created_at DESC, a.created_at ASC
		LIMIT 1
	`, repoID, slug).Scan(
		&a.RepoID,
		&a.RepoName,
		&a.PackageName,
		&a.Version,
		&a.FileName,
		&a.Digest,
		&a.SizeBytes,
		&a.BlobPath,
	)
	if err != nil {
		return Artifact{}, err
	}
	return a, nil
}

func (s *Store) GetProxyCache(ctx context.Context, repoID uuid.UUID, packageName, version string) (ProxyCacheEntry, error) {
	var entry ProxyCacheEntry
	err := s.db.QueryRow(ctx, `
		SELECT status::text, etag, expires_at, last_checked, last_error
		FROM proxy_cache
		WHERE repo_id = $1
		  AND package_name = $2
		  AND version = $3
	`, repoID, packageName, version).Scan(
		&entry.Status,
		&entry.ETag,
		&entry.ExpiresAt,
		&entry.LastChecked,
		&entry.LastError,
	)
	if err != nil {
		return ProxyCacheEntry{}, err
	}
	return entry, nil
}

func (s *Store) UpsertProxyCache(
	ctx context.Context,
	repoID uuid.UUID,
	packageName string,
	version string,
	status string,
	etag *string,
	expiresAt *time.Time,
	lastError *string,
) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO proxy_cache (repo_id, package_name, version, status, etag, expires_at, last_error, last_checked)
		VALUES ($1, $2, $3, $4::proxy_cache_status, $5, $6, $7, now())
		ON CONFLICT (repo_id, package_name, version)
		DO UPDATE SET
			status = EXCLUDED.status,
			etag = EXCLUDED.etag,
			expires_at = EXCLUDED.expires_at,
			last_error = EXCLUDED.last_error,
			last_checked = now()
	`, repoID, packageName, version, status, etag, expiresAt, lastError)
	return err
}

func (s *Store) SearchSkills(ctx context.Context, repoID uuid.UUID, query string, limit int) ([]SkillSearchResult, error) {
	pattern := "%" + query + "%"
	rows, err := s.db.Query(ctx, `
		SELECT
			p.name,
			p.display_name,
			p.summary,
			lv.version,
			CASE
				WHEN lower(p.name) = lower($2) THEN 1.0
				WHEN p.name ILIKE $3 THEN 0.9
				WHEN p.display_name ILIKE $3 THEN 0.8
				ELSE 0.6
			END AS score,
			p.updated_at
		FROM packages p
		LEFT JOIN LATERAL (
			SELECT v.version
			FROM versions v
			WHERE v.package_id = p.id
			ORDER BY v.created_at DESC
			LIMIT 1
		) lv ON true
		WHERE p.repo_id = $1
		  AND p.deleted_at IS NULL
		  AND (
			p.name ILIKE $4
			OR p.display_name ILIKE $4
			OR COALESCE(p.summary, '') ILIKE $4
		  )
		ORDER BY score DESC, p.updated_at DESC, p.name ASC
		LIMIT $5
	`, repoID, query, query+"%", pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SkillSearchResult
	for rows.Next() {
		var r SkillSearchResult
		if err := rows.Scan(&r.Slug, &r.DisplayName, &r.Summary, &r.Version, &r.Score, &r.UpdatedAt); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return results, nil
}

func (s *Store) ListSkills(ctx context.Context, repoID uuid.UUID, limit int, offset int, sort string) ([]SkillListItem, error) {
	sortClause := skillSortClause(sort)
	query := fmt.Sprintf(`
		SELECT
			p.id,
			p.name,
			p.display_name,
			p.summary,
			p.tags,
			p.downloads,
			p.stars,
			p.installs_current,
			p.installs_all_time,
			p.created_at,
			p.updated_at,
			lv.version,
			lv.created_at,
			lv.changelog
		FROM packages p
		LEFT JOIN LATERAL (
			SELECT v.version, v.created_at, v.changelog
			FROM versions v
			WHERE v.package_id = p.id
			ORDER BY v.created_at DESC
			LIMIT 1
		) lv ON true
		WHERE p.repo_id = $1
		  AND p.deleted_at IS NULL
		ORDER BY %s, p.name ASC
		LIMIT $2 OFFSET $3
	`, sortClause)

	rows, err := s.db.Query(ctx, query, repoID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []SkillListItem
	for rows.Next() {
		var item SkillListItem
		var latestVersion *string
		var latestCreated *time.Time
		var latestChangelog *string

		if err := rows.Scan(
			&item.ID,
			&item.Slug,
			&item.DisplayName,
			&item.Summary,
			&item.Tags,
			&item.Downloads,
			&item.Stars,
			&item.InstallsCurrent,
			&item.InstallsAllTime,
			&item.CreatedAt,
			&item.UpdatedAt,
			&latestVersion,
			&latestCreated,
			&latestChangelog,
		); err != nil {
			return nil, err
		}
		if latestVersion != nil && latestCreated != nil {
			changelog := ""
			if latestChangelog != nil {
				changelog = *latestChangelog
			}
			item.LatestVersion = &SkillVersionSummary{
				Version:   *latestVersion,
				CreatedAt: *latestCreated,
				Changelog: changelog,
			}
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return items, nil
}

func (s *Store) GetSkill(ctx context.Context, repoID uuid.UUID, slug string) (Skill, error) {
	var skill Skill
	err := s.db.QueryRow(ctx, `
		SELECT id, name, display_name, summary, tags, downloads, stars, installs_current, installs_all_time, created_at, updated_at
		FROM packages
		WHERE repo_id = $1
		  AND name = $2
		  AND deleted_at IS NULL
	`, repoID, slug).Scan(
		&skill.ID,
		&skill.Slug,
		&skill.DisplayName,
		&skill.Summary,
		&skill.Tags,
		&skill.Downloads,
		&skill.Stars,
		&skill.InstallsCurrent,
		&skill.InstallsAllTime,
		&skill.CreatedAt,
		&skill.UpdatedAt,
	)
	if err != nil {
		return Skill{}, err
	}
	return skill, nil
}

func (s *Store) GetLatestVersionForSkill(ctx context.Context, packageID uuid.UUID) (SkillVersion, error) {
	var version SkillVersion
	err := s.db.QueryRow(ctx, `
		SELECT id, package_id, version, digest, size_bytes, changelog, changelog_source, files, created_at
		FROM versions
		WHERE package_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, packageID).Scan(
		&version.ID,
		&version.PackageID,
		&version.Version,
		&version.Digest,
		&version.SizeBytes,
		&version.Changelog,
		&version.ChangelogSource,
		&version.Files,
		&version.CreatedAt,
	)
	if err != nil {
		return SkillVersion{}, err
	}
	return version, nil
}

func (s *Store) ListSkillVersions(
	ctx context.Context,
	repoID uuid.UUID,
	slug string,
	limit int,
	offset int,
) ([]SkillVersion, error) {
	rows, err := s.db.Query(ctx, `
		SELECT v.id, v.package_id, v.version, v.digest, v.size_bytes, v.changelog, v.changelog_source, v.files, v.created_at
		FROM versions v
		JOIN packages p ON p.id = v.package_id
		WHERE p.repo_id = $1
		  AND p.name = $2
		  AND p.deleted_at IS NULL
		ORDER BY v.created_at DESC
		LIMIT $3 OFFSET $4
	`, repoID, slug, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []SkillVersion
	for rows.Next() {
		var version SkillVersion
		if err := rows.Scan(
			&version.ID,
			&version.PackageID,
			&version.Version,
			&version.Digest,
			&version.SizeBytes,
			&version.Changelog,
			&version.ChangelogSource,
			&version.Files,
			&version.CreatedAt,
		); err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return versions, nil
}

func (s *Store) GetSkillVersion(ctx context.Context, repoID uuid.UUID, slug string, version string) (Skill, SkillVersion, error) {
	var skill Skill
	var sv SkillVersion
	err := s.db.QueryRow(ctx, `
		SELECT
			p.id,
			p.name,
			p.display_name,
			p.summary,
			p.tags,
			p.downloads,
			p.stars,
			p.installs_current,
			p.installs_all_time,
			p.created_at,
			p.updated_at,
			v.id,
			v.package_id,
			v.version,
			v.digest,
			v.size_bytes,
			v.changelog,
			v.changelog_source,
			v.files,
			v.created_at
		FROM packages p
		JOIN versions v ON v.package_id = p.id
		WHERE p.repo_id = $1
		  AND p.name = $2
		  AND p.deleted_at IS NULL
		  AND v.version = $3
		LIMIT 1
	`, repoID, slug, version).Scan(
		&skill.ID,
		&skill.Slug,
		&skill.DisplayName,
		&skill.Summary,
		&skill.Tags,
		&skill.Downloads,
		&skill.Stars,
		&skill.InstallsCurrent,
		&skill.InstallsAllTime,
		&skill.CreatedAt,
		&skill.UpdatedAt,
		&sv.ID,
		&sv.PackageID,
		&sv.Version,
		&sv.Digest,
		&sv.SizeBytes,
		&sv.Changelog,
		&sv.ChangelogSource,
		&sv.Files,
		&sv.CreatedAt,
	)
	if err != nil {
		return Skill{}, SkillVersion{}, err
	}
	return skill, sv, nil
}

func (s *Store) ResolveVersionByHash(ctx context.Context, repoID uuid.UUID, slug string, hash string) (*string, error) {
	var version string
	err := s.db.QueryRow(ctx, `
		SELECT v.version
		FROM versions v
		JOIN packages p ON p.id = v.package_id
		WHERE p.repo_id = $1
		  AND p.name = $2
		  AND p.deleted_at IS NULL
		  AND (v.digest = $3 OR replace(v.digest, 'sha256:', '') = $3)
		ORDER BY v.created_at DESC
		LIMIT 1
	`, repoID, slug, hash).Scan(&version)
	if err != nil {
		if IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &version, nil
}

func (s *Store) ResolveVersionByTag(ctx context.Context, repoID uuid.UUID, slug string, tag string) (*string, error) {
	var version *string
	err := s.db.QueryRow(ctx, `
		SELECT tags ->> $3
		FROM packages
		WHERE repo_id = $1
		  AND name = $2
		  AND deleted_at IS NULL
	`, repoID, slug, tag).Scan(&version)
	if err != nil {
		if IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return version, nil
}

func (s *Store) IncrementSkillDownloads(ctx context.Context, repoID uuid.UUID, slug string) error {
	tagWeight := 1.0
	cmdTag := "latest"
	_, _ = tagWeight, cmdTag // placeholders for future weighting logic.

	ct, err := s.db.Exec(ctx, `
		UPDATE packages
		SET
			downloads = downloads + 1,
			installs_current = installs_current + 1,
			installs_all_time = installs_all_time + 1,
			trending_score = trending_score + 1,
			updated_at = updated_at
		WHERE repo_id = $1
		  AND name = $2
		  AND deleted_at IS NULL
	`, repoID, slug)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) SetSkillDeleted(ctx context.Context, repoID uuid.UUID, slug string, deleted bool) error {
	var deletedAt any
	if deleted {
		deletedAt = time.Now().UTC()
	}
	ct, err := s.db.Exec(ctx, `
		UPDATE packages
		SET deleted_at = $3, updated_at = now()
		WHERE repo_id = $1
		  AND name = $2
	`, repoID, slug, deletedAt)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// APIToken represents a row in the api_tokens table.
type APIToken struct {
	ID         uuid.UUID  `json:"id"`
	Subject    string     `json:"subject"`
	Name       string     `json:"name"`
	TokenType  string     `json:"token_type"`
	IsAdmin    bool       `json:"is_admin"`
	Disabled   bool       `json:"disabled"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

const (
	TokenTypeSession  = "session"
	TokenTypePersonal = "personal"
)

// CreatePersonalToken inserts a new personal access token.
func (s *Store) CreatePersonalToken(ctx context.Context, subject, name, tokenHash string, isAdmin bool) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.db.QueryRow(ctx, `
		INSERT INTO api_tokens (token_hash, subject, name, token_type, is_admin)
		VALUES ($1, $2, $3, 'personal', $4)
		RETURNING id
	`, tokenHash, subject, name, isAdmin).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return uuid.Nil, ErrConflict
		}
		return uuid.Nil, err
	}
	return id, nil
}

// UpsertSessionToken creates or replaces the session token for a subject.
// Each subject has at most one active session token.
func (s *Store) UpsertSessionToken(ctx context.Context, subject, tokenHash string, isAdmin bool) error {
	_, err := s.db.Exec(ctx, `
		DELETE FROM api_tokens WHERE subject = $1 AND token_type = 'session'
	`, subject)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO api_tokens (token_hash, subject, name, token_type, is_admin)
		VALUES ($1, $2, '', 'session', $3)
	`, tokenHash, subject, isAdmin)
	return err
}

// AuthenticateToken looks up a token by hash and returns its metadata.
func (s *Store) AuthenticateToken(ctx context.Context, tokenHash string) (APIToken, error) {
	var t APIToken
	err := s.db.QueryRow(ctx, `
		SELECT id, subject, name, token_type, is_admin, disabled, created_at, last_used_at
		FROM api_tokens
		WHERE token_hash = $1
	`, tokenHash).Scan(&t.ID, &t.Subject, &t.Name, &t.TokenType, &t.IsAdmin, &t.Disabled, &t.CreatedAt, &t.LastUsedAt)
	if err != nil {
		return APIToken{}, err
	}
	return t, nil
}

// TouchTokenLastUsed updates the last_used_at timestamp.
func (s *Store) TouchTokenLastUsed(ctx context.Context, id uuid.UUID) {
	_, _ = s.db.Exec(ctx, `UPDATE api_tokens SET last_used_at = now() WHERE id = $1`, id)
}

// ListTokensBySubject returns all personal tokens for a given subject.
func (s *Store) ListTokensBySubject(ctx context.Context, subject string) ([]APIToken, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, subject, name, token_type, is_admin, disabled, created_at, last_used_at
		FROM api_tokens
		WHERE subject = $1 AND token_type = 'personal'
		ORDER BY created_at DESC
	`, subject)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tokens []APIToken
	for rows.Next() {
		var t APIToken
		if err := rows.Scan(&t.ID, &t.Subject, &t.Name, &t.TokenType, &t.IsAdmin, &t.Disabled, &t.CreatedAt, &t.LastUsedAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// RevokeToken deletes a token by id, scoped to a subject (unless allAccess).
func (s *Store) RevokeToken(ctx context.Context, tokenID uuid.UUID, subject string, allAccess bool) error {
	var ct pgconn.CommandTag
	var err error
	if allAccess {
		ct, err = s.db.Exec(ctx, `DELETE FROM api_tokens WHERE id = $1`, tokenID)
	} else {
		ct, err = s.db.Exec(ctx, `DELETE FROM api_tokens WHERE id = $1 AND subject = $2`, tokenID, subject)
	}
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func skillSortClause(sort string) string {
	switch sort {
	case "downloads":
		return "p.downloads DESC, p.updated_at DESC"
	case "stars":
		return "p.stars DESC, p.updated_at DESC"
	case "installsCurrent":
		return "p.installs_current DESC, p.updated_at DESC"
	case "installsAllTime":
		return "p.installs_all_time DESC, p.updated_at DESC"
	case "trending":
		return "p.trending_score DESC, p.updated_at DESC"
	default:
		return "p.updated_at DESC"
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func (s *Store) ListRepositories(ctx context.Context) ([]Repository, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, name, type::text, upstream_url, enabled
		FROM repositories
		ORDER BY type, name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []Repository
	for rows.Next() {
		var repo Repository
		if err := rows.Scan(&repo.ID, &repo.Name, &repo.Type, &repo.UpstreamURL, &repo.Enabled); err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}
	return repos, rows.Err()
}

type DashboardStats struct {
	TotalSkills   int64
	TotalVersions int64
	TotalDownloads int64
	TotalStars    int64
	TotalInstalls int64
}

func (s *Store) GetDashboardStats(ctx context.Context, repoIDs []uuid.UUID) (DashboardStats, error) {
	var stats DashboardStats
	err := s.db.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(downloads), 0),
			COALESCE(SUM(stars), 0),
			COALESCE(SUM(installs_all_time), 0)
		FROM packages
		WHERE repo_id = ANY($1)
		  AND deleted_at IS NULL
	`, repoIDs).Scan(&stats.TotalSkills, &stats.TotalDownloads, &stats.TotalStars, &stats.TotalInstalls)
	if err != nil {
		return DashboardStats{}, err
	}

	err = s.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM versions v
		JOIN packages p ON p.id = v.package_id
		WHERE p.repo_id = ANY($1)
		  AND p.deleted_at IS NULL
	`, repoIDs).Scan(&stats.TotalVersions)
	if err != nil {
		return DashboardStats{}, err
	}

	return stats, nil
}

type RepoStats struct {
	Repository Repository
	SkillCount int64
}

func (s *Store) GetRepoStats(ctx context.Context) ([]RepoStats, error) {
	rows, err := s.db.Query(ctx, `
		SELECT r.id, r.name, r.type::text, r.upstream_url, r.enabled,
			COALESCE(cnt.n, 0)
		FROM repositories r
		LEFT JOIN (
			SELECT repo_id, COUNT(*) AS n
			FROM packages
			WHERE deleted_at IS NULL
			GROUP BY repo_id
		) cnt ON cnt.repo_id = r.id
		ORDER BY r.type, r.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []RepoStats
	for rows.Next() {
		var rs RepoStats
		if err := rows.Scan(
			&rs.Repository.ID, &rs.Repository.Name, &rs.Repository.Type,
			&rs.Repository.UpstreamURL, &rs.Repository.Enabled,
			&rs.SkillCount,
		); err != nil {
			return nil, err
		}
		result = append(result, rs)
	}
	return result, rows.Err()
}

func (s *Store) DeleteRepository(ctx context.Context, id uuid.UUID) error {
	ct, err := s.db.Exec(ctx, `DELETE FROM repositories WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateRepositoryEnabled(ctx context.Context, id uuid.UUID, enabled bool) error {
	ct, err := s.db.Exec(ctx, `UPDATE repositories SET enabled = $2 WHERE id = $1`, id, enabled)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func ValidateRepoType(repoType string) error {
	switch repoType {
	case RepoTypeHosted, RepoTypeProxy, RepoTypeGroup:
		return nil
	default:
		return fmt.Errorf("invalid repository type: %s", repoType)
	}
}

func ValidateRole(role string) error {
	switch role {
	case RoleRead, RolePush, RoleAdmin:
		return nil
	default:
		return fmt.Errorf("invalid role: %s", role)
	}
}

// ---- Local Users ----

type User struct {
	ID           uuid.UUID `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	DisplayName  string    `json:"display_name"`
	Email        string    `json:"email"`
	IsAdmin      bool      `json:"is_admin"`
	Disabled     bool      `json:"disabled"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *Store) CreateUser(ctx context.Context, username, passwordHash, displayName, email string, isAdmin bool) (User, error) {
	var u User
	err := s.db.QueryRow(ctx, `
		INSERT INTO users (username, password_hash, display_name, email, is_admin)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, username, password_hash, display_name, email, is_admin, disabled, created_at
	`, username, passwordHash, displayName, email, isAdmin).Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Email, &u.IsAdmin, &u.Disabled, &u.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrConflict
		}
		return User{}, err
	}
	return u, nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (User, error) {
	var u User
	err := s.db.QueryRow(ctx, `
		SELECT id, username, password_hash, display_name, email, is_admin, disabled, created_at
		FROM users WHERE username = $1
	`, username).Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Email, &u.IsAdmin, &u.Disabled, &u.CreatedAt,
	)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, username, password_hash, display_name, email, is_admin, disabled, created_at
		FROM users ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Email, &u.IsAdmin, &u.Disabled, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (User, error) {
	var u User
	err := s.db.QueryRow(ctx, `
		SELECT id, username, password_hash, display_name, email, is_admin, disabled, created_at
		FROM users WHERE id = $1
	`, id).Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Email, &u.IsAdmin, &u.Disabled, &u.CreatedAt,
	)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

func (s *Store) UpdateUser(ctx context.Context, id uuid.UUID, displayName, email string, isAdmin, disabled bool) (User, error) {
	var u User
	err := s.db.QueryRow(ctx, `
		UPDATE users SET display_name = $2, email = $3, is_admin = $4, disabled = $5
		WHERE id = $1
		RETURNING id, username, password_hash, display_name, email, is_admin, disabled, created_at
	`, id, displayName, email, isAdmin, disabled).Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Email, &u.IsAdmin, &u.Disabled, &u.CreatedAt,
	)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

func (s *Store) UpdateUserPassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	ct, err := s.db.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, id, passwordHash)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteUser(ctx context.Context, id uuid.UUID) error {
	ct, err := s.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ---- Auth Config (LDAP) ----

type AuthConfig struct {
	ProviderType string          `json:"provider_type"`
	Enabled      bool            `json:"enabled"`
	Config       json.RawMessage `json:"config"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

func (s *Store) GetAuthConfig(ctx context.Context, providerType string) (AuthConfig, error) {
	var ac AuthConfig
	err := s.db.QueryRow(ctx,
		`SELECT provider_type, enabled, config, updated_at FROM auth_configs WHERE provider_type = $1`,
		providerType,
	).Scan(&ac.ProviderType, &ac.Enabled, &ac.Config, &ac.UpdatedAt)
	return ac, err
}

func (s *Store) ListAuthConfigs(ctx context.Context) ([]AuthConfig, error) {
	rows, err := s.db.Query(ctx, `SELECT provider_type, enabled, config, updated_at FROM auth_configs ORDER BY provider_type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var configs []AuthConfig
	for rows.Next() {
		var ac AuthConfig
		if err := rows.Scan(&ac.ProviderType, &ac.Enabled, &ac.Config, &ac.UpdatedAt); err != nil {
			return nil, err
		}
		configs = append(configs, ac)
	}
	return configs, rows.Err()
}

func (s *Store) UpsertAuthConfig(ctx context.Context, providerType string, enabled bool, config json.RawMessage) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO auth_configs (provider_type, enabled, config, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (provider_type) DO UPDATE
		SET enabled = EXCLUDED.enabled, config = EXCLUDED.config, updated_at = now()
	`, providerType, enabled, config)
	return err
}

func (s *Store) DeleteAuthConfig(ctx context.Context, providerType string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM auth_configs WHERE provider_type = $1`, providerType)
	return err
}

// ---- System Config (generic key-value) ----

func (s *Store) GetSystemConfig(ctx context.Context, key string) (json.RawMessage, error) {
	var raw json.RawMessage
	err := s.db.QueryRow(ctx,
		`SELECT config FROM system_configs WHERE config_key = $1`, key,
	).Scan(&raw)
	return raw, err
}

func (s *Store) UpsertSystemConfig(ctx context.Context, key string, config json.RawMessage) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO system_configs (config_key, config, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (config_key) DO UPDATE
		SET config = EXCLUDED.config, updated_at = now()
	`, key, config)
	return err
}
