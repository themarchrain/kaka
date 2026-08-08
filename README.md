# Kaka

Kaka is a small Go rate limiting library built around one core interface:

```go
type Limiter interface {
    Allow(ctx context.Context, key string) (Result, error)
}
```

The current implementation focuses on stable in-memory limiters and thin HTTP framework adapters. Redis, metrics, dashboards, and management APIs are planned as future layers, but they should stay behind the same `Limiter` / `Result` contract.

## Current Features

- In-memory token bucket, leaky bucket, and sliding window log limiters.
- Per-key state lifecycle controls with `WithMaxKeys`, `WithKeyTTL`, and `WithCleanupInterval`.
- Gin middleware adapter in `middleware/gin`.
- Standard library `net/http` middleware adapter in `middleware/http`.
- Shared behavior tests, CI coverage, and a local test script for the module matrix.

The first stable-cut milestone is tagged `v0.0.1`: the core `Limiter` /
`Result` contract, memory algorithms, lifecycle controls, and the gin / http
adapters are frozen; follow-up releases only add capability, never change the
contract.

## Basic Usage

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

## In-Memory Options

```go
limiter := memory.NewTokenBucket(
    100,
    10,
    memory.WithMaxKeys(10000),
    memory.WithKeyTTL(time.Hour),
    memory.WithCleanupInterval(time.Minute),
)
```

`maxKeys` protects local memory from unbounded key growth. When it is reached, new keys return an error while existing keys continue to use their own limiter state.

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

## Performance Verification

Kaka publishes reproducible performance evidence comparing its in-memory
limiters against mainstream Go rate limiters:

- `golang.org/x/time/rate` (token bucket)
- `github.com/uber-go/ratelimit` (leaky bucket)
- `github.com/juju/ratelimit` (token bucket)

Benchmark and HTTP load-test results, methodology, and conclusions live in
`docs/superpowers/benchmarks/` (local working documents, not part of the git
repository). Run them yourself:

```sh
sh scripts/bench.sh      # Go benchmark comparison
sh scripts/loadtest.sh   # HTTP load test with hey
```

Reports are regenerable with the scripts above; raw data and conclusions are
kept out of git by design.

## Testing

Run the local CI-style matrix:

```powershell
.\scripts\test.ps1
```

```sh
sh ./scripts/test.sh
```

Run the matrix plus root-module race tests when the local Go toolchain supports it:

```powershell
.\scripts\test.ps1 -Race
```

```sh
sh ./scripts/test.sh --race
```

On some Windows environments, race tests may fail because of cgo or race runtime toolchain limitations. The GitHub Actions workflow runs race tests on Linux with cgo enabled.
