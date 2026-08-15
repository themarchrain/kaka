"""Shared chart styling and source-note helpers (all charts are English-only)."""

import matplotlib

matplotlib.use("Agg")

import matplotlib.pyplot as plt  # noqa: E402

KAKA_BLUE = "#2f6fd0"
COMPARE_RED = "#d04f3f"
GRAY = "#8a8a8a"
GREEN = "#3f9d5c"
PALETTE = [KAKA_BLUE, COMPARE_RED, GREEN, GRAY, "#c29b2e", "#6b4fa0"]

plt.rcParams.update({
    "figure.dpi": 110,
    "font.size": 10,
    "axes.titlesize": 12,
    "axes.titleweight": "bold",
    "axes.grid": True,
    "grid.alpha": 0.3,
    "axes.spines.top": False,
    "axes.spines.right": False,
    "figure.constrained_layout.use": True,
})


def style_axis(ax, xlabel=None, ylabel=None, logy=False):
    if xlabel:
        ax.set_xlabel(xlabel)
    if ylabel:
        ax.set_ylabel(ylabel)
    if logy:
        ax.set_yscale("log")
    ax.grid(axis="y", alpha=0.3)


def add_source_note(fig, sources: list[str], commands: list[str]):
    """Footer note: raw data files + reproduce commands."""
    src = "; ".join(sources)
    cmd = " && ".join(commands)
    fig.text(0.01, 0.005, f"data: {src}\nreproduce: {cmd}",
             fontsize=7, color="#555555", va="bottom", ha="left")


def save(fig, path: str) -> str:
    fig.savefig(path, bbox_inches="tight")
    return path
