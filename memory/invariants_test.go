package memory

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/themarchrain/kaka"
)

// leakyStateOf 通过类型断言访问内部 state（同包测试）。
func leakyStateOf(t *testing.T, lb *LeakyBucket, key string) *leakyState {
	t.Helper()
	ss, ok := lb.store.(*shardedStore[*leakyState])
	if !ok {
		t.Fatal("unexpected store type")
	}
	sh := ss.shardFor(key)
	e, ok := sh.items[key]
	if !ok {
		t.Fatalf("key %q not found", key)
	}
	return e.value
}

func windowStateOf(t *testing.T, sw *SlidingWindow, key string) *windowState {
	t.Helper()
	ss, ok := sw.store.(*shardedStore[*windowState])
	if !ok {
		t.Fatal("unexpected store type")
	}
	sh := ss.shardFor(key)
	e, ok := sh.items[key]
	if !ok {
		t.Fatalf("key %q not found", key)
	}
	return e.value
}

// TestLeakyBucket_Invariants 逐请求断言漏桶 6 条不变量。
func TestLeakyBucket_Invariants(t *testing.T) {
	const capacity = 5.0
	const rate = 1.0
	ctx := context.Background()
	clock := newFakeClock(time.Unix(0, 0))
	lb := NewLeakyBucket(capacity, rate, withClock(clock))

	type step struct {
		delay time.Duration
	}
	steps := []step{
		{time.Millisecond}, {time.Millisecond}, {time.Millisecond},
		{time.Millisecond}, {time.Millisecond}, {time.Millisecond}, // 突发打满（第 6 个拒绝）
		{500 * time.Millisecond}, // 漏 0.5 → water 4.5 + 1 = 5.5 > 5 仍拒绝
		{500 * time.Millisecond}, // 漏 0.5 → water 4.0 + 1 = 5.0 <= 5 放行
		{60 * time.Second},       // 空闲漏空 → water 0 → 放行
	}

	water := 0.0 // 推演的期望水量
	for i, st := range steps {
		clock.Advance(st.delay)

		// 推演：先漏水（elapsed × rate，钳 0）
		water -= st.delay.Seconds() * rate
		if water < 0 {
			water = 0
		}
		wantAllowed := water+1 <= capacity

		r, err := lb.Allow(ctx, "user:1")
		if err != nil {
			t.Fatalf("step %d: unexpected error: %v", i, err)
		}
		if r.Allowed != wantAllowed {
			t.Fatalf("step %d: decision=%v want=%v", i, r.Allowed, wantAllowed)
		}

		state := leakyStateOf(t, lb, "user:1")
		// L1: 0 <= water <= capacity
		if state.water < 0 || state.water > capacity {
			t.Fatalf("step %d: water %v out of range", i, state.water)
		}
		// L4: 漏水精确（放行后 = 推演 water + 1；拒绝后 = 推演 water）
		var wantWater float64
		if r.Allowed {
			wantWater = water + 1
		} else {
			wantWater = water
		}
		if math.Abs(state.water-wantWater) > 1e-9 {
			t.Fatalf("step %d: water=%v want=%v", i, state.water, wantWater)
		}
		// L3: 允许时 Remaining = floor(capacity - water)
		if r.Allowed {
			wantRemaining := int64(capacity - state.water)
			if r.Remaining != wantRemaining {
				t.Fatalf("step %d: remaining=%d want=%d", i, r.Remaining, wantRemaining)
			}
		}
		water = state.water
	}

	// L5: 空闲后 water 钳 0（上面第 9 步已覆盖，这里显式复核：放行后 water==1）
	clock.Advance(time.Hour)
	if _, err := lb.Allow(ctx, "user:1"); err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if s := leakyStateOf(t, lb, "user:1"); s.water != 1 {
		t.Fatalf("after idle: water=%v want=1", s.water)
	}

	// L6: RetryAfter 边界——拒绝后 advance(retryAfter-1ns) 仍拒绝，advance(1ns) 放行
	var denied kaka.Result
	for {
		r, err := lb.Allow(ctx, "user:1")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Allowed {
			denied = r
			break
		}
	}
	if denied.RetryAfter <= 0 {
		t.Fatalf("expected positive retryAfter, got %+v", denied)
	}
	clock.Advance(denied.RetryAfter - time.Nanosecond)
	still, _ := lb.Allow(ctx, "user:1")
	if still.Allowed {
		t.Fatal("expected still denied before retryAfter boundary")
	}
	clock.Advance(time.Nanosecond)
	ok, _ := lb.Allow(ctx, "user:1")
	if !ok.Allowed {
		t.Fatal("expected allowed exactly at retryAfter boundary")
	}
}

