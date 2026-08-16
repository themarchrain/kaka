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
	leak     func(*leakyState, time.Time) (kaka.Result, error) // bound once at construction; runs under the shard lock
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

	lb := &LeakyBucket{
		capacity: capacity,
		rate:     rate,
		opts:     o,
		store: newStateStore[*leakyState](o, func(now time.Time) *leakyState {
			return &leakyState{
				water:    0, // 初始空桶
				lastLeak: now,
			}
		}, func() {
			if o.sink != nil {
				o.sink.OnEvict(kaka.TierSingle)
			}
		}),
	}
	lb.leak = lb.leakAndDecide
	return lb
}

// Allow reports whether key is permitted. It returns ErrInvalidKey when the key is empty or blank.
func (lb *LeakyBucket) Allow(ctx context.Context, key string) (kaka.Result, error) {
	if err := validateKey(key); err != nil {
		if lb.opts.sink != nil {
			lb.opts.sink.OnError(kaka.TierSingle, err)
		}
		return kaka.Result{}, err
	}

	now := lb.opts.clock.Now()
	result, err := lb.store.withState(key, now, lb.leak)
	if lb.opts.sink != nil {
		if err != nil {
			lb.opts.sink.OnError(kaka.TierSingle, err)
		} else if result.Allowed {
			lb.opts.sink.OnAllowed(kaka.TierSingle, result)
		} else {
			lb.opts.sink.OnRejected(kaka.TierSingle, result)
		}
		lb.opts.sink.SetKeys(kaka.TierSingle, lb.store.len())
	}
	return result, err
}

// leakAndDecide 在 shard 锁内执行：计算漏水并判定（同 key 并发串行化）。
func (lb *LeakyBucket) leakAndDecide(b *leakyState, now time.Time) (kaka.Result, error) {
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
