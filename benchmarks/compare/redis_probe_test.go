package compare

// 对照探针：定位端到端 HTTP 压测与直连 benchmark 差距的瓶颈层级。
// 纯 GET（单命令）vs EVALSHA（TokenBucket 脚本），1 并发与 64 并发。

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
	redislimiter "github.com/themarchrain/kaka/redis"
)

func newProbeClient(b *testing.B) *redis.Client {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr, MaxRetries: 0, PoolSize: 512})
	if err := client.Ping(context.Background()).Err(); err != nil {
		b.Skipf("redis not reachable at %s: %v", addr, err)
	}
	b.Cleanup(func() { client.Close() })
	return client
}

// BenchmarkRedisGetSingle 纯 GET 单命令，1 并发（裸 RTT 参考：网络往返 + 单命令执行）
func BenchmarkRedisGetSingle(b *testing.B) {
	client := newProbeClient(b)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := client.Get(ctx, "probe:key").Err(); err != nil && err != redis.Nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRedisGetParallel64 纯 GET 单命令，64 并发（网络路径 + Redis 的纯能力）
func BenchmarkRedisGetParallel64(b *testing.B) {
	client := newProbeClient(b)
	ctx := context.Background()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := client.Get(ctx, "probe:key").Err(); err != nil && err != redis.Nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkRedisEvalshaParallel64 我们的 TokenBucket 脚本，64 并发（对照）
func BenchmarkRedisEvalshaParallel64(b *testing.B) {
	client := newProbeClient(b)
	tb := redislimiter.NewTokenBucket(client, 1000000, 1000000, redislimiter.WithKeyTTL(0))
	ctx := context.Background()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := tb.Allow(ctx, "probe:key"); err != nil {
				b.Fatal(err)
			}
		}
	})
}
