"""Report 04: correctness verification matrix (differential + self-invariants)."""

from pathlib import Path

from visualize.parse_gotest import parse_pass_tests
from visualize.styles import COMPARE_RED, GREEN, add_source_note, save
import matplotlib.pyplot as plt
import numpy as np

ALGS = ["Token bucket", "Leaky bucket", "Sliding window"]
METHODS = ["external differential", "self invariants"]


def build_fig06(sources: dict, charts_dir: Path) -> str:
    src = sources["correctness"]
    if not src:
        raise RuntimeError("no correctness artifact (run scripts/bench-memory.sh)")
    passed = set(parse_pass_tests(src.read_text(encoding="utf-8", errors="replace")))

    # Every PASS cell is derived from the artifact: the differential suite and
    # each algorithm's invariant test must have actually passed. Only "n/a"
    # cells are static (no rejection-equivalent external library exists for
    # leaky bucket / sliding window — uber is blocking, no reject semantics).
    status = {
        ("Token bucket", "external differential"): "PASS" if "TestTokenBucket_DifferentialCorrectness" in passed else "FAIL",
        ("Token bucket", "self invariants"): "PASS" if "TestTokenBucket_PerKeyIsolationConsistency" in passed else "FAIL",
        ("Leaky bucket", "external differential"): "n/a",
        ("Leaky bucket", "self invariants"): "PASS" if "TestLeakyBucket_Invariants" in passed else "FAIL",
        ("Sliding window", "external differential"): "n/a",
        ("Sliding window", "self invariants"): "PASS" if "TestSlidingWindow_Invariants" in passed else "FAIL",
    }
    fig, ax = plt.subplots(figsize=(6.5, 2.6))
    # n/a rendered as a hole (masked) so it never blends with PASS/FAIL colors.
    data = np.array([[1 if status[(a, m)] == "PASS" else 0 for m in METHODS] for a in ALGS],
                    dtype=float)
    mask = np.array([[status[(a, m)] == "n/a" for m in METHODS] for a in ALGS])
    ax.imshow(np.ma.masked_array(data, mask=mask),
              cmap=plt.cm.colors.ListedColormap([COMPARE_RED, GREEN]), aspect="auto")
    ax.set_xticks(range(len(METHODS)))
    ax.set_xticklabels(METHODS)
    ax.set_yticks(range(len(ALGS)))
    ax.set_yticklabels(ALGS)
    for i, a in enumerate(ALGS):
        for j, m in enumerate(METHODS):
            ax.text(j, i, status[(a, m)], ha="center", va="center", fontsize=10)
    ax.set_title("Correctness verification matrix")
    add_source_note(fig, [src.name], ["sh scripts/bench-memory.sh"])
    return save(fig, charts_dir / "04_correctness_matrix.png")
