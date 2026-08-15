package kaka

import (
	"context"
	"time"
)

// Result is the outcome of an Allow call.
type Result struct {
	Allowed    bool          // Allowed reports whether the request was permitted.
	Remaining  int64         // Remaining is the remaining allowance for the key.
	RetryAfter time.Duration // RetryAfter is how long to wait before retrying; set only when denied.
}

// Limiter is the core rate limiter interface.
// Limiters identify distinct rate-limited objects by key.
type Limiter interface {
	Allow(ctx context.Context, key string) (Result, error)
}
