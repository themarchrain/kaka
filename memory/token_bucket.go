package memory

import (
	"context"
	"sync"
	"time"

	"github.com/themarchrain/kaka"
)

var _ kaka.Limiter = (*TokenBucket)(nil)

type TokenBucket struct {
	mu          sync.Mutex
	capacity    float64            // 桶的容量（最大突发量）
	rate        float64            // 令牌放入速率（个/秒）
	buckets     map[string]*bucket // 按 key 存储桶状态
	opts        options
	lastCleanup time.Time // 上次触发清理的时间
}

// bucket 单个 key 的桶状态
type bucket struct {
	tokens       float64   // 当前令牌数量
	lastRefilled time.Time // 上次补充令牌的时间
	lastSeen     time.Time // 最后一次被访问（用于 TTL 清理）
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
		buckets:  make(map[string]*bucket),
		opts:     o,
	}
}

func (tb *TokenBucket) Allow(ctx context.Context, key string) (kaka.Result, error) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	b, exists := tb.buckets[key]
	if !exists {
		// 新 key 前先触发惰性清理
		tb.lazyCleanup()
		if tb.opts.maxKeys > 0 && len(tb.buckets) >= tb.opts.maxKeys {
			return kaka.Result{}, ErrMaxKeysExceeded
		}
		now := time.Now()
		b = &bucket{
			tokens:       tb.capacity, // 初始默认满桶
			lastRefilled: now,
			lastSeen:     now,
		}
		tb.buckets[key] = b
	} else {
		b.lastSeen = time.Now()
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

// lazyCleanup 在 keyTTL 和 cleanupInterval 都启用时，按批次清理过期 key
// 调用方必须持有 tb.mu
func (tb *TokenBucket) lazyCleanup() {
	if tb.opts.keyTTL <= 0 || tb.opts.cleanupInterval <= 0 {
		return
	}
	now := time.Now()
	if now.Sub(tb.lastCleanup) < tb.opts.cleanupInterval {
		return
	}
	tb.lastCleanup = now

	expired := make([]string, 0, cleanupBatchSize)
	scanned := 0
	for k, b := range tb.buckets {
		if scanned >= cleanupBatchSize {
			break
		}
		scanned++
		if now.Sub(b.lastSeen) >= tb.opts.keyTTL {
			expired = append(expired, k)
		}
	}
	for _, k := range expired {
		delete(tb.buckets, k)
	}
}
