package service

import (
	"net/http"
	"net/url"
	"testing"
)

func TestBuildUpstreamDownloadURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		baseURL string
		slug    string
		version string
		want    string
		wantErr bool
	}{
		{
			name:    "simple",
			baseURL: "https://hub.example.com",
			slug:    "my-skill",
			version: "1.0.0",
			want:    "https://hub.example.com/api/v1/download?slug=my-skill&version=1.0.0",
		},
		{
			name:    "with trailing slash",
			baseURL: "https://hub.example.com/",
			slug:    "foo",
			version: "2.0.0",
			want:    "https://hub.example.com/api/v1/download?slug=foo&version=2.0.0",
		},
		{
			name:    "with base path",
			baseURL: "https://hub.example.com/registry",
			slug:    "bar",
			version: "0.1.0",
			want:    "https://hub.example.com/registry/api/v1/download?slug=bar&version=0.1.0",
		},
		{
			name:    "slug with special chars",
			baseURL: "https://hub.example.com",
			slug:    "my skill",
			version: "1.0.0",
			want:    "https://hub.example.com/api/v1/download?slug=my+skill&version=1.0.0",
		},
		{
			name:    "whitespace in base URL",
			baseURL: "  https://hub.example.com  ",
			slug:    "x",
			version: "1.0.0",
			want:    "https://hub.example.com/api/v1/download?slug=x&version=1.0.0",
		},
		{
			name:    "invalid base URL",
			baseURL: "://bad",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := buildUpstreamDownloadURL(tt.baseURL, tt.slug, tt.version)
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
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveFileName(t *testing.T) {
	t.Parallel()

	t.Run("from Content-Disposition", func(t *testing.T) {
		t.Parallel()
		resp := &http.Response{
			Header: http.Header{},
		}
		resp.Header.Set("Content-Disposition", `attachment; filename="custom-name.zip"`)
		got := resolveFileName(resp, "slug", "1.0.0")
		if got != "custom-name.zip" {
			t.Fatalf("got %q, want custom-name.zip", got)
		}
	})

	t.Run("from Content-Disposition without quotes", func(t *testing.T) {
		t.Parallel()
		resp := &http.Response{
			Header: http.Header{},
		}
		resp.Header.Set("Content-Disposition", `attachment; filename=archive.tar.gz`)
		got := resolveFileName(resp, "slug", "1.0.0")
		if got != "archive.tar.gz" {
			t.Fatalf("got %q, want archive.tar.gz", got)
		}
	})

	t.Run("from request URL path", func(t *testing.T) {
		t.Parallel()
		reqURL, _ := url.Parse("https://example.com/files/my-artifact.zip")
		resp := &http.Response{
			Header:  http.Header{},
			Request: &http.Request{URL: reqURL},
		}
		got := resolveFileName(resp, "slug", "1.0.0")
		if got != "my-artifact.zip" {
			t.Fatalf("got %q, want my-artifact.zip", got)
		}
	})

	t.Run("fallback to slug-version.zip", func(t *testing.T) {
		t.Parallel()
		resp := &http.Response{
			Header: http.Header{},
		}
		got := resolveFileName(resp, "my-tool", "2.3.0")
		if got != "my-tool-2.3.0.zip" {
			t.Fatalf("got %q, want my-tool-2.3.0.zip", got)
		}
	})

	t.Run("Content-Disposition takes precedence over URL", func(t *testing.T) {
		t.Parallel()
		reqURL, _ := url.Parse("https://example.com/files/from-url.zip")
		resp := &http.Response{
			Header:  http.Header{},
			Request: &http.Request{URL: reqURL},
		}
		resp.Header.Set("Content-Disposition", `attachment; filename="from-header.zip"`)
		got := resolveFileName(resp, "slug", "1.0.0")
		if got != "from-header.zip" {
			t.Fatalf("got %q, want from-header.zip", got)
		}
	})
}
