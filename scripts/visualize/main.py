"""Entry point: locate the newest raw artifacts and render every chart.

Usage:
  python scripts/visualize/main.py [--raw-dir docs/benchmarks/raw]
                                   [--charts-dir docs/benchmarks/charts]
                                   [--only fig01,fig02]

Each kind maps to a glob; the newest file per kind is used. A missing kind
skips its charts with a notice (never fabricates data).
"""

import argparse
import importlib
import sys
from pathlib import Path

# Defaults resolve from this file's location so the entry works from any cwd
# (scripts/visualize/main.py -> repo root).
REPO_ROOT = Path(__file__).resolve().parents[2]
SCRIPTS_DIR = REPO_ROOT / "scripts"
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

KINDS = {
    # Globs are anchored on the timestamp so they never capture sibling
    # kinds: benchmark-* must not match benchmark-redis-* / benchmark-layered-*,
    # loadtest-kaka-* must not match loadtest-kaka-many-*.
    "bench": "benchmark-[0-9]*.txt",
    "loadtest-kaka": "loadtest-kaka-[0-9]*.csv",
    "loadtest-xtime": "loadtest-xtime-[0-9]*.csv",
    "loadtest-kaka-many": "loadtest-kaka-many-*.csv",
    "loadtest-redis": "loadtest-redis-*.csv",
    "loadtest-layered": "loadtest-layered-*.csv",
    "memory-perkey": "memory-perkey-*.txt",
    "memory-lru": "memory-lru-*.txt",
    "longterm-mem": "longterm-mem-*.csv",
    "correctness": "correctness-*.txt",
    "lru": "lru-*.txt",
    "bench-redis": "benchmark-redis-[0-9]*.txt",
    "bench-redis-sweep": "benchmark-redis-sweep-*.txt",
    "bench-layered": "benchmark-layered-*.txt",
}

CHARTS = {
    "fig01": ("charts_01", "build_fig01"),
    "fig02": ("charts_02", "build_fig02"),
    "fig03": ("charts_02", "build_fig03"),
    "fig04": ("charts_03", "build_fig04"),
    "fig05": ("charts_03", "build_fig05"),
    "fig06": ("charts_04", "build_fig06"),
    "fig07": ("charts_05", "build_fig07"),
    "fig08": ("charts_05", "build_fig08"),
    "fig09": ("charts_06", "build_fig09"),
    "fig10": ("charts_06", "build_fig10"),
    "fig11": ("charts_06", "build_fig11"),
    "fig12": ("charts_07", "build_fig12"),
    "fig13": ("charts_07", "build_fig13"),
}


def newest(raw_dir: Path, pattern: str):
    hits = sorted(raw_dir.glob(pattern), key=lambda p: p.stat().st_mtime, reverse=True)
    return hits[0] if hits else None


def main() -> int:
    ap = argparse.ArgumentParser(description="Render benchmark charts from raw artifacts")
    ap.add_argument("--raw-dir", default=str(REPO_ROOT / "docs/benchmarks/raw"))
    ap.add_argument("--charts-dir", default=str(REPO_ROOT / "docs/benchmarks/charts"))
    ap.add_argument("--only", default=None, help="comma-separated fig ids, e.g. fig01,fig12")
    args = ap.parse_args()

    raw_dir = Path(args.raw_dir)
    charts_dir = Path(args.charts_dir)
    charts_dir.mkdir(parents=True, exist_ok=True)

    sources = {kind: newest(raw_dir, pat) for kind, pat in KINDS.items()}
    missing = [k for k, v in sources.items() if v is None]
    if missing:
        print(f"NOTE: no raw data for: {', '.join(missing)} "
              f"(run the matching script under scripts/ or scripts/visualize.sh)")

    only = set(args.only.split(",")) if args.only else None
    ok, skipped = [], []
    for fig_id, (module, fn_name) in CHARTS.items():
        if only and fig_id not in only:
            continue
        try:
            mod = importlib.import_module(f"visualize.{module}")
            path = getattr(mod, fn_name)(sources, charts_dir)
            ok.append(fig_id)
            print(f"[ok] {fig_id} -> {path}")
        except Exception as exc:  # noqa: BLE001 - one bad chart must not kill the rest
            skipped.append((fig_id, str(exc)))
            print(f"[skip] {fig_id}: {exc}")

    print(f"\nrendered {len(ok)} charts, skipped {len(skipped)}")
    if skipped:
        print("skipped:", ", ".join(f"{f} ({e})" for f, e in skipped))
    # A run that rendered nothing must fail loudly so visualize.sh / CI can
    # detect it (unless the user filtered with --only and it simply had gaps).
    if not ok and not only:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
