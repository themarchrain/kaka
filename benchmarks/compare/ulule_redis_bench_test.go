package compare

// ulule/limiter Redis 版权威对比：同为 Go + Redis 存储 + Lua 脚本。
// 直连对比我们的 TokenBucket 与 ulule RedisStore（RATE 固定窗口）。

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/ulule/limiter/v3"
	ululeredis "github.com/ulule/limiter/v3/drivers/store/redis"
)

func newUluleRedisLimiter(b *testing.B) *limiter.Limiter {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr, MaxRetries: 0, PoolSize: 512})
	if err := client.Ping(context.Background()).Err(); err != nil {
		b.Skipf("redis not reachable at %s: %v", addr, err)
	}
	b.Cleanup(func() { client.Close() })

	store, err := ululeredis.NewStoreWithOptions(client, limiter.StoreOptions{
		Prefix:   "ulule",
		MaxRetry: 0,
	})
	if err != nil {
		b.Fatal(err)
	}
	return limiter.New(store, limiter.Rate{Period: time.Second, Limit: 1000000})
}

// BenchmarkUluleRedisSingle ulule RedisStore 单请求延迟（对照）
func BenchmarkUluleRedisSingle(b *testing.B) {
	l := newUluleRedisLimiter(b)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := l.Get(ctx, "bench:key"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkUluleRedisParallel64 ulule RedisStore 64 并发吞吐（对照）
func BenchmarkUluleRedisParallel64(b *testing.B) {
	l := newUluleRedisLimiter(b)
	ctx := context.Background()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := l.Get(ctx, "bench:key"); err != nil {
				b.Fatal(err)
			}
		}
	})
}
