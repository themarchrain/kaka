# 01. Benchmark Comparison
> Charts: [01_algorithm_comparison.png](charts/01_algorithm_comparison.png)

Compares Kaka's in-memory limiters with `golang.org/x/time/rate` (token
bucket), `github.com/juju/ratelimit` (token bucket), and
`go.uber.org/ratelimit` (leaky bucket) under identical parameters.

**Machine:** 13th Gen Intel Core i7-13620H / Windows 11 / Go 1.25.1.
Median of 3 runs (`-benchmem`); **relative ratios are the reliable metric** —
absolute ns/op varies with machine load.

## Token bucket (burst=100, rate=10 — ns/op, allocs/op)

| Benchmark | Kaka | x/time/rate | juju | Kaka / x-time |
|---|---|---|---|---|
| single key | 72.3, **0** | 53.8, 0 | 33.3, 0 | **1.34×** |
| 1000-key round-robin | 150.3, 1 | 53.5¹ | 32.0¹ | per-key scaling |
| high burst (10000/1000) | 73.2, **0** | 53.3, 0 | 31.8, 0 | **1.37×** |
| reject path | 69.8, **0** | 51.8, 0 | 30.7, 0 | O(1) |

¹ Global single limiter — no per-key concept; shown as call-overhead reference.

## Leaky bucket (ns/op, allocs/op)

| Benchmark | Kaka | uber² |
|---|---|---|
| single key | 26.6, **0** | — |
| 1000-key round-robin | 72.4, 1 | — |
| `Take` call overhead | — | 19.2, 0 |

² `uber` is a blocking limiter (no rejection semantics); only call overhead is
comparable. Results are not rate-limit-equivalent.

## Sliding window log (ns/op, allocs/op) — no comparable library

| Benchmark | Kaka |
|---|---|
| single key | 91.5, **0** (43.0 on an idle machine) |
| 1000-key round-robin | 176.8, 1 |
| near-capacity compaction | 93.0, **0** |

## Concurrency

RunParallel (16 threads, same key): 61–80 ns/op with no scaling collapse.

## Results

- **0 allocations** on the hot path for all three algorithms.
- Token bucket stays within **1.34–1.37×** of the official `x/time/rate`
  (≤2× budget); the larger gap to `juju` (2.17×) reflects `juju`'s minimal
  single-bucket design (no per-key state, no lifecycle), not a defect.
- Reject path is O(1) — denial costs the same as allowance.
- Per-key scaling (1000 keys) adds only constant-level allocations.

## Reproduce

```sh
sh scripts/bench.sh
```
