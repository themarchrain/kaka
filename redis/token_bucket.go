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

// TokenBucket is a Redis-backed distributed token bucket limiter.
// Semantics match memory.NewTokenBucket: the bucket starts full, refills at rate
// tokens per second, and on denial RetryAfter is the time to refill one token.
type TokenBucket struct {
	base     *limiter
	script   *Script
	capacity float64
	rate     float64
}

// NewTokenBucket creates a Redis token bucket. It panics on invalid arguments:
// capacity must be >= 1 and rate > 0.
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

// Allow reports whether key is permitted. Underlying Redis errors degrade per
// ErrorPolicy (reported through the onError callback rather than returned);
// argument validation errors are returned explicitly.
func (tb *TokenBucket) Allow(ctx context.Context, key string) (kaka.Result, error) {
	if err := validateKey(key); err != nil {
		tb.base.emitError(err)
		return kaka.Result{}, err
	}

	ttlMs := tb.base.opts.keyTTL.Milliseconds()
	res, err := tb.script.Run(ctx, tb.base.client,
		[]string{tb.base.keyFor(key)},
		tb.capacity, tb.rate, ttlMs)
	if err != nil {
		tb.base.emitError(err)
		result := tb.base.fallback(err)
		tb.base.emit(result)
		return result, nil
	}

	arr, ok := res.([]interface{})
	if !ok || len(arr) != 3 {
		wrap := fmt.Errorf("%w: unexpected result type %T", ErrScript, res)
		tb.base.emitError(wrap)
		result := tb.base.fallback(wrap)
		tb.base.emit(result)
		return result, nil
	}
	allowed, ok1 := arr[0].(int64)
	remaining, ok2 := arr[1].(int64)
	retryMs, ok3 := arr[2].(int64)
	if !ok1 || !ok2 || !ok3 {
		wrap := fmt.Errorf("%w: unexpected result element type %T", ErrScript, res)
		tb.base.emitError(wrap)
		result := tb.base.fallback(wrap)
		tb.base.emit(result)
		return result, nil
	}
	result := kaka.Result{
		Allowed:    allowed == 1,
		Remaining:  remaining,
		RetryAfter: time.Duration(retryMs) * time.Millisecond,
	}
	tb.base.emit(result)
	return result, nil
}

func isFinite(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}
