"""Report 03: per-key memory and long-term stability."""

from pathlib import Path

from visualize.parse_gotest import parse_perkey_lines
from visualize.parse_longterm import parse_longterm_mem
from visualize.styles import KAKA_BLUE, COMPARE_RED, add_source_note, save, style_axis
import matplotlib.pyplot as plt


def build_fig04(sources: dict, charts_dir: Path) -> str:
    src = sources["memory-perkey"]
    if not src:
        raise RuntimeError("no memory-perkey artifact (run scripts/bench-memory.sh)")
    rows = parse_perkey_lines(src.read_text(encoding="utf-8", errors="replace"))
    if not rows:
        raise RuntimeError(f"no per-key rows parsed from {src.name}")

    fig, ax = plt.subplots(figsize=(6.5, 4))
    x = [str(r.keys) for r in rows]
    width = 0.35
    xi = range(len(rows))
    kaka_vals = [r.kaka_per_key for r in rows]
    ulule_vals = [r.ulule_per_key for r in rows]
    ax.bar([i - width / 2 for i in xi], kaka_vals, width=width, label="kaka", color=KAKA_BLUE)
    ax.bar([i + width / 2 for i in xi], ulule_vals, width=width, label="ulule/limiter", color=COMPARE_RED)
    for i, r in enumerate(rows):
        ax.text(i - width / 2, r.kaka_per_key * 1.05, f"{r.kaka_per_key:.0f}", ha="center", fontsize=8)
        ax.text(i + width / 2, r.ulule_per_key * 1.05, f"{r.ulule_per_key:.0f}", ha="center", fontsize=8)
    ax.set_xticks(list(xi))
    ax.set_xticklabels(x)
    ax.legend()
    style_axis(ax, xlabel="keys", ylabel="bytes / key (log)", logy=True)
    ax.set_title("Per-key memory: kaka vs ulule/limiter")
    add_source_note(fig, [src.name], ["sh scripts/bench-memory.sh"])
    return save(fig, charts_dir / "03_memory_per_key.png")


def build_fig05(sources: dict, charts_dir: Path) -> str:
    src = sources["longterm-mem"]
    if not src:
        raise RuntimeError("no longterm-mem artifact (run scripts/loadtest-long.sh)")
    df = parse_longterm_mem(src.read_text(encoding="utf-8", errors="replace"))
    if df.empty:
        raise RuntimeError(f"no sampling rows parsed from {src.name}")

    fig, ax = plt.subplots(figsize=(7, 3.8))
    ax.plot(df["ts"], df["heapAlloc"] / 1e6, color=KAKA_BLUE, marker="o", markersize=3,
            label="HeapAlloc")
    if df["numGC"].max() > 0:
        # Mark GC events: one tick per collection, size reflects collections.
        ax.scatter(df["ts"], [0.98 * df["heapAlloc"].min() / 1e6] * len(df),
                   s=df["numGC"].diff().fillna(0) * 8, color=COMPARE_RED, alpha=0.5,
                   label="GC events", marker="|")
        ax.legend()
    ax.set_xlabel("elapsed (s)")
    ax.set_ylabel("HeapAlloc (MB)")
    ax.set_title("Long-term stability: heap under sustained load")
    add_source_note(fig, [src.name], ["sh scripts/loadtest-long.sh 600"])
    return save(fig, charts_dir / "03_longterm_memory.png")
