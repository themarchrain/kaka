package compare

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/themarchrain/kaka"
	redislimiter "github.com/themarchrain/kaka/redis"
)

// BenchmarkRedisTokenBucket 测量 Redis TokenBucket 单请求延迟（EVALSHA 单往返）。
// 需要真实 Redis：REDIS_ADDR 环境变量（默认 127.0.0.1:6379），连不上则跳过。
func BenchmarkRedisTokenBucket(b *testing.B) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		b.Skipf("redis not reachable at %s: %v", addr, err)
	}
	defer client.Close()

	tb := redislimiter.NewTokenBucket(client, 1000000, 1000000, redislimiter.WithKeyTTL(0))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := tb.Allow(ctx, "bench:key"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRedisTokenBucketParallel 并发版（测量吞吐上限）。
func BenchmarkRedisTokenBucketParallel(b *testing.B) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr, MaxRetries: 0})
	if err := client.Ping(context.Background()).Err(); err != nil {
		b.Skipf("redis not reachable at %s: %v", addr, err)
	}
	defer client.Close()

	tb := redislimiter.NewTokenBucket(client, 1000000, 1000000, redislimiter.WithKeyTTL(0))
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := tb.Allow(ctx, "bench:key"); err != nil {
				b.Fatal(err)
			}
		}
	})
}

var _ = time.Second

// redisLimiter 是最小接口：三算法（TokenBucket/SlidingWindow/LeakyBucket）
// 都满足 Allow(ctx, key) (kaka.Result, error)。
type redisLimiter interface {
	Allow(ctx context.Context, key string) (kaka.Result, error)
}

// sweepRedisConcurrency 并发扫描：GOMAXPROCS=1 + SetParallelism 精确控制
// goroutine 数，PoolSize 给足（512）让 Redis 单实例成为唯一瓶颈。
// 输出：各并发档的聚合吞吐（1e9/nsPerOp = QPS）。
func sweepRedisConcurrency(b *testing.B, name string, newLimiter func(*redis.Client) redisLimiter) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr, MaxRetries: 0, PoolSize: 512})
	if err := client.Ping(context.Background()).Err(); err != nil {
		b.Skipf("redis not reachable at %s: %v", addr, err)
	}
	defer client.Close()

	ctx := context.Background()
	for _, p := range []int{1, 8, 32, 64, 128, 256} {
		p := p
		b.Run(fmt.Sprintf("%s/parallel-%d", name, p), func(b *testing.B) {
			prev := runtime.GOMAXPROCS(1)
			defer runtime.GOMAXPROCS(prev)

			lim := newLimiter(client)
			b.SetParallelism(p)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if _, err := lim.Allow(ctx, "bench:key"); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

// BenchmarkRedisConcurrencySweep 测三个算法各自的单实例并发上限。
func BenchmarkRedisConcurrencySweep(b *testing.B) {
	sweepRedisConcurrency(b, "tb", func(c *redis.Client) redisLimiter {
		return redislimiter.NewTokenBucket(c, 1000000, 1000000, redislimiter.WithKeyTTL(0))
	})
	sweepRedisConcurrency(b, "sw", func(c *redis.Client) redisLimiter {
		return redislimiter.NewSlidingWindow(c, 1000000, time.Hour, redislimiter.WithKeyTTL(0))
	})
	sweepRedisConcurrency(b, "lb", func(c *redis.Client) redisLimiter {
		return redislimiter.NewLeakyBucket(c, 1000000, 1000000, redislimiter.WithKeyTTL(0))
	})
}

// BenchmarkRedisSlidingWindow 测量 Redis SlidingWindow 单请求延迟。
func BenchmarkRedisSlidingWindow(b *testing.B) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		b.Skipf("redis not reachable at %s: %v", addr, err)
	}
	defer client.Close()

	sw := redislimiter.NewSlidingWindow(client, 1000000, time.Minute, redislimiter.WithKeyTTL(0))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sw.Allow(ctx, "bench:key"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRedisSlidingWindowParallel 并发版。
func BenchmarkRedisSlidingWindowParallel(b *testing.B) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr, MaxRetries: 0})
	if err := client.Ping(context.Background()).Err(); err != nil {
		b.Skipf("redis not reachable at %s: %v", addr, err)
	}
	defer client.Close()

	sw := redislimiter.NewSlidingWindow(client, 1000000, time.Minute, redislimiter.WithKeyTTL(0))
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := sw.Allow(ctx, "bench:key"); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkRedisLeakyBucket 测量 Redis LeakyBucket 单请求延迟。
func BenchmarkRedisLeakyBucket(b *testing.B) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		b.Skipf("redis not reachable at %s: %v", addr, err)
	}
	defer client.Close()

	lb := redislimiter.NewLeakyBucket(client, 1000000, 1000000, redislimiter.WithKeyTTL(0))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := lb.Allow(ctx, "bench:key"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRedisLeakyBucketParallel 并发版。
func BenchmarkRedisLeakyBucketParallel(b *testing.B) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr, MaxRetries: 0})
	if err := client.Ping(context.Background()).Err(); err != nil {
		b.Skipf("redis not reachable at %s: %v", addr, err)
	}
	defer client.Close()

	lb := redislimiter.NewLeakyBucket(client, 1000000, 1000000, redislimiter.WithKeyTTL(0))
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := lb.Allow(ctx, "bench:key"); err != nil {
				b.Fatal(err)
			}
		}
	})
}
