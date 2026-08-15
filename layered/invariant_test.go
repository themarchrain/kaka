package layered

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/themarchrain/kaka"
	"github.com/themarchrain/kaka/memory"
)

// scriptedRemote returns results from a fixed sequence, wrapping around;
// used to drive the remote layer through allow/deny patterns.
type scriptedRemote struct {
	mu      sync.Mutex
	results []kaka.Result
	next    int
}

func (s *scriptedRemote) Allow(_ context.Context, _ string) (kaka.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := s.results[s.next%len(s.results)]
	s.next++
	return res, nil
}

// TestInvariant_AllowedImpliesRemoteAllowed is the core correctness
// invariant: the layered limiter never allows what the remote layer would
// deny, so the distributed quota is never exceeded (spec §6.1).
func TestInvariant_AllowedImpliesRemoteAllowed(t *testing.T) {
	ctx := context.Background()
	script := []kaka.Result{
		{Allowed: true, Remaining: 5},
		{Allowed: false, Remaining: 0, RetryAfter: time.Second},
	}
	remote := &scriptedRemote{results: script}
	// Local capacity is far above the call count, so the local layer never
	// denies and the remote layer is consulted on every call.
	limiter := New(memory.NewTokenBucket(10000, 10000), remote)

	for i := 0; i < 100; i++ {
		result, err := limiter.Allow(ctx, "user:1")
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
		want := script[i%len(script)]
		if result != want {
			t.Fatalf("call %d: got %+v, want remote result %+v", i, result, want)
		}
	}
}

// TestInvariant_AlwaysDenyRemote_DeniesEverything pins down the authority
// direction: when the remote layer denies, the layered limiter denies,
// regardless of what the local layer would say.
func TestInvariant_AlwaysDenyRemote_DeniesEverything(t *testing.T) {
	ctx := context.Background()
	remote := &scriptedLimiter{result: kaka.Result{Allowed: false, Remaining: 0, RetryAfter: time.Second}}
	limiter := New(memory.NewTokenBucket(10000, 10000), remote)

	for i := 0; i < 50; i++ {
		result, err := limiter.Allow(ctx, "user:1")
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
		if result.Allowed {
			t.Fatalf("call %d: expected deny — the remote layer is the authority", i)
		}
	}
}

// TestDrift_LocalGrowsStricter documents the approximation (spec §6.2):
// after a request consumes the local bucket, the next request is denied
// locally even though a fresh remote layer would allow it.
func TestDrift_LocalGrowsStricter(t *testing.T) {
	ctx := context.Background()
	remote := &scriptedLimiter{result: kaka.Result{Allowed: true, Remaining: 1}}
	limiter := New(memory.NewTokenBucket(1, 1), remote)

	result, err := limiter.Allow(ctx, "user:1")
	if err != nil || !result.Allowed {
		t.Fatalf("first allow: result=%+v err=%v, want allowed", result, err)
	}

	result, err = limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected local deny after the local bucket was drained")
	}
	if remote.callCount() != 1 {
		t.Fatalf("expected remote consulted once, got %d", remote.callCount())
	}
}
