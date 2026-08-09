package redis

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/themarchrain/kaka"
)

//go:embed scripts/leaky_bucket.lua
var leakyBucketScript string

var _ kaka.Limiter = (*LeakyBucket)(nil)

// LeakyBucket 基于 Redis 的分布式漏桶限流器。
// 语义与 memory.NewLeakyBucket 对齐：初始空桶、按 rate/秒 漏水、
// 满桶溢出拒绝，RetryAfter = 漏掉溢出量所需时间。
type LeakyBucket struct {
	base     *limiter
	script   *Script
	capacity float64
	rate     float64
}

// NewLeakyBucket 创建 Redis 漏桶。capacity >= 1、rate > 0，非法参数 panic。
func NewLeakyBucket(client *redis.Client, capacity, rate float64, opts ...Option) *LeakyBucket {
	if !isFinite(capacity) || capacity < 1 {
		panic("kaka/redis: leaky bucket capacity must be >= 1")
	}
	if !isFinite(rate) || rate <= 0 {
		panic("kaka/redis: leaky bucket rate must be > 0")
	}
	return &LeakyBucket{
		base:     newLimiter(client, opts...),
		script:   NewScript("leaky_bucket", leakyBucketScript),
		capacity: capacity,
		rate:     rate,
	}
}

// Allow 判断 key 是否被允许。底层 Redis 错误按 ErrorPolicy 降级；
// 参数校验错误显式返回。
func (lb *LeakyBucket) Allow(ctx context.Context, key string) (kaka.Result, error) {
	if err := validateKey(key); err != nil {
		return kaka.Result{}, err
	}

	ttlMs := lb.base.opts.keyTTL.Milliseconds()
	res, err := lb.script.Run(ctx, lb.base.client,
		[]string{lb.base.keyFor(key)},
		lb.capacity, lb.rate, ttlMs)
	if err != nil {
		return lb.base.fallback(err), nil
	}

	arr, ok := res.([]interface{})
	if !ok || len(arr) != 3 {
		return lb.base.fallback(fmt.Errorf("%w: unexpected result type %T", ErrScript, res)), nil
	}
	allowed, ok1 := arr[0].(int64)
	remaining, ok2 := arr[1].(int64)
	retryMs, ok3 := arr[2].(int64)
	if !ok1 || !ok2 || !ok3 {
		return lb.base.fallback(fmt.Errorf("%w: unexpected result element type %T", ErrScript, res)), nil
	}
	return kaka.Result{
		Allowed:    allowed == 1,
		Remaining:  remaining,
		RetryAfter: time.Duration(retryMs) * time.Millisecond,
	}, nil
}
