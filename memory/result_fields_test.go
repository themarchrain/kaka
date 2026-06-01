package memory

import (
	"context"
	"testing"
	"time"
)

func TestTokenBucket_ResultFields(t *testing.T) {
	ctx := context.Background()
	limiter := NewTokenBucket(3, 0.25)

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

	time.Sleep(50 * time.Millisecond)

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

func TestLeakyBucket_ResultFields(t *testing.T) {
	ctx := context.Background()
	limiter := NewLeakyBucket(3, 0.25)

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

func TestSlidingWindow_ResultFields(t *testing.T) {
	ctx := context.Background()
	limiter := NewSlidingWindow(3, 150*time.Millisecond)

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

	time.Sleep(denied.RetryAfter + 20*time.Millisecond)

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
