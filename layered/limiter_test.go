package layered

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/themarchrain/kaka"
	"github.com/themarchrain/kaka/memory"
)

// scriptedLimiter is a kaka.Limiter stub that returns a fixed result and
// counts calls; used to observe which layer made the decision.
type scriptedLimiter struct {
	mu     sync.Mutex
	result kaka.Result
	err    error
	calls  int
}

func (s *scriptedLimiter) Allow(_ context.Context, _ string) (kaka.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return s.result, s.err
}

func (s *scriptedLimiter) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *scriptedLimiter) setErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

func TestNewPanicsOnNil(t *testing.T) {
	other := memory.NewTokenBucket(1, 1)
	for _, args := range [][2]kaka.Limiter{{nil, other}, {other, nil}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("expected panic for nil limiter argument")
				}
			}()
			New(args[0], args[1])
		}()
	}
}

func TestAllow_RemoteDecidesWhenLocalAllows(t *testing.T) {
	ctx := context.Background()
	remote := &scriptedLimiter{result: kaka.Result{Allowed: true, Remaining: 42}}
	limiter := New(memory.NewTokenBucket(100, 10), remote)

	result, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed || result.Remaining != 42 {
		t.Fatalf("expected remote result (allowed=true remaining=42), got %+v", result)
	}
	if remote.callCount() != 1 {
		t.Fatalf("expected remote consulted once, got %d calls", remote.callCount())
	}
}

func TestAllow_RemoteDenyReturnsRemoteRetryAfter(t *testing.T) {
	ctx := context.Background()
	remote := &scriptedLimiter{result: kaka.Result{Allowed: false, Remaining: 0, RetryAfter: 3 * time.Second}}
	limiter := New(memory.NewTokenBucket(100, 10), remote)

	result, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed || result.RetryAfter != 3*time.Second {
		t.Fatalf("expected remote deny with retryAfter=3s, got %+v", result)
	}
}

func TestAllow_LocalDenyShortCircuitsRemote(t *testing.T) {
	ctx := context.Background()
	remote := &scriptedLimiter{result: kaka.Result{Allowed: true, Remaining: 1}}
	limiter := New(memory.NewTokenBucket(1, 1), remote) // capacity 1: drained by the first call

	result, err := limiter.Allow(ctx, "user:1")
	if err != nil || !result.Allowed {
		t.Fatalf("first allow: result=%+v err=%v, want allowed", result, err)
	}

	result, err = limiter.Allow(ctx, "user:1") // immediately after: local bucket empty
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected local deny")
	}
	if result.RetryAfter <= 0 {
		t.Fatalf("expected positive local retryAfter, got %v", result.RetryAfter)
	}
	if remote.callCount() != 1 {
		t.Fatalf("expected remote consulted only on the first call, got %d", remote.callCount())
	}
}

func TestAllow_BlankKeySkipsBothLayers(t *testing.T) {
	ctx := context.Background()
	local := &scriptedLimiter{}
	remote := &scriptedLimiter{}
	limiter := New(local, remote)

	for _, key := range []string{"", "   "} {
		_, err := limiter.Allow(ctx, key)
		if !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("Allow(%q) err = %v, want ErrInvalidKey", key, err)
		}
	}
	if local.callCount() != 0 {
		t.Fatalf("expected local not consulted for blank keys, got %d calls", local.callCount())
	}
	if remote.callCount() != 0 {
		t.Fatalf("expected remote not consulted for blank keys, got %d calls", remote.callCount())
	}
}

func TestAllow_LocalErrorFallsThroughToRemote(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	local := &scriptedLimiter{err: boom}
	remote := &scriptedLimiter{result: kaka.Result{Allowed: true, Remaining: 3}}
	limiter := New(local, remote)

	result, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("expected local error to fall through, got error: %v", err)
	}
	if !result.Allowed || result.Remaining != 3 {
		t.Fatalf("expected remote result, got %+v", result)
	}
	if remote.callCount() != 1 {
		t.Fatalf("expected remote consulted once, got %d", remote.callCount())
	}
}

func TestAllow_LocalMaxKeysFallsThroughToRemote(t *testing.T) {
	ctx := context.Background()
	local := memory.NewTokenBucket(100, 10, memory.WithMaxKeys(1)) // EvictReject is the default
	remote := &scriptedLimiter{result: kaka.Result{Allowed: true, Remaining: 7}}
	limiter := New(local, remote)

	if _, err := limiter.Allow(ctx, "user:a"); err != nil { // occupies the only slot
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := limiter.Allow(ctx, "user:b") // local returns ErrMaxKeysExceeded
	if err != nil {
		t.Fatalf("expected fall-through to remote, got error: %v", err)
	}
	if !result.Allowed || result.Remaining != 7 {
		t.Fatalf("expected remote result, got %+v", result)
	}
	if remote.callCount() != 2 {
		t.Fatalf("expected remote consulted for both keys, got %d", remote.callCount())
	}
}

func TestAllow_RemoteErrorPropagates(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	remote := &scriptedLimiter{err: boom}
	limiter := New(memory.NewTokenBucket(100, 10), remote)

	_, err := limiter.Allow(ctx, "user:1")
	if !errors.Is(err, boom) {
		t.Fatalf("expected remote error to propagate, got %v", err)
	}
}

func TestAllow_RemoteErrorOnFallThroughPropagates(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	local := memory.NewTokenBucket(100, 10, memory.WithMaxKeys(1))
	remote := &scriptedLimiter{result: kaka.Result{Allowed: true, Remaining: 1}}
	limiter := New(local, remote)

	if _, err := limiter.Allow(ctx, "user:a"); err != nil { // occupies the only slot
		t.Fatalf("unexpected error: %v", err)
	}
	remote.setErr(boom)
	_, err := limiter.Allow(ctx, "user:b") // local ErrMaxKeysExceeded → remote errors
	if !errors.Is(err, boom) {
		t.Fatalf("expected remote error to propagate, got %v", err)
	}
}
