package layered

import (
	"context"
	"testing"

	"github.com/themarchrain/kaka"
	"github.com/themarchrain/kaka/memory"
)

// TestLocalRejectPath_ZeroAllocations guards the short-circuit hot path:
// denying a request locally must not allocate (spec §9 P2, CI-safe).
func TestLocalRejectPath_ZeroAllocations(t *testing.T) {
	ctx := context.Background()
	limiter := New(memory.NewTokenBucket(1, 1), &scriptedLimiter{result: kaka.Result{Allowed: true}})

	if _, err := limiter.Allow(ctx, "user:1"); err != nil { // drain the local bucket
		t.Fatalf("unexpected error warming up: %v", err)
	}

	allocs := testing.AllocsPerRun(10000, func() {
		if _, err := limiter.Allow(ctx, "user:1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if allocs != 0 {
		t.Fatalf("expected 0 allocations on the local reject path, got %v", allocs)
	}
}

// TestAllowPath_ZeroAllocations guards the allow path when the remote layer
// is a plain in-memory implementation (no I/O allocations).
func TestAllowPath_ZeroAllocations(t *testing.T) {
	ctx := context.Background()
	limiter := New(
		memory.NewTokenBucket(20000, 1000),
		&scriptedLimiter{result: kaka.Result{Allowed: true, Remaining: 1}},
	)

	if _, err := limiter.Allow(ctx, "user:1"); err != nil { // warm up (bucket creation)
		t.Fatalf("unexpected error warming up: %v", err)
	}

	allocs := testing.AllocsPerRun(10000, func() {
		if _, err := limiter.Allow(ctx, "user:1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if allocs != 0 {
		t.Fatalf("expected 0 allocations on the allow path, got %v", allocs)
	}
}
