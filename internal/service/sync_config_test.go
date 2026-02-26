package service

import "testing"

func TestProxySyncConfig_PageSizeOrDefault(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		pageSize int
		want     int
	}{
		{"positive", 50, 50},
		{"zero", 0, 100},
		{"negative", -1, 100},
		{"one", 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := ProxySyncConfig{PageSize: tt.pageSize}
			if got := c.PageSizeOrDefault(); got != tt.want {
				t.Fatalf("PageSizeOrDefault() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestProxySyncConfig_ConcurrencyOrDefault(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		concurrency int
		want        int
	}{
		{"positive", 8, 8},
		{"zero", 0, 4},
		{"negative", -1, 4},
		{"one", 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := ProxySyncConfig{Concurrency: tt.concurrency}
			if got := c.ConcurrencyOrDefault(); got != tt.want {
				t.Fatalf("ConcurrencyOrDefault() = %d, want %d", got, tt.want)
			}
		})
	}
}
