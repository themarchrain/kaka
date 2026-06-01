package memory

import (
	"context"
	"testing"
	"time"
)

type fakeClock struct {
	now time.Time
}

func newFakeClock(now time.Time) *fakeClock {
	return &fakeClock{now: now}
}

func (c *fakeClock) Now() time.Time {
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
}

func TestTokenBucket_UsesInjectedClock(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(0, 0))
	limiter := NewTokenBucket(1, 1, withClock(clock))

	first, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on first allow: %v", err)
	}
	if !first.Allowed {
		t.Fatal("expected first request to be allowed")
	}

	second, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on second allow: %v", err)
	}
	if second.Allowed {
		t.Fatal("expected second request to be denied before clock advances")
	}

	clock.Advance(time.Second)

	third, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error after clock advance: %v", err)
	}
	if !third.Allowed {
		t.Fatal("expected request to be allowed after clock advance")
	}
}

func TestSlidingWindow_UsesInjectedClock(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(0, 0))
	limiter := NewSlidingWindow(1, time.Second, withClock(clock))

	first, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on first allow: %v", err)
	}
	if !first.Allowed {
		t.Fatal("expected first request to be allowed")
	}

	second, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on second allow: %v", err)
	}
	if second.Allowed {
		t.Fatal("expected second request to be denied before clock advances")
	}

	clock.Advance(time.Second)

	third, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error after clock advance: %v", err)
	}
	if !third.Allowed {
		t.Fatal("expected request to be allowed after clock advance")
	}
}
