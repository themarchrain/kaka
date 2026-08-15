# 07. Layered Rate Limiting (Local Pre-check + Redis Authority)

> Measured: Go 1.25.1 · Redis at 127.0.0.1:6379 (local, same endpoint for both sides) · Intel i7-13620H · 2026-08-15
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

## Acceptance mapping (spec v0.2.0 §9)

| Criterion | Target | Measured | Verdict |
|-----------|--------|----------|---------|
| P1 reject path vs pure Redis | ≥ 10× | ~8,200× | PASS |
| P2 reject path allocations   | 0      | 0        | PASS |
| P3 allow path overhead       | ≤ +10% | ~0% (0.98×) | PASS |
| P4 deny-heavy throughput     | ≥ 5×   | ~8,200× | PASS |

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
# any local Redis (here: redis:7-alpine via docker, exposed on 127.0.0.1:6379)
docker run -d --name kaka-bench-redis -p 6379:6379 redis:7-alpine

cd benchmarks
go test -run='^$' -bench='Layered|RedisTokenBucket|RedisRejectHeavy|KakaTokenBucketRejectPath' -benchmem -count=3 ./compare/
```
