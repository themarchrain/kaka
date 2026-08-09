package redis

import (
	"testing"
	"time"

	"github.com/themarchrain/kaka/internal/contracttest"
)

// TestContractTokenBucket 复用与 memory 相同的共享行为测试
// （结果边界 / 同 key 并发 / 多 key 并发 / 空 key 报错）。
func TestContractTokenBucket(t *testing.T) {
	client := testClient(t)
	contracttest.RunLimiterBehaviorTests(t, []contracttest.LimiterCase{
		{
			Name: "redis-token-bucket",
			New: func() contracttest.Limiter {
				return NewTokenBucket(client, 1, 1)
			},
		},
	})
}

// TestContractSlidingWindow 复用与 memory 相同的共享行为测试。
func TestContractSlidingWindow(t *testing.T) {
	client := testClient(t)
	contracttest.RunLimiterBehaviorTests(t, []contracttest.LimiterCase{
		{
			Name: "redis-sliding-window",
			New: func() contracttest.Limiter {
				return NewSlidingWindow(client, 1, time.Minute, WithKeyPrefix(testKeyPrefix))
			},
		},
	})
}

// TestContractLeakyBucket 复用与 memory 相同的共享行为测试。
func TestContractLeakyBucket(t *testing.T) {
	client := testClient(t)
	contracttest.RunLimiterBehaviorTests(t, []contracttest.LimiterCase{
		{
			Name: "redis-leaky-bucket",
			New: func() contracttest.Limiter {
				return NewLeakyBucket(client, 1, 1, WithKeyPrefix(testKeyPrefix))
			},
		},
	})
}