// TestSlidingWindow_Invariants 逐请求断言滑动窗口 6 条不变量。
func TestSlidingWindow_Invariants(t *testing.T) {
	const limit = 3
	const window = time.Second
	ctx := context.Background()
	clock := newFakeClock(time.Unix(0, 0))
	sw := NewSlidingWindow(limit, window, withClock(clock))

	type step struct {
		delay time.Duration
	}
	steps := []step{
		{time.Millisecond}, {time.Millisecond}, {time.Millisecond},
		{time.Millisecond}, {time.Millisecond}, // 突发 5 次 → 3 放行 2 拒绝
		{500 * time.Millisecond}, {500 * time.Millisecond}, {500 * time.Millisecond}, // 窗口滑动
		{2 * time.Second}, // 空闲 → 清空
	}

	// 模拟 logs：推演期望
	sim := []time.Time{}
	now := time.Unix(0, 0)
	for i, st := range steps {
		clock.Advance(st.delay)
		now = now.Add(st.delay)

		// 移除过期日志
		windowStart := now.Add(-window)
		valid := sim[:0:0]
		for _, ts := range sim {
			if ts.After(windowStart) {
				valid = append(valid, ts)
			}
		}
		sim = valid
		wantAllowed := len(sim) < limit

		r, err := sw.Allow(ctx, "user:1")
		if err != nil {
			t.Fatalf("step %d: unexpected error: %v", i, err)
		}
		if r.Allowed != wantAllowed {
			t.Fatalf("step %d: decision=%v want=%v", i, r.Allowed, wantAllowed)
		}
		if r.Allowed {
			sim = append(sim, now)
		}

		state := windowStateOf(t, sw, "user:1")
		// S1: len(logs) <= limit
		if len(state.logs) > limit {
			t.Fatalf("step %d: logs len %d > limit %d", i, len(state.logs), limit)
		}
		// S3: 允许时 Remaining = limit - len(logs)
		if r.Allowed && int(r.Remaining) != limit-len(state.logs) {
			t.Fatalf("step %d: remaining=%d want=%d", i, r.Remaining, limit-len(state.logs))
		}
		// S4: 窗口内日志全有效
		for _, ts := range state.logs {
			if !ts.After(now.Add(-window)) {
				t.Fatalf("step %d: stale log %v not removed", i, ts)
			}
		}
	}

	// S6: RetryAfter 边界——拒绝后 advance(retryAfter-1ns) 仍拒绝，advance 后放行
	var denied kaka.Result
	for {
		r, err := sw.Allow(ctx, "user:1")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Allowed {
			denied = r
			break
		}
	}
	if denied.RetryAfter <= 0 {
		t.Fatalf("expected positive retryAfter, got %+v", denied)
	}
	clock.Advance(denied.RetryAfter - time.Nanosecond)
	still, _ := sw.Allow(ctx, "user:1")
	if still.Allowed {
		t.Fatal("expected still denied before retryAfter boundary")
	}
	clock.Advance(time.Nanosecond)
	ok, _ := sw.Allow(ctx, "user:1")
	if !ok.Allowed {
		t.Fatal("expected allowed exactly at retryAfter boundary")
	}
}
