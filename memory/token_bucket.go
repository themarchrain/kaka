package memory

import (
	"context"
	"sync"
	"time"

	"github.com/themarchrain/kaka"
)

var _ kaka.Limiter = (*TokenBucket)(nil)

type TokenBucket struct {
	mu       sync.Mutex
	capacity float64 // 桶的容量（最大突发量）
	rate     float64 // 令牌放入速率（个/秒）
	opts     options
	store    stateStore[*bucket]
}

// bucket 单个 key 的桶状态
type bucket struct {
	tokens       float64   // 当前令牌数量
	lastRefilled time.Time // 上次补充令牌的时间
}

func NewTokenBucket(capacity, rate float64, opts ...Option) *TokenBucket {
	if capacity <= 0 {
		panic("memory: token bucket capacity must be > 0")
	}
	if rate <= 0 {
		panic("memory: token bucket rate must be > 0")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	o.applyDefaults()
	o.validate()

	return &TokenBucket{
		capacity: capacity,
		rate:     rate,
		opts:     o,
		store:    newStateStore[*bucket](o),
	}
}

func (tb *TokenBucket) Allow(ctx context.Context, key string) (kaka.Result, error) {
	if err := validateKey(key); err != nil {
		return kaka.Result{}, err
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := tb.opts.clock.Now()
	b, err := tb.store.getOrCreate(key, now, func(now time.Time) *bucket {
		return &bucket{
			tokens:       tb.capacity, // 初始默认满桶
			lastRefilled: now,
		}
	})
	if err != nil {
		return kaka.Result{}, err
	}

	// 计算距离上次请求过去了多久，并计算这段时间应该生成多少新令牌
	elapsed := now.Sub(b.lastRefilled).Seconds()
	b.tokens += elapsed * tb.rate

	// 令牌数不能超过桶的容量
	if b.tokens > tb.capacity {
		b.tokens = tb.capacity
	}
	b.lastRefilled = now

	// 判断是否还有令牌可以扣除
	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return kaka.Result{
			Allowed:   true,
			Remaining: int64(b.tokens),
		}, nil
	}

	// 被限流，计算需要等待的时间（恢复1个令牌需要的时间）
	retryAfter := time.Duration((1.0 - b.tokens) / tb.rate * float64(time.Second))
	return kaka.Result{
		Allowed:    false,
		Remaining:  0,
		RetryAfter: retryAfter,
	}, nil
}
