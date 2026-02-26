package middlewares

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"hermit/internal/auth"
	"hermit/internal/ratelimit"

	"github.com/labstack/echo/v4"
)

type tokenVerifier interface {
	Authenticate(context.Context, string) (auth.Claims, error)
}

func NewRateLimitMiddleware(verifier tokenVerifier) echo.MiddlewareFunc {
	return newRateLimitMiddlewareWithConfig(verifier, ratelimit.Config{
		Window:   time.Minute,
		ReadIP:   120,
		ReadKey:  600,
		WriteIP:  30,
		WriteKey: 120,
	})
}

func newRateLimitMiddlewareWithConfig(verifier tokenVerifier, cfg ratelimit.Config) echo.MiddlewareFunc {
	limiter := ratelimit.New(cfg)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			scope := requestScope(c.Request().Method)
			kind, bucket := resolveRateLimitBucket(c, verifier)

			result := limiter.Take(time.Now().UTC(), scope, kind, bucket)
			setRateLimitHeaders(c.Response().Header(), result)

			if !result.Allowed {
				c.Response().Header().Set("Retry-After", strconv.FormatInt(result.ResetIn, 10))
				return c.JSON(http.StatusTooManyRequests, map[string]any{
					"error": "rate limit exceeded",
				})
			}
			return next(c)
		}
	}
}

func requestScope(method string) ratelimit.Scope {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return ratelimit.ScopeRead
	default:
		return ratelimit.ScopeWrite
	}
}

func resolveRateLimitBucket(c echo.Context, verifier tokenVerifier) (ratelimit.BucketKind, string) {
	token := extractToken(c.Request())
	if token != "" && verifier != nil {
		claims, err := verifier.Authenticate(c.Request().Context(), token)
		if err == nil {
			subject := strings.TrimSpace(claims.Subject)
			if subject != "" {
				return ratelimit.BucketKey, subject
			}
		}
	}

	ip := strings.TrimSpace(c.RealIP())
	if ip == "" {
		ip = clientIPFromRemoteAddr(c.Request().RemoteAddr)
	}
	if ip == "" {
		ip = "unknown"
	}
	return ratelimit.BucketIP, ip
}

func setRateLimitHeaders(header http.Header, result ratelimit.Result) {
	limit := strconv.Itoa(result.Limit)
	remaining := strconv.Itoa(result.Remaining)
	resetEpoch := strconv.FormatInt(result.ResetAt, 10)
	resetDelay := strconv.FormatInt(result.ResetIn, 10)

	header.Set("X-RateLimit-Limit", limit)
	header.Set("X-RateLimit-Remaining", remaining)
	header.Set("X-RateLimit-Reset", resetEpoch)

	header.Set("RateLimit-Limit", limit)
	header.Set("RateLimit-Remaining", remaining)
	header.Set("RateLimit-Reset", resetDelay)
}

func extractToken(r *http.Request) string {
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return strings.TrimSpace(authz[7:])
	}
	return strings.TrimSpace(r.Header.Get("X-API-Token"))
}

func clientIPFromRemoteAddr(remoteAddr string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		return strings.TrimSpace(remoteAddr)
	}
	return strings.TrimSpace(host)
}
