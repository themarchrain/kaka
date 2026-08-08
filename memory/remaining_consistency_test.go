package memory

import (
	"context"
	"testing"
	"time"

	"github.com/themarchrain/kaka"
)

// TestLimiters_RemainingDecrementsConsistently 验证三个算法在
// capacity=5 且无时间流逝时，Remaining 精确递减 4,3,2,1,0，
// 第 6 次请求拒绝且 Remaining==0。这是 memory/Redis 共享行为的参照。
func TestLimiters_RemainingDecrementsConsistently(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name string
		new  func() kaka.Limiter
	}{
		{"token bucket", func() kaka.Limiter { return NewTokenBucket(5, 1) }},
		{"leaky bucket", func() kaka.Limiter { return NewLeakyBucket(5, 1) }},
		{"sliding window", func() kaka.Limiter { return NewSlidingWindow(5, time.Minute) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limiter := tc.new()

			for want := int64(4); want >= 0; want-- {
				r, err := limiter.Allow(ctx, "user:1")
				if err != nil {
					t.Fatalf("unexpected error at remaining=%d: %v", want, err)
				}
				if !r.Allowed {
					t.Fatalf("expected allow at remaining=%d", want)
				}
				if r.Remaining != want {
					t.Fatalf("expected remaining=%d, got %d", want, r.Remaining)
				}
				if r.RetryAfter != 0 {
					t.Fatalf("expected retryAfter=0 on allow, got %v", r.RetryAfter)
				}
			}

			denied, err := limiter.Allow(ctx, "user:1")
			if err != nil {
				t.Fatalf("unexpected error on deny: %v", err)
			}
			if denied.Allowed {
				t.Fatal("expected deny when capacity is exhausted")
			}
			if denied.Remaining != 0 {
				t.Fatalf("expected remaining=0 on deny, got %d", denied.Remaining)
			}
			if denied.RetryAfter <= 0 {
				t.Fatalf("expected positive retryAfter on deny, got %v", denied.RetryAfter)
			}
		})
	}
}
