# Kaka

[![CI](https://github.com/themarchrain/kaka/actions/workflows/ci.yml/badge.svg)](https://github.com/themarchrain/kaka/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/themarchrain/kaka.svg)](https://pkg.go.dev/github.com/themarchrain/kaka)
[![Go Version](https://img.shields.io/badge/go-1.25-blue)](https://go.dev/dl/)

Kaka is a small, framework-agnostic rate limiting library for Go, built around one core interface:

```go
type Limiter interface {
    Allow(ctx context.Context, key string) (Result, error)
}
```

**One contract, three algorithms.** Kaka ships token bucket, leaky bucket, and
sliding window log limiters behind the same interface — swap strategies without
touching your call sites.

**Per-key state, managed for you.** Unlike global single-limiter libraries,
Kaka isolates state per key and handles the lifecycle (`maxKeys` cap, `keyTTL`
expiry, lazy batched cleanup) so you don't have to build a map with cleanup
yourself. That lifecycle management is verified end-to-end: 10 minutes of
load at 1M requests holds memory flat at ~3 MB.

**Verified against the ecosystem.** Correctness is checked request-by-request
against `golang.org/x/time/rate` and `juju/ratelimit` (differential testing),
and the hot path runs at **0 allocations**. See the
[performance & correctness reports](docs/benchmarks/).

## Features

- Three in-memory algorithms: token bucket, leaky bucket, sliding window log.
- Per-key isolation with `WithMaxKeys`, `WithKeyTTL`, `WithCleanupInterval`.
- `WithEvictionPolicy`: LRU eviction when the key cap is reached, instead of rejecting new keys.
- Zero-allocation hot path for all three algorithms.
- `net/http` middleware adapter in `middleware/http`.
- Gin middleware adapter in `middleware/gin`.
- Shared behavior tests, CI coverage, and a reproducible benchmark/load-test suite.

## Quick Start

```go
package main

import (
    "context"
    "fmt"

    "github.com/themarchrain/kaka/memory"
)

func main() {
    limiter := memory.NewTokenBucket(10, 2)

    result, err := limiter.Allow(context.Background(), "user:42")
    if err != nil {
        panic(err)
    }

    fmt.Println(result.Allowed, result.Remaining, result.RetryAfter)
}
```

## Per-key Lifecycle

```go
limiter := memory.NewTokenBucket(
    100,
    10,
    memory.WithMaxKeys(10000),     // cap the number of tracked keys
    memory.WithKeyTTL(time.Hour),  // idle keys expire and are cleaned up lazily
    memory.WithCleanupInterval(time.Minute),
)
```

By default, when the cap is reached new keys are rejected with
`ErrMaxKeysExceeded`. To evict the least recently used key instead of
rejecting, opt in:

```go
limiter := memory.NewTokenBucket(
    100,
    10,
    memory.WithMaxKeys(10000),
    memory.WithEvictionPolicy(memory.EvictLRU), // evict least-recently-used key for new keys
)
```

Note: with `EvictLRU`, an evicted key starts fresh (full bucket) when it
returns — its previous rate-limit state is gone. Use the default reject
policy when strict limit semantics must not be bypassed by eviction.

Existing keys keep working when the cap is reached; only new keys are rejected.

## HTTP Middleware

```go
handler := httpmiddleware.Middleware(httpmiddleware.Config{
    Limiter: limiter,
})(mux)
```

See `examples/http-example` for a runnable standard library server.

## Gin Middleware

```go
router.Use(ginmiddleware.NewLimiterMiddleware(ginmiddleware.Config{
    Limiter: limiter,
}))
```

See `examples/gin-example` for a runnable Gin server.

## Documentation

- [Performance & correctness reports](docs/benchmarks/) — benchmarks, load
  tests, resource usage, and differential correctness verification.

## Redis (Distributed)

The `redis` submodule provides the same contract backed by Redis state.
Every decision happens in a single atomic Lua script (server time, per-key
TTL, rollback-guarded); no per-instance clocks or GET/compute/SET races.

```go
import (
    "github.com/redis/go-redis/v9"
    redislimiter "github.com/themarchrain/kaka/redis"
)

client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})

tb := redislimiter.NewTokenBucket(client, 100, 10)          // 100 tokens, refill 10/s
sw := redislimiter.NewSlidingWindow(client, 100, time.Minute) // 100 req / minute
lb := redislimiter.NewLeakyBucket(client, 100, 10)          // capacity 100, leak 10/s
```

Options: `WithKeyPrefix`, `WithKeyTTL`, `WithErrorPolicy`, `WithOnError`.
On Redis failure the limiter degrades per policy — `ErrorFailClosed`
(default) denies, `ErrorFailOpen` allows — and the error is always passed
to `WithOnError`. Contract tests are shared with the in-memory
implementations.

Differences vs in-memory: server time (millisecond precision), one atomic
Lua round-trip per request (~0.1 ms), and clock-rollback guards in
token/leaky bucket. See [06. Redis distributed](docs/benchmarks/06-redis.md)
for measured cost and semantics.

## Testing

Run the local CI-style matrix:

```powershell
.\scripts\test.ps1
```

```sh
sh ./scripts/test.sh
```

Race tests run on Linux via GitHub Actions (local Windows environments may
lack the cgo/race toolchain; run `.\scripts\test.ps1 -Race` when supported).

## Roadmap

Redis-backed limiters, metrics, and management APIs are planned as future
layers — behind the same `Limiter` / `Result` contract.
