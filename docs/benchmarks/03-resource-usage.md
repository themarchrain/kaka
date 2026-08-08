# 03. Resource Usage

Per-key memory, allocation rate, and long-term stability, compared with
`github.com/ulule/limiter/v3` — the only widely used Go limiter with a
per-key in-memory store.

**Machine:** 13th Gen Intel Core i7-13620H / Windows 11 / Go 1.25.1.

## Memory per key (HeapAlloc after GC, amortized)

| keys | Kaka | ulule | ratio |
|---|---|---|---|
| 100 | 3032 B | 3272 B | ~1:1 (fixed overhead) |
| 10 000 | **31 B** | 230 B | 7.4× lower |
| 100 000 | **3.3 B** | 201 B | 61× lower |

## Hot path (single key)

| Metric | Kaka | ulule |
|---|---|---|
| ns/op | **25.6** | 69.6 |
| allocs/op | **0** | 2 (24 B) |

## Storage model

| Aspect | Kaka | ulule | x/time/rate, juju, uber |
|---|---|---|---|
| per-key isolation | ✅ | ✅ | ❌ global single limiter |
| key cap | ✅ `maxKeys` (reject new) | ❌ TTL only | — |
| cleanup | lazy, batched (≤100/access) | background goroutine, periodic | — |
| hot-path allocs | 0 | 2 | 0 |
| bytes/key (100k keys) | 3.3 | 201 | — |

ulule uses counter-style limiting; this comparison targets the **storage
mechanism**, not algorithm throughput (see report 01).

## Long-term stability (10 min, 100 concurrent, 10k-key pool)

`maxKeys=100000` + `keyTTL=60s`, 1M requests, memory sampled every 10 s.

| Metric | Result |
|---|---|
| HeapAlloc start → end | 3.0 → 3.1 MB (**flat, no leak**) |
| HeapAlloc min / max | 2.2 / 4.4 MB |
| QPS | 1328 |
| p50 / p95 / p99 (ms) | 0.50 / 2.60 / 3.80 |
| allowed / limited | 17620 / 982380 |

`maxKeys` + `keyTTL` keep memory flat as requests accumulate; the GC pressure
comes from the HTTP layer (JSON responses, key concatenation), not the
limiting hot path (0 allocs, above).

## Reproduce

```sh
cd benchmarks && go test -run TestMemoryFootprintPerKey -v ./compare/
cd benchmarks && go test -run='^$' -bench='KakaTokenBucketHotPath|UluleHotPath' -benchmem ./compare/
sh scripts/loadtest-long.sh 600
```
