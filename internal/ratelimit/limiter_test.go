package ratelimit

import (
	"testing"
	"time"
)

func TestLimiter_UsesDifferentLimitsByScopeAndBucket(t *testing.T) {
	t.Parallel()

	limiter := New(Config{
		Window:   time.Minute,
		ReadIP:   2,
		ReadKey:  4,
		WriteIP:  1,
		WriteKey: 3,
	})
	now := time.Unix(1_700_000_000, 0).UTC()

	// Read/IP allows 2 then blocks.
	if r := limiter.Take(now, ScopeRead, BucketIP, "1.1.1.1"); !r.Allowed || r.Remaining != 1 {
		t.Fatalf("read ip #1 = %#v", r)
	}
	if r := limiter.Take(now, ScopeRead, BucketIP, "1.1.1.1"); !r.Allowed || r.Remaining != 0 {
		t.Fatalf("read ip #2 = %#v", r)
	}
	if r := limiter.Take(now, ScopeRead, BucketIP, "1.1.1.1"); r.Allowed || r.Remaining != 0 {
		t.Fatalf("read ip #3 = %#v", r)
	}

	// Read/key has higher limit.
	for i := 0; i < 4; i++ {
		r := limiter.Take(now, ScopeRead, BucketKey, "user-a")
		if !r.Allowed {
			t.Fatalf("read key #%d denied: %#v", i+1, r)
		}
	}
	if r := limiter.Take(now, ScopeRead, BucketKey, "user-a"); r.Allowed {
		t.Fatalf("read key #5 should be denied: %#v", r)
	}

	// Write/IP limit 1.
	if r := limiter.Take(now, ScopeWrite, BucketIP, "2.2.2.2"); !r.Allowed {
		t.Fatalf("write ip #1 denied: %#v", r)
	}
	if r := limiter.Take(now, ScopeWrite, BucketIP, "2.2.2.2"); r.Allowed {
		t.Fatalf("write ip #2 should be denied: %#v", r)
	}
}

func TestLimiter_ResetsAfterWindow(t *testing.T) {
	t.Parallel()

	limiter := New(Config{
		Window:   time.Minute,
		ReadIP:   1,
		ReadKey:  1,
		WriteIP:  1,
		WriteKey: 1,
	})
	t0 := time.Unix(1_700_000_000, 0).UTC()

	if r := limiter.Take(t0, ScopeRead, BucketIP, "1.1.1.1"); !r.Allowed {
		t.Fatalf("first request denied: %#v", r)
	}
	if r := limiter.Take(t0.Add(10*time.Second), ScopeRead, BucketIP, "1.1.1.1"); r.Allowed {
		t.Fatalf("second request should be denied: %#v", r)
	}
	if r := limiter.Take(t0.Add(61*time.Second), ScopeRead, BucketIP, "1.1.1.1"); !r.Allowed {
		t.Fatalf("request after reset denied: %#v", r)
	}
}
