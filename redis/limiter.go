package redis

import (
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/themarchrain/kaka"
)

// ErrRedisUnavailable 表示底层 Redis 连接/执行失败。
var ErrRedisUnavailable = fmt.Errorf("kaka/redis: redis unavailable")

// ErrScript 表示 Lua 脚本执行异常。
var ErrScript = fmt.Errorf("kaka/redis: script error")

// limiter 是三算法共享的底座：key 前缀、TTL、错误语义。
// 具体算法（M2-M4）包装它并实现 kaka.Limiter 的 Allow。
type limiter struct {
	client *redis.Client
	opts   Options
}

func newLimiter(client *redis.Client, opts ...Option) *limiter {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	return &limiter{client: client, opts: o}
}

// keyFor 拼接业务 key 与配置前缀。
func (l *limiter) keyFor(key string) string {
	return l.opts.keyPrefix + key
}

// onError 上报错误；未注册回调则忽略。
func (l *limiter) onError(err error) {
	if l.opts.onError != nil {
		l.opts.onError(err)
	}
}

// fallback 按 ErrorPolicy 返回降级结果（同时上报错误）。
func (l *limiter) fallback(err error) kaka.Result {
	l.onError(err)
	switch l.opts.policy {
	case ErrorFailOpen:
		return kaka.Result{Allowed: true}
	default:
		return kaka.Result{Allowed: false}
	}
}
