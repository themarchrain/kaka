package memory

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/themarchrain/kaka"
)

func TestLimiters_ResultBoundaries(t *testing.T) {
	cases := []struct {
		name       string
		newLimiter func() kaka.Limiter
	}{
		{
			name: "token bucket",
			newLimiter: func() kaka.Limiter {
				return NewTokenBucket(1, 1)
			},
		},
		{
			name: "leaky bucket",
			newLimiter: func() kaka.Limiter {
				return NewLeakyBucket(1, 1)
			},
		},
		{
			name: "sliding window",
			newLimiter: func() kaka.Limiter {
				return NewSlidingWindow(1, time.Second)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limiter := tc.newLimiter()
			ctx := context.Background()

			allowed, err := limiter.Allow(ctx, "user:1")
			if err != nil {
				t.Fatalf("unexpected error on first allow: %v", err)
			}
			if !allowed.Allowed {
				t.Fatal("expected first request to be allowed")
			}
			if allowed.Remaining < 0 {
				t.Fatalf("expected non-negative remaining on allow, got %d", allowed.Remaining)
			}
			if allowed.RetryAfter != 0 {
				t.Fatalf("expected retryAfter=0 on allow, got %v", allowed.RetryAfter)
			}

			denied, err := limiter.Allow(ctx, "user:1")
			if err != nil {
				t.Fatalf("unexpected error on second allow: %v", err)
			}
			if denied.Allowed {
				t.Fatal("expected second request to be denied")
			}
			if denied.Remaining != 0 {
				t.Fatalf("expected remaining=0 on deny, got %d", denied.Remaining)
			}
			if denied.RetryAfter < 0 {
				t.Fatalf("expected non-negative retryAfter on deny, got %v", denied.RetryAfter)
			}
			if denied.RetryAfter == 0 {
				t.Fatal("expected positive retryAfter on deny")
			}
		})
	}
}

func TestLimiters_ConcurrentAllow_SameKey(t *testing.T) {
	cases := []struct {
		name       string
		newLimiter func() kaka.Limiter
	}{
		{
			name: "token bucket",
			newLimiter: func() kaka.Limiter {
				return NewTokenBucket(10, 10)
			},
		},
		{
			name: "leaky bucket",
			newLimiter: func() kaka.Limiter {
				return NewLeakyBucket(10, 10)
			},
		},
		{
			name: "sliding window",
			newLimiter: func() kaka.Limiter {
				return NewSlidingWindow(10, time.Second)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limiter := tc.newLimiter()
			ctx := context.Background()

			const workers = 64
			type outcome struct {
				result kaka.Result
				err    error
			}
			results := make(chan outcome, workers)
			var wg sync.WaitGroup

			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					result, err := limiter.Allow(ctx, "user:1")
					results <- outcome{result: result, err: err}
				}()
			}

			wg.Wait()
			close(results)

			for out := range results {
				if out.err != nil {
					t.Fatalf("unexpected error during concurrent allow: %v", out.err)
				}
				if out.result.Remaining < 0 {
					t.Fatalf("expected non-negative remaining, got %d", out.result.Remaining)
				}
				if out.result.RetryAfter < 0 {
					t.Fatalf("expected non-negative retryAfter, got %v", out.result.RetryAfter)
				}
			}
		})
	}
}

