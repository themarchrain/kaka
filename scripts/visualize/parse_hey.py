"""Parse hey CSV output (-o csv, verified v0.1.5).

Columns: response-time,DNS+dialup,DNS,Request-write,Response-delay,
Response-read,status-code,offset — all time columns in seconds.
`offset` is the request start time; on Windows it is a QueryPerformanceCounter
reading with an arbitrary origin, so only differences are meaningful:
QPS = total / (max(offset) - min(offset)).
"""

from dataclasses import dataclass


@dataclass
class HeyStats:
    total: int
    allowed: int
    denied: int
    qps: float
    p50: float  # seconds
    p95: float
    p99: float


def _percentile(sorted_vals: list[float], p: float) -> float:
    if not sorted_vals:
        return 0.0
    idx = (len(sorted_vals) - 1) * p
    lo = int(idx)
    hi = min(lo + 1, len(sorted_vals) - 1)
    frac = idx - lo
    return sorted_vals[lo] * (1 - frac) + sorted_vals[hi] * frac


def parse_hey_csv(fileobj) -> HeyStats:
    header = None
    times: list[float] = []
    offsets: list[float] = []
    allowed = 0
    denied = 0
    for line in fileobj:
        line = line.strip()
        if not line:
            continue
        if header is None:
            header = line
            continue
        cols = line.split(",")
        if len(cols) < 8:
            continue
        try:
            times.append(float(cols[0]))
            offsets.append(float(cols[7]))
        except ValueError:
            continue
        if cols[6] == "200":
            allowed += 1
        elif cols[6] == "429":
            denied += 1
    total = len(times)
    qps = 0.0
    if total and len(offsets) > 1:
        span = max(offsets) - min(offsets)
        if span > 0:
            qps = total / span
    times_sorted = sorted(times)
    return HeyStats(
        total=total,
        allowed=allowed,
        denied=denied,
        qps=qps,
        p50=_percentile(times_sorted, 0.50),
        p95=_percentile(times_sorted, 0.95),
        p99=_percentile(times_sorted, 0.99),
    )
