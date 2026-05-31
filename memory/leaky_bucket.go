package memory

import (
	"context"
	"sync"
	"time"

	"github.com/themarchrain/kaka"
)

type LeakyBucket struct {
	mu       sync.Mutex
	capacity float64                // 桶的容量（最大积压量）
	rate     float64                // 漏水速率（滴/秒）
	buckets  map[string]*leakyState // 按 key 存储桶状态
}

type leakyState struct {
	water    float64   // 当前桶里的水量
	lastLeak time.Time // 上次漏水的时间
}

func NewLeakyBucket(capacity, rate float64) *LeakyBucket {
	return &LeakyBucket{
		capacity: capacity,
		rate:     rate,
		buckets:  make(map[string]*leakyState),
	}
}

func (lb *LeakyBucket) Allow(ctx context.Context, key string) (kaka.Result, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	b, exists := lb.buckets[key]
	if !exists {
		b = &leakyState{
			water:    0, // 初始空桶
			lastLeak: time.Now(),
		}
		lb.buckets[key] = b
	}

	now := time.Now()
	// 计算过去这段时间漏掉了多少水
	elapsed := now.Sub(b.lastLeak).Seconds()
	leakedWater := elapsed * lb.rate

	// 扣除漏掉的水，水量不能小于 0
	b.water -= leakedWater
	if b.water < 0 {
		b.water = 0
	}
	b.lastLeak = now

	// 尝试加入一滴新水（一个新请求）
	// 如果当前水量 + 1 滴水没有超过容量，则允许
	if b.water+1 <= lb.capacity {
		b.water += 1
		return kaka.Result{
			Allowed:   true,
			Remaining: int64(lb.capacity - b.water),
		}, nil
	}
	// 桶满了，溢出拒绝
	// 计算需要等待的时间（漏掉1滴水需要的时间）
	retryAfter := time.Duration(1.0 / lb.rate * float64(time.Second))
	return kaka.Result{
		Allowed:    false,
		Remaining:  0,
		RetryAfter: retryAfter,
	}, nil
}
