package layered

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
)

// testKeyPrefix is the test-only key prefix for layered integration tests.
const testKeyPrefix = "kaka:test:layered:"

// testClient connects to a real Redis (REDIS_ADDR env, default
// 127.0.0.1:6379) and fatals when unreachable, mirroring the redis module.
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

// cleanupTestKeys deletes keys written by this package's tests (SCAN loop).
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
