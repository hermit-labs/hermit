package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"hermit/internal/auth"
	"hermit/internal/service"
	"hermit/internal/store"

	"github.com/labstack/echo/v4"
)

func (h *Handler) Whoami(c echo.Context) error {
	claims, ok := auth.GetClaims(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	return c.JSON(http.StatusOK, map[string]any{
		"user": map[string]any{
			"handle":      claims.Subject,
			"displayName": claims.Subject,
			"image":       nil,
		},
	})
}

func (h *Handler) PublishSkill(c echo.Context) error {
	claims, ok := auth.GetClaims(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	repo, err := h.svc.GetPublishRepository(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}
	allowed, err := h.svc.HasRepoPermission(c.Request().Context(), repo, claims.Subject, store.RolePush, claims.IsAdmin)
	if err != nil {
		return mapServiceError(err)
	}
	if !allowed {
		return echo.NewHTTPError(http.StatusForbidden, "missing push permission")
	}

	if err := c.Request().ParseMultipartForm(h.cfg.MaxUploadBytes); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid multipart form")
	}
	form := c.Request().MultipartForm
	if form == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "multipart form required")
	}

	payloadRaw := strings.TrimSpace(c.FormValue("payload"))
	if payloadRaw == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "payload field is required")
	}
	var payload struct {
		Slug        string   `json:"slug"`
		DisplayName string   `json:"displayName"`
		Version     string   `json:"version"`
		Changelog   string   `json:"changelog"`
		Summary     *string  `json:"summary"`
		Tags        []string `json:"tags"`
	}
	if err := json.Unmarshal([]byte(payloadRaw), &payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload json")
	}

	fileHeaders := form.File["files"]
	if len(fileHeaders) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "files[] is required")
	}

	files := make([]service.PublishFileInput, 0, len(fileHeaders))
	for _, header := range fileHeaders {
		if header.Size > h.cfg.MaxUploadBytes {
			return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "file too large")
		}
		f, err := header.Open()
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid uploaded file")
		}
		data, readErr := io.ReadAll(io.LimitReader(f, h.cfg.MaxUploadBytes+1))
		_ = f.Close()
		if readErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "failed to read uploaded file")
		}
		if int64(len(data)) > h.cfg.MaxUploadBytes {
			return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "file too large")
		}
		files = append(files, service.PublishFileInput{
			Path:        header.Filename,
			ContentType: header.Header.Get(echo.HeaderContentType),
			Bytes:       data,
		})
	}

	result, err := h.svc.PublishSkill(
		c.Request().Context(),
		repo,
		service.PublishPayload{
			Slug:        payload.Slug,
			DisplayName: payload.DisplayName,
			Version:     payload.Version,
			Changelog:   payload.Changelog,
			Summary:     payload.Summary,
			Tags:        payload.Tags,
		},
		files,
		claims.Subject,
	)
	if err != nil {
		return mapServiceError(err)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"ok":        true,
		"skillId":   result.SkillID,
		"versionId": result.VersionID,
	})
}

func (h *Handler) DeleteSkill(c echo.Context) error {
	return h.setDelete(c, true)
}

func (h *Handler) UndeleteSkill(c echo.Context) error {
	return h.setDelete(c, false)
}

func (h *Handler) setDelete(c echo.Context, deleted bool) error {
	claims, ok := auth.GetClaims(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	repo, err := h.svc.GetPublishRepository(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}
	allowed, err := h.svc.HasRepoPermission(c.Request().Context(), repo, claims.Subject, store.RolePush, claims.IsAdmin)
	if err != nil {
		return mapServiceError(err)
	}
	if !allowed {
		return echo.NewHTTPError(http.StatusForbidden, "missing push permission")
	}

	slug := strings.TrimSpace(c.Param("slug"))
	if err := h.svc.DeleteSkill(c.Request().Context(), repo, slug, deleted); err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) CreateToken(c echo.Context) error {
	claims, ok := auth.GetClaims(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	if !claims.IsAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "admin only")
	}

	var req struct {
		Subject string `json:"subject"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	token, err := h.svc.CreateToken(c.Request().Context(), req.Subject)
	if err != nil {
		return mapServiceError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"subject": req.Subject,
		"token":   token,
	})
}
