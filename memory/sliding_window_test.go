package memory

import (
	"context"
	"testing"
	"time"
)

func TestSlidingWindow_Allow(t *testing.T) {
	ctx := context.Background()
	limiter := NewSlidingWindow(5, time.Second) // 窗口内最多5个请求

	// 测试1：初始状态应该放行
	result, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("expected first request to be allowed")
	}

	// 测试2：消耗窗口内所有名额
	for i := 0; i < 4; i++ {
		result, err = limiter.Allow(ctx, "user:1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Allowed {
			t.Errorf("expected request %d to be allowed", i+2)
		}
	}

	// 测试3：窗口满了，应该限流
	result, err = limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("expected request to be rejected when window is full")
	}
}

func TestSlidingWindow_KeyIsolation(t *testing.T) {
	ctx := context.Background()
	limiter := NewSlidingWindow(3, time.Second) // 窗口内最多3个请求

	// 消耗 user:1 的所有名额
	for i := 0; i < 3; i++ {
		if _, err := limiter.Allow(ctx, "user:1"); err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
	}

	// user:1 应该被限流
	result, _ := limiter.Allow(ctx, "user:1")
	if result.Allowed {
		t.Error("expected user:1 to be rejected")
	}

	// user:2 应该正常放行
	result, _ = limiter.Allow(ctx, "user:2")
	if !result.Allowed {
		t.Error("expected user:2 to be allowed")
	}
}

func TestSlidingWindow_WindowSlide(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewSlidingWindow(3, 500*time.Millisecond, withClock(clock)) // 500ms窗口内最多3个请求

	// 消耗所有名额
	for i := 0; i < 3; i++ {
		if _, err := limiter.Allow(ctx, "user:1"); err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
	}

	// 推进窗口滑动
	clock.Advance(600 * time.Millisecond)

	// 应该可以放行
	result, _ := limiter.Allow(ctx, "user:1")
	if !result.Allowed {
		t.Error("expected request to be allowed after window slides")
	}
}

func TestSlidingWindow_LogCapacityDoesNotExceedLimit(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewSlidingWindow(3, 3*time.Second, withClock(clock))

	for i := 0; i < 3; i++ {
		result, err := limiter.Allow(ctx, "user:1")
		if err != nil {
			t.Fatalf("unexpected error on request %d: %v", i+1, err)
		}
		if !result.Allowed {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
		clock.Advance(time.Second)
	}

	state := slidingWindowStateForTest(t, limiter, "user:1")
	if got := cap(state.logs); got > limiter.limit {
		t.Fatalf("expected log capacity <= limit after filling window, got cap=%d limit=%d", got, limiter.limit)
	}

	clock.Advance(500 * time.Millisecond)
	result, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error after window slides: %v", err)
	}
	if !result.Allowed {
		t.Fatal("expected request to be allowed after oldest log expires")
	}

	state = slidingWindowStateForTest(t, limiter, "user:1")
	if got := cap(state.logs); got > limiter.limit {
		t.Fatalf("expected log capacity <= limit after sliding window compaction, got cap=%d limit=%d", got, limiter.limit)
	}
}

func TestSlidingWindow_LogCapacityGrowsLazily(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewSlidingWindow(1024, time.Second, withClock(clock))

	result, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error on first request: %v", err)
	}
	if !result.Allowed {
		t.Fatal("expected first request to be allowed")
	}

	state := slidingWindowStateForTest(t, limiter, "user:1")
	if got := cap(state.logs); got >= limiter.limit {
		t.Fatalf("expected log capacity to grow lazily, got cap=%d limit=%d", got, limiter.limit)
	}
}

func slidingWindowStateForTest(t *testing.T, limiter *SlidingWindow, key string) *windowState {
	t.Helper()

	store, ok := limiter.store.(*mapStore[*windowState])
	if !ok {
		t.Fatal("expected sliding window to use mapStore in memory tests")
	}
	entry, ok := store.items[key]
	if !ok {
		t.Fatalf("expected key %q to exist in store", key)
	}
	return entry.value
}

func TestNewSlidingWindow_InvalidLimit(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on limit <= 0")
		}
	}()
	NewSlidingWindow(0, time.Second)
}

func TestNewSlidingWindow_InvalidWindow(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on window <= 0")
		}
	}()
	NewSlidingWindow(3, 0)
}

func TestSlidingWindow_Cleanup_ExpiredKeyRemoved(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewSlidingWindow(5, time.Second,
		WithMaxKeys(2),
		WithKeyTTL(100*time.Millisecond),
		WithCleanupInterval(50*time.Millisecond),
		withClock(clock),
	)

	if _, err := limiter.Allow(ctx, "user:1"); err != nil {

		t.Fatalf("Allow() error = %v", err)

	}
	if _, err := limiter.Allow(ctx, "user:2"); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	clock.Advance(160 * time.Millisecond)

	result, err := limiter.Allow(ctx, "user:3")
	if err != nil {
		t.Fatalf("expected no error after cleanup, got %v", err)
	}
	if !result.Allowed {
		t.Error("expected user:3 to be allowed after expired keys cleaned up")
	}
}

func TestSlidingWindow_Cleanup_UnexpiredKeyKept(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewSlidingWindow(5, time.Second,
		WithMaxKeys(2),
		WithKeyTTL(10*time.Second),
		WithCleanupInterval(50*time.Millisecond),
		withClock(clock),
	)

	if _, err := limiter.Allow(ctx, "user:1"); err != nil {

		t.Fatalf("Allow() error = %v", err)

	}
	if _, err := limiter.Allow(ctx, "user:2"); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	clock.Advance(100 * time.Millisecond)

	_, err := limiter.Allow(ctx, "user:3")
	if err != ErrMaxKeysExceeded {
		t.Errorf("expected ErrMaxKeysExceeded for unexpired keys, got %v", err)
	}
}

func TestSlidingWindow_Cleanup_LastSeenRefreshed(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewSlidingWindow(5, time.Second,
		WithMaxKeys(2),
		WithKeyTTL(200*time.Millisecond),
		WithCleanupInterval(50*time.Millisecond),
		withClock(clock),
	)

	if _, err := limiter.Allow(ctx, "user:1"); err != nil {

		t.Fatalf("Allow() error = %v", err)

	}
	clock.Advance(100 * time.Millisecond)
	if _, err := limiter.Allow(ctx, "user:1"); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	clock.Advance(150 * time.Millisecond)

	if _, err := limiter.Allow(ctx, "user:2"); err != nil {

		t.Fatalf("Allow() error = %v", err)

	}
	result, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error for refreshed key: %v", err)
	}
	if !result.Allowed {
		t.Error("expected user:1 to be allowed after lastSeen refresh")
	}
}

func TestSlidingWindow_Cleanup_IntervalNotReached(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewSlidingWindow(5, time.Second,
		WithMaxKeys(2),
		WithKeyTTL(50*time.Millisecond),
		WithCleanupInterval(10*time.Second),
		withClock(clock),
	)

	if _, err := limiter.Allow(ctx, "user:1"); err != nil {

		t.Fatalf("Allow() error = %v", err)

	}
	if _, err := limiter.Allow(ctx, "user:2"); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	clock.Advance(100 * time.Millisecond)

	_, err := limiter.Allow(ctx, "user:3")
	if err != ErrMaxKeysExceeded {
		t.Errorf("expected ErrMaxKeysExceeded when cleanupInterval not reached, got %v", err)
	}
}
