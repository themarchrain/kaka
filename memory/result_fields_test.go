package memory

import (
	"context"
	"testing"
	"time"
)

func TestTokenBucket_ResultFields(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewTokenBucket(3, 0.25, withClock(clock))

	for i, wantRemaining := range []int64{2, 1, 0} {
		result, err := limiter.Allow(ctx, "user:1")
		if err != nil {
			t.Fatalf("unexpected error on request %d: %v", i+1, err)
		}
		if !result.Allowed {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
		if result.Remaining != wantRemaining {
			t.Fatalf("expected remaining=%d on request %d, got %d", wantRemaining, i+1, result.Remaining)
		}
		if result.RetryAfter != 0 {
			t.Fatalf("expected retryAfter=0 on allowed request, got %v", result.RetryAfter)
		}
	}

	denied, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on denied request: %v", err)
	}
	if denied.Allowed {
		t.Fatal("expected request to be denied after tokens are exhausted")
	}
	if denied.Remaining != 0 {
		t.Fatalf("expected remaining=0 on deny, got %d", denied.Remaining)
	}
	if denied.RetryAfter <= 0 || denied.RetryAfter > 4*time.Second {
		t.Fatalf("expected retryAfter within (0, 4s], got %v", denied.RetryAfter)
	}

	clock.Advance(50 * time.Millisecond)

	deniedAfterPartialRefill, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error after partial refill: %v", err)
	}
	if deniedAfterPartialRefill.Allowed {
		t.Fatal("expected partial refill to still be denied")
	}
	if deniedAfterPartialRefill.RetryAfter <= 0 {
		t.Fatalf("expected positive retryAfter after partial refill, got %v", deniedAfterPartialRefill.RetryAfter)
	}
	if deniedAfterPartialRefill.RetryAfter >= denied.RetryAfter {
		t.Fatalf("expected retryAfter to decrease after partial refill, before=%v after=%v", denied.RetryAfter, deniedAfterPartialRefill.RetryAfter)
	}
}

func TestTokenBucket_AllowsAtExactRetryAfterBoundary(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewTokenBucket(1, 1, withClock(clock))

	first, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on first allow: %v", err)
	}
	if !first.Allowed {
		t.Fatal("expected first request to be allowed")
	}

	denied, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on denied request: %v", err)
	}
	if denied.Allowed {
		t.Fatal("expected request to be denied when token is exhausted")
	}

	clock.Advance(denied.RetryAfter)

	allowed, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error at retryAfter boundary: %v", err)
	}
	if !allowed.Allowed {
		t.Fatal("expected request to be allowed exactly at retryAfter boundary")
	}
}

func TestTokenBucket_RetryAfterRoundsUpSubNanosecondWait(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewTokenBucket(1, 2e9, withClock(clock))

	first, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on first allow: %v", err)
	}
	if !first.Allowed {
		t.Fatal("expected first request to be allowed")
	}

	denied, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on denied request: %v", err)
	}
	if denied.Allowed {
		t.Fatal("expected request to be denied when token is exhausted")
	}
	if denied.RetryAfter != time.Nanosecond {
		t.Fatalf("expected retryAfter to round up to 1ns, got %v", denied.RetryAfter)
	}
}

func TestTokenBucket_RetryAfterClampsHugeWait(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewTokenBucket(1, 1e-300, withClock(clock))

	first, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on first allow: %v", err)
	}
	if !first.Allowed {
		t.Fatal("expected first request to be allowed")
	}

	denied, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on denied request: %v", err)
	}
	if denied.Allowed {
		t.Fatal("expected request to be denied when token is exhausted")
	}
	if denied.RetryAfter != maxDuration {
		t.Fatalf("expected retryAfter to clamp to max duration, got %v", denied.RetryAfter)
	}
}

func TestTokenBucket_RetryAfterTracksPartialRefill(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewTokenBucket(1, 10, withClock(clock))

	first, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on first allow: %v", err)
	}
	if !first.Allowed {
		t.Fatal("expected first request to be allowed")
	}

	clock.Advance(90 * time.Millisecond)

	denied, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on denied request: %v", err)
	}
	if denied.Allowed {
		t.Fatal("expected request to be denied before enough tokens refill")
	}
	if denied.RetryAfter != 10*time.Millisecond {
		t.Fatalf("expected retryAfter=10ms after partial refill, got %v", denied.RetryAfter)
	}
}

