"""Report 01: in-memory algorithm comparison vs the ecosystem (ns/op, allocs)."""

from pathlib import Path

from visualize.parse_bench import median_rows, parse_bench_text
from visualize.styles import KAKA_BLUE, COMPARE_RED, GREEN, add_source_note, save, style_axis
import matplotlib.pyplot as plt

# Per algorithm: (kaka bench name, [(impl label, ref bench name), ...]).
# juju/uber numbers are call-overhead references (no per-key semantics).
GROUPS = [
    ("Token bucket", "BenchmarkKakaTokenBucketLowBurstSingleKey", [
        ("x/time/rate", "BenchmarkXTimeRateLowBurstSingleKey"),
        ("juju", "BenchmarkJujuTokenBucketLowBurstSingleKey"),
    ]),
    ("Leaky bucket", "BenchmarkKakaLeakyBucketSingleKey", [
        ("uber (overhead)", None),  # blocking limiter; call overhead only
    ]),
    ("Sliding window", "BenchmarkKakaSlidingWindowSingleKey", []),
]


def build_fig01(sources: dict, charts_dir: Path) -> str:
    src = sources["bench"]
    if not src:
        raise RuntimeError("no bench artifact (run scripts/bench.sh)")
    meta, rows = parse_bench_text(src.read_text(encoding="utf-8", errors="replace"))
    median = median_rows(rows)

    fig, ax = plt.subplots(figsize=(8, 4.2))
    x = range(len(GROUPS))
    for i, (alg, kaka_name, refs) in enumerate(GROUPS):
        kaka_row = median.get(kaka_name)
        if kaka_row is None:
            continue
        ax.bar([i], [kaka_row.ns_per_op], color=KAKA_BLUE, width=0.5, log=True)
        ax.text(i, kaka_row.ns_per_op * 1.5, f"{kaka_row.ns_per_op:.0f} ns\n{kaka_row.allocs_per_op} allocs",
                ha="center", fontsize=8, color="#333333")
        for label, ref_name in refs:
            ref = median.get(ref_name) if ref_name else None
            if ref is None:
                continue
            ax.scatter([i], [ref.ns_per_op], color=COMPARE_RED if label == "x/time/rate" else GREEN,
                       zorder=5, s=55, marker="^")
            ax.text(i, ref.ns_per_op * 0.55, label, ha="center", fontsize=7, color="#666666")
            ax.text(i, max(kaka_row.ns_per_op, ref.ns_per_op) * 1.9,
                    f"{kaka_row.ns_per_op / ref.ns_per_op:.2f}x", ha="center",
                    fontsize=9, weight="bold", color="#444444")
    ax.set_xticks(list(x))
    ax.set_xticklabels([g[0] for g in GROUPS])
    style_axis(ax, ylabel="ns/op (log)")
    ax.set_title("In-memory algorithm latency vs ecosystem (median of 3, -benchmem)")
    add_source_note(fig, [src.name], ["sh scripts/bench.sh"])
    return save(fig, charts_dir / "01_algorithm_comparison.png")
