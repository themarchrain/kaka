package kaka_test

import (
	"testing"
	"time"

	"github.com/themarchrain/kaka/internal/contracttest"
	"github.com/themarchrain/kaka/memory"
)

func TestMemoryLimiters_SharedBehavior(t *testing.T) {
	contracttest.RunLimiterBehaviorTests(t, []contracttest.LimiterCase{
		{
			Name: "memory token bucket",
			New: func() contracttest.Limiter {
				return memory.NewTokenBucket(1, 1)
			},
		},
		{
			Name: "memory leaky bucket",
			New: func() contracttest.Limiter {
				return memory.NewLeakyBucket(1, 1)
			},
		},
		{
			Name: "memory sliding window",
			New: func() contracttest.Limiter {
				return memory.NewSlidingWindow(1, time.Second)
			},
		},
	})
}
