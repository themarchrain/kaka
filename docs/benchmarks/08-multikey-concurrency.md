# 08. Multi-key Concurrency (Sharded Store)

> Charts: [08_multikey_throughput.png](charts/08_multikey_throughput.png) · [08_multikey_speedup.png](charts/08_multikey_speedup.png)

> Measured: Go 1.25.1 · Intel i7-13620H · 2026-08-16 · same machine, same
> session (the v0.3.0 baseline is archived under `raw/archive/v0.3.0/`)
> Reproduce: see commands at the bottom

## Conclusion

- **v0.3.0 serializes every key on one limiter-level mutex**: multi-key
  throughput *degrades* as concurrency grows (151→190 ns/op at 32–64
  goroutines) — the lock contention outweighs any parallelism.
- **v0.4.0 (64 shards, striped locks) turns that around**: at 32/64
  goroutines × 64/1024 keys the throughput improves **12.7×–26.3×** versus
  the archived baseline (far beyond the 2× acceptance target), with 0
  allocations on every path.
- **The single-key hot path stays allocation-free**: 30.0–30.2 ns/op, 0
  allocs/op (`raw/benchmark-hotpath-20260816-113513.txt`, this session) —
  about +3 ns over the v0.3.0 figure (~27 ns/op), the price of serializing
  the per-key refill under the shard lock (see Design).
- **Semantics preserved**: `maxKeys` stays exact across shards (atomic
  counter + CAS reservation, verified under 32-way concurrent new-key
  insertion); `EvictLRU` becomes approximate per shard — the total key count
  stays bounded by `maxKeys`, but which key gets evicted may differ from the
  globally least-recently-used one. Same-key bursts are serialized per shard
  lock: a 64-way concurrent burst on one key allows exactly `limit`
  requests.

## Design

- Each shard owns a `sync.Mutex` plus its own map (plus its own LRU list
  under `EvictLRU`); a key is routed by an inline FNV-1a 64 hash (`& 63`
  with the default 64 shards; `WithShardCount` requires a power of two).
- The per-key state mutation (refill math, sliding-window log append) runs
  *inside* the shard lock via a store callback (`withState`, pre-bound at
  construction so the hot path stays 0 alloc). This preserves the
  same-key serialization the old per-limiter mutex provided.
- The hit path takes one hash + one shard lock and never touches the atomic
  counter. New keys reserve a slot via CAS so `maxKeys` cannot be exceeded
  even under race.
- TTL cleanup stays lazy and per shard; when the store is full the reject
  path additionally runs a rate-limited global sweep so expired keys in
  other shards are reclaimed before the `maxKeys` check (this preserves the
  pre-sharding contract that cleanup precedes the cap check; the sweep is
  gated by `cleanupInterval` like the old per-store cleanup was).
- When `EvictLRU` is full and the target shard is empty, eviction falls back
  to another shard (one shard lock at a time — no nested locks).

## Measurements

`BenchmarkMultiKeyParallel` (benchtime=3s, count=1), old = archived v0.3.0
global-lock run, new = v0.4.0 64-shard run. `g` = goroutines, `k` = keys in
the working set. Speedup = old ns/op ÷ new ns/op (computed, not hand-typed).

| g | k | v0.3.0 ns/op | v0.4.0 ns/op | speedup |
|---|---|---|---|---|
| 1 | 8 | 26.06 | 27.74 | 0.94× |
| 1 | 64 | 43.97 | 30.93 | 1.42× |
| 1 | 1024 | 75.37 | 37.99 | 1.98× |
| 8 | 8 | 130.4 | 5.96 | 21.87× |
| 8 | 64 | 134.6 | 11.73 | 11.48× |
| 8 | 1024 | 143.6 | 17.40 | 8.25× |
| 32 | 8 | 151.4 | 15.22 | 9.95× |
| 32 | 64 | 160.3 | 6.68 | 24.01× |
| 32 | 1024 | 187.7 | 14.61 | 12.85× |
| 64 | 8 | 159.2 | 13.65 | 11.66× |
| 64 | 64 | 153.0 | 5.82 | 26.28× |
| 64 | 1024 | 190.3 | 14.98 | 12.70× |

Reading the table honestly: the single-goroutine rows are flat (0.94×–1.98×;
one goroutine cycling many keys is noisy — the k=1024 row benefits from the
per-shard maps being 16× smaller). The story is the concurrency rows: the
global lock made 8–64 goroutines *slower* than one, while the sharded store
keeps every worker near the uncontended per-key cost and scales with cores
(per-op wall time drops below single-op latency under parallel workers).

## Reproduce

```sh
# baseline (requires checking out v0.3.0 code first)
sh scripts/bench-multikey.sh   # move the artifact into raw/archive/v0.3.0/

# current implementation
sh scripts/bench-multikey.sh
sh scripts/visualize.sh --only fig14,fig15
```
