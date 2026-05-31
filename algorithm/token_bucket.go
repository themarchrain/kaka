package algorithm

import (
	"sync"
	"time"
)

type TokenBucket struct {
	mu           sync.Mutex
	capacity     float64   // 桶的容量 (最大突发量)
	rate         float64   // 令牌放入速率 (个/秒)
	tokens       float64   // 当前令牌数量
	lastRefilled time.Time // 上次补充令牌的时间
}

func NewTokenBucket(capacity, rate float64) *TokenBucket {
	return &TokenBucket{
		capacity:     capacity,
		rate:         rate,
		tokens:       capacity, // 初始默认满桶
		lastRefilled: time.Now(),
	}
}

func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	// 计算距离上次请求过去了多久，并计算这段时间应该生成多少新令牌
	elapsed := now.Sub(tb.lastRefilled).Seconds()
	tb.tokens += elapsed * tb.rate

	// 令牌数不能超过桶的容量
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastRefilled = now

	// 判断是否还有令牌可以扣除
	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}
	return false
}
