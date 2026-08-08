package compare

import (
	"context"
	"math"
	"testing"

	"github.com/themarchrain/kaka/memory"
	"go.uber.org/ratelimit"
)

// 语义差异：kaka LeakyBucket 是非阻塞 Allow（拒绝返回 Result）；
// uber-go/ratelimit 是阻塞式 Take（无容量概念）。为对比"调用开销"，
// uber 使用极高速率使 Take 几乎不等待；限流结果语义不做对齐。
// 报告必须注明该差异。

func BenchmarkKakaLeakyBucketSingleKey(b *testing.B) {
	ctx := context.Background()
	l := memory.NewLeakyBucket(lowCap, lowRate)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = l.Allow(ctx, "user:1")
	}
}

func BenchmarkKakaLeakyBucketManyKeys(b *testing.B) {
	ctx := context.Background()
	l := memory.NewLeakyBucket(lowCap, lowRate)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = l.Allow(ctx, keys(i))
	}
}

func BenchmarkUberRateLimitCallOverhead(b *testing.B) {
	// 20 亿/秒：让 Take 基本不阻塞，仅测调用开销
	l := ratelimit.New(int(math.MaxInt32))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = l.Take()
	}
}
