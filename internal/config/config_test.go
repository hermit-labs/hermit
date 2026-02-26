package config

import (
	"reflect"
	"testing"
)

func TestParseUpstreamURLs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		listRaw   string
		singleRaw string
		want      []string
	}{
		{
			name:      "multi delimiters and dedupe",
			listRaw:   " https://a.example ; https://b.example,\nhttps://a.example ",
			singleRaw: "https://ignored.example",
			want:      []string{"https://a.example", "https://b.example"},
		},
		{
			name:      "fallback single",
			listRaw:   "",
			singleRaw: "https://single.example",
			want:      []string{"https://single.example"},
		},
		{
			name:      "empty",
			listRaw:   " , ; \n ",
			singleRaw: "",
			want:      []string{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseUpstreamURLs(tt.listRaw, tt.singleRaw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseUpstreamURLs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestLoadProxySyncConfig(t *testing.T) {
	t.Setenv("PROXY_SYNC_PAGE_SIZE", "123")
	t.Setenv("PROXY_SYNC_CONCURRENCY", "7")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173;http://example.com")
	t.Setenv("PROXY_UPSTREAM_URLS", "https://a.example,https://b.example")
	t.Setenv("ADMIN_TOKEN", "token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ProxySyncPageSize != 123 {
		t.Fatalf("ProxySyncPageSize = %d, want 123", cfg.ProxySyncPageSize)
	}
	if cfg.ProxySyncConcurrency != 7 {
		t.Fatalf("ProxySyncConcurrency = %d, want 7", cfg.ProxySyncConcurrency)
	}
	wantOrigins := []string{"http://localhost:5173", "http://example.com"}
	if !reflect.DeepEqual(cfg.CORSAllowedOrigins, wantOrigins) {
		t.Fatalf("CORSAllowedOrigins = %#v, want %#v", cfg.CORSAllowedOrigins, wantOrigins)
	}
	if len(cfg.ProxyUpstreamURLs) != 2 {
		t.Fatalf("ProxyUpstreamURLs len = %d, want 2", len(cfg.ProxyUpstreamURLs))
	}
}
