package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"hermit/internal/service"

	"github.com/labstack/echo/v4"
)

func (h *Handler) WellKnown(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"apiBase": strings.TrimSuffix(baseURL(c.Request()), "/"),
	})
}

func (h *Handler) Search(c echo.Context) error {
	repo, err := h.svc.GetReadRepository(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}

	query := strings.TrimSpace(c.QueryParam("q"))
	if query == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "q is required")
	}
	limit := clampInt(queryInt(c, "limit", 20), 1, 200)

	results, err := h.svc.SearchSkills(c.Request().Context(), repo, query, limit)
	if err != nil {
		return mapServiceError(err)
	}

	out := make([]map[string]any, 0, len(results))
	for _, item := range results {
		out = append(out, map[string]any{
			"slug":        item.Slug,
			"displayName": item.DisplayName,
			"summary":     item.Summary,
			"version":     item.Version,
			"score":       item.Score,
			"updatedAt":   toMillisPtr(item.UpdatedAt),
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"results": out})
}

func (h *Handler) ListSkills(c echo.Context) error {
	repo, err := h.svc.GetReadRepository(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}

	limit := clampInt(queryInt(c, "limit", 25), 1, 200)
	offset, err := decodeCursor(c.QueryParam("cursor"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid cursor")
	}
	sortBy, err := normalizeSort(c.QueryParam("sort"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	items, err := h.svc.ListSkills(c.Request().Context(), repo, limit+1, offset, sortBy)
	if err != nil {
		return mapServiceError(err)
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	respItems := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload := map[string]any{
			"slug":        item.Slug,
			"displayName": item.DisplayName,
			"summary":     item.Summary,
			"tags":        decodeAnyJSON(item.Tags, map[string]any{}),
			"stats": map[string]any{
				"downloads":       item.Downloads,
				"stars":           item.Stars,
				"installsCurrent": item.InstallsCurrent,
				"installsAllTime": item.InstallsAllTime,
			},
			"createdAt": toMillis(item.CreatedAt),
			"updatedAt": toMillis(item.UpdatedAt),
		}
		if item.LatestVersion != nil {
			payload["latestVersion"] = map[string]any{
				"version":   item.LatestVersion.Version,
				"createdAt": toMillis(item.LatestVersion.CreatedAt),
				"changelog": item.LatestVersion.Changelog,
			}
		}
		respItems = append(respItems, payload)
	}

	var nextCursor any = nil
	if hasMore {
		nextCursor = encodeCursor(offset + limit)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"items":      respItems,
		"nextCursor": nextCursor,
	})
}

func (h *Handler) GetSkill(c echo.Context) error {
	repo, err := h.svc.GetReadRepository(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}

	slug := strings.TrimSpace(c.Param("slug"))
	view, err := h.svc.GetSkill(c.Request().Context(), repo, slug)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return c.JSON(http.StatusOK, map[string]any{
				"skill":         nil,
				"latestVersion": nil,
				"owner":         nil,
			})
		}
		return mapServiceError(err)
	}

	resp := map[string]any{
		"skill": map[string]any{
			"slug":        view.Skill.Slug,
			"displayName": view.Skill.DisplayName,
			"summary":     view.Skill.Summary,
			"tags":        decodeAnyJSON(view.Skill.Tags, map[string]any{}),
			"stats": map[string]any{
				"downloads":       view.Skill.Downloads,
				"stars":           view.Skill.Stars,
				"installsCurrent": view.Skill.InstallsCurrent,
				"installsAllTime": view.Skill.InstallsAllTime,
			},
			"createdAt": toMillis(view.Skill.CreatedAt),
			"updatedAt": toMillis(view.Skill.UpdatedAt),
		},
		"latestVersion": nil,
		"owner":         nil,
	}
	if view.LatestVersion != nil {
		resp["latestVersion"] = map[string]any{
			"version":   view.LatestVersion.Version,
			"createdAt": toMillis(view.LatestVersion.CreatedAt),
			"changelog": view.LatestVersion.Changelog,
		}
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListVersions(c echo.Context) error {
	repo, err := h.svc.GetReadRepository(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}

	slug := strings.TrimSpace(c.Param("slug"))
	limit := clampInt(queryInt(c, "limit", 25), 1, 200)
	offset, err := decodeCursor(c.QueryParam("cursor"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid cursor")
	}

	versions, err := h.svc.ListSkillVersions(c.Request().Context(), repo, slug, limit+1, offset)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return c.JSON(http.StatusOK, map[string]any{"items": []any{}, "nextCursor": nil})
		}
		return mapServiceError(err)
	}

	hasMore := len(versions) > limit
	if hasMore {
		versions = versions[:limit]
	}
	items := make([]map[string]any, 0, len(versions))
	for _, v := range versions {
		items = append(items, map[string]any{
			"version":         v.Version,
			"createdAt":       toMillis(v.CreatedAt),
			"changelog":       v.Changelog,
			"changelogSource": v.ChangelogSource,
		})
	}
	var nextCursor any = nil
	if hasMore {
		nextCursor = encodeCursor(offset + limit)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items, "nextCursor": nextCursor})
}

