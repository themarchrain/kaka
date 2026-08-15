package memory

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestTokenBucket_Allow(t *testing.T) {
	ctx := context.Background()
	limiter := NewTokenBucket(10, 1) // 容量10，每秒1个令牌

	// 测试1：初始状态应该放行
	result, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("expected first request to be allowed")
	}

	// 测试2：消耗所有令牌
	for i := 0; i < 9; i++ {
		result, err = limiter.Allow(ctx, "user:1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Allowed {
			t.Errorf("expected request %d to be allowed", i+2)
		}
	}

	// 测试3：令牌耗尽，应该限流
	result, err = limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("expected request to be rejected after tokens exhausted")
	}
}

func TestTokenBucket_KeyIsolation(t *testing.T) {
	ctx := context.Background()
	limiter := NewTokenBucket(5, 1) // 容量5，每秒1个令牌

	// 消耗 user:1 的所有令牌
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

func TestTokenBucket_TokenRefill(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	limiter := NewTokenBucket(5, 10, withClock(clock)) // 容量5，每秒10个令牌

	// 消耗所有令牌
	for i := 0; i < 5; i++ {
		if _, err := limiter.Allow(ctx, "user:1"); err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
	}

	// 推进时间，应该补充约2个令牌
	clock.Advance(200 * time.Millisecond)

	// 应该可以放行
	result, _ := limiter.Allow(ctx, "user:1")
	if !result.Allowed {
		t.Error("expected request to be allowed after token refill")
	}
}

func TestNewTokenBucket_InvalidCapacity(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on capacity < 1")
		}
	}()
	NewTokenBucket(0, 1)
}

func TestNewTokenBucket_CapacityLessThanOne(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on capacity < 1")
		}
	}()
	NewTokenBucket(0.5, 1)
}

func TestNewTokenBucket_InvalidRate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on rate <= 0")
		}
	}()
	NewTokenBucket(10, 0)
}

func TestNewTokenBucket_NonFiniteCapacity(t *testing.T) {
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
			NewTokenBucket(tc.capacity, 1)
		})
	}
}

func TestNewTokenBucket_NonFiniteRate(t *testing.T) {
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
			NewTokenBucket(10, tc.rate)
		})
	}
}

func TestNewTokenBucket_DefaultOptions(t *testing.T) {
	limiter := NewTokenBucket(10, 1)
	if limiter.opts.maxKeys != 0 {
		t.Errorf("expected maxKeys=0, got %d", limiter.opts.maxKeys)
	}
	if limiter.opts.keyTTL != 0 {
		t.Errorf("expected keyTTL=0, got %v", limiter.opts.keyTTL)
	}
	if limiter.opts.cleanupInterval != 0 {
		t.Errorf("expected cleanupInterval=0, got %v", limiter.opts.cleanupInterval)
	}
}

func TestNewTokenBucket_WithOptions(t *testing.T) {
	limiter := NewTokenBucket(10, 1,
		WithMaxKeys(1000),
		WithKeyTTL(5*time.Minute),
		WithCleanupInterval(time.Minute),
	)
	if limiter.opts.maxKeys != 1000 {
		t.Errorf("expected maxKeys=1000, got %d", limiter.opts.maxKeys)
	}
	if limiter.opts.keyTTL != 5*time.Minute {
		t.Errorf("expected keyTTL=5m, got %v", limiter.opts.keyTTL)
	}
	if limiter.opts.cleanupInterval != time.Minute {
		t.Errorf("expected cleanupInterval=1m, got %v", limiter.opts.cleanupInterval)
	}
}

func TestOptions_InvalidMaxKeys(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on maxKeys < 0")
		}
	}()
	NewTokenBucket(10, 1, WithMaxKeys(-1))
}

func TestOptions_InvalidKeyTTL(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on keyTTL < 0")
		}
	}()
	NewTokenBucket(10, 1, WithKeyTTL(-1*time.Second))
}

func TestOptions_InvalidCleanupInterval(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on cleanupInterval < 0")
		}
	}()
	NewTokenBucket(10, 1, WithCleanupInterval(-1*time.Second))
}

func TestNewTokenBucket_CleanupIntervalDefault(t *testing.T) {
	// 设置 keyTTL 但未设置 cleanupInterval，默认应为 1 分钟
	limiter := NewTokenBucket(10, 1, WithKeyTTL(5*time.Minute))
	if limiter.opts.cleanupInterval != time.Minute {
		t.Errorf("expected default cleanupInterval=1m, got %v", limiter.opts.cleanupInterval)
	}

	// 显式设置 cleanupInterval 时不覆盖
	limiter2 := NewTokenBucket(10, 1, WithKeyTTL(5*time.Minute), WithCleanupInterval(30*time.Second))
	if limiter2.opts.cleanupInterval != 30*time.Second {
		t.Errorf("expected cleanupInterval=30s, got %v", limiter2.opts.cleanupInterval)
	}

	// 没有 keyTTL 时不设置
	limiter3 := NewTokenBucket(10, 1)
	if limiter3.opts.cleanupInterval != 0 {
		t.Errorf("expected cleanupInterval=0, got %v", limiter3.opts.cleanupInterval)
	}
}

