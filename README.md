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
- Sharded store (`WithShardCount`): striped locks, so concurrent requests on different keys do not serialize on one mutex.
- `WithMetricSink`: opt-in observability (allowed/rejected/error/key-count/eviction, tiered for layered) with zero overhead when unused.
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

Concurrency: each limiter shards its key store across 64 striped locks
(`WithShardCount`, a power of two), so concurrent requests on different keys
do not serialize on a single mutex. `maxKeys` stays exact across shards;
`EvictLRU` eviction is approximate (each shard evicts from its own LRU list,
so the evicted key may differ from the globally least-recently-used one,
while the total key count stays bounded by `maxKeys`).

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
  tests, resource usage, and differential correctness verification. Every
  report has companion charts rendered from the raw measurement artifacts
  (see the [charts index](docs/benchmarks/README.md#charts)); regenerate
  everything with `sh scripts/visualize.sh` (requires Python + matplotlib +
  pandas, see `scripts/visualize/requirements.txt`).

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

## Layered Rate Limiting

For deployments that need Redis' distributed quota but want to absorb
rejection floods locally, the `layered` module composes an in-memory limiter
(local pre-check) with a Redis limiter (authoritative remote layer):

```go
import (
    "github.com/themarchrain/kaka/layered"
    "github.com/themarchrain/kaka/memory"
    redislimiter "github.com/themarchrain/kaka/redis"
)

local := memory.NewTokenBucket(100, 10)
remote := redislimiter.NewTokenBucket(client, 100, 10)
limiter := layered.New(local, remote)
```

Requests denied by the local layer never touch Redis — denial costs
nanoseconds instead of a network round trip — while every allowed request
is still confirmed by the remote layer, so the distributed quota is never
exceeded. Measured: ~8,200× faster denial path, zero allocations, no
overhead on the allow path (see
[07. Layered](docs/benchmarks/07-layered.md)).

The semantics are approximate: the local layer drifts stricter than the
remote layer over time, and a local denial can overestimate `RetryAfter`.
Use the same algorithm and parameters for both layers, give the local layer
a `WithMaxKeys` bound and `WithKeyTTL` for lifecycle management, and
`go get github.com/themarchrain/kaka/layered` to use it.

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

Completed:
- v0.1.0: Redis-backed limiters (atomic Lua scripts, distributed)
- v0.1.1: English API documentation
- v0.2.0: Layered rate limiting (local pre-check + Redis authority)
- v0.3.0: Benchmark visualization (Python chart pipeline, one-shot regenerate)
- v0.4.0: Sharded store (striped locks, multi-key concurrency)
- v0.4.1: Metrics observability (MetricSink, tiered layered reporting)

Planned: metrics and management APIs — behind the same `Limiter` / `Result`
contract.
