package compare

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/themarchrain/kaka/memory"
	"github.com/ulule/limiter/v3"
	mstore "github.com/ulule/limiter/v3/drivers/store/memory"
)

var memStats runtime.MemStats

// measureHeapAlloc 执行 fn 后，GC 一次并返回 HeapAlloc 增量。
// 通过 runtime.GC 后采样并取增量，降低 GC 时序噪音。
func measureHeapAlloc(fn func()) uint64 {
	// 首次预热（丢弃），规避首次分配/运行时初始化噪音
	func() {
		runtime.GC()
		runtime.ReadMemStats(&memStats)
	}()
	fn()
	runtime.GC()
	runtime.ReadMemStats(&memStats)
	return memStats.HeapAlloc
}

// TestMemoryFootprintPerKey 对比 Kaka 与 ulule 在不同 key 基数下的内存占用（每 key 均摊）。
// 语义注明：ulule 为计数器式限流，本对比是 per-key 存储/内存机制对比，非算法吞吐对比。
func TestMemoryFootprintPerKey(t *testing.T) {
	ctx := context.Background()
	for _, n := range []int{100, 10000, 100000} {
		kakaMem := measureHeapAlloc(func() {
			limiter := memory.NewTokenBucket(100, 10,
				memory.WithMaxKeys(int(n+1000)),
				memory.WithKeyTTL(time.Hour),
			)
			for i := 0; i < n; i++ {
				_, _ = limiter.Allow(ctx, fmt.Sprintf("user:%d", i))
			}
		})

		store := mstore.NewStore()
		rate := limiter.Rate{Period: time.Minute, Limit: 100}
		instance := limiter.New(store, rate)
		ululeMem := measureHeapAlloc(func() {
			for i := 0; i < n; i++ {
				_, _ = instance.Get(ctx, fmt.Sprintf("user:%d", i))
			}
		})

		t.Logf("keys=%-6d  kaka=%dB (%.1f B/key)  ulule=%dB (%.1f B/key)",
			n, kakaMem, float64(kakaMem)/float64(n), ululeMem, float64(ululeMem)/float64(n))
	}
}

// measureTotalAlloc 返回 fn 期间累计分配字节（TotalAlloc 差值，单调递增，
// 不受 GC 空闲堆复用影响——适合同进程内对比两个实现的每 key 存储成本）。
func measureTotalAlloc(fn func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// TestMemoryFootprintLRU 对比同一 limiter 在默认拒绝（mapStore）与 LRU 淘汰（lruStore）
// 下的每 key 累计分配（含存储结构 + map 扩容）。
func TestMemoryFootprintLRU(t *testing.T) {
	ctx := context.Background()
	for _, n := range []int{100, 10000, 100000} {
		rejectAlloc := measureTotalAlloc(func() {
			limiter := memory.NewTokenBucket(100, 10,
				memory.WithMaxKeys(int(n+1000)),
				memory.WithKeyTTL(time.Hour),
			)
			for i := 0; i < n; i++ {
				_, _ = limiter.Allow(ctx, fmt.Sprintf("user:%d", i))
			}
		})

		lruAlloc := measureTotalAlloc(func() {
			limiter := memory.NewTokenBucket(100, 10,
				memory.WithMaxKeys(int(n+1000)),
				memory.WithKeyTTL(time.Hour),
				memory.WithEvictionPolicy(memory.EvictLRU),
			)
			for i := 0; i < n; i++ {
				_, _ = limiter.Allow(ctx, fmt.Sprintf("user:%d", i))
			}
		})

		t.Logf("keys=%-6d  mapStore=%dB (%.1f B/key)  lruStore=%dB (%.1f B/key)  diff=+%dB/key",
			n, rejectAlloc, float64(rejectAlloc)/float64(n), lruAlloc, float64(lruAlloc)/float64(n), (lruAlloc-rejectAlloc)/uint64(n))
	}
}

// BenchmarkKakaTokenBucketHotPath 与 BenchmarkUluleHotPath 对比热路径分配率。
func BenchmarkKakaTokenBucketHotPath(b *testing.B) {
	ctx := context.Background()
	l := memory.NewTokenBucket(100, 10)
	_, _ = l.Allow(ctx, "user:1")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = l.Allow(ctx, "user:1")
	}
}

func BenchmarkUluleHotPath(b *testing.B) {
	ctx := context.Background()
	store := mstore.NewStore()
	rate := limiter.Rate{Period: time.Minute, Limit: 1000000}
	instance := limiter.New(store, rate)
	_, _ = instance.Get(ctx, "user:1")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = instance.Get(ctx, "user:1")
	}
}
