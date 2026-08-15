package memory

// EvictionPolicy defines how new keys are handled when maxKeys is reached.
type EvictionPolicy int

const (
	// EvictReject rejects new keys with ErrMaxKeysExceeded (default; matches the zero value).
	EvictReject EvictionPolicy = iota
	// EvictLRU evicts the least recently used key to make room for a new one.
	EvictLRU
)

// WithEvictionPolicy sets the policy applied when maxKeys is reached.
// The default is EvictReject: new keys return an error once the limit is reached.
func WithEvictionPolicy(p EvictionPolicy) Option {
	return func(o *options) { o.eviction = p }
}