func TestTokenBucket_RemainingFloorsFractionalTokens(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewTokenBucket(2, 2, withClock(clock))

	for i := 0; i < 2; i++ {
		result, err := limiter.Allow(ctx, "user:1")
		if err != nil {
			t.Fatalf("unexpected error on initial request %d: %v", i+1, err)
		}
		if !result.Allowed {
			t.Fatalf("expected initial request %d to be allowed", i+1)
		}
	}

	clock.Advance(750 * time.Millisecond)

	result, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error after fractional refill: %v", err)
	}
	if !result.Allowed {
		t.Fatal("expected request to be allowed after fractional refill reaches one token")
	}
	if result.Remaining != 0 {
		t.Fatalf("expected fractional remaining tokens to floor to 0, got %d", result.Remaining)
	}
}

func TestLeakyBucket_ResultFields(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewLeakyBucket(3, 0.25, withClock(clock))

	for i, wantRemaining := range []int64{2, 1, 0} {
		result, err := limiter.Allow(ctx, "user:1")
		if err != nil {
			t.Fatalf("unexpected error on request %d: %v", i+1, err)
		}
		if !result.Allowed {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
		if result.Remaining != wantRemaining {
			t.Fatalf("expected remaining=%d on request %d, got %d", wantRemaining, i+1, result.Remaining)
		}
		if result.RetryAfter != 0 {
			t.Fatalf("expected retryAfter=0 on allowed request, got %v", result.RetryAfter)
		}
	}

	denied, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on denied request: %v", err)
	}
	if denied.Allowed {
		t.Fatal("expected request to be denied when bucket is full")
	}
	if denied.Remaining != 0 {
		t.Fatalf("expected remaining=0 on deny, got %d", denied.Remaining)
	}
	if denied.RetryAfter != 4*time.Second {
		t.Fatalf("expected retryAfter=4s on deny, got %v", denied.RetryAfter)
	}
}

func TestLeakyBucket_RemainingFloorsFractionalCapacity(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewLeakyBucket(2, 2, withClock(clock))

	for i := 0; i < 2; i++ {
		result, err := limiter.Allow(ctx, "user:1")
		if err != nil {
			t.Fatalf("unexpected error on initial request %d: %v", i+1, err)
		}
		if !result.Allowed {
			t.Fatalf("expected initial request %d to be allowed", i+1)
		}
	}

	clock.Advance(750 * time.Millisecond)

	result, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error after fractional leak: %v", err)
	}
	if !result.Allowed {
		t.Fatal("expected request to be allowed after enough water leaks")
	}
	if result.Remaining != 0 {
		t.Fatalf("expected fractional remaining capacity to floor to 0, got %d", result.Remaining)
	}
}

func TestLeakyBucket_RetryAfterTracksPartialLeak(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewLeakyBucket(1, 1, withClock(clock))

	first, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on first allow: %v", err)
	}
	if !first.Allowed {
		t.Fatal("expected first request to be allowed")
	}

	clock.Advance(900 * time.Millisecond)

	denied, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on denied request: %v", err)
	}
	if denied.Allowed {
		t.Fatal("expected request to be denied before enough water leaks")
	}
	if denied.RetryAfter != 100*time.Millisecond {
		t.Fatalf("expected retryAfter=100ms after partial leak, got %v", denied.RetryAfter)
	}
}

func TestLeakyBucket_AllowsAtExactRetryAfterBoundary(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewLeakyBucket(1, 1, withClock(clock))

	first, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on first allow: %v", err)
	}
	if !first.Allowed {
		t.Fatal("expected first request to be allowed")
	}

	denied, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on denied request: %v", err)
	}
	if denied.Allowed {
		t.Fatal("expected request to be denied when bucket is full")
	}

	clock.Advance(denied.RetryAfter)

	allowed, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error at retryAfter boundary: %v", err)
	}
	if !allowed.Allowed {
		t.Fatal("expected request to be allowed exactly at retryAfter boundary")
	}
}

func TestLeakyBucket_RetryAfterRoundsUpSubNanosecondWait(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewLeakyBucket(1, 2e9, withClock(clock))

	first, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on first allow: %v", err)
	}
	if !first.Allowed {
		t.Fatal("expected first request to be allowed")
	}

	denied, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on denied request: %v", err)
	}
	if denied.Allowed {
		t.Fatal("expected request to be denied when bucket is full")
	}
	if denied.RetryAfter != time.Nanosecond {
		t.Fatalf("expected retryAfter to round up to 1ns, got %v", denied.RetryAfter)
	}
}

