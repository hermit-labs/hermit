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

func (s *Store) CreateToken(ctx context.Context, subject, tokenHash string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO api_tokens (token_hash, subject)
		VALUES ($1, $2)
	`, tokenHash, subject)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return err
	}
	return nil
}

func (s *Store) GetTokenSubjectByHash(ctx context.Context, tokenHash string) (string, bool, error) {
	var subject string
	var disabled bool
	err := s.db.QueryRow(ctx, `
		SELECT subject, disabled
		FROM api_tokens
		WHERE token_hash = $1
	`, tokenHash).Scan(&subject, &disabled)
	if err != nil {
		return "", false, err
	}
	return subject, disabled, nil
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
