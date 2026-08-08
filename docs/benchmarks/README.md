# Benchmarks & Verification

Performance, resource-usage, and correctness evidence for Kaka, all reproducible from this repository.

| Report | What it covers | Reproduce |
|---|---|---|
| [01. Benchmark comparison](01-benchmark-comparison.md) | Throughput & allocations vs `x/time/rate`, `juju`, `uber` | `sh scripts/bench.sh` |
| [02. HTTP load tests](02-http-load-test.md) | End-to-end QPS / latency / error rate (single & multi key) | `sh scripts/loadtest.sh` |
| [03. Resource usage](03-resource-usage.md) | Memory per key, allocation rate, GC, long-term stability | commands inside |
| [04. Correctness verification](04-correctness-verification.md) | Request-by-request differential tests + algorithm invariants | commands inside |

## Key results

- **Hot path:** 0 allocations for all three algorithms; token bucket runs at
  1.34–1.37× `x/time/rate` (same-round relative, within the ≤2× budget).
- **End-to-end:** 97.4% of `x/time/rate`'s QPS through HTTP middleware, with
  lower tail latency (p99) and identical allow/deny behavior.
- **Memory:** per-key cost is 1/7–1/61 of `ulule/limiter`'s memory store;
  1M requests over 10 minutes holds memory flat (~3 MB).
- **Correctness:** token bucket decisions match `x/time/rate` and `juju`
  request-by-request, including float-sensitive boundary scenarios.

## Reproduce

```sh
sh scripts/bench.sh          # algorithm-level comparison
sh scripts/loadtest.sh       # HTTP load test (requires hey)
sh scripts/loadtest-long.sh  # long-term stability (default 10 min)
```

## Test machine

13th Gen Intel Core i7-13620H / Windows 11 / Go 1.25.1 / hey v0.1.5.
Relative ratios are the reliable metric; absolute numbers vary with machine
load. Raw data is kept locally — re-run the scripts to regenerate it.
