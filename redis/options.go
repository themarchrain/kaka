package redis

import "time"

// ErrorPolicy 决定底层 Redis 错误时限流器的降级行为。
type ErrorPolicy int

const (
	// ErrorFailClosed 限流失败按拒绝处理（默认，安全优先）。
	ErrorFailClosed ErrorPolicy = iota
	// ErrorFailOpen 限流失败按放行处理（可用性优先）。
	ErrorFailOpen
)

// Options 保存 redis 限流器的可配置项。
type Options struct {
	keyPrefix string
	keyTTL    time.Duration
	policy    ErrorPolicy
	onError   func(err error)
}

// Option 是 functional option。
type Option func(*Options)

func defaultOptions() Options {
	return Options{
		keyPrefix: "kaka:",
		keyTTL:    30 * time.Minute,
		policy:    ErrorFailClosed,
	}
}

// WithKeyPrefix 设置 Redis key 前缀（默认 "kaka:"）。
func WithKeyPrefix(prefix string) Option {
	return func(o *Options) { o.keyPrefix = prefix }
}

// WithKeyTTL 设置每个限流 key 的过期时间（默认 30 分钟；<=0 表示不设置 TTL）。
func WithKeyTTL(ttl time.Duration) Option {
	return func(o *Options) { o.keyTTL = ttl }
}

// WithErrorPolicy 设置底层错误时的降级行为（默认 ErrorFailClosed）。
func WithErrorPolicy(policy ErrorPolicy) Option {
	return func(o *Options) { o.policy = policy }
}

// WithOnError 注册错误回调（无论何种 policy 都会被调用；nil 函数体安全）。
func WithOnError(fn func(err error)) Option {
	return func(o *Options) { o.onError = fn }
}
