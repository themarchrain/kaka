package redis

import (
	"testing"

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
