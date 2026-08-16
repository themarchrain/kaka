package redis

import (
	"time"

	"github.com/themarchrain/kaka"
)

// ErrorPolicy decides how the limiter degrades when an underlying Redis call fails.
type ErrorPolicy int

const (
	// ErrorFailClosed denies the request on failure (default; safety first).
	ErrorFailClosed ErrorPolicy = iota
	// ErrorFailOpen allows the request on failure (availability first).
	ErrorFailOpen
)

// Options holds the configurable settings of a Redis limiter.
type Options struct {
	keyPrefix string
	keyTTL    time.Duration
	policy    ErrorPolicy
	onError   func(err error)
	sink      kaka.MetricSink
}

// Option configures a Redis limiter.
type Option func(*Options)

func defaultOptions() Options {
	return Options{
		keyPrefix: "kaka:",
		keyTTL:    30 * time.Minute,
		policy:    ErrorFailClosed,
	}
}

// WithKeyPrefix sets the Redis key prefix (default "kaka:").
func WithKeyPrefix(prefix string) Option {
	return func(o *Options) { o.keyPrefix = prefix }
}

// WithKeyTTL sets the per-key expiry (default 30 minutes; <= 0 disables the TTL).
func WithKeyTTL(ttl time.Duration) Option {
	return func(o *Options) { o.keyTTL = ttl }
}

// WithErrorPolicy sets the degradation behavior on underlying errors (default ErrorFailClosed).
func WithErrorPolicy(policy ErrorPolicy) Option {
	return func(o *Options) { o.policy = policy }
}

// WithOnError registers an error callback, called regardless of the policy (safe with a nil function).
func WithOnError(fn func(err error)) Option {
	return func(o *Options) { o.onError = fn }
}

// WithMetricSink registers a sink that receives decision events. Zero value
// (nil) disables reporting with no hot-path cost. Sink errors are counted in
// addition to (not instead of) the WithOnError callback. Under ErrorFailOpen
// a single degraded request emits both OnError and OnAllowed (the error and
// the final decision are reported independently).
func WithMetricSink(sink kaka.MetricSink) Option {
	return func(o *Options) { o.sink = sink }
}
