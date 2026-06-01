package memory

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type benchmarkClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *benchmarkClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *benchmarkClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func BenchmarkTokenBucketAllowSingleKey(b *testing.B) {
	ctx := context.Background()
	limiter := NewTokenBucket(float64(b.N)+1, 1)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := limiter.Allow(ctx, "user:1")
		if err != nil {
			b.Fatal(err)
		}
		_ = result.Allowed
	}
}

func BenchmarkLeakyBucketAllowSingleKey(b *testing.B) {
	ctx := context.Background()
	limiter := NewLeakyBucket(float64(b.N)+1, 1)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := limiter.Allow(ctx, "user:1")
		if err != nil {
			b.Fatal(err)
		}
		_ = result.Allowed
	}
}

func BenchmarkSlidingWindowAllowSingleKey(b *testing.B) {
	ctx := context.Background()
	clock := &benchmarkClock{now: time.Unix(100, 0)}
	limiter := NewSlidingWindow(1024, time.Second, withClock(clock))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := limiter.Allow(ctx, "user:1")
		if err != nil {
			b.Fatal(err)
		}
		_ = result.Allowed
		if i%1024 == 1023 {
			clock.Advance(2 * time.Second)
		}
	}
}

func BenchmarkTokenBucketAllowManyKeys(b *testing.B) {
	ctx := context.Background()
	limiter := NewTokenBucket(100, 1)
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = fmt.Sprintf("user:%d", i)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := limiter.Allow(ctx, keys[i%len(keys)])
		if err != nil {
			b.Fatal(err)
		}
		_ = result.Allowed
	}
}

func BenchmarkLeakyBucketAllowManyKeys(b *testing.B) {
	ctx := context.Background()
	limiter := NewLeakyBucket(100, 1)
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = fmt.Sprintf("user:%d", i)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := limiter.Allow(ctx, keys[i%len(keys)])
		if err != nil {
			b.Fatal(err)
		}
		_ = result.Allowed
	}
}

func BenchmarkSlidingWindowAllowManyKeys(b *testing.B) {
	ctx := context.Background()
	clock := &benchmarkClock{now: time.Unix(100, 0)}
	limiter := NewSlidingWindow(1024, time.Second, withClock(clock))
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = fmt.Sprintf("user:%d", i)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := limiter.Allow(ctx, keys[i%len(keys)])
		if err != nil {
			b.Fatal(err)
		}
		_ = result.Allowed
		if i%1024 == 1023 {
			clock.Advance(2 * time.Second)
		}
	}
}

func BenchmarkSlidingWindowAllowWithCompaction(b *testing.B) {
	ctx := context.Background()
	clock := &benchmarkClock{now: time.Unix(100, 0)}
	limiter := NewSlidingWindow(128, 64*time.Millisecond, withClock(clock))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := limiter.Allow(ctx, "user:1")
		if err != nil {
			b.Fatal(err)
		}
		_ = result.Allowed
		clock.Advance(time.Millisecond)
	}
}

func BenchmarkTokenBucketAllowParallel(b *testing.B) {
	ctx := context.Background()
	limiter := NewTokenBucket(float64(b.N)+1, 1)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result, err := limiter.Allow(ctx, "user:1")
			if err != nil {
				panic(err)
			}
			_ = result.Allowed
		}
	})
}

func BenchmarkLeakyBucketAllowParallel(b *testing.B) {
	ctx := context.Background()
	limiter := NewLeakyBucket(float64(b.N)+1, 1)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result, err := limiter.Allow(ctx, "user:1")
			if err != nil {
				panic(err)
			}
			_ = result.Allowed
		}
	})
}

func BenchmarkMapStoreGetExisting(b *testing.B) {
	now := time.Unix(100, 0)
	store := newMapStore[*bucket](defaultOptions())
	state, err := store.getOrCreate("user:1", now, func(now time.Time) *bucket {
		return &bucket{tokens: 1, lastRefilled: now}
	})
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		got, err := store.getOrCreate("user:1", now, func(now time.Time) *bucket {
			return &bucket{tokens: 2, lastRefilled: now}
		})
		if err != nil {
			b.Fatal(err)
		}
		if got != state {
			b.Fatal("expected existing state")
		}
		_ = got
	}
}

func BenchmarkMapStoreCreateNewKey(b *testing.B) {
	now := time.Unix(100, 0)
	store := newMapStore[*bucket](defaultOptions())

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		got, err := store.getOrCreate(fmt.Sprintf("user:%d", i), now, func(now time.Time) *bucket {
			return &bucket{tokens: 1, lastRefilled: now}
		})
		if err != nil {
			b.Fatal(err)
		}
		_ = got
	}
}

func BenchmarkMapStoreRejectNewKeyWhenFull(b *testing.B) {
	now := time.Unix(100, 0)
	store := newMapStore[*bucket](options{maxKeys: 1})
	_, err := store.getOrCreate("user:1", now, func(now time.Time) *bucket {
		return &bucket{tokens: 1, lastRefilled: now}
	})
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := store.getOrCreate("user:2", now, func(now time.Time) *bucket {
			return &bucket{tokens: 1, lastRefilled: now}
		})
		if err != ErrMaxKeysExceeded {
			b.Fatalf("expected ErrMaxKeysExceeded, got %v", err)
		}
		_ = err
	}
}

func BenchmarkMapStoreCleanupScan(b *testing.B) {
	for _, size := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			now := time.Unix(100, 0)
			store := newMapStore[*bucket](options{
				maxKeys:         size + 1,
				keyTTL:          time.Hour,
				cleanupInterval: time.Nanosecond,
			})
			for i := 0; i < size; i++ {
				store.items[fmt.Sprintf("seed:%d", i)] = &keyEntry[*bucket]{
					value:    &bucket{tokens: 1, lastRefilled: now},
					lastSeen: now,
				}
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				store.cleanup(now.Add(time.Duration(i) * time.Nanosecond))
			}
		})
	}
}
