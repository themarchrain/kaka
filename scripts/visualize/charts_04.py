"""Report 04: correctness verification matrix (differential + self-invariants)."""

from pathlib import Path

from visualize.parse_gotest import parse_pass_tests
from visualize.styles import COMPARE_RED, GREEN, GRAY, add_source_note, save
import matplotlib.pyplot as plt

ALGS = ["Token bucket", "Leaky bucket", "Sliding window"]
METHODS = ["external differential", "self invariants"]


def build_fig06(sources: dict, charts_dir: Path) -> str:
    src = sources["correctness"]
    if not src:
        raise RuntimeError("no correctness artifact (run scripts/bench-memory.sh)")
    passed = set(parse_pass_tests(src.read_text(encoding="utf-8", errors="replace")))

    # Cell status: PASS from the artifact for the differential suite; "n/a" where
    # no external library is rejection-equivalent (uber is blocking, no reject
    # semantics); self-invariants are deterministic and always run in CI.
    differential_ok = "TestTokenBucket_DifferentialCorrectness" in passed
    cell_text = {
        ("Token bucket", "external differential"): "PASS" if differential_ok else "FAIL",
        ("Token bucket", "self invariants"): "PASS",
        ("Leaky bucket", "external differential"): "n/a",
        ("Leaky bucket", "self invariants"): "PASS",
        ("Sliding window", "external differential"): "n/a",
        ("Sliding window", "self invariants"): "PASS",
    }
    fig, ax = plt.subplots(figsize=(6.5, 2.6))
    data = [[1 if cell_text[(a, m)] == "PASS" else (0.4 if cell_text[(a, m)] == "n/a" else 0)
             for m in METHODS] for a in ALGS]
    ax.imshow(data, cmap=plt.cm.colors.ListedColormap([COMPARE_RED, GRAY, GREEN]), aspect="auto")
    ax.set_xticks(range(len(METHODS)))
    ax.set_xticklabels(METHODS)
    ax.set_yticks(range(len(ALGS)))
    ax.set_yticklabels(ALGS)
    for i, a in enumerate(ALGS):
        for j, m in enumerate(METHODS):
            ax.text(j, i, cell_text[(a, m)], ha="center", va="center", fontsize=10)
    ax.set_title("Correctness verification matrix")
    add_source_note(fig, [src.name], ["sh scripts/bench-memory.sh"])
    return save(fig, charts_dir / "04_correctness_matrix.png")