func TestTokenBucket_MaxKeys(t *testing.T) {
	ctx := context.Background()
	limiter := NewTokenBucket(10, 1, WithMaxKeys(2))

	// 第一个 key 应该正常放行
	result, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("expected user:1 to be allowed")
	}

	// 第二个 key 应该正常放行
	result, err = limiter.Allow(ctx, "user:2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("expected user:2 to be allowed")
	}

	// 第三个新 key 应该返回 ErrMaxKeysExceeded
	_, err = limiter.Allow(ctx, "user:3")
	if err != ErrMaxKeysExceeded {
		t.Errorf("expected ErrMaxKeysExceeded, got %v", err)
	}

	// 已有 key 应该继续按限流逻辑运行
	result, err = limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error for existing key: %v", err)
	}
	if !result.Allowed {
		t.Error("expected existing user:1 to still be allowed")
	}

	// maxKeys=0 表示不限制
	limiter2 := NewTokenBucket(10, 1, WithMaxKeys(0))
	for i := 0; i < 100; i++ {
		_, err := limiter2.Allow(ctx, "user:"+string(rune('a'+i%26))+string(rune('0'+i/26)))
		if err != nil {
			t.Fatalf("unexpected error with maxKeys=0: %v", err)
		}
	}
}

func TestTokenBucket_Cleanup_ExpiredKeyRemoved(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	// TTL=100ms, cleanupInterval=50ms
	limiter := NewTokenBucket(10, 1,
		WithMaxKeys(2),
		WithKeyTTL(100*time.Millisecond),
		WithCleanupInterval(50*time.Millisecond),
		withClock(clock),
	)

	// 创建两个 key
	if _, err := limiter.Allow(ctx, "user:1"); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if _, err := limiter.Allow(ctx, "user:2"); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	// 推进到 key 过期 + 超过 cleanupInterval
	clock.Advance(160 * time.Millisecond)

	// 新 key 应该能进入，因为过期 key 已被清理
	result, err := limiter.Allow(ctx, "user:3")
	if err != nil {
		t.Fatalf("expected no error after cleanup, got %v", err)
	}
	if !result.Allowed {
		t.Error("expected user:3 to be allowed after expired keys cleaned up")
	}
}

func TestTokenBucket_Cleanup_UnexpiredKeyKept(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	// TTL=10s, cleanupInterval=50ms
	limiter := NewTokenBucket(10, 1,
		WithMaxKeys(2),
		WithKeyTTL(10*time.Second),
		WithCleanupInterval(50*time.Millisecond),
		withClock(clock),
	)

	// 创建两个 key
	if _, err := limiter.Allow(ctx, "user:1"); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if _, err := limiter.Allow(ctx, "user:2"); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	// 推进超过 cleanupInterval 但未超过 TTL
	clock.Advance(100 * time.Millisecond)

	// 新 key 不应该能进入，因为 key 未过期
	_, err := limiter.Allow(ctx, "user:3")
	if err != ErrMaxKeysExceeded {
		t.Errorf("expected ErrMaxKeysExceeded for unexpired keys, got %v", err)
	}

	// 原有 key 应该继续正常工作
	result, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error for existing key: %v", err)
	}
	if !result.Allowed {
		t.Error("expected user:1 to still be allowed")
	}
}

func TestTokenBucket_Cleanup_LastSeenRefreshed(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	// TTL=200ms, cleanupInterval=50ms
	limiter := NewTokenBucket(10, 1,
		WithMaxKeys(2),
		WithKeyTTL(200*time.Millisecond),
		WithCleanupInterval(50*time.Millisecond),
		withClock(clock),
	)

	// 创建 key
	if _, err := limiter.Allow(ctx, "user:1"); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	// 100ms 后访问一次，刷新 lastSeen
	clock.Advance(100 * time.Millisecond)
	if _, err := limiter.Allow(ctx, "user:1"); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	// 再等 150ms（距首次 250ms，但距最后访问只有 150ms < TTL=200ms）
	clock.Advance(150 * time.Millisecond)

	// 创建新 key，触发清理
	// user:1 不应该被清理，因为 lastSeen 被刷新了
	if _, err := limiter.Allow(ctx, "user:2"); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	// user:1 应该还能用
	result, err := limiter.Allow(ctx, "user:1")
	if err != nil {
		t.Fatalf("unexpected error for refreshed key: %v", err)
	}
	if !result.Allowed {
		t.Error("expected user:1 to be allowed after lastSeen refresh")
	}
}

func TestTokenBucket_Cleanup_IntervalNotReached(t *testing.T) {
	ctx := context.Background()
	clock := newFakeClock(time.Unix(100, 0))
	// TTL=50ms, cleanupInterval=10s（很长的间隔）
	limiter := NewTokenBucket(10, 1,
		WithMaxKeys(2),
		WithKeyTTL(50*time.Millisecond),
		WithCleanupInterval(10*time.Second),
		withClock(clock),
	)

	// 创建两个 key
	if _, err := limiter.Allow(ctx, "user:1"); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if _, err := limiter.Allow(ctx, "user:2"); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	// 推进到 key 过期
	clock.Advance(100 * time.Millisecond)

	// cleanupInterval 未到，不应该清理
	_, err := limiter.Allow(ctx, "user:3")
	if err != ErrMaxKeysExceeded {
		t.Errorf("expected ErrMaxKeysExceeded when cleanupInterval not reached, got %v", err)
	}
}
