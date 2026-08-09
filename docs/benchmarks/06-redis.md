# 06. Redis Distributed Rate Limiting

> Measured: 2026-08-09 · Go 1.25.1 · WSL Redis 7 (172.23.155.132) · Intel i7-13620H
> Reproduce: see commands at the bottom

## Conclusion

- All three algorithms are backed by a single atomic Lua round-trip
  (EVALSHA) — no GET/compute/SET race window across instances.
- Per-request latency is ~0.11–0.14 ms (network round-trip dominated),
  ~1600× the in-memory version — this is the physical cost of distributed
  decision making.
- Single-instance throughput: ~47k–55k QPS concurrent.
- Redis server time is authoritative; clock-rollback guards prevent
  catch-up refill/leak that could bypass limits.

## Algorithm baseline (single request, one EVALSHA)

| Algorithm     | Latency  | Concurrent QPS | Allocs/op |
|---------------|----------|----------------|-----------|
| TokenBucket   | 114.5µs  | ~55k           | 11        |
| LeakyBucket   | 112.6µs  | ~53k           | 11        |
| SlidingWindow | 140.3µs  | ~47k           | 37        |

Hash-state algorithms (TB/LB) are equivalent; ZSet-based SWL is ~23%
heavier (ZREMRANGEBYSCORE + ZCARD + ZADD + ZRANGE). Allocs are go-redis
internal (encoding/decoding); library code adds none.

## End-to-end (HTTP, hey 100k × 100 concurrency)

| Impl  | QPS  | p50   | p99   | Allowed/Total |
|-------|------|-------|-------|---------------|
| kaka  | 1248 | 0.5ms | 4.1ms | 108/99892     |
| xtime | 1322 | 0.5ms | 4.0ms | 107/99893     |
| redis | 412  | 2.3ms | 4.7ms | 124/99876     |

Redis QPS is ~3× lower and p50 ~4.6× higher than the in-memory
implementations — the measurable cost of a network round-trip per
decision. The 200/429 distribution stays aligned (small delta is
millisecond server-time granularity). p99 converges because it is
dominated by hey connection scheduling at 100 concurrency.

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

# end-to-end
cd benchmarks && go build -o /tmp/bench-server ./cmd/bench-server
/tmp/bench-server --impl redis --addr 127.0.0.1:8081 &
hey -n 100000 -c 100 -m GET -o csv http://127.0.0.1:8081/api/test
```
