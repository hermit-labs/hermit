package handlers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"hermit/internal/service"

	"github.com/labstack/echo/v4"
)

func mapServiceError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrConflict):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
}

func toMillis(t time.Time) int64 {
	return t.UTC().UnixMilli()
}

func toMillisPtr(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	v := t.UTC().UnixMilli()
	return &v
}

func queryInt(c echo.Context, key string, fallback int) int {
	raw := strings.TrimSpace(c.QueryParam(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func encodeCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func decodeCursor(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return 0, err
	}
	offset, err := strconv.Atoi(string(data))
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("invalid cursor")
	}
	return offset, nil
}

func normalizeSort(raw string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "", "updated", "newest":
		return "updated", nil
	case "downloads", "download":
		return "downloads", nil
	case "stars", "star", "rating":
		return "stars", nil
	case "installscurrent", "installs-current", "installs", "install", "current":
		return "installsCurrent", nil
	case "installsalltime", "installs-all-time":
		return "installsAllTime", nil
	case "trending":
		return "trending", nil
	default:
		return "", fmt.Errorf("invalid sort")
	}
}

func decodeAnyJSON(raw json.RawMessage, fallback any) any {
	if len(raw) == 0 {
		return fallback
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return fallback
	}
	if out == nil {
		return fallback
	}
	return out
}

func baseURL(r *http.Request) string {
	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host
}
