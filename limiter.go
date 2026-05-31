package kaka

import (
	"context"
	"time"
)

// Result 请求放行结果
type Result struct {
	Allowed    bool          // 是否允许放行
	Remaining  int64         // 剩余的额度
	RetryAfter time.Duration // 等待重试时间（仅被限流生效）
}

// Limiter 限流器核心接口
// 限流器根据唯一 key 识别不同的限流对象
type Limiter interface {
	Allow(ctx context.Context, key string) (Result, error)
}
