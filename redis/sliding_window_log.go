package redis

import (
	"context"
	_ "embed"
	"fmt"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/themarchrain/kaka"
)

//go:embed scripts/sliding_window_log.lua
var slidingWindowLogScript string

var _ kaka.Limiter = (*SlidingWindow)(nil)

// SlidingWindow 基于 Redis 的分布式滑动窗口日志限流器。
// 语义与 memory.NewSlidingWindow 对齐：窗口内最多 limit 次请求，
// 拒绝时 RetryAfter = 最早请求过期时间。
type SlidingWindow struct {
	base   *limiter
	script *Script
	limit  int
	window time.Duration
}

// NewSlidingWindow 创建 Redis 滑动窗口限流器。limit > 0、window > 0，非法参数 panic。
func NewSlidingWindow(client *redis.Client, limit int, window time.Duration, opts ...Option) *SlidingWindow {
	if limit <= 0 {
		panic("kaka/redis: sliding window limit must be > 0")
	}
	if window <= 0 {
		panic("kaka/redis: sliding window window must be > 0")
	}
	return &SlidingWindow{
		base:   newLimiter(client, opts...),
		script: NewScript("sliding_window_log", slidingWindowLogScript),
		limit:  limit,
		window: window,
	}
}

// Allow 判断 key 是否被允许。底层 Redis 错误按 ErrorPolicy 降级；
// 参数校验错误显式返回。
func (sw *SlidingWindow) Allow(ctx context.Context, key string) (kaka.Result, error) {
	if err := validateKey(key); err != nil {
		return kaka.Result{}, err
	}

	// member 唯一盐：同毫秒并发请求不互相覆盖（ZADD 同 score 不同 member 共存）
	member := strconv.FormatUint(rand.Uint64(), 16)
	res, err := sw.script.Run(ctx, sw.base.client,
		[]string{sw.base.keyFor(key)},
		sw.limit, sw.window.Milliseconds(), sw.base.opts.keyTTL.Milliseconds(), member)
	if err != nil {
		return sw.base.fallback(err), nil
	}

	arr, ok := res.([]interface{})
	if !ok || len(arr) != 3 {
		return sw.base.fallback(fmt.Errorf("%w: unexpected result type %T", ErrScript, res)), nil
	}
	allowed, ok1 := arr[0].(int64)
	remaining, ok2 := arr[1].(int64)
	retryMs, ok3 := arr[2].(int64)
	if !ok1 || !ok2 || !ok3 {
		return sw.base.fallback(fmt.Errorf("%w: unexpected result element type %T", ErrScript, res)), nil
	}
	return kaka.Result{
		Allowed:    allowed == 1,
		Remaining:  remaining,
		RetryAfter: time.Duration(retryMs) * time.Millisecond,
	}, nil
}
