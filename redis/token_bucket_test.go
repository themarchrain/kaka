package redis

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/themarchrain/kaka"
)

func TestTokenBucketBasic(t *testing.T) {
	client := testClient(t)
	tb := NewTokenBucket(client, 2, 1, WithKeyPrefix(testKeyPrefix))

	res, err := tb.Allow(context.Background(), "user:1")
	if err != nil || !res.Allowed {
		t.Fatalf("first allow: allowed=%v err=%v, want allowed", res.Allowed, err)
	}
	if res.Remaining != 1 {
		t.Errorf("remaining = %d, want 1", res.Remaining)
	}
	res, err = tb.Allow(context.Background(), "user:1")
	if err != nil || !res.Allowed {
		t.Fatalf("second allow: allowed=%v err=%v, want allowed", res.Allowed, err)
	}
	if res.Remaining != 0 {
		t.Errorf("remaining = %d, want 0", res.Remaining)
	}
	res, err = tb.Allow(context.Background(), "user:1")
	if err != nil {
		t.Fatalf("third allow err: %v", err)
	}
	if res.Allowed {
		t.Fatal("third allow should be denied")
	}
	if res.Remaining != 0 {
		t.Errorf("denied remaining = %d, want 0", res.Remaining)
	}
	if res.RetryAfter <= 0 {
		t.Errorf("denied retryAfter = %v, want > 0", res.RetryAfter)
	}
}

func TestTokenBucketKeyIsolation(t *testing.T) {
	client := testClient(t)
	tb := NewTokenBucket(client, 1, 1, WithKeyPrefix(testKeyPrefix))

	if res, _ := tb.Allow(context.Background(), "a"); !res.Allowed {
		t.Fatal("key a first allow should be allowed")
	}
	if res, _ := tb.Allow(context.Background(), "b"); !res.Allowed {
		t.Fatal("key b first allow should be allowed (isolated)")
	}
	if res, _ := tb.Allow(context.Background(), "a"); res.Allowed {
		t.Fatal("key a second allow should be denied")
	}
}

func TestTokenBucketImplementsContract(t *testing.T) {
	var _ kaka.Limiter = (*TokenBucket)(nil)
}

func TestTokenBucketInvalidParams(t *testing.T) {
	client := testClient(t)
	for _, c := range []struct {
		capacity, rate float64
	}{
		{0, 1}, {0.5, 1}, {-1, 1},
		{1, 0}, {1, -1},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("NewTokenBucket(%v, %v) should panic", c.capacity, c.rate)
				}
			}()
			NewTokenBucket(client, c.capacity, c.rate)
		}()
	}
}

func TestTokenBucketBlankKeyError(t *testing.T) {
	client := testClient(t)
	tb := NewTokenBucket(client, 1, 1, WithKeyPrefix(testKeyPrefix))
	for _, key := range []string{"", "   "} {
		if _, err := tb.Allow(context.Background(), key); err == nil {
			t.Errorf("Allow(%q) should error", key)
		}
	}
}

func TestTokenBucketPrefixApplied(t *testing.T) {
	client := testClient(t)
	tb := NewTokenBucket(client, 1, 1, WithKeyPrefix("kaka:test:tb:"))
	if _, err := tb.Allow(context.Background(), "k1"); err != nil {
		t.Fatalf("allow: %v", err)
	}
	// 校验 Redis 中实际 key 存在（前缀拼接正确）
	vals := mustGetHash(t, client, "kaka:test:tb:k1")
	if _, ok := vals["tokens"]; !ok {
		t.Errorf("hash fields = %v, want tokens field", vals)
	}
	if strings.HasPrefix("kaka:test:tb:k1", testKeyPrefix) == false {
		t.Error("test key must use testKeyPrefix for cleanup")
	}
}

func TestTokenBucketRefillOverTime(t *testing.T) {
	client := testClient(t)
	// rate = 4/s，等 ~300ms 应补充 1+ 个令牌
	tb := NewTokenBucket(client, 4, 4, WithKeyPrefix(testKeyPrefix))

	for i := 0; i < 4; i++ {
		if res, _ := tb.Allow(context.Background(), "k1"); !res.Allowed {
			t.Fatalf("allow %d should be allowed (capacity)", i)
		}
	}
	if res, _ := tb.Allow(context.Background(), "k1"); res.Allowed {
		t.Fatal("5th allow should be denied (bucket empty)")
	}

	time.Sleep(300 * time.Millisecond)
	res, err := tb.Allow(context.Background(), "k1")
	if err != nil {
		t.Fatalf("allow after refill: %v", err)
	}
	if !res.Allowed {
		t.Fatal("allow after 300ms should be allowed (refill ~1.2 tokens)")
	}
}

func TestTokenBucketTTLReset(t *testing.T) {
	client := testClient(t)
	tb := NewTokenBucket(client, 1, 1, WithKeyTTL(1*time.Second), WithKeyPrefix(testKeyPrefix))

	if res, _ := tb.Allow(context.Background(), "ttl-key"); !res.Allowed {
		t.Fatal("first allow should be allowed")
	}
	if res, _ := tb.Allow(context.Background(), "ttl-key"); res.Allowed {
		t.Fatal("second allow should be denied")
	}

	time.Sleep(1200 * time.Millisecond) // 等 TTL 过期
	res, err := tb.Allow(context.Background(), "ttl-key")
	if err != nil {
		t.Fatalf("allow after ttl: %v", err)
	}
	if !res.Allowed {
		t.Fatal("allow after TTL expiry should be allowed (key recreated full)")
	}
}

func TestTokenBucketDeletedKeyResets(t *testing.T) {
	client := testClient(t)
	tb := NewTokenBucket(client, 1, 1, WithKeyPrefix(testKeyPrefix))

	if _, err := tb.Allow(context.Background(), "del-key"); err != nil {
		t.Fatalf("first allow: %v", err)
	}
	client.Del(context.Background(), testKeyPrefix+"del-key")

	res, err := tb.Allow(context.Background(), "del-key")
	if err != nil {
		t.Fatalf("allow after delete: %v", err)
	}
	if !res.Allowed {
		t.Fatal("allow after DEL should be allowed (recreated full bucket)")
	}
}

func TestTokenBucketClockRollback(t *testing.T) {
	client := testClient(t)
	tb := NewTokenBucket(client, 1, 1, WithKeyPrefix(testKeyPrefix))

	if _, err := tb.Allow(context.Background(), "k1"); err != nil {
		t.Fatalf("first allow: %v", err)
	}
	// 模拟时钟回拨：把状态时间改到未来 5 秒
	future := time.Now().UnixMilli() + 5000
	if err := client.HSet(context.Background(), testKeyPrefix+"k1", "last_refilled_ms", future).Err(); err != nil {
		t.Fatalf("hset: %v", err)
	}

	res, err := tb.Allow(context.Background(), "k1")
	if err != nil {
		t.Fatalf("allow after rollback: %v", err)
	}
	if res.Allowed {
		t.Fatal("rollback should not grant extra tokens")
	}
	// 关键断言：状态时间不被改写为过去（回拨防护生效）
	got, err := client.HGet(context.Background(), testKeyPrefix+"k1", "last_refilled_ms").Result()
	if err != nil {
		t.Fatalf("hget: %v", err)
	}
	gotMs, err := strconv.ParseInt(got, 10, 64)
	if err != nil {
		t.Fatalf("parse last_refilled_ms %q: %v", got, err)
	}
	if gotMs < future {
		t.Errorf("last_refilled_ms rewritten to %d < future %d (rollback guard missing)", gotMs, future)
	}
}
