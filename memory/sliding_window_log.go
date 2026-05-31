package memory

import (
	"context"
	"sync"
	"time"

	"github.com/themarchrain/kaka"
)

type SlidingWindow struct {
	mu      sync.Mutex
	limit   int                     // 窗口内允许的最大请求数
	window  time.Duration           // 窗口大小 (如 1 * time.Second)
	buckets map[string]*windowState // 按 key 存储窗口状态
}

type windowState struct {
	logs []time.Time // 记录请求时间戳的日志切片
}

func NewSlidingWindow(limit int, window time.Duration) *SlidingWindow {
	return &SlidingWindow{
		limit:   limit,
		window:  window,
		buckets: make(map[string]*windowState),
	}
}

func (sw *SlidingWindow) Allow(ctx context.Context, key string) (kaka.Result, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	state, exists := sw.buckets[key]
	if !exists {
		state = &windowState{
			logs: make([]time.Time, 0),
		}
		sw.buckets[key] = state
	}

	now := time.Now()
	windowStart := now.Add(-sw.window)

	// 查找第一个在窗口内的时间戳索引
	validIndex := len(state.logs) // 默认全部过期
	left, right := 0, len(state.logs)-1
	for left <= right {
		mid := left + ((right - left) >> 1)
		if state.logs[mid].After(windowStart) {
			validIndex = mid
			right = mid - 1
		} else {
			left = mid + 1
		}
	}
	// 从有效索引开始截取时间戳
	state.logs = state.logs[validIndex:]

	// 判断当前有效窗口内的请求数是否达到上限
	if len(state.logs) < sw.limit {
		// 未超限，追加当前请求时间，放行
		state.logs = append(state.logs, now)
		return kaka.Result{
			Allowed:   true,
			Remaining: int64(sw.limit - len(state.logs)),
		}, nil
	}

	// 超限，拒绝
	// 计算需要等待的时间（最早的日志过期时间）
	retryAfter := state.logs[0].Add(sw.window).Sub(now)
	return kaka.Result{
		Allowed:    false,
		Remaining:  0,
		RetryAfter: retryAfter,
	}, nil
}
