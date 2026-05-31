package memory

import (
	"context"
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
		limiter.Allow(ctx, "user:1")
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
	limiter := NewTokenBucket(5, 10) // 容量5，每秒10个令牌

	// 消耗所有令牌
	for i := 0; i < 5; i++ {
		limiter.Allow(ctx, "user:1")
	}

	// 等待令牌补充
	time.Sleep(200 * time.Millisecond) // 应该补充约2个令牌

	// 应该可以放行
	result, _ := limiter.Allow(ctx, "user:1")
	if !result.Allowed {
		t.Error("expected request to be allowed after token refill")
	}
}
