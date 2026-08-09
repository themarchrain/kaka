package redis

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
)

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
	t.Cleanup(func() { client.Close() })
	return client
}

func TestScriptLoadAndRun(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	s := NewScript("time_probe", "return tonumber(redis.call('TIME')[1])")
	if err := s.Load(ctx, client); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.sha == "" {
		t.Fatal("sha not cached after Load")
	}

	res, err := s.Run(ctx, client, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// TIME 返回秒级 Unix 时间，应 > 0
	if sec, ok := res.(int64); !ok || sec <= 0 {
		t.Errorf("TIME result = %v (%T), want positive int64", res, res)
	}
}

func TestScriptNoScriptFallback(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	s := NewScript("time_probe2", "return tonumber(redis.call('TIME')[1])")
	s.sha = "deadbeef00000000000000000000000000000000" // 伪造错误 SHA 触发 NOSCRIPT
	res, err := s.Run(ctx, client, nil)
	if err != nil {
		t.Fatalf("Run with stale sha: %v", err)
	}
	if sec, ok := res.(int64); !ok || sec <= 0 {
		t.Errorf("TIME result = %v (%T), want positive int64", res, res)
	}
}

func TestScriptRunAgainstDeadClient(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()
	s := NewScript("time_probe3", "return tonumber(redis.call('TIME')[1])")
	client.Close() // 制造网络错误

	if _, err := s.Run(ctx, client, nil); err == nil {
		t.Fatal("expected error from closed client")
	}
}
