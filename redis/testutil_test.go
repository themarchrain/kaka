package redis

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
)

// testKeyPrefix 是测试专用 key 前缀，运行结束统一清理。
const testKeyPrefix = "kaka:test:"

// testClient 连接真实 Redis（REDIS_ADDR 环境变量，默认 127.0.0.1:6379）。
// 连不上直接 Fatal（不静默跳过），保证测试真实性。
func testClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("cannot connect to redis at %s (set REDIS_ADDR): %v", addr, err)
	}
	t.Cleanup(func() {
		cleanupTestKeys(t, client)
		client.Close()
	})
	return client
}

// cleanupTestKeys 删除本包测试写入的 kaka:test:* key（SCAN 迭代删除）。
func cleanupTestKeys(t *testing.T, client *redis.Client) {
	t.Helper()
	ctx := context.Background()
	var cursor uint64
	for {
		keys, next, err := client.Scan(ctx, cursor, testKeyPrefix+"*", 100).Result()
		if err != nil {
			t.Logf("cleanup scan: %v", err)
			return
		}
		if len(keys) > 0 {
			if err := client.Del(ctx, keys...).Err(); err != nil {
				t.Logf("cleanup del: %v", err)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
}

// mustGetHash 断言 Hash 字段值，便于测试校验状态。
func mustGetHash(t *testing.T, client *redis.Client, key string) map[string]string {
	t.Helper()
	vals, err := client.HGetAll(context.Background(), key).Result()
	if err != nil {
		t.Fatalf("HGetAll %s: %v", key, err)
	}
	return vals
}
