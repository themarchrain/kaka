package redis

import "time"

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
