package layered

import (
	"testing"
	"time"

	"github.com/themarchrain/kaka/internal/contracttest"
	"github.com/themarchrain/kaka/memory"
	redislimiter "github.com/themarchrain/kaka/redis"
)

// TestContractTokenBucket runs the shared behavior tests (result boundary /
// same-key concurrency / multi-key concurrency / blank key) on a two-tier
// limiter: in-memory local pre-check + Redis remote authority.
func TestContractTokenBucket(t *testing.T) {
	client := testClient(t)
	contracttest.RunLimiterBehaviorTests(t, []contracttest.LimiterCase{
		{
			Name: "layered-token-bucket",
			New: func() contracttest.Limiter {
				return New(
					memory.NewTokenBucket(1, 1),
					redislimiter.NewTokenBucket(client, 1, 1, redislimiter.WithKeyPrefix(testKeyPrefix)),
				)
			},
		},
	})
}

// TestContractLeakyBucket runs the shared behavior tests on the leaky
// bucket combination.
func TestContractLeakyBucket(t *testing.T) {
	client := testClient(t)
	contracttest.RunLimiterBehaviorTests(t, []contracttest.LimiterCase{
		{
			Name: "layered-leaky-bucket",
			New: func() contracttest.Limiter {
				return New(
					memory.NewLeakyBucket(1, 1),
					redislimiter.NewLeakyBucket(client, 1, 1, redislimiter.WithKeyPrefix(testKeyPrefix)),
				)
			},
		},
	})
}

// TestContractSlidingWindow runs the shared behavior tests on the sliding
// window combination.
func TestContractSlidingWindow(t *testing.T) {
	client := testClient(t)
	contracttest.RunLimiterBehaviorTests(t, []contracttest.LimiterCase{
		{
			Name: "layered-sliding-window",
			New: func() contracttest.Limiter {
				return New(
					memory.NewSlidingWindow(1, time.Minute),
					redislimiter.NewSlidingWindow(client, 1, time.Minute, redislimiter.WithKeyPrefix(testKeyPrefix)),
				)
			},
		},
	})
}
