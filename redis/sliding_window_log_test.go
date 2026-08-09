package redis

import (
	"context"
	"testing"
	"time"

	"github.com/themarchrain/kaka"
)

func TestSlidingWindowBasic(t *testing.T) {
	client := testClient(t)
	sw := NewSlidingWindow(client, 2, time.Minute, WithKeyPrefix(testKeyPrefix))

	res, err := sw.Allow(context.Background(), "user:1")
	if err != nil || !res.Allowed {
		t.Fatalf("first allow: allowed=%v err=%v", res.Allowed, err)
	}
	if res.Remaining != 1 {
		t.Errorf("remaining = %d, want 1", res.Remaining)
	}
	res, err = sw.Allow(context.Background(), "user:1")
	if err != nil || !res.Allowed {
		t.Fatalf("second allow: allowed=%v err=%v", res.Allowed, err)
	}
	if res.Remaining != 0 {
		t.Errorf("remaining = %d, want 0", res.Remaining)
	}
	res, err = sw.Allow(context.Background(), "user:1")
	if err != nil {
		t.Fatalf("third allow err: %v", err)
	}
	if res.Allowed {
		t.Fatal("third allow should be denied")
	}
	if res.Remaining != 0 {
		t.Errorf("denied remaining = %d, want 0", res.Remaining)
	}
	if res.RetryAfter <= 0 || res.RetryAfter > time.Minute {
		t.Errorf("denied retryAfter = %v, want in (0, window]", res.RetryAfter)
	}
}

func TestSlidingWindowKeyIsolation(t *testing.T) {
	client := testClient(t)
	sw := NewSlidingWindow(client, 1, time.Minute, WithKeyPrefix(testKeyPrefix))

	if res, _ := sw.Allow(context.Background(), "a"); !res.Allowed {
		t.Fatal("key a first allow should be allowed")
	}
	if res, _ := sw.Allow(context.Background(), "b"); !res.Allowed {
		t.Fatal("key b first allow should be allowed (isolated)")
	}
	if res, _ := sw.Allow(context.Background(), "a"); res.Allowed {
		t.Fatal("key a second allow should be denied")
	}
}

func TestSlidingWindowImplementsContract(t *testing.T) {
	var _ kaka.Limiter = (*SlidingWindow)(nil)
}

func TestSlidingWindowInvalidParams(t *testing.T) {
	client := testClient(t)
	for _, c := range []struct {
		limit  int
		window time.Duration
	}{
		{0, time.Minute}, {-1, time.Minute},
		{1, 0}, {1, -time.Minute},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("NewSlidingWindow(%d, %v) should panic", c.limit, c.window)
				}
			}()
			NewSlidingWindow(client, c.limit, c.window)
		}()
	}
}

func TestSlidingWindowBlankKeyError(t *testing.T) {
	client := testClient(t)
	sw := NewSlidingWindow(client, 1, time.Minute, WithKeyPrefix(testKeyPrefix))
	for _, key := range []string{"", "   "} {
		if _, err := sw.Allow(context.Background(), key); err == nil {
			t.Errorf("Allow(%q) should error", key)
		}
	}
}

func TestSlidingWindowZSetState(t *testing.T) {
	client := testClient(t)
	sw := NewSlidingWindow(client, 3, time.Minute, WithKeyPrefix(testKeyPrefix))

	for i := 0; i < 3; i++ {
		if _, err := sw.Allow(context.Background(), "k1"); err != nil {
			t.Fatalf("allow %d: %v", i, err)
		}
	}
	// 校验 ZSet 中有 3 个唯一 member（member 唯一性：同毫秒并发不覆盖）
	count, err := client.ZCard(context.Background(), testKeyPrefix+"k1").Result()
	if err != nil {
		t.Fatalf("ZCard: %v", err)
	}
	if count != 3 {
		t.Errorf("ZCard = %d, want 3 (unique members)", count)
	}
}

func TestSlidingWindowSlides(t *testing.T) {
	client := testClient(t)
	// limit=1, window=300ms：窗口滑出后恢复
	sw := NewSlidingWindow(client, 1, 300*time.Millisecond, WithKeyPrefix(testKeyPrefix))

	if res, _ := sw.Allow(context.Background(), "k1"); !res.Allowed {
		t.Fatal("first allow should be allowed")
	}
	if res, _ := sw.Allow(context.Background(), "k1"); res.Allowed {
		t.Fatal("second allow should be denied (inside window)")
	}

	time.Sleep(350 * time.Millisecond) // 等窗口滑出
	res, err := sw.Allow(context.Background(), "k1")
	if err != nil {
		t.Fatalf("allow after slide: %v", err)
	}
	if !res.Allowed {
		t.Fatal("allow after window slide should be allowed")
	}
}

func TestSlidingWindowTTLReset(t *testing.T) {
	client := testClient(t)
	sw := NewSlidingWindow(client, 1, time.Minute,
		WithKeyTTL(1*time.Second), WithKeyPrefix(testKeyPrefix))

	if res, _ := sw.Allow(context.Background(), "ttl-key"); !res.Allowed {
		t.Fatal("first allow should be allowed")
	}
	if res, _ := sw.Allow(context.Background(), "ttl-key"); res.Allowed {
		t.Fatal("second allow should be denied")
	}

	time.Sleep(1200 * time.Millisecond) // 等 TTL 过期
	res, err := sw.Allow(context.Background(), "ttl-key")
	if err != nil {
		t.Fatalf("allow after ttl: %v", err)
	}
	if !res.Allowed {
		t.Fatal("allow after TTL expiry should be allowed (key recreated)")
	}
}

func TestSlidingWindowDeletedKeyResets(t *testing.T) {
	client := testClient(t)
	sw := NewSlidingWindow(client, 1, time.Minute, WithKeyPrefix(testKeyPrefix))

	if _, err := sw.Allow(context.Background(), "del-key"); err != nil {
		t.Fatalf("first allow: %v", err)
	}
	client.Del(context.Background(), testKeyPrefix+"del-key")

	res, err := sw.Allow(context.Background(), "del-key")
	if err != nil {
		t.Fatalf("allow after delete: %v", err)
	}
	if !res.Allowed {
		t.Fatal("allow after DEL should be allowed (recreated)")
	}
}

func TestSlidingWindowRetryAfterBoundary(t *testing.T) {
	client := testClient(t)
	sw := NewSlidingWindow(client, 1, 500*time.Millisecond, WithKeyPrefix(testKeyPrefix))

	if _, err := sw.Allow(context.Background(), "k1"); err != nil {
		t.Fatalf("first allow: %v", err)
	}
	time.Sleep(100 * time.Millisecond) // 让最早条目"变老" 100ms

	res, err := sw.Allow(context.Background(), "k1")
	if err != nil {
		t.Fatalf("second allow: %v", err)
	}
	if res.Allowed {
		t.Fatal("second allow should be denied")
	}
	// RetryAfter 应 ≈ 400ms（500 - 100），允许少量误差
	if res.RetryAfter <= 0 || res.RetryAfter > 500*time.Millisecond {
		t.Errorf("retryAfter = %v, want in (0, 500ms]", res.RetryAfter)
	}
	if res.RetryAfter < 300*time.Millisecond {
		t.Errorf("retryAfter = %v, want >= ~300ms (oldest entry is ~100ms old)", res.RetryAfter)
	}
}
