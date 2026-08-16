# 05. LRU Eviction
> Charts: [05_lru_throughput.png](charts/05_lru_throughput.png) · [05_lru_memory.png](charts/05_lru_memory.png)

`WithEvictionPolicy(EvictLRU)` — when the key cap is reached, evict the
least-recently-used key to make room for new ones, instead of rejecting.

**Result:** eviction keeps the hot path at **baseline speed with 0 extra
allocations**; the cost shows only in the pathological every-access-evicts
case and in ~65 B/key extra memory.

## Machine & method

13th Gen Intel Core i7-13620H / Windows 11 / Go 1.25.1.
Each benchmark runs **alone** (`-bench '<name>'` exact match, `-count=3`,
median). This matters: running many benchmarks in one process inflates
later results (GC heap growth) — LRU numbers measured that way come out
3–4× higher and are wrong.

## Throughput (TokenBucket, capacity=1000, rate=10, maxKeys=10000)

| Benchmark | Reject (baseline) | LRU eviction | Δ |
|---|---|---|---|
| Hot path, 10k-key round-robin | 101.0 ns/op | 101.8 ns/op | +0.8% |
| Concurrent (16 threads) | 157.0 ns/op | 176.6 ns/op | +12.5% |
| Thrash (maxKeys=100, every access evicts) | — | 218.0 ns/op | — |

All cases: 23 B/op, 1 allocs/op (key formatting) — **0 additional
allocations** on both hot and eviction paths (`LRUThrash` adds 152 B/5 allocs
per evicted key: list node + bucket creation).

## Memory (cumulative allocations per key, 100k keys)

| Store | B/key |
|---|---|
| `mapStore` (reject) | 158.2 |
| `lruStore` (EvictLRU) | 221.9 |
| **LRU overhead** | **+63.7** (list node + key copy) |

Stable across key counts (100 / 10k / 100k → +64 / +67 / +64 B/key).

## Semantics to be aware of

- An **evicted key starts fresh (full bucket)** when it returns — its
  previous rate-limit state is gone. With 100% accurate LRU, eviction only
  hits the least-recently-used key, so genuinely active keys are never reset.
- If strict limit semantics must not be bypassed by eviction, keep the
  default `EvictReject` policy.
- LRU eviction is orthogonal to `WithKeyTTL`: eviction manages space, TTL
  manages time; both can be enabled together.

## Reproduce

```sh
cd benchmarks
go test -run TestMemoryFootprintLRU -v ./compare/   # memory per key
for name in MaxKeysReject$ LRUEviction$ LRUThrash$ LRUEvictionParallel$; do
  go test -run '^$' -bench "$name" -benchmem -count=3 ./compare/
done
```
