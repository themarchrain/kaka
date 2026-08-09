package compare

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
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