func TestLimiters_ConcurrentAllow_MultipleKeys(t *testing.T) {
	cases := []struct {
		name       string
		newLimiter func() kaka.Limiter
	}{
		{
			name: "token bucket",
			newLimiter: func() kaka.Limiter {
				return NewTokenBucket(10, 10)
			},
		},
		{
			name: "leaky bucket",
			newLimiter: func() kaka.Limiter {
				return NewLeakyBucket(10, 10)
			},
		},
		{
			name: "sliding window",
			newLimiter: func() kaka.Limiter {
				return NewSlidingWindow(10, time.Second)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limiter := tc.newLimiter()
			ctx := context.Background()

			const workers = 64
			type outcome struct {
				result kaka.Result
				err    error
			}
			results := make(chan outcome, workers)
			var wg sync.WaitGroup

			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					result, err := limiter.Allow(ctx, "user:"+string(rune('0'+i%8)))
					results <- outcome{result: result, err: err}
				}(i)
			}

			wg.Wait()
			close(results)

			for out := range results {
				if out.err != nil {
					t.Fatalf("unexpected error during concurrent allow: %v", out.err)
				}
				if out.result.Remaining < 0 {
					t.Fatalf("expected non-negative remaining, got %d", out.result.Remaining)
				}
				if out.result.RetryAfter < 0 {
					t.Fatalf("expected non-negative retryAfter, got %v", out.result.RetryAfter)
				}
			}
		})
	}
}

func TestLimiters_ConcurrentAllow_RespectsMaxKeys(t *testing.T) {
	cases := []struct {
		name       string
		newLimiter func() kaka.Limiter
	}{
		{
			name: "token bucket",
			newLimiter: func() kaka.Limiter {
				return NewTokenBucket(100, 100, WithMaxKeys(4))
			},
		},
		{
			name: "leaky bucket",
			newLimiter: func() kaka.Limiter {
				return NewLeakyBucket(100, 100, WithMaxKeys(4))
			},
		},
		{
			name: "sliding window",
			newLimiter: func() kaka.Limiter {
				return NewSlidingWindow(100, time.Second, WithMaxKeys(4))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limiter := tc.newLimiter()
			ctx := context.Background()

			const workers = 32
			type outcome struct {
				result kaka.Result
				err    error
			}
			results := make(chan outcome, workers)
			var wg sync.WaitGroup

			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					result, err := limiter.Allow(ctx, "user:"+strconv.Itoa(i))
					results <- outcome{result: result, err: err}
				}(i)
			}

			wg.Wait()
			close(results)

			allowed := 0
			rejectedNewKeys := 0
			for out := range results {
				if out.err != nil {
					if !errors.Is(out.err, ErrMaxKeysExceeded) {
						t.Fatalf("expected only ErrMaxKeysExceeded under maxKeys pressure, got %v", out.err)
					}
					rejectedNewKeys++
					continue
				}
				if !out.result.Allowed {
					t.Fatal("expected created keys to be allowed on their first request")
				}
				if out.result.Remaining < 0 {
					t.Fatalf("expected non-negative remaining, got %d", out.result.Remaining)
				}
				if out.result.RetryAfter != 0 {
					t.Fatalf("expected retryAfter=0 on first allow, got %v", out.result.RetryAfter)
				}
				allowed++
			}

			if allowed != 4 {
				t.Fatalf("expected exactly maxKeys allowed creations, got %d", allowed)
			}
			if rejectedNewKeys != workers-4 {
				t.Fatalf("expected %d new keys to be rejected, got %d", workers-4, rejectedNewKeys)
			}
		})
	}
}

func TestLimiters_RejectBlankKey(t *testing.T) {
	cases := []struct {
		name       string
		newLimiter func() kaka.Limiter
	}{
		{
			name: "token bucket",
			newLimiter: func() kaka.Limiter {
				return NewTokenBucket(1, 1)
			},
		},
		{
			name: "leaky bucket",
			newLimiter: func() kaka.Limiter {
				return NewLeakyBucket(1, 1)
			},
		},
		{
			name: "sliding window",
			newLimiter: func() kaka.Limiter {
				return NewSlidingWindow(1, time.Second)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limiter := tc.newLimiter()
			_, err := limiter.Allow(context.Background(), "")
			if !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("expected ErrInvalidKey for empty key, got %v", err)
			}

			_, err = limiter.Allow(context.Background(), "   ")
			if !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("expected ErrInvalidKey for blank key, got %v", err)
			}
		})
	}
}
