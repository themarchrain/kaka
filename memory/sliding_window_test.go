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

func TestSlidingWindow_WindowSlide(t *testing.T) {
	ctx := context.Background()
	limiter := NewSlidingWindow(3, 500*time.Millisecond) // 500ms窗口内最多3个请求

	// 消耗所有名额
	for i := 0; i < 3; i++ {
		limiter.Allow(ctx, "user:1")
	}

	// 等待窗口滑动
	time.Sleep(600 * time.Millisecond) // 窗口已过期

	// 应该可以放行
	result, _ := limiter.Allow(ctx, "user:1")
	if !result.Allowed {
		t.Error("expected request to be allowed after window slides")
	}
}
