package compare

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/juju/ratelimit"
	"github.com/themarchrain/kaka/memory"
	"golang.org/x/time/rate"
)

// fakeClock 同时满足 memory.Clock 与 juju ratelimit.Clock（均为 Now() time.Time）。
type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time { return c.now }
func (c *fakeClock) Set(t time.Time) { c.now = t }
func (c *fakeClock) Sleep(d time.Duration) { c.now = c.now.Add(d) }

type scenario struct {
	name  string
	times []time.Time
}

// buildScenarios 生成确定性时间-请求序列（固定 seed，任何环境可复现）。
func buildScenarios() []scenario {
	base := time.Unix(0, 0)
	refillInterval := 100 * time.Millisecond // rate=10/s 时补 1 令牌的时间

	var sc []scenario

	// 1. 恒速：5/s < 10/s，应全部放行
	{
		times := make([]time.Time, 0, 200)
		t := base
		for i := 0; i < 200; i++ {
			t = t.Add(200 * time.Millisecond)
			times = append(times, t)
		}
		sc = append(sc, scenario{name: "steady rate", times: times})
	}

	// 2. 突发耗尽：连续 1ms 间隔 150 次（打满 100 → 拒绝 50）
	{
		times := make([]time.Time, 0, 150)
		t := base
		for i := 0; i < 150; i++ {
			t = t.Add(time.Millisecond)
			times = append(times, t)
		}
		sc = append(sc, scenario{name: "burst exhaustion", times: times})
	}

	// 3. 恰好边界：间隔精确等于补 1 令牌时间（浮点舍入敏感区）
	{
		times := make([]time.Time, 0, 300)
		t := base
		for i := 0; i < 300; i++ {
			t = t.Add(refillInterval)
			times = append(times, t)
		}
		sc = append(sc, scenario{name: "exact refill boundary", times: times})
	}

	// 4. 空闲恢复：打满 → 空闲 60s → 再突发 50
	{
		times := make([]time.Time, 0, 150)
		t := base
		for i := 0; i < 100; i++ {
			t = t.Add(time.Millisecond)
			times = append(times, t)
		}
		t = t.Add(60 * time.Second)
		for i := 0; i < 50; i++ {
			t = t.Add(time.Millisecond)
			times = append(times, t)
		}
		sc = append(sc, scenario{name: "idle recovery", times: times})
	}

	// 5. 随机：seed=42，1ms–1s 间隔 × 1000
	{
		rng := rand.New(rand.NewSource(42))
		times := make([]time.Time, 0, 1000)
		t := base
		for i := 0; i < 1000; i++ {
			t = t.Add(time.Duration(rng.Int63n(1000)+1) * time.Millisecond)
			times = append(times, t)
		}
		sc = append(sc, scenario{name: "random seed=42", times: times})
	}

	return sc
}

func runKaka(times []time.Time) []bool {
	ctx := context.Background()
	clock := &fakeClock{now: times[0]}
	limiter := memory.NewTokenBucket(100, 10, memory.WithClock(clock))
	out := make([]bool, 0, len(times))
	for _, t := range times {
		clock.Set(t)
		r, err := limiter.Allow(ctx, "user:1")
		if err != nil {
			panic(err)
		}
		out = append(out, r.Allowed)
	}
	return out
}

func runXTime(times []time.Time) []bool {
	limiter := rate.NewLimiter(rate.Limit(10), 100)
	out := make([]bool, 0, len(times))
	for _, t := range times {
		out = append(out, limiter.AllowN(t, 1))
	}
	return out
}

func runJuju(times []time.Time) []bool {
	clock := &fakeClock{now: times[0]}
	limiter := ratelimit.NewBucketWithRateAndClock(10, 100, clock)
	out := make([]bool, 0, len(times))
	for _, t := range times {
		clock.Set(t)
		out = append(out, limiter.TakeAvailable(1) > 0)
	}
	return out
}

// TestTokenBucket_DifferentialCorrectness：同一时间-请求序列下，三方逐请求决策必须完全一致。
func TestTokenBucket_DifferentialCorrectness(t *testing.T) {
	for _, sc := range buildScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			kaka := runKaka(sc.times)
			xtime := runXTime(sc.times)
			juju := runJuju(sc.times)
			for i := range kaka {
				if kaka[i] != xtime[i] || kaka[i] != juju[i] {
					t.Errorf("请求 %d t=%v：kaka=%v x-time=%v juju=%v",
						i, sc.times[i], kaka[i], xtime[i], juju[i])
				}
			}
		})
	}
}

// TestTokenBucket_PerKeyIsolationConsistency：多 key 下，每个 key 的决策必须与
// 该 key 单独使用 limiter 时完全一致（per-key 隔离性；x-time/juju 无 per-key，故为 Kaka 自身一致性）。
func TestTokenBucket_PerKeyIsolationConsistency(t *testing.T) {
	ctx := context.Background()
	seq := buildScenarios()[1].times // 复用"突发耗尽"序列作为每个 key 的独立序列
	keys := []string{"user:a", "user:b", "user:c"}

	// 每个 key 单独 limiter 的期望决策
	expect := make(map[string][]bool)
	for _, key := range keys {
		c := &fakeClock{now: seq[0]}
		l := memory.NewTokenBucket(100, 10, memory.WithClock(c))
		var out []bool
		for _, ts := range seq {
			c.Set(ts)
			r, err := l.Allow(ctx, key)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, r.Allowed)
		}
		expect[key] = out
	}

	// 同一多 key limiter 按时间交错喂入 → 逐 key 决策必须与单独 limiter 一致
	sharedClock := &fakeClock{now: seq[0]}
	shared := memory.NewTokenBucket(100, 10, memory.WithClock(sharedClock))
	got := make(map[string][]bool)
	for _, key := range keys {
		got[key] = nil
	}
	for i := range seq {
		for _, key := range keys {
			sharedClock.Set(seq[i])
			r, err := shared.Allow(ctx, key)
			if err != nil {
				t.Fatal(err)
			}
			got[key] = append(got[key], r.Allowed)
		}
	}
	for _, key := range keys {
		for i := range expect[key] {
			if got[key][i] != expect[key][i] {
				t.Errorf("key %s 第 %d 次：共享=%v 单独=%v", key, i, got[key][i], expect[key][i])
			}
		}
	}
}
