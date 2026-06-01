package memory

import (
	"context"
	"sync"
	"time"

	"github.com/themarchrain/kaka"
)

var _ kaka.Limiter = (*SlidingWindow)(nil)

type SlidingWindow struct {
	mu          sync.Mutex
	limit       int                     // 窗口内允许的最大请求数
	window      time.Duration           // 窗口大小 (如 1 * time.Second)
	buckets     map[string]*windowState // 按 key 存储窗口状态
	opts        options
	lastCleanup time.Time
}

type windowState struct {
	logs     []time.Time // 记录请求时间戳的日志切片
	lastSeen time.Time   // 最后一次被访问（用于 TTL 清理）
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
		limit:   limit,
		window:  window,
		buckets: make(map[string]*windowState),
		opts:    o,
	}
}

func (sw *SlidingWindow) Allow(ctx context.Context, key string) (kaka.Result, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	state, exists := sw.buckets[key]
	if !exists {
		sw.lazyCleanup()
		if sw.opts.maxKeys > 0 && len(sw.buckets) >= sw.opts.maxKeys {
			return kaka.Result{}, ErrMaxKeysExceeded
		}
		now := time.Now()
		state = &windowState{
			logs:     make([]time.Time, 0),
			lastSeen: now,
		}
		sw.buckets[key] = state
	} else {
		state.lastSeen = time.Now()
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

// lazyCleanup 在 keyTTL 和 cleanupInterval 都启用时，按批次清理过期 key
// 调用方必须持有 sw.mu
func (sw *SlidingWindow) lazyCleanup() {
	if sw.opts.keyTTL <= 0 || sw.opts.cleanupInterval <= 0 {
		return
	}
	now := time.Now()
	if now.Sub(sw.lastCleanup) < sw.opts.cleanupInterval {
		return
	}
	sw.lastCleanup = now

	expired := make([]string, 0, cleanupBatchSize)
	scanned := 0
	for k, s := range sw.buckets {
		if scanned >= cleanupBatchSize {
			break
		}
		scanned++
		if now.Sub(s.lastSeen) >= sw.opts.keyTTL {
			expired = append(expired, k)
		}
	}
	for _, k := range expired {
		delete(sw.buckets, k)
	}
}
