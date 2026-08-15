# 02. HTTP Load Tests
> Charts: [02_http_qps.png](charts/02_http_qps.png) · [02_http_latency_percentiles.png](charts/02_http_latency_percentiles.png)

End-to-end measurements through real HTTP middleware: `benchmarks/cmd/bench-server`
with a token bucket (burst=100, rate=10/s), hammered with hey.

**Machine:** 13th Gen Intel Core i7-13620H / Windows 11 / Go 1.25.1 / hey v0.1.5.
`-n 100000 -c 100`, CSV output.

## Single key — Kaka vs x/time/rate middleware

Both use a fixed global key so the comparison is apples-to-apples (a
per-connection key would give each keep-alive connection its own fresh bucket).

| Metric | Kaka | x/time/rate |
|---|---|---|
| QPS | **1220** | 1252 |
| p50 / p95 / p99 (ms) | 0.50 / 2.60 / **3.90** | 0.50 / 2.70 / 4.10 |
| allowed / limited (200/429) | 108 / 99892 | 108 / 99892 |

## Multi key — Kaka `--keymode many` (10 000-key pool)

| Metric | single key | multi key |
|---|---|---|
| QPS | 1237 | 1223 (-1%) |
| p50 / p95 / p99 (ms) | 0.50 / 2.50 / 3.80 | 0.50 / 2.60 / 3.90 |
| allowed / limited | 108 / 99892 | 10795 / 89205 |

The 10 795 allowances equal the ~100 concurrent connections, each holding its
own bucket — an end-to-end demonstration of per-key isolation.

## Results

- **Behavior parity:** identical allow/deny counts (108 / 99892) vs
  `x/time/rate`.
- **Throughput:** 97.4% of `x/time/rate` — within noise.
- **Latency:** equal p50, lower p95/p99.
- **Per-key scaling:** multi-key throughput drops only 1% with flat latency.

## Reproduce

```sh
sh scripts/loadtest.sh   # requires hey: go install github.com/rakyll/hey@latest
```
