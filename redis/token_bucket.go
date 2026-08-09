package redis

import (
	"context"
	_ "embed"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/themarchrain/kaka"
)

//go:embed scripts/token_bucket.lua
var tokenBucketScript string

var _ kaka.Limiter = (*TokenBucket)(nil)

// TokenBucket 基于 Redis 的分布式令牌桶限流器。
// 语义与 memory.NewTokenBucket 对齐：初始满桶、按 rate/秒 补充、
// 拒绝时 RetryAfter = 恢复到 1 个令牌所需时间。
type TokenBucket struct {
	base     *limiter
	script   *Script
	capacity float64
	rate     float64
}

// NewTokenBucket 创建 Redis 令牌桶。capacity >= 1、rate > 0，非法参数 panic。
func NewTokenBucket(client *redis.Client, capacity, rate float64, opts ...Option) *TokenBucket {
	if !isFinite(capacity) || capacity < 1 {
		panic("kaka/redis: token bucket capacity must be >= 1")
	}
	if !isFinite(rate) || rate <= 0 {
		panic("kaka/redis: token bucket rate must be > 0")
	}
	return &TokenBucket{
		base:     newLimiter(client, opts...),
		script:   NewScript("token_bucket", tokenBucketScript),
		capacity: capacity,
		rate:     rate,
	}
}

// Allow 判断 key 是否被允许。底层 Redis 错误按 ErrorPolicy 降级
// （错误通过 onError 回调上报，不返回 error）；参数校验错误显式返回。
func (tb *TokenBucket) Allow(ctx context.Context, key string) (kaka.Result, error) {
	if err := validateKey(key); err != nil {
		return kaka.Result{}, err
	}

	ttlMs := tb.base.opts.keyTTL.Milliseconds()
	res, err := tb.script.Run(ctx, tb.base.client,
		[]string{tb.base.keyFor(key)},
		tb.capacity, tb.rate, ttlMs)
	if err != nil {
		return tb.base.fallback(err), nil
	}

	arr, ok := res.([]interface{})
	if !ok || len(arr) != 3 {
		return tb.base.fallback(fmt.Errorf("%w: unexpected result type %T", ErrScript, res)), nil
	}
	allowed, ok1 := arr[0].(int64)
	remaining, ok2 := arr[1].(int64)
	retryMs, ok3 := arr[2].(int64)
	if !ok1 || !ok2 || !ok3 {
		return tb.base.fallback(fmt.Errorf("%w: unexpected result element type %T", ErrScript, res)), nil
	}
	return kaka.Result{
		Allowed:    allowed == 1,
		Remaining:  remaining,
		RetryAfter: time.Duration(retryMs) * time.Millisecond,
	}, nil
}

func isFinite(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}
