# 06. Redis Distributed Rate Limiting

> Measured: 2026-08-09 · Go 1.25.1 · WSL Redis 7 (172.23.155.132) · Intel i7-13620H
> Reproduce: see commands at the bottom

## Conclusion

- All three algorithms are backed by a single atomic Lua round-trip
  (EVALSHA) — no GET/compute/SET race window across instances.
- Per-request latency is ~0.11–0.16 ms (network round-trip dominated),
  ~1600× the in-memory version — this is the physical cost of distributed
  decision making.
- Measured single-instance peak throughput (direct RunParallel, no HTTP):
  TokenBucket ~55k QPS, LeakyBucket ~44k QPS, SlidingWindow ~37k QPS,
  reached at 32–64 concurrent goroutines; adding concurrency beyond that
  does not scale (Redis executes scripts single-threaded).
- Redis server time is authoritative; clock-rollback guards prevent
  catch-up refill/leak that could bypass limits.

## Algorithm baseline (single request, one EVALSHA)

| Algorithm     | Latency  | Concurrent QPS (measured) | Allocs/op |
|---------------|----------|---------------------------|-----------|
| TokenBucket   | 114.5µs  | ~55k (peak @64)           | 11        |
| LeakyBucket   | 112.6µs  | ~44k (peak @32)           | 11        |
| SlidingWindow | 140.3µs  | ~37k (peak @64)           | 37        |

Hash-state algorithms (TB/LB) are equivalent; ZSet-based SWL is ~23%
heavier (ZREMRANGEBYSCORE + ZCARD + ZADD + ZRANGE). Allocs are go-redis
internal (encoding/decoding); library code adds none.

### Concurrency scan (direct, no HTTP)

Measured with `BenchmarkRedisConcurrencySweep`: GOMAXPROCS=1 +
`SetParallelism` sweep over 1/8/32/64/128/256 goroutines, client
`PoolSize=512` so Redis is the only bottleneck. QPS = 1e9 / ns-per-op.

| Goroutines | TB QPS  | LB QPS  | SWL QPS |
|------------|---------|---------|---------|
| 1          | 8.1k    | 7.8k    | 6.3k    |
| 8          | 39.8k   | 32.8k   | 29.9k   |
| 32         | 54.1k   | **43.6k** | 35.6k |
| 64         | **55.3k** | 42.4k | **36.7k** |
| 128        | 53.7k   | 39.4k   | 36.6k   |
| 256        | 46.8k   | 39.0k   | 36.3k   |

8 goroutines already reach ~70–80% of the peak; the peak sits at 32–64 and
then flattens or slightly drops — Redis executes Lua single-threaded, so
extra concurrency only adds queueing. These numbers measure pure decision
throughput (never-denied full-speed traffic); the end-to-end HTTP numbers
below are a different measure (application layer + real 429 traffic).

### Bottleneck attribution (why the HTTP number is low)

Same network path (Windows → WSL2 virtual NIC), direct go-redis:

| Probe                        | ns/op  | QPS   |
|------------------------------|--------|-------|
| GET single, 1 goroutine      | 109.6µs| 9.1k  |
| GET parallel, 64 goroutines  | 14.1µs | 70.8k |
| EVALSHA (TB script), 64      | 20.3µs | 49.2k |
| ulule/limiter Redis, 64      | 15.8µs | 63.3k |
| ulule/limiter Redis, single  | 134.1µs| —     |

- ~110µs per single request is the virtual-NIC round-trip (a bare GET
  costs the same); it disappears under concurrency once requests pipeline.
- The script is only ~44% slower than a single GET (4–6 commands + Lua
  interpretation) — the real Redis-side cost of this design.
- Against the mainstream alternative: our single-request latency is 17%
  lower than `ulule/limiter` (114.5µs vs 134.1µs); its fixed-window script
  is 14% faster at 64 concurrency (63.3k vs 55.3k QPS) because it issues
  fewer commands. Same magnitude — both sit at Redis's single-threaded Lua
  boundary.
- The end-to-end HTTP result (412 QPS) is therefore **not bounded by
  Redis**: the same path sustains ~49k EVALSHA QPS directly, and ulule
  scores 467 QPS on the identical HTTP load (±13%). The HTTP bottleneck
  lives in the client/server connection handling of the load test (hey
  100 connections → bench-server), not in the limiter or Redis.

## End-to-end (HTTP, hey 100k × 100 concurrency)

| Impl       | QPS  | p50   | p99   | Allowed/Total |
|------------|------|-------|-------|---------------|
| kaka       | 1248 | 0.5ms | 4.1ms | 108/99892     |
| xtime      | 1322 | 0.5ms | 4.0ms | 107/99893     |
| redis      | 412  | 2.3ms | 4.7ms | 124/99876     |
| ulule redis| 467  | 2.0ms | 5.0ms | 300/99700     |

Redis QPS is ~3× lower and p50 ~4.6× higher than the in-memory
implementations — the measurable cost of a network round-trip per
decision. The 200/429 distribution stays aligned (small delta is
millisecond server-time granularity). p99 converges because it is
dominated by hey connection scheduling at 100 concurrency.

**Authority comparison:** the mainstream Go Redis limiter (`ulule/limiter`
v3, fixed-window, same link) ends at 467 QPS under the identical load —
within ±13% of ours. The 412 QPS number is therefore a property of the
load-test link (hey 100 connections → bench-server → WSL virtual NIC),
not of our implementation. ulule allows more requests (300 vs 124) because
a fixed window bursts its whole quota at each boundary; our token bucket is
stricter and smoother.

## Design semantics

- **Atomicity**: full decision inside Lua (single EVALSHA per request).
- **Time**: `redis.call('TIME')` server time, `tonumber` applied (TIME
  returns strings); rollback guard keeps state timestamps monotonic.
- **TTL**: per-key `PEXPIRE` refreshed on every request (default 30min),
  Redis own eviction handles memory pressure.
- **Error policy**: `ErrorFailClosed` (default, deny on Redis failure) or
  `ErrorFailOpen` (allow), errors always reported via `WithOnError`.
- **State**: key deleted/expired ⇒ recreated full (same as in-memory).

## Reproduce

```bash
# algorithm baseline (needs real Redis)
REDIS_ADDR=172.23.155.132:6379 go test -run '^$' -bench 'BenchmarkRedis' -benchmem -count=1 -benchtime=3s ./benchmarks/compare/

# concurrency sweep (direct, no HTTP)
REDIS_ADDR=172.23.155.132:6379 go test -run '^$' -bench 'BenchmarkRedisConcurrencySweep' -benchtime=3s -count=1 ./benchmarks/compare/

# end-to-end
cd benchmarks && go build -o /tmp/bench-server ./cmd/bench-server
/tmp/bench-server --impl redis --addr 127.0.0.1:8081 &
hey -n 100000 -c 100 -m GET -o csv http://127.0.0.1:8081/api/test
```