func (h *Handler) GetVersion(c echo.Context) error {
	repo, err := h.svc.GetReadRepository(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}

	slug := strings.TrimSpace(c.Param("slug"))
	version := strings.TrimSpace(c.Param("version"))
	view, err := h.svc.GetSkillVersion(c.Request().Context(), repo, slug, version)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return c.JSON(http.StatusOK, map[string]any{"version": nil, "skill": nil})
		}
		return mapServiceError(err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"version": map[string]any{
			"version":         view.Version.Version,
			"createdAt":       toMillis(view.Version.CreatedAt),
			"changelog":       view.Version.Changelog,
			"changelogSource": view.Version.ChangelogSource,
			"files":           decodeAnyJSON(view.Version.Files, []any{}),
		},
		"skill": map[string]any{
			"slug":        view.Skill.Slug,
			"displayName": view.Skill.DisplayName,
		},
	})
}

func (h *Handler) GetSkillFile(c echo.Context) error {
	repo, err := h.svc.GetReadRepository(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}

	slug := strings.TrimSpace(c.Param("slug"))
	filePath := strings.TrimSpace(c.QueryParam("path"))
	version := strings.TrimSpace(c.QueryParam("version"))
	tag := strings.TrimSpace(c.QueryParam("tag"))

	content, err := h.svc.ReadSkillFile(c.Request().Context(), repo, slug, version, tag, filePath)
	if err != nil {
		return mapServiceError(err)
	}
	return c.Blob(http.StatusOK, "text/plain; charset=utf-8", content)
}

func (h *Handler) Resolve(c echo.Context) error {
	repo, err := h.svc.GetReadRepository(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}

	slug := strings.TrimSpace(c.QueryParam("slug"))
	hash := strings.TrimSpace(c.QueryParam("hash"))
	if slug == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "slug is required")
	}

	result, err := h.svc.ResolveSkillVersion(c.Request().Context(), repo, slug, hash)
	if err != nil {
		return mapServiceError(err)
	}

	resp := map[string]any{
		"match":         nil,
		"latestVersion": nil,
	}
	if result.MatchVersion != nil {
		resp["match"] = map[string]any{"version": *result.MatchVersion}
	}
	if result.LatestVersion != nil {
		resp["latestVersion"] = map[string]any{"version": *result.LatestVersion}
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) Download(c echo.Context) error {
	repo, err := h.svc.GetReadRepository(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}

	slug := strings.TrimSpace(c.QueryParam("slug"))
	version := strings.TrimSpace(c.QueryParam("version"))
	tag := strings.TrimSpace(c.QueryParam("tag"))
	if slug == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "slug is required")
	}

	artifact, err := h.svc.DownloadArtifact(c.Request().Context(), repo, slug, version, tag, true)
	if err != nil {
		return mapServiceError(err)
	}

	file, info, err := h.svc.BlobStore().Open(artifact.BlobPath)
	if err != nil {
		return err
	}
	defer file.Close()

	c.Response().Header().Set(echo.HeaderContentType, "application/zip")
	c.Response().Header().Set(echo.HeaderContentLength, strconv.FormatInt(info.Size(), 10))
	c.Response().Header().Set("ETag", artifact.Digest)
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", artifact.FileName))
	return c.Stream(http.StatusOK, "application/zip", file)
}
