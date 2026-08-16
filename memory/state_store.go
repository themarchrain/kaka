package memory

import (
	"time"

	"github.com/themarchrain/kaka"
)

// stateStore is the per-key state storage interface used by the limiters.
// Implementations must be safe for concurrent use.
type stateStore[T any] interface {
	getOrCreate(key string, now time.Time) (T, error)
	// withState runs fn with the key's state under the store's lock, so the
	// per-key mutation (refill math, log append) is serialized per key.
	withState(key string, now time.Time, fn func(T, time.Time) (kaka.Result, error)) (kaka.Result, error)
	len() int
}

// keyEntry is the stored entry for a single key (EvictReject mode).
type keyEntry[T any] struct {
	value    T
	lastSeen time.Time
}

// newStateStore builds the sharded store (the eviction policy decides
// whether each shard keeps an LRU list).
func newStateStore[T any](opts options, create func(time.Time) T) stateStore[T] {
	switch opts.eviction {
	case EvictReject, EvictLRU:
		return newShardedStore[T](opts, create)
	default:
		panic("memory: invalid eviction policy")
	}
}
