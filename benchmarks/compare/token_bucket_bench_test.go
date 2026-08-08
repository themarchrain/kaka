package compare

import (
	"context"
	"testing"

	"github.com/juju/ratelimit"
	"github.com/themarchrain/kaka/memory"
	"golang.org/x/time/rate"
)

const (
	lowCap  = 100
	lowRate = 10.0
	// 高突发组：容量 10000、速率 1000/s
	highCap  = 10000
	highRate = 1000.0
	// 多 key 组：1000 个 key 轮转
	keyCount = 1000
)

func keys(i int) string {
	return "user:" + string(rune('0'+i%keyCount))
}

// ---- Kaka ----

func BenchmarkKakaTokenBucketLowBurstSingleKey(b *testing.B) {
	ctx := context.Background()
	l := memory.NewTokenBucket(lowCap, lowRate)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = l.Allow(ctx, "user:1")
	}
}

func BenchmarkKakaTokenBucketLowBurstManyKeys(b *testing.B) {
	ctx := context.Background()
	l := memory.NewTokenBucket(lowCap, lowRate)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = l.Allow(ctx, keys(i))
	}
}

func BenchmarkKakaTokenBucketHighBurstSingleKey(b *testing.B) {
	ctx := context.Background()
	l := memory.NewTokenBucket(highCap, highRate)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = l.Allow(ctx, "user:1")
	}
}

// ---- golang.org/x/time/rate ----

func BenchmarkXTimeRateLowBurstSingleKey(b *testing.B) {
	l := rate.NewLimiter(rate.Limit(lowRate), lowCap)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = l.Allow()
	}
}

func BenchmarkXTimeRateLowBurstManyKeys(b *testing.B) {
	l := rate.NewLimiter(rate.Limit(lowRate), lowCap)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = l.Allow()
	}
}

// x/time/rate 为全局单 limiter，无 per-key 概念；ManyKeys 仅作调用开销参照。

func BenchmarkXTimeRateHighBurstSingleKey(b *testing.B) {
	l := rate.NewLimiter(rate.Limit(highRate), highCap)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = l.Allow()
	}
}

// ---- juju/ratelimit ----

func BenchmarkJujuTokenBucketLowBurstSingleKey(b *testing.B) {
	l := ratelimit.NewBucketWithRate(lowRate, lowCap)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = l.TakeAvailable(1)
	}
}

func BenchmarkJujuTokenBucketLowBurstManyKeys(b *testing.B) {
	l := ratelimit.NewBucketWithRate(lowRate, lowCap)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = l.TakeAvailable(1)
	}
}

// juju 为全局单桶，无 per-key 概念；ManyKeys 仅作调用开销参照。

func BenchmarkJujuTokenBucketHighBurstSingleKey(b *testing.B) {
	l := ratelimit.NewBucketWithRate(highRate, highCap)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = l.TakeAvailable(1)
	}
}

// ---- 拒绝路径（超限后 O(1) 成本）----

func BenchmarkKakaTokenBucketRejectPath(b *testing.B) {
	ctx := context.Background()
	l := memory.NewTokenBucket(lowCap, lowRate)
	// 预热：填满并耗尽
	for i := 0; i < lowCap; i++ {
		_, _ = l.Allow(ctx, "user:1")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = l.Allow(ctx, "user:1")
	}
}

func BenchmarkXTimeRateRejectPath(b *testing.B) {
	l := rate.NewLimiter(rate.Limit(lowRate), lowCap)
	for i := 0; i < lowCap; i++ {
		_ = l.Allow()
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = l.Allow()
	}
}

func BenchmarkJujuRejectPath(b *testing.B) {
	l := ratelimit.NewBucketWithRate(lowRate, lowCap)
	for i := 0; i < lowCap; i++ {
		_ = l.TakeAvailable(1)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = l.TakeAvailable(1)
	}
}
