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

// SlidingWindow is a Redis-backed distributed sliding window log limiter.
// Semantics match memory.NewSlidingWindow: at most limit requests per window,
// and on denial RetryAfter is when the oldest request expires.
type SlidingWindow struct {
	base   *limiter
	script *Script
	limit  int
	window time.Duration
}

// NewSlidingWindow creates a Redis sliding window limiter. It panics on invalid
// arguments: limit and window must be > 0.
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

// Allow reports whether key is permitted. Underlying Redis errors degrade per
// ErrorPolicy; argument validation errors are returned explicitly.
func (sw *SlidingWindow) Allow(ctx context.Context, key string) (kaka.Result, error) {
	if err := validateKey(key); err != nil {
		sw.base.emitError(err)
		return kaka.Result{}, err
	}

	// member 唯一盐：同毫秒并发请求不互相覆盖（ZADD 同 score 不同 member 共存）
	member := strconv.FormatUint(rand.Uint64(), 16)
	res, err := sw.script.Run(ctx, sw.base.client,
		[]string{sw.base.keyFor(key)},
		sw.limit, sw.window.Milliseconds(), sw.base.opts.keyTTL.Milliseconds(), member)
	if err != nil {
		sw.base.emitError(err)
		result := sw.base.fallback(err)
		sw.base.emit(result)
		return result, nil
	}

	arr, ok := res.([]interface{})
	if !ok || len(arr) != 3 {
		wrap := fmt.Errorf("%w: unexpected result type %T", ErrScript, res)
		sw.base.emitError(wrap)
		result := sw.base.fallback(wrap)
		sw.base.emit(result)
		return result, nil
	}
	allowed, ok1 := arr[0].(int64)
	remaining, ok2 := arr[1].(int64)
	retryMs, ok3 := arr[2].(int64)
	if !ok1 || !ok2 || !ok3 {
		wrap := fmt.Errorf("%w: unexpected result element type %T", ErrScript, res)
		sw.base.emitError(wrap)
		result := sw.base.fallback(wrap)
		sw.base.emit(result)
		return result, nil
	}
	result := kaka.Result{
		Allowed:    allowed == 1,
		Remaining:  remaining,
		RetryAfter: time.Duration(retryMs) * time.Millisecond,
	}
	sw.base.emit(result)
	return result, nil
}
