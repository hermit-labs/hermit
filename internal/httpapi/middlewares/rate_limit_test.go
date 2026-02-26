package middlewares

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hermit/internal/auth"
	"hermit/internal/ratelimit"

	"github.com/labstack/echo/v4"
)

type fakeVerifier struct {
	subjectByToken map[string]string
}

func (f fakeVerifier) Authenticate(_ context.Context, token string) (auth.Claims, error) {
	if subject, ok := f.subjectByToken[token]; ok {
		return auth.Claims{Subject: subject}, nil
	}
	return auth.Claims{}, errors.New("invalid token")
}

func TestRateLimitMiddleware_ValidTokenUsesKeyBucket(t *testing.T) {
	t.Parallel()

	e := echo.New()
	e.Use(newRateLimitMiddlewareWithConfig(
		fakeVerifier{subjectByToken: map[string]string{"good": "user-a"}},
		ratelimit.Config{
			Window:   time.Minute,
			ReadIP:   1,
			ReadKey:  2,
			WriteIP:  1,
			WriteKey: 1,
		},
	))
	e.GET("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("Authorization", "Bearer good")
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request #%d status = %d, want 200", i+1, rec.Code)
		}
		if rec.Header().Get("X-RateLimit-Limit") != "2" {
			t.Fatalf("request #%d X-RateLimit-Limit = %q, want 2", i+1, rec.Header().Get("X-RateLimit-Limit"))
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer good")
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("third request status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatalf("Retry-After should be set on 429")
	}
}

func TestRateLimitMiddleware_InvalidTokenFallsBackToIP(t *testing.T) {
	t.Parallel()

	e := echo.New()
	e.Use(newRateLimitMiddlewareWithConfig(
		fakeVerifier{subjectByToken: map[string]string{"good": "user-a"}},
		ratelimit.Config{
			Window:   time.Minute,
			ReadIP:   1,
			ReadKey:  5,
			WriteIP:  1,
			WriteKey: 1,
		},
	))
	e.GET("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	req1 := httptest.NewRequest(http.MethodGet, "/x", nil)
	req1.Header.Set("Authorization", "Bearer bad")
	req1.RemoteAddr = "5.6.7.8:4321"
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", rec1.Code)
	}
	if rec1.Header().Get("X-RateLimit-Limit") != "1" {
		t.Fatalf("first request should use IP bucket limit 1, got %q", rec1.Header().Get("X-RateLimit-Limit"))
	}

	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	req2.Header.Set("Authorization", "Bearer bad")
	req2.RemoteAddr = "5.6.7.8:4321"
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want 429", rec2.Code)
	}
	if rec2.Header().Get("RateLimit-Reset") == "" || rec2.Header().Get("X-RateLimit-Reset") == "" {
		t.Fatalf("reset headers should be present")
	}
}
