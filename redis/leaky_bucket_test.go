package redis

import (
	"context"
	"testing"
	"time"

	"github.com/themarchrain/kaka"
)

func TestLeakyBucketBasic(t *testing.T) {
	client := testClient(t)
	lb := NewLeakyBucket(client, 2, 1, WithKeyPrefix(testKeyPrefix))

	res, err := lb.Allow(context.Background(), "user:1")
	if err != nil || !res.Allowed {
		t.Fatalf("first allow: allowed=%v err=%v", res.Allowed, err)
	}
	if res.Remaining != 1 {
		t.Errorf("remaining = %d, want 1", res.Remaining)
	}
	res, err = lb.Allow(context.Background(), "user:1")
	if err != nil || !res.Allowed {
		t.Fatalf("second allow: allowed=%v err=%v", res.Allowed, err)
	}
	if res.Remaining != 0 {
		t.Errorf("remaining = %d, want 0", res.Remaining)
	}
	res, err = lb.Allow(context.Background(), "user:1")
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

func TestLeakyBucketKeyIsolation(t *testing.T) {
	client := testClient(t)
	lb := NewLeakyBucket(client, 1, 1, WithKeyPrefix(testKeyPrefix))

	if res, _ := lb.Allow(context.Background(), "a"); !res.Allowed {
		t.Fatal("key a first allow should be allowed")
	}
	if res, _ := lb.Allow(context.Background(), "b"); !res.Allowed {
		t.Fatal("key b first allow should be allowed (isolated)")
	}
	if res, _ := lb.Allow(context.Background(), "a"); res.Allowed {
		t.Fatal("key a second allow should be denied")
	}
}

func TestLeakyBucketImplementsContract(t *testing.T) {
	var _ kaka.Limiter = (*LeakyBucket)(nil)
}

func TestLeakyBucketInvalidParams(t *testing.T) {
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
					t.Errorf("NewLeakyBucket(%v, %v) should panic", c.capacity, c.rate)
				}
			}()
			NewLeakyBucket(client, c.capacity, c.rate)
		}()
	}
}

func TestLeakyBucketBlankKeyError(t *testing.T) {
	client := testClient(t)
	lb := NewLeakyBucket(client, 1, 1, WithKeyPrefix(testKeyPrefix))
	for _, key := range []string{"", "   "} {
		if _, err := lb.Allow(context.Background(), key); err == nil {
			t.Errorf("Allow(%q) should error", key)
		}
	}
}

func TestLeakyBucketHashState(t *testing.T) {
	client := testClient(t)
	lb := NewLeakyBucket(client, 3, 1, WithKeyPrefix(testKeyPrefix))

	for i := 0; i < 3; i++ {
		if _, err := lb.Allow(context.Background(), "k1"); err != nil {
			t.Fatalf("allow %d: %v", i, err)
		}
	}
	vals := mustGetHash(t, client, testKeyPrefix+"k1")
	if _, ok := vals["water"]; !ok {
		t.Errorf("hash fields = %v, want water field", vals)
	}
	if _, ok := vals["last_leak_ms"]; !ok {
		t.Errorf("hash fields = %v, want last_leak_ms field", vals)
	}
}

func TestLeakyBucketLeaksOverTime(t *testing.T) {
	client := testClient(t)
	// capacity=1, rate=4/s：等 ~300ms 漏掉 1+ 滴水后恢复
	lb := NewLeakyBucket(client, 1, 4, WithKeyPrefix(testKeyPrefix))

	if res, _ := lb.Allow(context.Background(), "k1"); !res.Allowed {
		t.Fatal("first allow should be allowed")
	}
	if res, _ := lb.Allow(context.Background(), "k1"); res.Allowed {
		t.Fatal("second allow should be denied (bucket full)")
	}

	time.Sleep(300 * time.Millisecond) // 漏掉 ~1.2 滴水
	res, err := lb.Allow(context.Background(), "k1")
	if err != nil {
		t.Fatalf("allow after leak: %v", err)
	}
	if !res.Allowed {
		t.Fatal("allow after 300ms should be allowed (water leaked)")
	}
}

func TestLeakyBucketTTLReset(t *testing.T) {
	client := testClient(t)
	lb := NewLeakyBucket(client, 1, 1,
		WithKeyTTL(1*time.Second), WithKeyPrefix(testKeyPrefix))

	if res, _ := lb.Allow(context.Background(), "ttl-key"); !res.Allowed {
		t.Fatal("first allow should be allowed")
	}
	if res, _ := lb.Allow(context.Background(), "ttl-key"); res.Allowed {
		t.Fatal("second allow should be denied")
	}

	time.Sleep(1200 * time.Millisecond) // 等 TTL 过期
	res, err := lb.Allow(context.Background(), "ttl-key")
	if err != nil {
		t.Fatalf("allow after ttl: %v", err)
	}
	if !res.Allowed {
		t.Fatal("allow after TTL expiry should be allowed (key recreated empty)")
	}
}

func TestLeakyBucketDeletedKeyResets(t *testing.T) {
	client := testClient(t)
	lb := NewLeakyBucket(client, 1, 1, WithKeyPrefix(testKeyPrefix))

	if _, err := lb.Allow(context.Background(), "del-key"); err != nil {
		t.Fatalf("first allow: %v", err)
	}
	client.Del(context.Background(), testKeyPrefix+"del-key")

	res, err := lb.Allow(context.Background(), "del-key")
	if err != nil {
		t.Fatalf("allow after delete: %v", err)
	}
	if !res.Allowed {
		t.Fatal("allow after DEL should be allowed (recreated empty bucket)")
	}
}

func TestLeakyBucketRetryAfterBoundary(t *testing.T) {
	client := testClient(t)
	// capacity=1, rate=10/s：满桶后 retry ≈ 100ms
	lb := NewLeakyBucket(client, 1, 10, WithKeyPrefix(testKeyPrefix))

	if _, err := lb.Allow(context.Background(), "k1"); err != nil {
		t.Fatalf("first allow: %v", err)
	}
	res, err := lb.Allow(context.Background(), "k1")
	if err != nil {
		t.Fatalf("second allow: %v", err)
	}
	if res.Allowed {
		t.Fatal("second allow should be denied")
	}
	// overflow = 1，retry = ceil(1 / 10 * 1000) = 100ms，允许误差
	if res.RetryAfter <= 0 || res.RetryAfter > 300*time.Millisecond {
		t.Errorf("retryAfter = %v, want ~100ms", res.RetryAfter)
	}
}
