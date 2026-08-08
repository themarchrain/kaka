package memory

import (
	"context"
	"testing"
)

// TestTokenBucket_Allow_ZeroAllocations 防止热路径分配回归。
// v0.0.2 之前 create 闭包每轮逃逸分配 16B（1 alloc/op）；修复后必须为 0。
// 断言前先预热一次使 key 状态就绪，随后 10000 次 Allow 应零分配。
func TestTokenBucket_Allow_ZeroAllocations(t *testing.T) {
	ctx := context.Background()
	limiter := NewTokenBucket(100, 10)

	if _, err := limiter.Allow(ctx, "user:1"); err != nil {
		t.Fatalf("unexpected error warming up: %v", err)
	}

	allocs := testing.AllocsPerRun(10000, func() {
		_, _ = limiter.Allow(ctx, "user:1")
	})
	if allocs != 0 {
		t.Fatalf("expected 0 allocations on token bucket hot path, got %v", allocs)
	}
}
