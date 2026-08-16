# Benchmarks & Verification

Performance, resource-usage, and correctness evidence for Kaka, all reproducible from this repository.

| Report | What it covers | Reproduce |
|---|---|---|
| [01. Benchmark comparison](01-benchmark-comparison.md) | Throughput & allocations vs `x/time/rate`, `juju`, `uber` | `sh scripts/bench.sh` |
| [02. HTTP load tests](02-http-load-test.md) | End-to-end QPS / latency / error rate (single & multi key) | `sh scripts/loadtest.sh` |
| [03. Resource usage](03-resource-usage.md) | Memory per key, allocation rate, GC, long-term stability | commands inside |
| [04. Correctness verification](04-correctness-verification.md) | Request-by-request differential tests + algorithm invariants | commands inside |
| [05. LRU eviction](05-lru-eviction.md) | Cost of `WithEvictionPolicy(EvictLRU)`: throughput, memory, semantics | `go test -run TestMemoryFootprintLRU -v ./compare/` |
| [06. Redis distributed](06-redis.md) | Atomic Lua algorithms, latency / QPS baseline, end-to-end vs in-memory | commands inside |
| [07. Layered](07-layered.md) | Local short-circuit vs pure Redis deny, end-to-end layered | commands inside |
| [08. Multi-key concurrency](08-multikey-concurrency.md) | Sharded store vs global lock: multi-key throughput, speedup | `sh scripts/bench-multikey.sh` |

## Charts

All charts below are rendered from the raw artifacts in `docs/benchmarks/raw/`
(kept locally, not committed). Regenerate everything with:

```sh
sh scripts/visualize.sh        # rerun missing data, render all charts
sh scripts/visualize.sh --full # also rerun the 600 s long-term stability test
```

| Chart | Report | What it shows | Data source | Reproduce |
|---|---|---|---|---|
| `01_algorithm_comparison.png` | 01 | In-memory latency vs ecosystem (log) | `benchmark-*.txt` | `sh scripts/bench.sh` |
| `02_http_qps.png` | 02 | HTTP QPS kaka vs x/time/rate | `loadtest-*.csv` | `sh scripts/loadtest.sh` |
| `02_http_latency_percentiles.png` | 02 | p50/p95/p99 latency | `loadtest-*.csv` | `sh scripts/loadtest.sh` |
| `03_memory_per_key.png` | 03 | Bytes/key kaka vs ulule (log) | `memory-perkey-*.txt` | `sh scripts/bench-memory.sh` |
| `03_longterm_memory.png` | 03 | HeapAlloc over 10 min | `longterm-mem-*.csv` | `sh scripts/loadtest-long.sh 600` |
| `04_correctness_matrix.png` | 04 | Verification matrix (differential + invariants) | `correctness-*.txt` | `sh scripts/bench-memory.sh` |
| `05_lru_throughput.png` | 05 | LRU eviction cost | `lru-*.txt` | `sh scripts/bench-lru.sh` |
| `05_lru_memory.png` | 05 | LRU memory overhead | `memory-lru-*.txt` | `sh scripts/bench-memory.sh` |
| `06_redis_concurrency_sweep.png` | 06 | QPS vs concurrency (TB/LB/SW) | `benchmark-redis-sweep-*.txt` | `sh scripts/bench-redis.sh` |
| `06_redis_baseline.png` | 06 | Per-algorithm EVALSHA latency | `benchmark-redis-*.txt` | `sh scripts/bench-redis.sh` |
| `06_redis_probe.png` | 06 | GET vs EVALSHA vs ulule probes | `benchmark-redis-*.txt` | `sh scripts/bench-redis.sh` |
| `07_layered_reject_path.png` | 07 | Local short-circuit vs pure Redis deny | `benchmark-layered-*.txt` | `sh scripts/bench-layered.sh` |
| `07_layered_http.png` | 07 | End-to-end QPS layered vs redis | `loadtest-redis-*.csv`, `loadtest-layered-*.csv` | `sh scripts/bench-layered.sh` |
| `08_multikey_throughput.png` | 08 | Multi-key throughput, v0.3.0 vs v0.4.0 | `benchmark-multikey-*.txt` (+ archived baseline) | `sh scripts/bench-multikey.sh` |
| `08_multikey_speedup.png` | 08 | Speedup ratio per key set (auto-computed) | `benchmark-multikey-*.txt` (+ archived baseline) | `sh scripts/bench-multikey.sh` |

Charts live in `docs/benchmarks/charts/` (not committed; regenerate locally).
Every chart footer states its exact data-source file and reproduce command.

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
