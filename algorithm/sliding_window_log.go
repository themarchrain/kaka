package algorithm

import (
	"sync"
	"time"
)

type SlidingWindow struct {
	mu     sync.Mutex
	limit  int           // 窗口内允许的最大请求数
	window time.Duration // 窗口大小 (如 1 * time.Second)
	logs   []time.Time   // 记录请求时间戳的日志切片
}

func NewSlidingWindow(limit int, window time.Duration) *SlidingWindow {
	return &SlidingWindow{
		limit:  limit,
		window: window,
		logs:   make([]time.Time, 0),
	}
}

func (sw *SlidingWindow) Allow() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-sw.window)

	// 查找第一个在窗口内的时间戳索引
	validIndex := len(sw.logs) // 默认全部过期
	left, right := 0, len(sw.logs)-1
	for left <= right {
		mid := left + ((right - left) >> 1)
		if sw.logs[mid].After(windowStart) {
			validIndex = mid
			right = mid - 1
		} else {
			left = mid + 1
		}
	}
	// 从有效索引开始截取时间戳
	sw.logs = sw.logs[validIndex:]

	// 判断当前有效窗口内的请求数是否达到上限
	if len(sw.logs) < sw.limit {
		// 未超限，追加当前请求时间，放行
		sw.logs = append(sw.logs, now)
		return true
	}

	// 超限，拒绝
	return false
}
