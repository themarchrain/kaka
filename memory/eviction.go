package memory

// EvictionPolicy defines how new keys are handled when maxKeys is reached.
type EvictionPolicy int

const (
	// EvictReject rejects new keys with ErrMaxKeysExceeded (default; matches the zero value).
	EvictReject EvictionPolicy = iota
	// EvictLRU evicts the least recently used key to make room for a new one.
	// With the sharded store the eviction target is approximate: each shard
	// keeps its own LRU order and evicts from its own list, so the exact key
	// evicted may differ from the globally least-recently-used one. The total
	// number of keys remains bounded by maxKeys exactly.
	EvictLRU
)

// WithEvictionPolicy sets the policy applied when maxKeys is reached.
// The default is EvictReject: new keys return an error once the limit is reached.
func WithEvictionPolicy(p EvictionPolicy) Option {
	return func(o *options) { o.eviction = p }
}
