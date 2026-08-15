"""Report 02: end-to-end HTTP load tests (QPS, latency percentiles)."""

from pathlib import Path

from visualize.parse_hey import parse_hey_csv
from visualize.styles import KAKA_BLUE, COMPARE_RED, add_source_note, save, style_axis
import matplotlib.pyplot as plt


def build_fig02(sources: dict, charts_dir: Path) -> str:
    impls = [("kaka", "loadtest-kaka", KAKA_BLUE), ("x/time/rate", "loadtest-xtime", COMPARE_RED)]
    stats = []
    files = []
    for label, kind, _ in impls:
        src = sources[kind]
        if not src:
            continue
        with src.open(encoding="utf-8", errors="replace") as f:
            stats.append(parse_hey_csv(f))
        files.append(src.name)
    if len(stats) != 2:
        raise RuntimeError("need both loadtest-kaka and loadtest-xtime CSVs")

    fig, ax = plt.subplots(figsize=(6, 4))
    ax.bar([impls[0][0], impls[1][0]], [s.qps for s in stats],
           color=[KAKA_BLUE, COMPARE_RED])
    for i, s in enumerate(stats):
        ax.text(i, s.qps * 1.02, f"{s.qps:,.0f}", ha="center", fontsize=10)
    style_axis(ax, ylabel="QPS")
    ax.set_title("HTTP load: QPS (parameters from the data-source run)")
    add_source_note(fig, files, ["sh scripts/loadtest.sh"])
    return save(fig, charts_dir / "02_http_qps.png")


def build_fig03(sources: dict, charts_dir: Path) -> str:
    impls = [("kaka", "loadtest-kaka", KAKA_BLUE), ("x/time/rate", "loadtest-xtime", COMPARE_RED)]
    stats, files = [], []
    for label, kind, _ in impls:
        src = sources[kind]
        if not src:
            continue
        with src.open() as f:
            stats.append(parse_hey_csv(f))
        files.append(src.name)
    if len(stats) != 2:
        raise RuntimeError("need both loadtest-kaka and loadtest-xtime CSVs")

    fig, ax = plt.subplots(figsize=(6, 4))
    pcts = ["p50", "p95", "p99"]
    width = 0.35
    x = range(len(pcts))
    for i, (label, _, color) in enumerate(impls):
        vals = [getattr(stats[i], p) * 1000 for p in pcts]  # s -> ms
        ax.bar([xi + i * width for xi in x], vals, width=width, label=label, color=color)
    ax.set_xticks([xi + width / 2 for xi in x])
    ax.set_xticklabels([p.upper() for p in pcts])
    ax.legend()
    style_axis(ax, ylabel="latency (ms)")
    ax.set_title("HTTP load: latency percentiles")
    add_source_note(fig, files, ["sh scripts/loadtest.sh"])
    return save(fig, charts_dir / "02_http_latency_percentiles.png")
