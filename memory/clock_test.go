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

func TestLeakyBucket_UsesInjectedClock(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(0, 0))
	limiter := NewLeakyBucket(1, 1, withClock(clock))

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

func TestLimiters_RejectNilInjectedClock(t *testing.T) {
	cases := []struct {
		name string
		run  func()
	}{
		{
			name: "token bucket",
			run: func() {
				NewTokenBucket(1, 1, withClock(nil))
			},
		},
		{
			name: "leaky bucket",
			run: func() {
				NewLeakyBucket(1, 1, withClock(nil))
			},
		},
		{
			name: "sliding window",
			run: func() {
				NewSlidingWindow(1, time.Second, withClock(nil))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatal("expected panic on nil injected clock")
				}
			}()
			tc.run()
		})
	}
}
