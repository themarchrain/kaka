# 07. Layered Rate Limiting (Local Pre-check + Redis Authority)

> Measured: Go 1.25.1 · Redis at 127.0.0.1:6379 (local WSL2 instance,
> same instance verified by matching run_id) · Intel i7-13620H · 2026-08-15
> Reproduce: see commands at the bottom

## Conclusion

- **Rejection floods are absorbed locally**: the layered reject path costs
  **27.7 ns/op** versus **229 µs/op** for a pure Redis deny — an **~8,200×**
  speedup, far beyond the 10× acceptance target. Redis is not contacted at
  all when the local layer denies.
- **Zero allocation on the short-circuit path**: 0 B/op, 0 allocs/op
  (regression-guarded by `TestLocalRejectPath_ZeroAllocations` in CI).
- **No overhead on the allow path**: layered ~227 µs vs pure Redis ~233 µs
  per allowed request (ratio 0.98×, within noise) — the ~28 ns local check
  is invisible next to the network round trip. Acceptance target was ≤ +10%.
- **Deny-heavy throughput**: ~36M decisions/s (layered) vs ~4.4k decisions/s
  (pure Redis) — ~8,200×, far beyond the 5× target. The layered limiter
  converts a Redis-bound rejection flood into a local in-memory operation.
- The local short-circuit is only ~2.6 ns slower than the pure in-memory
  reject path (27.7 vs 25.2 ns/op) — layering adds one interface call and a
  key validation, nothing else.

## Measured numbers (median of 3 runs, `-benchmem`)

| Benchmark                          | ns/op    | B/op | allocs/op |
|------------------------------------|----------|------|-----------|
| LayeredLocalReject                 | 27.7     | 0    | 0         |
| LayeredLocalRejectParallel         | 55.2     | 0    | 0         |
| LayeredRejectHeavyViaRedis         | 27.8     | 0    | 0         |
| LayeredAllowViaRedis               | 226.8µs  | 408  | 11        |
| RedisRejectHeavy (pure)            | 229.2µs  | 406  | 10        |
| RedisTokenBucket (pure, allow)     | 232.7µs  | 400  | 11        |
| KakaTokenBucketRejectPath (memory) | 25.2     | 0    | 0         |

Allocs on the Redis paths are go-redis internals (encoding/decoding); the
layered library code adds none on either path.

## End-to-end HTTP (bench-server + hey, 10 s at 64 connections)

Same middleware, same 100/10 token bucket, fixed key `global` — the reject
flood (rate 10/s against ~140k req/s) is what the local layer absorbs:

| Impl            | QPS      | allowed / denied |
|-----------------|----------|------------------|
| Pure Redis      | 25,944   | 199 / 259,310    |
| Layered         | 140,832  | 168 / 999,832    |
| Ratio           | **5.4×** | —                |

Even through the HTTP stack the layered limiter holds the ≥ 5× deny-heavy
target (spec P4). The pure-Redis number is go-redis pool throughput under
parallel round trips; the layered number is the same middleware with the
denial flood short-circuited in memory.

## Acceptance mapping (spec v0.2.0 §9)

| Criterion | Target | Measured | Verdict |
|-----------|--------|----------|---------|
| P1 reject path vs pure Redis | ≥ 10× | ~8,200× | PASS |
| P2 reject path allocations   | 0      | 0        | PASS |
| P3 allow path overhead       | ≤ +10% | ~0% (0.98×) | PASS |
| P4 deny-heavy throughput     | ≥ 5×   | ~8,200× direct, 5.4× over HTTP | PASS |

## Semantics note

The reject-path numbers assume the local bucket is already drained (the
short-circuit is active). In mixed traffic the benefit scales with the
rejection ratio: an all-allowed workload saves no round trips (the remote
layer must confirm every allow), while an all-denied flood saves all of
them. This is the intended trade-off — the layered limiter never exceeds
the remote limiter's allowance, so the distributed quota holds in every
traffic mix.

## Reproduce

```bash
# any local Redis (here: the local WSL2 redis at 127.0.0.1:6379)

# 1. Direct benchmarks (limiter level)
cd benchmarks
go test -run='^$' -bench='Layered|RedisTokenBucket|RedisRejectHeavy|KakaTokenBucketRejectPath' -benchmem -count=3 ./compare/

# 2. End-to-end HTTP (middleware level)
go build -o /tmp/bs.exe ./cmd/bench-server
REDIS_ADDR=127.0.0.1:6379 /tmp/bs.exe --impl redis --addr :8082 &
REDIS_ADDR=127.0.0.1:6379 /tmp/bs.exe --impl layered --addr :8083 &
hey -n 20000 -c 64 -z 10s http://localhost:8082/api/test   # pure redis
hey -n 20000 -c 64 -z 10s http://localhost:8083/api/test   # layered
```
