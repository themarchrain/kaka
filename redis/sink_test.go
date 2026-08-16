package redis

import (
	"context"
	"sync"
	"testing"
	"time"

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
// 使用 testKeyPrefix 使 key 落在测试清理范围内：每次测试结束清桶，
// 保证 -count=N 重复运行时从满桶开始（10 allowed / 1 rejected 确定）。
func TestTokenBucket_WithSink_CountsAllowedRejected(t *testing.T) {
	client := testClient(t)
	sink := &recordingSink{}
	tb := NewTokenBucket(client, 10, 1, WithKeyPrefix(testKeyPrefix), WithMetricSink(sink))
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

// fakeScript 返回固定结果/错误，用于覆盖解析错误分支。
type fakeScript struct {
	res interface{}
	err error
}

func (f *fakeScript) Run(ctx context.Context, client *redis.Client, keys []string, args ...interface{}) (interface{}, error) {
	return f.res, f.err
}

func newSinkTokenBucket(client *redis.Client, sink kaka.MetricSink) kaka.Limiter {
	return NewTokenBucket(client, 10, 1, WithKeyPrefix(testKeyPrefix), WithMetricSink(sink))
}
func newSinkLeakyBucket(client *redis.Client, sink kaka.MetricSink) kaka.Limiter {
	return NewLeakyBucket(client, 10, 1, WithKeyPrefix(testKeyPrefix), WithMetricSink(sink))
}
func newSinkSlidingWindow(client *redis.Client, sink kaka.MetricSink) kaka.Limiter {
	return NewSlidingWindow(client, 10, time.Minute, WithKeyPrefix(testKeyPrefix), WithMetricSink(sink))
}

// 三个算法同一契约：容量 10、速率 1 → 10 allowed / 1 rejected / 0 errors。
func TestLimiters_WithSink_CountsAllowedRejected(t *testing.T) {
	client := testClient(t)
	for _, tc := range []struct {
		name string
		new  func(*redis.Client, kaka.MetricSink) kaka.Limiter
	}{
		{"token bucket", newSinkTokenBucket},
		{"leaky bucket", newSinkLeakyBucket},
		{"sliding window", newSinkSlidingWindow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sink := &recordingSink{}
			limiter := tc.new(client, sink)
			ctx := context.Background()
			// 每个算法独立 key：同一 Redis 上不同数据结构（string vs ZSET）
			// 不能共用 key 名，否则 WRONGTYPE。
			key := "sink:" + tc.name
			for i := 0; i < 11; i++ {
				if _, err := limiter.Allow(ctx, key); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
			allowed, rejected, errs := sink.counts()
			if allowed != 10 || rejected != 1 || errs != 0 {
				t.Fatalf("expected 10/1/0, got %d/%d/%d", allowed, rejected, errs)
			}
		})
	}
}

// fail-open 降级：一次降级请求 → OnError 1 次 + OnAllowed 1 次（spec §5.3）。
func TestLimiters_WithSink_FailOpenEmitsErrorAndDecision(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: 0})
	defer client.Close()

	sink := &recordingSink{}
	tb := NewTokenBucket(client, 10, 1, WithErrorPolicy(ErrorFailOpen), WithMetricSink(sink))
	ctx := context.Background()
	if _, err := tb.Allow(ctx, "sink:key"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	allowed, rejected, errs := sink.counts()
	if allowed != 1 || rejected != 0 || errs != 1 {
		t.Fatalf("expected 1/0/1 (fail-open), got %d/%d/%d", allowed, rejected, errs)
	}
}


// 解析错误分支（脚本返回畸形结果）在三个算法上一致：fail-closed → OnError 1 + OnRejected 1。
func TestLimiters_WithSink_CountsParseError(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", MaxRetries: 0})
	defer client.Close()

	ctx := context.Background()
	cases := []struct {
		name string
		new  func(kaka.MetricSink) kaka.Limiter
		stub func(kaka.Limiter)
	}{
		{"token bucket", func(sink kaka.MetricSink) kaka.Limiter { return newSinkTokenBucket(client, sink) }, func(l kaka.Limiter) { l.(*TokenBucket).script = &fakeScript{res: "not-an-array"} }},
		{"leaky bucket", func(sink kaka.MetricSink) kaka.Limiter { return newSinkLeakyBucket(client, sink) }, func(l kaka.Limiter) { l.(*LeakyBucket).script = &fakeScript{res: "not-an-array"} }},
		{"sliding window", func(sink kaka.MetricSink) kaka.Limiter { return newSinkSlidingWindow(client, sink) }, func(l kaka.Limiter) { l.(*SlidingWindow).script = &fakeScript{res: "not-an-array"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sink := &recordingSink{}
			limiter := tc.new(sink)
			tc.stub(limiter)
			if _, err := limiter.Allow(ctx, "sink:parse"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			allowed, rejected, errs := sink.counts()
			if allowed != 0 || rejected != 1 || errs != 1 {
				t.Fatalf("expected 0/1/1 (parse error, fail-closed), got %d/%d/%d", allowed, rejected, errs)
			}
		})
	}
}

// 空 key：OnError 1 次，且不触达 Redis。
func TestLimiters_WithSink_CountsBlankKeyError(t *testing.T) {
	client := testClient(t)
	for _, tc := range []struct {
		name string
		new  func(*redis.Client, kaka.MetricSink) kaka.Limiter
	}{
		{"token bucket", newSinkTokenBucket},
		{"leaky bucket", newSinkLeakyBucket},
		{"sliding window", newSinkSlidingWindow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sink := &recordingSink{}
			limiter := tc.new(client, sink)
			ctx := context.Background()
			if _, err := limiter.Allow(ctx, "  "); err == nil {
				t.Fatal("expected ErrInvalidKey")
			}
			allowed, rejected, errs := sink.counts()
			if allowed != 0 || rejected != 0 || errs != 1 {
				t.Fatalf("expected 0/0/1 (blank key), got %d/%d/%d", allowed, rejected, errs)
			}
		})
	}
}
