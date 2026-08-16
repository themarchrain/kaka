package memory

import (
	"context"
	"time"

	"github.com/themarchrain/kaka"
)

var _ kaka.Limiter = (*LeakyBucket)(nil)

// LeakyBucket is a per-key leaky bucket rate limiter.
type LeakyBucket struct {
	capacity float64 // 桶的容量（最大积压量）
	rate     float64 // 漏水速率（滴/秒）
	opts     options
	store    stateStore[*leakyState]
}

type leakyState struct {
	water    float64   // 当前桶里的水量
	lastLeak time.Time // 上次漏水的时间
}

// NewLeakyBucket creates a leaky bucket limiter with the given capacity and leak
// rate (drops per second). It panics if capacity is not finite or < 1, or if rate
// is not finite or <= 0.
func NewLeakyBucket(capacity, rate float64, opts ...Option) *LeakyBucket {
	if !isFinite(capacity) || capacity < 1 {
		panic("memory: leaky bucket capacity must be >= 1")
	}
	if !isFinite(rate) || rate <= 0 {
		panic("memory: leaky bucket rate must be > 0")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	o.applyDefaults()
	o.validate()

	return &LeakyBucket{
		capacity: capacity,
		rate:     rate,
		opts:     o,
		store: newStateStore[*leakyState](o, func(now time.Time) *leakyState {
			return &leakyState{
				water:    0, // 初始空桶
				lastLeak: now,
			}
		}),
	}
}

// Allow reports whether key is permitted. It returns ErrInvalidKey when the key is empty or blank.
func (lb *LeakyBucket) Allow(ctx context.Context, key string) (kaka.Result, error) {
	if err := validateKey(key); err != nil {
		return kaka.Result{}, err
	}

	now := lb.opts.clock.Now()
	b, err := lb.store.getOrCreate(key, now)
	if err != nil {
		return kaka.Result{}, err
	}

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
			Remaining: remainingFloor(lb.capacity - b.water),
		}, nil
	}
	// 桶满了，溢出拒绝
	// 计算需要等待的时间（再漏掉多少水才能容纳当前请求）
	overflow := b.water + 1 - lb.capacity
	retryAfter := durationFromSecondsCeil(overflow / lb.rate)
	return kaka.Result{
		Allowed:    false,
		Remaining:  0,
		RetryAfter: retryAfter,
	}, nil
}
