package memory

import (
	"errors"
	"time"
)

// ErrMaxKeysExceeded 达到 maxKeys 上限且请求的是新 key 时返回
var ErrMaxKeysExceeded = errors.New("memory: max keys exceeded")

// cleanupBatchSize 单次惰性清理最多扫描的 key 数量
const cleanupBatchSize = 100

type Option func(*options)

// options 限流器内部通用配置
type options struct {
	maxKeys         int
	keyTTL          time.Duration
	cleanupInterval time.Duration
	clock           clock
}

func defaultOptions() options {
	return options{
		clock: realClock{},
	}
}

func (o *options) applyDefaults() {
	if o.keyTTL > 0 && o.cleanupInterval == 0 {
		o.cleanupInterval = time.Minute
	}
}

func (o *options) validate() {
	if o.maxKeys < 0 {
		panic("memory: maxKeys must be >= 0")
	}
	if o.keyTTL < 0 {
		panic("memory: keyTTL must be >= 0")
	}
	if o.cleanupInterval < 0 {
		panic("memory: cleanupInterval must be >= 0")
	}
	if o.clock == nil {
		panic("memory: clock must not be nil")
	}
}

// WithMaxKeys 设置最大 key 数量
// 0 表示不限制（默认）
// 达到上限后新 key 会返回 error
func WithMaxKeys(n int) Option {
	return func(o *options) { o.maxKeys = n }
}

// WithKeyTTL 设置 key 空闲过期时间
// 0 表示不按空闲时间清理（默认）
func WithKeyTTL(d time.Duration) Option {
	return func(o *options) { o.keyTTL = d }
}

// WithCleanupInterval 设置惰性清理扫描间隔
// 0 表示不自动清理（默认）
// 设置了 keyTTL 但未设置 cleanupInterval 时，默认 1 分钟
func WithCleanupInterval(d time.Duration) Option {
	return func(o *options) { o.cleanupInterval = d }
}
