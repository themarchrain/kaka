package compare

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/themarchrain/kaka"
	"github.com/themarchrain/kaka/layered"
	"github.com/themarchrain/kaka/memory"
	redislimiter "github.com/themarchrain/kaka/redis"
)

// fixedLimiter is a kaka.Limiter stub that always allows; used as the
// remote layer when measuring the local short-circuit path.
type fixedLimiter struct{}

func (fixedLimiter) Allow(context.Context, string) (kaka.Result, error) {
	return kaka.Result{Allowed: true, Remaining: 1}, nil
}

// BenchmarkLayeredLocalReject measures the local short-circuit path: the
// local bucket is drained, so every call is denied without a network round
// trip. Compare with BenchmarkKakaTokenBucketRejectPath (pure memory) and
// BenchmarkRedisTokenBucket (pure Redis).
func BenchmarkLayeredLocalReject(b *testing.B) {
	ctx := context.Background()
	limiter := layered.New(memory.NewTokenBucket(1, 1), fixedLimiter{})
	if _, err := limiter.Allow(ctx, "bench:layered-reject"); err != nil { // drain local
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := limiter.Allow(ctx, "bench:layered-reject"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLayeredLocalRejectParallel is the parallel variant of
// BenchmarkLayeredLocalReject.
func BenchmarkLayeredLocalRejectParallel(b *testing.B) {
	ctx := context.Background()
	limiter := layered.New(memory.NewTokenBucket(1, 1), fixedLimiter{})
	if _, err := limiter.Allow(ctx, "bench:layered-reject"); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := limiter.Allow(ctx, "bench:layered-reject"); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkLayeredAllowViaRedis measures the full allow path: local allow
// plus one Redis round trip for the authoritative decision. Compare with
// BenchmarkRedisTokenBucket; the layered overhead must stay small (spec P3).
func BenchmarkLayeredAllowViaRedis(b *testing.B) {
	client := benchRedisClient(b)
	defer client.Close()
	ctx := context.Background()
	limiter := layered.New(
		memory.NewTokenBucket(1e9, 1e9),
		redislimiter.NewTokenBucket(client, 1e9, 1e9, redislimiter.WithKeyTTL(0)),
	)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := limiter.Allow(ctx, "bench:layered-allow"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLayeredRejectHeavyViaRedis measures the deny-heavy workload:
// both layers are drained, so every call is denied — locally for the
// layered limiter, with a Redis round trip for the pure Redis limiter.
func BenchmarkLayeredRejectHeavyViaRedis(b *testing.B) {
	client := benchRedisClient(b)
	defer client.Close()
	ctx := context.Background()
	limiter := layered.New(
		memory.NewTokenBucket(1, 1),
		redislimiter.NewTokenBucket(client, 1, 1, redislimiter.WithKeyTTL(0)),
	)
	if _, err := limiter.Allow(ctx, "bench:layered-reject"); err != nil { // drain both layers
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := limiter.Allow(ctx, "bench:layered-reject"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRedisRejectHeavy measures the same deny-heavy workload on a pure
// Redis limiter: every denial costs a round trip.
func BenchmarkRedisRejectHeavy(b *testing.B) {
	client := benchRedisClient(b)
	defer client.Close()
	ctx := context.Background()
	limiter := redislimiter.NewTokenBucket(client, 1, 1, redislimiter.WithKeyTTL(0))
	if _, err := limiter.Allow(ctx, "bench:redis-reject"); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := limiter.Allow(ctx, "bench:redis-reject"); err != nil {
			b.Fatal(err)
		}
	}
}

func benchRedisClient(b *testing.B) *redis.Client {
	b.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		b.Skipf("redis not reachable at %s: %v", addr, err)
	}
	return client
}
