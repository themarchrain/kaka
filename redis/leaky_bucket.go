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

// LeakyBucket is a Redis-backed distributed leaky bucket limiter.
// Semantics match memory.NewLeakyBucket: the bucket starts empty, leaks at rate
// drops per second, overflow is denied, and RetryAfter is the time to leak the overflow.
type LeakyBucket struct {
	base     *limiter
	script   scriptRunner
	capacity float64
	rate     float64
}

// NewLeakyBucket creates a Redis leaky bucket. It panics on invalid arguments:
// capacity must be >= 1 and rate > 0.
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

// Allow reports whether key is permitted. Underlying Redis errors degrade per
// ErrorPolicy; argument validation errors are returned explicitly.
func (lb *LeakyBucket) Allow(ctx context.Context, key string) (kaka.Result, error) {
	if err := validateKey(key); err != nil {
		lb.base.emitError(err)
		return kaka.Result{}, err
	}

	ttlMs := lb.base.opts.keyTTL.Milliseconds()
	res, err := lb.script.Run(ctx, lb.base.client,
		[]string{lb.base.keyFor(key)},
		lb.capacity, lb.rate, ttlMs)
	if err != nil {
		lb.base.emitError(err)
		result := lb.base.fallback(err)
		lb.base.emit(result)
		return result, nil
	}

	arr, ok := res.([]interface{})
	if !ok || len(arr) != 3 {
		wrap := fmt.Errorf("%w: unexpected result type %T", ErrScript, res)
		lb.base.emitError(wrap)
		result := lb.base.fallback(wrap)
		lb.base.emit(result)
		return result, nil
	}
	allowed, ok1 := arr[0].(int64)
	remaining, ok2 := arr[1].(int64)
	retryMs, ok3 := arr[2].(int64)
	if !ok1 || !ok2 || !ok3 {
		wrap := fmt.Errorf("%w: unexpected result element type %T", ErrScript, res)
		lb.base.emitError(wrap)
		result := lb.base.fallback(wrap)
		lb.base.emit(result)
		return result, nil
	}
	result := kaka.Result{
		Allowed:    allowed == 1,
		Remaining:  remaining,
		RetryAfter: time.Duration(retryMs) * time.Millisecond,
	}
	lb.base.emit(result)
	return result, nil
}
