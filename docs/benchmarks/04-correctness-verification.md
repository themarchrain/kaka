# 04. Correctness Verification
> Charts: [04_correctness_matrix.png](charts/04_correctness_matrix.png)

Request-by-request differential testing against the ecosystem, plus
mathematical invariants for the algorithms that have no counterpart.

**Go 1.25.1.** All tests are deterministic (fixed seeds, injected clocks) and
independent of machine load.

## Differential tests — token bucket vs x/time/rate & juju

The same deterministic time/request sequence is fed to all three
implementations via injectable clocks (`WithClock`, `AllowN(t, n)`,
`NewBucketWithRateAndClock`), with identical parameters (burst=100, rate=10,
initially full). Decisions are compared per request.

| Scenario | What it exercises |
|---|---|
| steady rate (5/s < 10/s) | all allowed |
| burst exhaustion | drain → reject → recover |
| exact refill boundary (100 ms) | float-sensitive region |
| idle recovery | refill to full after 60 s idle |
| random, seed=42, ×1000 | combination coverage |
| per-key isolation | each key matches a standalone limiter |

**Result: all scenarios pass** — Kaka's allow/deny decisions match both
libraries request-by-request, including the float-sensitive boundary
scenario; per-key decisions are identical to standalone limiters.

> Test-harness note: an earlier run showed `juju` rejecting broadly; this
> traced to the harness clock starting at year 1, overflowing `time.Duration`
> in `juju`'s elapsed-time math (1969 years > int64 ns). Fixed by initializing
> the clock at the sequence base. No library behavior difference exists.

## Invariants — leaky bucket & sliding window log (no external counterpart)

Deterministic sequences asserting each algorithm's mathematical properties
every request, including a strict ±1 ns `RetryAfter` boundary check (denied
1 ns early, allowed exactly on time).

| Algorithm | Invariants |
|---|---|
| Leaky bucket | water conservation & bounds / rejection ⇔ overflow / exact leak rate / idle drain to zero / `Remaining` consistency / `RetryAfter` boundary |
| Sliding window log | log count ≤ limit / rejection ⇔ window full / log validity in window / stale-log removal / `Remaining` consistency / `RetryAfter` boundary |

## Verification matrix

| Algorithm | External differential | Self invariants |
|---|---|---|
| Token bucket | ✅ vs `x/time/rate` + `juju` | ✅ unit/boundary |
| Leaky bucket | — (uber is blocking, no rejection) | ✅ 6 invariants |
| Sliding window log | — (unique algorithm) | ✅ 6 invariants |

## Reproduce

```sh
cd benchmarks && go test -run 'Correctness|PerKeyIsolation' -v ./compare/
go test -count=1 ./memory/ -run 'Invariants' -v
```
