package compare

import (
	"context"
	"fmt"
	"testing"

	"github.com/themarchrain/kaka/memory"
)

// 基线：EvictReject（默认）+ maxKeys——当前行为，作为两分支对比基准
func BenchmarkKakaTokenBucketMaxKeysReject(b *testing.B) {
	ctx := context.Background()
	limiter := memory.NewTokenBucket(1000, 10, memory.WithMaxKeys(10000))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = limiter.Allow(ctx, fmt.Sprintf("user:%d", i%10000))
	}
}

func BenchmarkKakaTokenBucketMaxKeysRejectParallel(b *testing.B) {
	ctx := context.Background()
	limiter := memory.NewTokenBucket(1000, 10, memory.WithMaxKeys(10000))

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, _ = limiter.Allow(ctx, fmt.Sprintf("user:%d", i%10000))
			i++
		}
	})
}

// EvictLRU 场景（分支实验用；最终合并时只保留胜者对应实现）
func BenchmarkKakaTokenBucketLRUEviction(b *testing.B) {
	ctx := context.Background()
	limiter := memory.NewTokenBucket(1000, 10,
		memory.WithMaxKeys(10000),
		memory.WithEvictionPolicy(memory.EvictLRU))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = limiter.Allow(ctx, fmt.Sprintf("user:%d", i%10000))
	}
}

// 淘汰压力：maxKeys 小 + 持续新 key，每次插入都可能触发淘汰
func BenchmarkKakaTokenBucketLRUThrash(b *testing.B) {
	ctx := context.Background()
	limiter := memory.NewTokenBucket(1000, 10,
		memory.WithMaxKeys(100),
		memory.WithEvictionPolicy(memory.EvictLRU))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = limiter.Allow(ctx, fmt.Sprintf("user:%d", i))
	}
}

func BenchmarkKakaTokenBucketLRUEvictionParallel(b *testing.B) {
	ctx := context.Background()
	limiter := memory.NewTokenBucket(1000, 10,
		memory.WithMaxKeys(10000),
		memory.WithEvictionPolicy(memory.EvictLRU))

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, _ = limiter.Allow(ctx, fmt.Sprintf("user:%d", i%10000))
			i++
		}
	})
}
