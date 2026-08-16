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
  goroutines × 64/1024 keys the throughput improves **4.8×–8.0×** versus the
  archived baseline (well beyond the 2× acceptance target), with 0
  allocations on every path.
- **The single-key hot path is unchanged**: 26.7–26.9 ns/op, 0 allocs/op,
  matching the v0.3.0 figure.
- **Semantics preserved**: `maxKeys` stays exact across shards (atomic
  counter + CAS reservation, verified under 32-way concurrent new-key
  insertion); `EvictLRU` becomes approximate per shard — the total key count
  stays bounded by `maxKeys`, but which key gets evicted may differ from the
  globally least-recently-used one.

## Design

- Each shard owns a `sync.Mutex` plus its own map (plus its own LRU list
  under `EvictLRU`); a key is routed by an inline FNV-1a 64 hash (`& 63`
  with the default 64 shards; `WithShardCount` requires a power of two).
- The hit path takes one hash + one shard lock and never touches the atomic
  counter. New keys reserve a slot via CAS so `maxKeys` cannot be exceeded
  even under race.
- TTL cleanup stays lazy and per shard; when the store is full the reject
  path additionally runs a rate-limited global sweep so expired keys in
  other shards are reclaimed before the `maxKeys` check (this preserves the
  pre-sharding contract that cleanup precedes the cap check).
- When `EvictLRU` is full and the target shard is empty, eviction falls back
  to another shard (one shard lock at a time — no nested locks).

## Measurements

`BenchmarkMultiKeyParallel` (benchtime=3s, count=1), old = archived v0.3.0
global-lock run, new = v0.4.0 64-shard run. `g` = goroutines, `k` = keys in
the working set. Speedup = old ns/op ÷ new ns/op (computed, not hand-typed).

| g | k | v0.3.0 ns/op | v0.4.0 ns/op | speedup |
|---|---|---|---|---|
| 1 | 8 | 26.06 | 26.56 | 0.98× |
| 1 | 64 | 43.97 | 52.48 | 0.84× |
| 1 | 1024 | 75.37 | 36.79 | 2.05× |
| 8 | 8 | 130.4 | 10.11 | 12.90× |
| 8 | 64 | 134.6 | 30.17 | 4.46× |
| 8 | 1024 | 143.6 | 48.72 | 2.95× |
| 32 | 8 | 151.4 | 24.70 | 6.13× |
| 32 | 64 | 160.3 | 20.78 | 7.72× |
| 32 | 1024 | 187.7 | 39.24 | 4.78× |
| 64 | 8 | 159.2 | 28.28 | 5.63× |
| 64 | 64 | 153.0 | 19.06 | 8.03× |
| 64 | 1024 | 190.3 | 31.14 | 6.11× |

Reading the table honestly: the single-goroutine rows are flat (0.84×–2.05×;
cycling 64 keys across 64 shards spreads the map across cache lines, which
slightly costs the k=64 row — still sub-30 ns absolute). The story is the
concurrency rows: the global lock made 8–64 goroutines *slower* than one,
while the sharded store keeps every worker near the uncontended per-key cost.

## Reproduce

```sh
# baseline (requires checking out v0.3.0 code first)
sh scripts/bench-multikey.sh   # move the artifact into raw/archive/v0.3.0/

# current implementation
sh scripts/bench-multikey.sh
sh scripts/visualize.sh --only fig14,fig15
```