func TestLeakyBucket_RetryAfterClampsHugeWait(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewLeakyBucket(1, 1e-300, withClock(clock))

	first, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on first allow: %v", err)
	}
	if !first.Allowed {
		t.Fatal("expected first request to be allowed")
	}

	denied, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on denied request: %v", err)
	}
	if denied.Allowed {
		t.Fatal("expected request to be denied when bucket is full")
	}
	if denied.RetryAfter != maxDuration {
		t.Fatalf("expected retryAfter to clamp to max duration, got %v", denied.RetryAfter)
	}
}

func TestSlidingWindow_ResultFields(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewSlidingWindow(3, 150*time.Millisecond, withClock(clock))

	for i, wantRemaining := range []int64{2, 1, 0} {
		result, err := limiter.Allow(ctx, "user:1")
		if err != nil {
			t.Fatalf("unexpected error on request %d: %v", i+1, err)
		}
		if !result.Allowed {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
		if result.Remaining != wantRemaining {
			t.Fatalf("expected remaining=%d on request %d, got %d", wantRemaining, i+1, result.Remaining)
		}
		if result.RetryAfter != 0 {
			t.Fatalf("expected retryAfter=0 on allowed request, got %v", result.RetryAfter)
		}
	}

	denied, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on denied request: %v", err)
	}
	if denied.Allowed {
		t.Fatal("expected request to be denied when window is full")
	}
	if denied.Remaining != 0 {
		t.Fatalf("expected remaining=0 on deny, got %d", denied.Remaining)
	}
	if denied.RetryAfter <= 0 || denied.RetryAfter > 150*time.Millisecond {
		t.Fatalf("expected retryAfter within (0, 150ms], got %v", denied.RetryAfter)
	}

	clock.Advance(denied.RetryAfter + 20*time.Millisecond)

	allowedAfterWindow, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error after window slides: %v", err)
	}
	if !allowedAfterWindow.Allowed {
		t.Fatal("expected request to be allowed after oldest event leaves the window")
	}
	if allowedAfterWindow.RetryAfter != 0 {
		t.Fatalf("expected retryAfter=0 after window slides, got %v", allowedAfterWindow.RetryAfter)
	}
}

func TestSlidingWindow_RetryAfterTracksOldestRequestExpiry(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewSlidingWindow(2, 100*time.Millisecond, withClock(clock))

	first, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on first allow: %v", err)
	}
	if !first.Allowed {
		t.Fatal("expected first request to be allowed")
	}

	clock.Advance(40 * time.Millisecond)

	second, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on second allow: %v", err)
	}
	if !second.Allowed {
		t.Fatal("expected second request to be allowed")
	}

	denied, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on denied request: %v", err)
	}
	if denied.Allowed {
		t.Fatal("expected request to be denied while the window is full")
	}
	if denied.RetryAfter != 60*time.Millisecond {
		t.Fatalf("expected retryAfter to wait for oldest request expiry, got %v", denied.RetryAfter)
	}
}

func TestSlidingWindow_DropsRequestsAtWindowStart(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewSlidingWindow(2, 100*time.Millisecond, withClock(clock))

	first, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on first allow: %v", err)
	}
	if !first.Allowed {
		t.Fatal("expected first request to be allowed")
	}

	clock.Advance(50 * time.Millisecond)

	second, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on second allow: %v", err)
	}
	if !second.Allowed {
		t.Fatal("expected second request to be allowed")
	}

	clock.Advance(50 * time.Millisecond)

	third, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error when oldest request reaches window start: %v", err)
	}
	if !third.Allowed {
		t.Fatal("expected request at exact window-start boundary to be allowed")
	}
	if third.Remaining != 0 {
		t.Fatalf("expected only one slot to be freed at boundary, got remaining=%d", third.Remaining)
	}
}

func TestSlidingWindow_AllowsAtExactRetryAfterBoundary(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewSlidingWindow(1, time.Second, withClock(clock))

	first, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on first allow: %v", err)
	}
	if !first.Allowed {
		t.Fatal("expected first request to be allowed")
	}

	denied, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on denied request: %v", err)
	}
	if denied.Allowed {
		t.Fatal("expected request to be denied while the window is full")
	}

	clock.Advance(denied.RetryAfter)

	allowed, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error at retryAfter boundary: %v", err)
	}
	if !allowed.Allowed {
		t.Fatal("expected request to be allowed exactly at retryAfter boundary")
	}
}
