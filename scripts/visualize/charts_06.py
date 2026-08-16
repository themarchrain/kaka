"""Report 06: Redis distributed baseline, concurrency sweep, bottleneck probes."""

from pathlib import Path

from visualize.parse_bench import median_rows, parse_bench_text, split_subbench
from visualize.styles import KAKA_BLUE, COMPARE_RED, GRAY, add_source_note, save, style_axis
import matplotlib.pyplot as plt

ALG_LABELS = {"tb": "TokenBucket", "sw": "SlidingWindow", "lb": "LeakyBucket"}


def build_fig09(sources: dict, charts_dir: Path) -> str:
    src = sources["bench-redis-sweep"]
    if not src:
        raise RuntimeError("no bench-redis-sweep artifact (run scripts/bench-redis.sh)")
    _, rows = parse_bench_text(src.read_text(encoding="utf-8", errors="replace"))

    series = {"tb": {}, "sw": {}, "lb": {}}
    for r in rows:
        base, parts = split_subbench(r.name)
        if base != "BenchmarkRedisConcurrencySweep" or "alg" not in parts:
            continue
        series[parts["alg"]][int(parts["parallel"])] = 1e9 / r.ns_per_op  # QPS

    fig, ax = plt.subplots(figsize=(6.5, 4))
    for alg, qps in series.items():
        if not qps:
            continue
        xs = sorted(qps)
        ax.plot(xs, [qps[x] for x in xs], marker="o", label=ALG_LABELS[alg], linewidth=2)
        peak = max(qps, key=qps.get)
        ax.annotate(f"peak {qps[peak]:,.0f} @{peak}",
                    xy=(peak, qps[peak]), xytext=(peak * 1.05, qps[peak] * 0.92),
                    fontsize=8, color="#444444",
                    arrowprops=dict(arrowstyle="->", color="#444444", lw=0.8))
    if not any(series.values()):
        raise RuntimeError("no sweep rows parsed (Redis reachable?)")
    ax.legend()
    style_axis(ax, xlabel="concurrency (goroutines)", ylabel="QPS")
    ax.set_title("Redis concurrency sweep (direct, single Redis)")
    add_source_note(fig, [src.name], ["sh scripts/bench-redis.sh"])
    return save(fig, charts_dir / "06_redis_concurrency_sweep.png")


def build_fig10(sources: dict, charts_dir: Path) -> str:
    src = sources["bench-redis"]
    if not src:
        raise RuntimeError("no bench-redis artifact (run scripts/bench-redis.sh)")
    _, rows = parse_bench_text(src.read_text(encoding="utf-8", errors="replace"))
    median = median_rows(rows)

    fig, ax = plt.subplots(figsize=(6, 4))
    labels = ["TokenBucket", "LeakyBucket", "SlidingWindow"]
    names = ["BenchmarkRedisTokenBucket", "BenchmarkRedisLeakyBucket", "BenchmarkRedisSlidingWindow"]
    vals, allocs = [], []
    for n in names:
        row = median.get(n)
        if row is None:
            raise RuntimeError(f"missing {n} in {src.name} (Redis reachable?)")
        vals.append(row.ns_per_op)
        allocs.append(row.allocs_per_op)
    ax.bar(labels, vals, color=KAKA_BLUE)
    for i, (v, a) in enumerate(zip(vals, allocs)):
        ax.text(i, v * 1.03, f"{v/1000:.1f} us", ha="center", fontsize=9)
        if a is not None:
            ax.text(i, v * 0.5, f"{a} allocs", ha="center", fontsize=8, color="white")
    style_axis(ax, ylabel="ns/op")
    ax.set_title("Redis algorithm baseline (one EVALSHA per request)")
    add_source_note(fig, [src.name], ["sh scripts/bench-redis.sh"])
    return save(fig, charts_dir / "06_redis_baseline.png")


def build_fig11(sources: dict, charts_dir: Path) -> str:
    src = sources["bench-redis"]
    if not src:
        raise RuntimeError("no bench-redis artifact (run scripts/bench-redis.sh)")
    _, rows = parse_bench_text(src.read_text(encoding="utf-8", errors="replace"))
    median = median_rows(rows)

    probes = [
        ("GET single", "BenchmarkRedisGetSingle", GRAY),
        ("GET 64-conn", "BenchmarkRedisGetParallel64", GRAY),
        ("EVALSHA 64-conn", "BenchmarkRedisEvalshaParallel64", KAKA_BLUE),
        ("ulule single", "BenchmarkUluleRedisSingle", COMPARE_RED),
        ("ulule 64-conn", "BenchmarkUluleRedisParallel64", COMPARE_RED),
    ]
    labels, vals, colors = [], [], []
    for label, name, color in probes:
        row = median.get(name)
        if row is None:
            continue
        labels.append(label)
        vals.append(row.ns_per_op)
        colors.append(color)
    if not vals:
        raise RuntimeError("no probe rows parsed (Redis reachable?)")

    fig, ax = plt.subplots(figsize=(7, 4))
    ax.bar(labels, vals, color=colors)
    for i, v in enumerate(vals):
        ax.text(i, v * 1.02, f"{v/1000:.1f} us", ha="center", fontsize=8)
    style_axis(ax, ylabel="ns/op")
    ax.set_title("Bottleneck attribution: GET vs EVALSHA vs ulule/limiter")
    add_source_note(fig, [src.name], ["sh scripts/bench-redis.sh"])
    return save(fig, charts_dir / "06_redis_probe.png")
