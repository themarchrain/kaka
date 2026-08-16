package memory

import (
	"errors"
	"time"
)

// ErrMaxKeysExceeded is returned when maxKeys is reached and the requested key is new.
var ErrMaxKeysExceeded = errors.New("memory: max keys exceeded")

// cleanupBatchSize 单次惰性清理最多扫描的 key 数量
const cleanupBatchSize = 100

// Option configures a limiter at construction time.
type Option func(*options)

// options 限流器内部通用配置
type options struct {
	maxKeys         int
	keyTTL          time.Duration
	cleanupInterval time.Duration
	clock           clock
	eviction        EvictionPolicy
	shards          int
}

func defaultOptions() options {
	return options{
		clock:  realClock{},
		shards: defaultShardCount,
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
	if o.eviction < EvictReject || o.eviction > EvictLRU {
		panic("memory: invalid eviction policy")
	}
}

// WithMaxKeys sets the maximum number of tracked keys.
// Zero means unlimited (default). New keys return an error once the limit is reached.
func WithMaxKeys(n int) Option {
	return func(o *options) { o.maxKeys = n }
}

// WithKeyTTL sets how long an idle key is kept before it expires.
// Zero disables idle-time cleanup (default).
func WithKeyTTL(d time.Duration) Option {
	return func(o *options) { o.keyTTL = d }
}

// WithCleanupInterval sets how often lazy cleanup scans for expired keys.
// Zero uses the default behavior: no cleanup when keyTTL is unset, a 1-minute interval when keyTTL is set.
func WithCleanupInterval(d time.Duration) Option {
	return func(o *options) { o.cleanupInterval = d }
}
