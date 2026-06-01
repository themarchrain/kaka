package memory

import (
	"context"
	"sync"
	"time"

	"github.com/themarchrain/kaka"
)

var _ kaka.Limiter = (*SlidingWindow)(nil)

type SlidingWindow struct {
	mu     sync.Mutex
	limit  int           // 窗口内允许的最大请求数
	window time.Duration // 窗口大小 (如 1 * time.Second)
	opts   options
	store  stateStore[*windowState]
}

type windowState struct {
	logs []time.Time // 记录请求时间戳的日志切片
}

func NewSlidingWindow(limit int, window time.Duration, opts ...Option) *SlidingWindow {
	if limit <= 0 {
		panic("memory: sliding window limit must be > 0")
	}
	if window <= 0 {
		panic("memory: sliding window window must be > 0")
	}

	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	o.applyDefaults()
	o.validate()

	return &SlidingWindow{
		limit:  limit,
		window: window,
		opts:   o,
		store:  newStateStore[*windowState](o),
	}
}

func (sw *SlidingWindow) Allow(ctx context.Context, key string) (kaka.Result, error) {
	if err := validateKey(key); err != nil {
		return kaka.Result{}, err
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := sw.opts.clock.Now()
	state, err := sw.store.getOrCreate(key, now, func(now time.Time) *windowState {
		return &windowState{
			logs: make([]time.Time, 0),
		}
	})
	if err != nil {
		return kaka.Result{}, err
	}

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
