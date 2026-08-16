"""Report 08: multi-key concurrency — v0.3.0 baseline vs sharded store."""

import re
from pathlib import Path

import matplotlib.pyplot as plt

from visualize.parse_bench import parse_bench_text
from visualize.styles import KAKA_BLUE, COMPARE_RED, add_source_note, save, style_axis

# parse_bench strips the -<GOMAXPROCS> suffix, so sub-benchmark names arrive
# as "BenchmarkMultiKeyParallel/g8k64" (no trailing -16).
_MULTIKEY_RE = re.compile(r"^BenchmarkMultiKeyParallel/g(\d+)k(\d+)$")


def parse_multikey_subbench(name: str):
    """Parse 'BenchmarkMultiKeyParallel/g8k64-16' -> (base, g, k); None if unrelated."""
    m = _MULTIKEY_RE.match(name)
    if not m:
        return None
    return "BenchmarkMultiKeyParallel", int(m.group(1)), int(m.group(2))


def _sweep(path: Path) -> dict[tuple[int, int], float]:
    """{(g, k): QPS} parsed from a benchmark-multikey raw file."""
    _, rows = parse_bench_text(path.read_text(encoding="utf-8", errors="replace"))
    out: dict[tuple[int, int], float] = {}
    for r in rows:
        parsed = parse_multikey_subbench(r.name)
        if parsed is None:
            continue
        _, g, k = parsed
        out[(g, k)] = 1e9 / r.ns_per_op  # QPS
    return out


def _baseline_candidates(raw_dir: Path) -> list[Path]:
    return sorted((raw_dir / "archive" / "v0.3.0").glob("benchmark-multikey-*.txt"))


def _load_old_new(sources: dict) -> tuple[Path, Path, dict, dict]:
    new_src = sources["bench-multikey"]
    if not new_src:
        raise RuntimeError("no bench-multikey artifact (run scripts/bench-multikey.sh)")
    old_candidates = _baseline_candidates(new_src.parent)
    if not old_candidates:
        raise RuntimeError("no archived v0.3.0 baseline under raw/archive/v0.3.0/")
    old = _sweep(old_candidates[-1])
    new = _sweep(new_src)
    if not old or not new:
        raise RuntimeError("no multi-key rows parsed")
    return old_candidates[-1], new_src, old, new


def build_fig14(sources: dict, charts_dir: Path) -> str:
    """Throughput vs goroutines: v0.3.0 (dashed) vs v0.4.0 (solid), per key set."""
    old_src, new_src, old, new = _load_old_new(sources)

    fig, ax = plt.subplots(figsize=(7, 4.5))
    for k in sorted({k for (_, k) in old} | {k for (_, k) in new}):
        gs_old = sorted(g for (g, kk) in old if kk == k)
        gs_new = sorted(g for (g, kk) in new if kk == k)
        if gs_old and gs_new:
            ax.plot(gs_old, [old[(g, k)] / 1e6 for g in gs_old], ls="--", marker="o",
                    color=COMPARE_RED, alpha=0.6, label=f"v0.3.0 k={k}")
            ax.plot(gs_new, [new[(g, k)] / 1e6 for g in gs_new], ls="-", marker="o",
                    color=KAKA_BLUE, label=f"v0.4.0 k={k}")
    ax.legend(fontsize=8)
    style_axis(ax, xlabel="concurrency (goroutines)", ylabel="QPS (millions)")
    ax.set_title("Multi-key throughput: global lock vs 64 shards")
    add_source_note(fig, [old_src.name, new_src.name],
                    ["sh scripts/bench-multikey.sh (v0.3.0 code, archived)",
                     "sh scripts/bench-multikey.sh"])
    return save(fig, charts_dir / "08_multikey_throughput.png")


def build_fig15(sources: dict, charts_dir: Path) -> str:
    """Speedup (new/old) vs goroutines, one line per key set (computed, never hardcoded)."""
    old_src, new_src, old, new = _load_old_new(sources)

    fig, ax = plt.subplots(figsize=(6.5, 4))
    for k in sorted({k for (_, k) in old} | {k for (_, k) in new}):
        pairs = sorted((g, new[(g, k)] / old[(g, k)])
                       for (g, kk) in old if kk == k and (g, k) in new)
        if not pairs:
            continue
        gs = [p[0] for p in pairs]
        ratios = [p[1] for p in pairs]
        ax.plot(gs, ratios, marker="o", label=f"k={k}")
    ax.axhline(1.0, color="#888888", lw=0.8)
    ax.legend(fontsize=8)
    style_axis(ax, xlabel="concurrency (goroutines)", ylabel="speedup (v0.4.0 / v0.3.0)")
    ax.set_title("Multi-key speedup vs v0.3.0")
    add_source_note(fig, [old_src.name, new_src.name],
                    ["sh scripts/bench-multikey.sh (v0.3.0 code, archived)",
                     "sh scripts/bench-multikey.sh"])
    return save(fig, charts_dir / "08_multikey_speedup.png")
