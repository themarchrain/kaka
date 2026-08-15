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
//   - A local ErrMaxKeysExceeded (the local key store is full) falls
//     through to the remote layer instead of being returned.
//   - A blank key returns ErrInvalidKey without consulting either layer.
//
// Limiter is stateless and safe for concurrent use; each layer applies its
// own synchronization.
type Limiter struct {
	local  kaka.Limiter
	remote kaka.Limiter
}

// New creates a two-tier limiter that consults local first and remote
// second. It panics if either argument is nil.
func New(local, remote kaka.Limiter) *Limiter {
	if local == nil {
		panic("layered: local limiter must not be nil")
	}
	if remote == nil {
		panic("layered: remote limiter must not be nil")
	}
	return &Limiter{local: local, remote: remote}
}

// Allow reports whether key is permitted. It returns ErrInvalidKey when the
// key is empty or blank. See the Limiter type documentation for the
// two-tier decision rules.
func (l *Limiter) Allow(ctx context.Context, key string) (kaka.Result, error) {
	if err := validateKey(key); err != nil {
		return kaka.Result{}, err
	}

	localResult, err := l.local.Allow(ctx, key)
	if err != nil {
		// Local errors (e.g. memory.ErrMaxKeysExceeded) mean the local
		// layer cannot serve this key; fall through to the authoritative
		// remote layer instead of surfacing a local-only failure.
		return l.remote.Allow(ctx, key)
	}
	if !localResult.Allowed {
		return localResult, nil
	}
	return l.remote.Allow(ctx, key)
}
