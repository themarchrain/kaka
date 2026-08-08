package compare

import (
	"context"
	"testing"
	"time"

	"github.com/themarchrain/kaka/memory"
)

// 三家对标库均无滑动窗口日志实现，仅报告 Kaka 自身基线。

func BenchmarkKakaSlidingWindowSingleKey(b *testing.B) {
	ctx := context.Background()
	l := memory.NewSlidingWindow(lowCap, time.Minute)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = l.Allow(ctx, "user:1")
	}
}

func BenchmarkKakaSlidingWindowManyKeys(b *testing.B) {
	ctx := context.Background()
	l := memory.NewSlidingWindow(lowCap, time.Minute)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = l.Allow(ctx, keys(i))
	}
}

// 窗口内日志接近上限时的压缩成本
func BenchmarkKakaSlidingWindowCompaction(b *testing.B) {
	ctx := context.Background()
	l := memory.NewSlidingWindow(lowCap, time.Second)
	// 预填窗口到上限
	for i := 0; i < lowCap; i++ {
		_, _ = l.Allow(ctx, "user:1")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = l.Allow(ctx, "user:1")
	}
}
