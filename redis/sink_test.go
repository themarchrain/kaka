package redis

import (
	"context"
	"sync"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/themarchrain/kaka"
)

type recordingSink struct {
	mu       sync.Mutex
	allowed  []kaka.Result
	rejected []kaka.Result
	errs     []error
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
func (s *recordingSink) SetKeys(tier string, n int) {}
func (s *recordingSink) OnEvict(tier string)        {}

func (s *recordingSink) counts() (int, int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.allowed), len(s.rejected), len(s.errs)
}

// 需要真实 Redis（与既有契约测试同一约定：REDIS_ADDR 或 127.0.0.1:6379）。
func TestTokenBucket_WithSink_CountsAllowedRejected(t *testing.T) {
	client := testClient(t)
	sink := &recordingSink{}
	tb := NewTokenBucket(client, 10, 1, WithMetricSink(sink))
	ctx := context.Background()
	for i := 0; i < 11; i++ {
		if _, err := tb.Allow(ctx, "sink:key"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	allowed, rejected, errs := sink.counts()
	if allowed != 10 || rejected != 1 || errs != 0 {
		t.Fatalf("expected 10/1/0, got %d/%d/%d", allowed, rejected, errs)
	}
}

// 不可达地址：fail-closed 降级 → 错误计数 + 拒绝计数各 1。
func TestTokenBucket_WithSink_CountsErrorDegradation(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: 0})
	defer client.Close()

	sink := &recordingSink{}
	tb := NewTokenBucket(client, 10, 1, WithMetricSink(sink))
	ctx := context.Background()
	if _, err := tb.Allow(ctx, "sink:key"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	allowed, rejected, errs := sink.counts()
	if allowed != 0 || rejected != 1 || errs != 1 {
		t.Fatalf("expected 0/1/1 (fail-closed), got %d/%d/%d", allowed, rejected, errs)
	}
}
