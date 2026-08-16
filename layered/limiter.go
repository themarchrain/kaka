package layered

import (
	"context"

	"github.com/themarchrain/kaka"
)

var _ kaka.Limiter = (*Limiter)(nil)

// Limiter is a two-tier rate limiter: a local layer that rejects requests
// without touching the remote layer, and a remote layer that makes the
// final decision for every request the local layer allows.
//
// Construct the two layers with the same algorithm and parameters for the
// closest approximation; the local layer is expected to be an in-memory
// limiter (kaka/memory) and the remote layer a distributed limiter
// (kaka/redis). Any kaka.Limiter implementation works in either position.
//
// Semantics:
//   - A request denied by the local layer is denied immediately and the
//     remote layer is not consulted.
//   - A request allowed by the local layer is passed to the remote layer,
//     whose Result is returned as-is. Allowed is therefore only ever true
//     when the remote layer allows, so the layered limiter never exceeds
//     the remote limiter's allowance.
//   - The local layer is approximate and drifts stricter than the remote
//     layer: a request the remote layer would allow may be denied locally,
//     and RetryAfter from a local denial can overestimate. This is the
//     intended trade-off for absorbing rejection floods locally.
//   - Any local-layer error (for example memory.ErrMaxKeysExceeded when the
//     local key store is full) falls through to the remote layer instead of
//     being returned: the local layer is an optimization, not a correctness
//     gate, so a local failure never blocks the authoritative decision.
//   - A blank key returns ErrInvalidKey without consulting either layer.
//
// Limiter is stateless and safe for concurrent use; each layer applies its
// own synchronization.
type Limiter struct {
	local  kaka.Limiter
	remote kaka.Limiter
	sink   kaka.MetricSink
}

// New creates a two-tier limiter that consults local first and remote
// second. It panics if either argument is nil.
func New(local, remote kaka.Limiter, opts ...Option) *Limiter {
	if local == nil {
		panic("layered: local limiter must not be nil")
	}
	if remote == nil {
		panic("layered: remote limiter must not be nil")
	}
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	return &Limiter{local: local, remote: remote, sink: o.sink}
}

// Option configures a layered limiter.
type Option func(*options)

type options struct {
	sink kaka.MetricSink
}

// WithMetricSink registers a sink that receives tiered decision events
// (TierLocal for local-layer outcomes, TierRemote for remote-layer
// outcomes). Zero value (nil) disables reporting with no hot-path cost.
func WithMetricSink(sink kaka.MetricSink) Option {
	return func(o *options) { o.sink = sink }
}

// Allow reports whether key is permitted. It returns ErrInvalidKey when the
// key is empty or blank. See the Limiter type documentation for the
// two-tier decision rules.
func (l *Limiter) Allow(ctx context.Context, key string) (kaka.Result, error) {
	if err := validateKey(key); err != nil {
		l.emitError(kaka.TierSingle, err)
		return kaka.Result{}, err
	}

	localResult, err := l.local.Allow(ctx, key)
	if err != nil {
		// The local layer is an optimization, not a correctness gate: any
		// local failure (e.g. memory.ErrMaxKeysExceeded) falls through to
		// the authoritative remote layer instead of surfacing a local-only
		// error. The failure is still observable via the sink.
		l.emitError(kaka.TierLocal, err)
		return l.remoteAllow(ctx, key)
	}
	if !localResult.Allowed {
		l.emit(kaka.TierLocal, localResult)
		return localResult, nil
	}
	return l.remoteAllow(ctx, key)
}

// remoteAllow consults the remote layer and reports its outcome.
func (l *Limiter) remoteAllow(ctx context.Context, key string) (kaka.Result, error) {
	remoteResult, err := l.remote.Allow(ctx, key)
	if err != nil {
		l.emitError(kaka.TierRemote, err)
		return remoteResult, err
	}
	l.emit(kaka.TierRemote, remoteResult)
	return remoteResult, nil
}

func (l *Limiter) emit(tier string, result kaka.Result) {
	if l.sink == nil {
		return
	}
	if result.Allowed {
		l.sink.OnAllowed(tier, result)
	} else {
		l.sink.OnRejected(tier, result)
	}
}

func (l *Limiter) emitError(tier string, err error) {
	if l.sink != nil {
		l.sink.OnError(tier, err)
	}
}
