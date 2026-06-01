package contracttest

import (
	"context"
	"sync"
	"testing"

	"github.com/themarchrain/kaka"
)

type Limiter interface {
	Allow(context.Context, string) (kaka.Result, error)
}

type LimiterCase struct {
	Name string
	New  func() Limiter
}

func RunLimiterBehaviorTests(t *testing.T, cases []LimiterCase) {
	t.Helper()

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			limiter := tc.New()
			runResultBoundaryTest(t, limiter)
			runConcurrentSameKeyTest(t, limiter)
			runConcurrentMultipleKeysTest(t, limiter)
			runBlankKeyErrorTest(t, limiter)
		})
	}
}

func runResultBoundaryTest(t *testing.T, limiter Limiter) {
	t.Helper()

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
}

func runConcurrentSameKeyTest(t *testing.T, limiter Limiter) {
	t.Helper()

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
		go func() {
			defer wg.Done()
			result, err := limiter.Allow(ctx, "user:shared")
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
}

func runConcurrentMultipleKeysTest(t *testing.T, limiter Limiter) {
	t.Helper()

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
			result, err := limiter.Allow(ctx, "user:"+string(rune('0'+i%4)))
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
}

func runBlankKeyErrorTest(t *testing.T, limiter Limiter) {
	t.Helper()

	_, err := limiter.Allow(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty key")
	}

	_, err = limiter.Allow(context.Background(), "   ")
	if err == nil {
		t.Fatal("expected error for blank key")
	}
}
