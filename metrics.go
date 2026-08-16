package kaka

// Tier values for MetricSink methods: "" = standalone limiter,
// "local"/"remote" = the two layers of a layered limiter.
const (
	TierSingle = ""       // standalone limiter (memory / redis)
	TierLocal  = "local"  // layered: local pre-check layer
	TierRemote = "remote" // layered: authoritative remote layer
)

// MetricSink receives rate-limiter decision events. Implementations map
// these to a monitoring backend (Prometheus, statsd, log). All counters are
// low-cardinality: no per-key labels; tier is the only label dimension.
//
// Sink methods may be called concurrently from many goroutines and must be
// fast; implementations must be safe for concurrent use. They are invoked
// outside any internal limiter lock (memory drains evictions after the
// shard lock is released), so a sink may re-enter the limiter, but should
// avoid doing so on hot paths.
type MetricSink interface {
	// OnAllowed reports a request that was permitted.
	OnAllowed(tier string, result Result)
	// OnRejected reports a request that was denied.
	OnRejected(tier string, result Result)
	// OnError reports a limiter error (invalid key, maxKeys exceeded,
	// Redis failure, or a swallowed local-layer error in layered).
	OnError(tier string, err error)
	// SetKeys reports the current number of tracked keys (gauge).
	// memory only; called on every Allow with a valid key while a sink
	// is configured (blank-key errors return before the store is touched).
	SetKeys(tier string, n int)
	// OnEvict reports an LRU eviction (memory EvictLRU only).
	OnEvict(tier string)
}
