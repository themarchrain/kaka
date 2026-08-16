package memory

import (
	"context"
	"time"

	"github.com/themarchrain/kaka"
)

var _ kaka.Limiter = (*TokenBucket)(nil)

// TokenBucket is a per-key token bucket rate limiter.
type TokenBucket struct {
	capacity float64 // 桶的容量（最大突发量）
	rate     float64 // 令牌放入速率（个/秒）
	opts     options
	store    stateStore[*bucket]
	refill   func(*bucket, time.Time) (kaka.Result, error) // bound once at construction; runs under the shard lock
}

// bucket 单个 key 的桶状态
type bucket struct {
	tokens       float64   // 当前令牌数量
	lastRefilled time.Time // 上次补充令牌的时间
}

// NewTokenBucket creates a token bucket limiter with the given capacity and refill
// rate (tokens per second). It panics if capacity is not finite or < 1, or if rate
// is not finite or <= 0.
func NewTokenBucket(capacity, rate float64, opts ...Option) *TokenBucket {
	if !isFinite(capacity) || capacity < 1 {
		panic("memory: token bucket capacity must be >= 1")
	}
	if !isFinite(rate) || rate <= 0 {
		panic("memory: token bucket rate must be > 0")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	o.applyDefaults()
	o.validate()

	tb := &TokenBucket{
		capacity: capacity,
		rate:     rate,
		opts:     o,
		store: newStateStore[*bucket](o, func(now time.Time) *bucket {
			return &bucket{
				tokens:       capacity, // 初始默认满桶
				lastRefilled: now,
			}
		}),
	}
	tb.refill = tb.refillAndDecide
	return tb
}

// Allow reports whether key is permitted. It returns ErrInvalidKey when the key is empty or blank.
func (tb *TokenBucket) Allow(ctx context.Context, key string) (kaka.Result, error) {
	if err := validateKey(key); err != nil {
		if tb.opts.sink != nil {
			tb.opts.sink.OnError(kaka.TierSingle, err)
		}
		return kaka.Result{}, err
	}

	now := tb.opts.clock.Now()
	result, err := tb.store.withState(key, now, tb.refill)
	if tb.opts.sink != nil {
		if err != nil {
			tb.opts.sink.OnError(kaka.TierSingle, err)
		} else if result.Allowed {
			tb.opts.sink.OnAllowed(kaka.TierSingle, result)
		} else {
			tb.opts.sink.OnRejected(kaka.TierSingle, result)
		}
		tb.opts.sink.SetKeys(kaka.TierSingle, tb.store.len())
	}
	return result, err
}

// refillAndDecide 在 shard 锁内执行：计算补充令牌并判定（同 key 并发串行化）。
func (tb *TokenBucket) refillAndDecide(b *bucket, now time.Time) (kaka.Result, error) {
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
			Remaining: remainingFloor(b.tokens),
		}, nil
	}

	// 被限流，计算需要等待的时间（恢复1个令牌需要的时间）
	retryAfter := durationFromSecondsCeil((1.0 - b.tokens) / tb.rate)
	return kaka.Result{
		Allowed:    false,
		Remaining:  0,
		RetryAfter: retryAfter,
	}, nil
}
