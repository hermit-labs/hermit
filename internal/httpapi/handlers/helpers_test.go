package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hermit/internal/service"

	"github.com/labstack/echo/v4"
)

func TestClampInt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		v, min, max  int
		want         int
	}{
		{"within range", 5, 1, 10, 5},
		{"below min", -1, 0, 10, 0},
		{"above max", 15, 0, 10, 10},
		{"at min", 0, 0, 10, 0},
		{"at max", 10, 0, 10, 10},
		{"equal min max", 5, 5, 5, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := clampInt(tt.v, tt.min, tt.max)
			if got != tt.want {
				t.Fatalf("clampInt(%d, %d, %d) = %d, want %d", tt.v, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestEncodeDecode_Cursor(t *testing.T) {
	t.Parallel()
	tests := []int{0, 1, 25, 100, 9999}
	for _, offset := range tests {
		encoded := encodeCursor(offset)
		decoded, err := decodeCursor(encoded)
		if err != nil {
			t.Fatalf("decodeCursor(%q) error = %v", encoded, err)
		}
		if decoded != offset {
			t.Fatalf("roundtrip: encoded %d, decoded %d", offset, decoded)
		}
	}
}

func TestDecodeCursor_Empty(t *testing.T) {
	t.Parallel()
	got, err := decodeCursor("")
	if err != nil {
		t.Fatalf("decodeCursor(\"\") error = %v", err)
	}
	if got != 0 {
		t.Fatalf("decodeCursor(\"\") = %d, want 0", got)
	}
}

func TestDecodeCursor_Invalid(t *testing.T) {
	t.Parallel()
	tests := []string{
		"not-base64!@#",
		encodeCursor(-1),
	}
	for _, raw := range tests {
		_, err := decodeCursor(raw)
		if err == nil {
			t.Fatalf("decodeCursor(%q) should error", raw)
		}
	}
}

func TestNormalizeSort(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"", "updated", false},
		{"updated", "updated", false},
		{"newest", "updated", false},
		{"downloads", "downloads", false},
		{"download", "downloads", false},
		{"stars", "stars", false},
		{"star", "stars", false},
		{"rating", "stars", false},
		{"installsCurrent", "installsCurrent", false},
		{"installs-current", "installsCurrent", false},
		{"installs", "installsCurrent", false},
		{"current", "installsCurrent", false},
		{"installsAllTime", "installsAllTime", false},
		{"installs-all-time", "installsAllTime", false},
		{"trending", "trending", false},
		{"DOWNLOADS", "downloads", false},
		{"  Stars  ", "stars", false},
		{"invalid-sort", "", true},
		{"alphabetical", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeSort(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeSort(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDecodeAnyJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		raw      json.RawMessage
		fallback any
		wantNil  bool
	}{
		{"null json", json.RawMessage(`null`), "default", false},
		{"empty", nil, "fallback", false},
		{"valid object", json.RawMessage(`{"key":"value"}`), nil, false},
		{"valid array", json.RawMessage(`[1,2,3]`), nil, false},
		{"invalid json", json.RawMessage(`{broken`), "fb", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := decodeAnyJSON(tt.raw, tt.fallback)
			if got == nil && !tt.wantNil {
				if tt.fallback != nil {
					t.Fatal("got nil, expected fallback")
				}
			}
		})
	}
}

func TestDecodeAnyJSON_UseFallbackForEmpty(t *testing.T) {
	t.Parallel()
	got := decodeAnyJSON(nil, "fallback")
	if got != "fallback" {
		t.Fatalf("got %v, want fallback", got)
	}
}

func TestDecodeAnyJSON_UseFallbackForNull(t *testing.T) {
	t.Parallel()
	got := decodeAnyJSON(json.RawMessage(`null`), "default")
	if got != "default" {
		t.Fatalf("got %v, want default", got)
	}
}

func TestDecodeAnyJSON_UseFallbackForInvalid(t *testing.T) {
	t.Parallel()
	got := decodeAnyJSON(json.RawMessage(`{broken`), "fb")
	if got != "fb" {
		t.Fatalf("got %v, want fb", got)
	}
}

func TestToMillis(t *testing.T) {
	t.Parallel()
	ts := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	got := toMillis(ts)
	want := ts.UTC().UnixMilli()
	if got != want {
		t.Fatalf("toMillis() = %d, want %d", got, want)
	}
}

func TestToMillisPtr(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		got := toMillisPtr(nil)
		if got != nil {
			t.Fatal("toMillisPtr(nil) should return nil")
		}
	})

	t.Run("non-nil", func(t *testing.T) {
		t.Parallel()
		ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		got := toMillisPtr(&ts)
		if got == nil {
			t.Fatal("toMillisPtr should not return nil")
		}
		if *got != ts.UTC().UnixMilli() {
			t.Fatalf("got %d, want %d", *got, ts.UTC().UnixMilli())
		}
	})
}

func TestBaseURL(t *testing.T) {
	t.Parallel()

	t.Run("plain HTTP", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequest("GET", "http://localhost:8080/api", nil)
		got := baseURL(r)
		if got != "http://localhost:8080" {
			t.Fatalf("got %q, want http://localhost:8080", got)
		}
	})

	t.Run("with X-Forwarded headers", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequest("GET", "http://localhost/api", nil)
		r.Header.Set("X-Forwarded-Proto", "https")
		r.Header.Set("X-Forwarded-Host", "public.example.com")
		got := baseURL(r)
		if got != "https://public.example.com" {
			t.Fatalf("got %q, want https://public.example.com", got)
		}
	})
}

func TestMapServiceError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"not found", service.ErrNotFound, http.StatusNotFound},
		{"invalid input", service.ErrInvalidInput, http.StatusBadRequest},
		{"conflict", service.ErrConflict, http.StatusConflict},
		{"unknown", errors.New("something else"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mapServiceError(tt.err)
			httpErr, ok := got.(*echo.HTTPError)
			if !ok {
				t.Fatalf("expected *echo.HTTPError, got %T", got)
			}
			if httpErr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", httpErr.Code, tt.wantStatus)
			}
		})
	}
}

func TestQueryInt(t *testing.T) {
	t.Parallel()

	e := echo.New()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest("GET", "/?limit=42", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		got := queryInt(c, "limit", 10)
		if got != 42 {
			t.Fatalf("got %d, want 42", got)
		}
	})

	t.Run("missing uses fallback", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		got := queryInt(c, "limit", 25)
		if got != 25 {
			t.Fatalf("got %d, want 25", got)
		}
	})

	t.Run("non-numeric uses fallback", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest("GET", "/?limit=abc", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		got := queryInt(c, "limit", 10)
		if got != 10 {
			t.Fatalf("got %d, want 10", got)
		}
	})
}
