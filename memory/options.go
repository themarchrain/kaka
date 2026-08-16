package memory

import (
	"errors"
	"time"

	"github.com/themarchrain/kaka"
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
	sink            kaka.MetricSink
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
	if o.shards < 1 || o.shards > maxShardCount || o.shards&(o.shards-1) != 0 {
		panic("memory: shardCount must be a power of two in [1, 1024]")
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

// WithShardCount sets the number of internal shards (striped locks) used by
// the key store. Must be a power of two in [1, 1024]; default 64. More shards
// reduce lock contention when many keys are accessed concurrently, at a small
// memory cost for the extra maps.
func WithShardCount(n int) Option {
	return func(o *options) { o.shards = n }
}

// WithMetricSink registers a sink that receives decision events (allowed,
// rejected, errors, key count, evictions). Zero value (nil) disables
// reporting with no hot-path cost.
func WithMetricSink(sink kaka.MetricSink) Option {
	return func(o *options) { o.sink = sink }
}
