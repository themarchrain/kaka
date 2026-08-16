"""Report 05: LRU eviction cost (throughput + memory)."""

from pathlib import Path

from visualize.parse_bench import median_rows, parse_bench_text
from visualize.parse_gotest import parse_lru_mem_lines
from visualize.styles import KAKA_BLUE, COMPARE_RED, GRAY, add_source_note, save, style_axis
import matplotlib.pyplot as plt

LRU_BENCHES = [
    ("reject (baseline)", "BenchmarkKakaTokenBucketMaxKeysReject", KAKA_BLUE),
    ("LRU eviction", "BenchmarkKakaTokenBucketLRUEviction", COMPARE_RED),
    ("LRU eviction (parallel)", "BenchmarkKakaTokenBucketLRUEvictionParallel", GRAY),
    ("thrash (every access evicts)", "BenchmarkKakaTokenBucketLRUThrash", "#c29b2e"),
]


def build_fig07(sources: dict, charts_dir: Path) -> str:
    src = sources["lru"]
    if not src:
        raise RuntimeError("no lru artifact (run scripts/bench-lru.sh)")
    _, rows = parse_bench_text(src.read_text(encoding="utf-8", errors="replace"))
    median = median_rows(rows)

    fig, ax = plt.subplots(figsize=(7, 4))
    labels, vals, colors = [], [], []
    for label, bench, color in LRU_BENCHES:
        row = median.get(bench)
        if row is None:
            continue
        labels.append(label)
        vals.append(row.ns_per_op)
        colors.append(color)
    if not vals:
        raise RuntimeError(f"no LRU benchmarks parsed from {src.name}")
    ax.bar(labels, vals, color=colors)
    for i, v in enumerate(vals):
        ax.text(i, v * 1.02, f"{v:.1f} ns", ha="center", fontsize=9)
    style_axis(ax, ylabel="ns/op")
    ax.set_title("LRU eviction throughput (separate processes, count=3 median)")
    add_source_note(fig, [src.name], ["sh scripts/bench-lru.sh"])
    return save(fig, charts_dir / "05_lru_throughput.png")


def build_fig08(sources: dict, charts_dir: Path) -> str:
    src = sources["memory-lru"]
    if not src:
        raise RuntimeError("no memory-lru artifact (run scripts/bench-memory.sh)")
    rows = parse_lru_mem_lines(src.read_text(encoding="utf-8", errors="replace"))
    if not rows:
        raise RuntimeError(f"no LRU memory rows parsed from {src.name}")

    fig, ax = plt.subplots(figsize=(6.5, 4))
    xi = range(len(rows))
    width = 0.35
    ax.bar([i - width / 2 for i in xi], [r.map_per_key for r in rows], width=width, label="mapStore (reject)", color=KAKA_BLUE)
    ax.bar([i + width / 2 for i in xi], [r.lru_per_key for r in rows], width=width, label="lruStore (EvictLRU)", color=COMPARE_RED)
    for i, r in enumerate(rows):
        ax.text(i, max(r.map_per_key, r.lru_per_key) * 1.03, f"+{r.diff_per_key:.0f} B/key", ha="center", fontsize=8)
    ax.set_xticks(list(xi))
    ax.set_xticklabels([str(r.keys) for r in rows])
    ax.legend()
    style_axis(ax, xlabel="keys", ylabel="B / key")
    ax.set_title("LRU memory overhead (cumulative allocations)")
    add_source_note(fig, [src.name], ["sh scripts/bench-memory.sh"])
    return save(fig, charts_dir / "05_lru_memory.png")
