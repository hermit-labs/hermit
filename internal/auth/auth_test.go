package auth

import (
	"net/http"
	"testing"
)

func TestExtractToken_BearerHeader(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		authz string
		want  string
	}{
		{"standard bearer", "Bearer my-token-123", "my-token-123"},
		{"lowercase bearer", "bearer my-token", "my-token"},
		{"bearer with extra spaces", "Bearer   spaced  ", "spaced"},
		{"empty bearer", "Bearer ", ""},
		{"non-bearer auth", "Basic dXNlcjpwYXNz", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r, _ := http.NewRequest("GET", "/", nil)
			r.Header.Set("Authorization", tt.authz)
			got := extractToken(r)
			if got != tt.want {
				t.Fatalf("extractToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractToken_XAPITokenHeader(t *testing.T) {
	t.Parallel()
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("X-API-Token", "  tok-456  ")
	got := extractToken(r)
	if got != "tok-456" {
		t.Fatalf("extractToken() = %q, want %q", got, "tok-456")
	}
}

func TestExtractToken_BearerTakesPrecedence(t *testing.T) {
	t.Parallel()
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer from-bearer")
	r.Header.Set("X-API-Token", "from-header")
	got := extractToken(r)
	if got != "from-bearer" {
		t.Fatalf("extractToken() = %q, want %q", got, "from-bearer")
	}
}

func TestExtractToken_NoHeaders(t *testing.T) {
	t.Parallel()
	r, _ := http.NewRequest("GET", "/", nil)
	got := extractToken(r)
	if got != "" {
		t.Fatalf("extractToken() = %q, want empty", got)
	}
}

func TestAnonymousActor(t *testing.T) {
	t.Parallel()
	a := AnonymousActor()
	if !a.Anonymous {
		t.Fatal("AnonymousActor().Anonymous should be true")
	}
	if a.Subject != "" {
		t.Fatalf("AnonymousActor().Subject = %q, want empty", a.Subject)
	}
	if a.IsAdmin {
		t.Fatal("AnonymousActor().IsAdmin should be false")
	}
}

func TestActorFromClaims(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		claims  Claims
		wantSub string
		wantAdm bool
	}{
		{"admin", Claims{Subject: "admin", IsAdmin: true}, "admin", true},
		{"regular user", Claims{Subject: "user1", IsAdmin: false}, "user1", false},
		{"empty", Claims{}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := ActorFromClaims(tt.claims)
			if a.Subject != tt.wantSub {
				t.Fatalf("Subject = %q, want %q", a.Subject, tt.wantSub)
			}
			if a.IsAdmin != tt.wantAdm {
				t.Fatalf("IsAdmin = %v, want %v", a.IsAdmin, tt.wantAdm)
			}
			if a.Anonymous {
				t.Fatal("ActorFromClaims should not be anonymous")
			}
		})
	}
}
