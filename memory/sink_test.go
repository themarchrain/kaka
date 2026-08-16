package memory

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/themarchrain/kaka"
)

// recordingSink 记录所有 sink 事件，供断言。
type recordingSink struct {
	mu       sync.Mutex
	allowed  []kaka.Result
	rejected []kaka.Result
	errs     []error
	keys     []int
	evicts   int
}

func (s *recordingSink) OnAllowed(tier string, r kaka.Result) {
	s.mu.Lock()
	s.allowed = append(s.allowed, r)
	s.mu.Unlock()
}
func (s *recordingSink) OnRejected(tier string, r kaka.Result) {
	s.mu.Lock()
	s.rejected = append(s.rejected, r)
	s.mu.Unlock()
}
func (s *recordingSink) OnError(tier string, err error) {
	s.mu.Lock()
	s.errs = append(s.errs, err)
	s.mu.Unlock()
}
func (s *recordingSink) SetKeys(tier string, n int) {
	s.mu.Lock()
	s.keys = append(s.keys, n)
	s.mu.Unlock()
}
func (s *recordingSink) OnEvict(tier string) {
	s.mu.Lock()
	s.evicts++
	s.mu.Unlock()
}

func (s *recordingSink) snapshot() (int, int, int, []int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.allowed), len(s.rejected), len(s.errs), append([]int(nil), s.keys...), s.evicts
}

func TestTokenBucket_WithSink_CountsAllowedRejected(t *testing.T) {
	sink := &recordingSink{}
	limiter := NewTokenBucket(10, 1, WithMetricSink(sink))
	ctx := context.Background()

	for i := 0; i < 11; i++ {
		if _, err := limiter.Allow(ctx, "user:1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if _, err := limiter.Allow(ctx, "  "); err == nil {
		t.Fatal("expected ErrInvalidKey for blank key")
	}

	allowed, rejected, errs, keys, _ := sink.snapshot()
	if allowed != 10 || rejected != 1 || errs != 1 {
		t.Fatalf("expected 10 allowed / 1 rejected / 1 error, got %d / %d / %d", allowed, rejected, errs)
	}
	// 空 key 在 validateKey 处提前返回（不触发 SetKeys）：11 次合法请求 → 11 次 SetKeys，值恒为 1。
	if len(keys) != 11 || keys[10] != 1 {
		t.Fatalf("expected 11 SetKeys calls with value 1, got len=%d last=%d", len(keys), keys[len(keys)-1])
	}
}

func TestTokenBucket_WithSink_CountsMaxKeysError(t *testing.T) {
	sink := &recordingSink{}
	limiter := NewTokenBucket(10, 1, WithMaxKeys(1), WithMetricSink(sink))
	ctx := context.Background()

	if _, err := limiter.Allow(ctx, "user:1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err := limiter.Allow(ctx, "user:2")
	if !errors.Is(err, ErrMaxKeysExceeded) {
		t.Fatalf("expected ErrMaxKeysExceeded, got %v", err)
	}

	_, _, errs, _, _ := sink.snapshot()
	if errs != 1 {
		t.Fatalf("expected 1 error, got %d", errs)
	}
}

func TestTokenBucket_NoSink_IsNoop(t *testing.T) {
	limiter := NewTokenBucket(10, 1)
	ctx := context.Background()
	if _, err := limiter.Allow(ctx, "user:1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTokenBucket_WithSink_CountsEviction(t *testing.T) {
	sink := &recordingSink{}
	limiter := NewTokenBucket(10, 1, WithMaxKeys(1), WithEvictionPolicy(EvictLRU), WithMetricSink(sink))
	ctx := context.Background()

	if _, err := limiter.Allow(ctx, "user:1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 满 + 新 key → 淘汰 user:1 腾位。
	if _, err := limiter.Allow(ctx, "user:2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, _, _, _, evicts := sink.snapshot()
	if evicts != 1 {
		t.Fatalf("expected 1 eviction, got %d", evicts)
	}
}

func TestLeakyBucket_WithSink_CountsAllowedRejected(t *testing.T) {
	sink := &recordingSink{}
	limiter := NewLeakyBucket(10, 1, WithMetricSink(sink))
	ctx := context.Background()

	for i := 0; i < 11; i++ {
		if _, err := limiter.Allow(ctx, "user:1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	allowed, rejected, errs, _, _ := sink.snapshot()
	if allowed != 10 || rejected != 1 || errs != 0 {
		t.Fatalf("expected 10 allowed / 1 rejected / 0 errors, got %d / %d / %d", allowed, rejected, errs)
	}
}

func TestSlidingWindow_WithSink_CountsAllowedRejected(t *testing.T) {
	sink := &recordingSink{}
	limiter := NewSlidingWindow(10, time.Hour, WithMetricSink(sink))
	ctx := context.Background()

	for i := 0; i < 11; i++ {
		if _, err := limiter.Allow(ctx, "user:1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	allowed, rejected, errs, _, _ := sink.snapshot()
	if allowed != 10 || rejected != 1 || errs != 0 {
		t.Fatalf("expected 10 allowed / 1 rejected / 0 errors, got %d / %d / %d", allowed, rejected, errs)
	}
}
