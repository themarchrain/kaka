package memory

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestLeakyBucket_Allow(t *testing.T) {
	ctx := context.Background()
	limiter := NewLeakyBucket(10, 1) // 容量10，每秒漏1滴

	// 测试1：初始状态应该放行
	result, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("expected first request to be allowed")
	}

	// 测试2：快速消耗容量
	for i := 0; i < 9; i++ {
		result, err = limiter.Allow(ctx, "user:1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Allowed {
			t.Errorf("expected request %d to be allowed", i+2)
		}
	}

	// 测试3：桶满了，应该限流
	result, err = limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("expected request to be rejected when bucket is full")
	}
}

func TestLeakyBucket_KeyIsolation(t *testing.T) {
	ctx := context.Background()
	limiter := NewLeakyBucket(5, 1) // 容量5，每秒漏1滴

	// 消耗 user:1 的所有容量
	for i := 0; i < 5; i++ {
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

func TestLeakyBucket_Leak(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewLeakyBucket(5, 10, withClock(clock)) // 容量5，每秒漏10滴

	// 消耗所有容量
	for i := 0; i < 5; i++ {
		if _, err := limiter.Allow(ctx, "user:1"); err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
	}

	// 推进时间，应该漏掉约2滴水
	clock.Advance(200 * time.Millisecond)

	// 应该可以放行
	result, _ := limiter.Allow(ctx, "user:1")
	if !result.Allowed {
		t.Error("expected request to be allowed after water leaks")
	}
}

func TestNewLeakyBucket_InvalidCapacity(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on capacity < 1")
		}
	}()
	NewLeakyBucket(0, 1)
}

func TestNewLeakyBucket_CapacityLessThanOne(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on capacity < 1")
		}
	}()
	NewLeakyBucket(0.5, 1)
}

func TestNewLeakyBucket_InvalidRate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on rate <= 0")
		}
	}()
	NewLeakyBucket(10, 0)
}

func TestNewLeakyBucket_NonFiniteCapacity(t *testing.T) {
	cases := []struct {
		name     string
		capacity float64
	}{
		{name: "NaN", capacity: math.NaN()},
		{name: "+Inf", capacity: math.Inf(1)},
		{name: "-Inf", capacity: math.Inf(-1)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected panic on non-finite capacity")
				}
			}()
			NewLeakyBucket(tc.capacity, 1)
		})
	}
}

func TestNewLeakyBucket_NonFiniteRate(t *testing.T) {
	cases := []struct {
		name string
		rate float64
	}{
		{name: "NaN", rate: math.NaN()},
		{name: "+Inf", rate: math.Inf(1)},
		{name: "-Inf", rate: math.Inf(-1)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected panic on non-finite rate")
				}
			}()
			NewLeakyBucket(10, tc.rate)
		})
	}
}

func TestLeakyBucket_Cleanup_ExpiredKeyRemoved(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewLeakyBucket(10, 1,
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

func TestLeakyBucket_Cleanup_UnexpiredKeyKept(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewLeakyBucket(10, 1,
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

func TestLeakyBucket_Cleanup_LastSeenRefreshed(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewLeakyBucket(10, 1,
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

func TestLeakyBucket_Cleanup_IntervalNotReached(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewLeakyBucket(10, 1,
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
