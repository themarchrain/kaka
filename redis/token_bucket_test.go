package redis

import (
	"context"
	"strings"
	"testing"

	"github.com/themarchrain/kaka"
)

func TestTokenBucketBasic(t *testing.T) {
	client := testClient(t)
	tb := NewTokenBucket(client, 2, 1)

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
	tb := NewTokenBucket(client, 1, 1)

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
	tb := NewTokenBucket(client, 1, 1)
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
