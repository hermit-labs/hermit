package ratelimit

import (
	"sync"
	"time"
)

type Scope string

const (
	ScopeRead  Scope = "read"
	ScopeWrite Scope = "write"
)

type BucketKind string

const (
	BucketIP  BucketKind = "ip"
	BucketKey BucketKind = "key"
)

type Config struct {
	Window   time.Duration
	ReadIP   int
	ReadKey  int
	WriteIP  int
	WriteKey int
}

type Result struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetAt   int64
	ResetIn   int64
}

type key struct {
	scope  Scope
	kind   BucketKind
	bucket string
}

type counter struct {
	windowStart int64
	count       int
}

type Limiter struct {
	cfg     Config
	windowS int64

	mu      sync.Mutex
	entries map[key]counter
}

func New(cfg Config) *Limiter {
	if cfg.Window <= 0 {
		cfg.Window = time.Minute
	}
	return &Limiter{
		cfg:     cfg,
		windowS: int64(cfg.Window.Seconds()),
		entries: make(map[key]counter, 4096),
	}
}

func (l *Limiter) Take(now time.Time, scope Scope, kind BucketKind, bucket string) Result {
	limit := l.limit(scope, kind)
	if limit <= 0 {
		return Result{
			Allowed:   true,
			Limit:     0,
			Remaining: 0,
			ResetAt:   now.Unix(),
			ResetIn:   0,
		}
	}

	if l.windowS <= 0 {
		l.windowS = 60
	}
	unixNow := now.Unix()
	windowStart := unixNow / l.windowS * l.windowS
	resetAt := windowStart + l.windowS

	k := key{
		scope:  scope,
		kind:   kind,
		bucket: bucket,
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[k]
	if !ok || entry.windowStart != windowStart {
		entry = counter{windowStart: windowStart, count: 0}
	}

	allowed := entry.count < limit
	if allowed {
		entry.count++
	} else if entry.count < limit {
		entry.count = limit
	}

	remaining := limit - entry.count
	if remaining < 0 {
		remaining = 0
	}
	l.entries[k] = entry

	if len(l.entries) > 100000 {
		l.cleanup(windowStart - l.windowS*2)
	}

	resetIn := resetAt - unixNow
	if resetIn < 0 {
		resetIn = 0
	}
	return Result{
		Allowed:   allowed,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   resetAt,
		ResetIn:   resetIn,
	}
}

func (l *Limiter) limit(scope Scope, kind BucketKind) int {
	switch scope {
	case ScopeRead:
		if kind == BucketKey {
			return l.cfg.ReadKey
		}
		return l.cfg.ReadIP
	case ScopeWrite:
		if kind == BucketKey {
			return l.cfg.WriteKey
		}
		return l.cfg.WriteIP
	default:
		return 0
	}
}

func (l *Limiter) cleanup(olderThanWindowStart int64) {
	for k, v := range l.entries {
		if v.windowStart <= olderThanWindowStart {
			delete(l.entries, k)
		}
	}
}
