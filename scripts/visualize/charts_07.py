"""Report 07: layered rate limiting (local short-circuit vs pure Redis)."""

from pathlib import Path

from visualize.parse_bench import median_rows, parse_bench_text
from visualize.parse_hey import parse_hey_csv
from visualize.styles import KAKA_BLUE, COMPARE_RED, add_source_note, save, style_axis
import matplotlib.pyplot as plt


def build_fig12(sources: dict, charts_dir: Path) -> str:
    src = sources["bench-layered"]
    if not src:
        raise RuntimeError("no bench-layered artifact (run scripts/bench-layered.sh)")
    _, rows = parse_bench_text(src.read_text(encoding="utf-8", errors="replace"))
    median = median_rows(rows)

    local = median.get("BenchmarkLayeredLocalReject")
    remote = median.get("BenchmarkRedisRejectHeavy")
    if local is None or remote is None:
        raise RuntimeError("layered reject-path benchmarks missing (Redis reachable?)")

    fig, ax = plt.subplots(figsize=(6, 4))
    ax.bar(["layered\n(local short-circuit)", "pure redis\n(reject flood)"],
           [local.ns_per_op, remote.ns_per_op], color=[KAKA_BLUE, COMPARE_RED], log=True)
    ax.text(0, local.ns_per_op * 1.4, f"{local.ns_per_op:.1f} ns", ha="center", fontsize=10)
    ax.text(1, remote.ns_per_op * 1.4, f"{remote.ns_per_op/1000:.1f} us", ha="center", fontsize=10)
    ratio = remote.ns_per_op / local.ns_per_op
    ax.text(0.5, remote.ns_per_op * 1.4, f"{ratio:,.0f}x", ha="center", fontsize=12, weight="bold", color="#444444")
    style_axis(ax, ylabel="ns/op (log)")
    ax.set_title("Layered reject path vs pure Redis (ratio computed from data)")
    add_source_note(fig, [src.name], ["sh scripts/bench-layered.sh"])
    return save(fig, charts_dir / "07_layered_reject_path.png")


def build_fig13(sources: dict, charts_dir: Path) -> str:
    stats, files = [], []
    for label, kind, color in [("pure redis", "loadtest-redis", COMPARE_RED),
                               ("layered", "loadtest-layered", KAKA_BLUE)]:
        src = sources[kind]
        if not src:
            continue
        with src.open(encoding="utf-8", errors="replace") as f:
            stats.append(parse_hey_csv(f))
        files.append(src.name)
    if len(stats) != 2:
        raise RuntimeError("need both loadtest-redis and loadtest-layered CSVs")

    fig, ax = plt.subplots(figsize=(6, 4))
    labels = ["pure redis", "layered"]
    ax.bar(labels, [s.qps for s in stats], color=[COMPARE_RED, KAKA_BLUE])
    for i, s in enumerate(stats):
        ax.text(i, s.qps * 1.02, f"{s.qps:,.0f}", ha="center", fontsize=10)
    ratio = stats[1].qps / stats[0].qps if stats[0].qps else 0
    ax.text(0.5, max(s.qps for s in stats) * 1.06, f"{ratio:.1f}x", ha="center", fontsize=11, weight="bold")
    style_axis(ax, ylabel="QPS")
    ax.set_title("End-to-end HTTP: layered vs pure redis (parameters from the data-source run)")
    add_source_note(fig, files, ["sh scripts/bench-layered.sh"])
    return save(fig, charts_dir / "07_layered_http.png")
