package memory

import (
	"context"
	"sync"
	"time"

	"github.com/themarchrain/kaka"
)

type TokenBucket struct {
	mu       sync.Mutex
	capacity float64            // 桶的容量（最大突发量）
	rate     float64            // 令牌放入速率（个/秒）
	buckets  map[string]*bucket // 按 key 存储桶状态
}

// bucket 单个 key 的桶状态
type bucket struct {
	tokens       float64   // 当前令牌数量
	lastRefilled time.Time // 上次补充令牌的时间
}

func NewTokenBucket(capacity, rate float64) *TokenBucket {
	return &TokenBucket{
		capacity: capacity,
		rate:     rate,
		buckets:  make(map[string]*bucket),
	}
}

func (tb *TokenBucket) Allow(ctx context.Context, key string) (kaka.Result, error) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	b, exists := tb.buckets[key]
	if !exists {
		b = &bucket{
			tokens:       tb.capacity, // 初始默认满桶
			lastRefilled: time.Now(),
		}
		tb.buckets[key] = b
	}

	now := time.Now()
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
