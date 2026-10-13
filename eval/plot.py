# /// script
# requires-python = ">=3.12"
# dependencies = ["matplotlib==3.10.7"]
# ///
"""Draws eyeful's grounded recall beside the reviewers on ReviewBench's leaderboard, as one picture to share."""

from __future__ import annotations

import argparse
import csv
from pathlib import Path

import matplotlib

matplotlib.use("svg")
import matplotlib.pyplot as plt

SURFACE, INK, MUTED = "#fcfcfb", "#0b0b0b", "#6b6a66"
OURS, OTHERS = "#2a78d6", "#d3d2cb"

plt.rcParams.update({
    "font.family": ["Helvetica Neue", "Helvetica", "Arial", "sans-serif"], "text.color": INK,
    "axes.facecolor": SURFACE, "figure.facecolor": SURFACE, "savefig.facecolor": SURFACE, "svg.fonttype": "none",
})


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--leaderboard", type=Path, required=True, help="the leaderboard's rows, as a CSV with agent and grounded_recall")
    parser.add_argument("--standing", type=Path, required=True, help="the CSV reviewbench.py standing wrote")
    parser.add_argument("--run", required=True, help="the run of the standing table to draw")
    parser.add_argument("--title", required=True)
    parser.add_argument("--note", required=True, help="the one line under the figure: what each group was measured on")
    parser.add_argument("--out", type=Path, required=True, help="the picture; an SVG and a PNG are written")
    args = parser.parse_args()
    with args.leaderboard.open(newline="") as file:
        rows = [(r["agent"], float(r["grounded_recall"]), False) for r in csv.DictReader(file)]
    with args.standing.open(newline="") as file:
        for r in csv.DictReader(file):
            if r["run"] == args.run and r["config"] == "panel":
                rows.append(("eyeful" if r["first"] == "all" else f"eyeful, top {r['first']} only", float(r["grounded_recall"]), True))
    rows.sort(key=lambda row: row[1])

    fig, ax = plt.subplots(figsize=(12, 6.75))
    reach = max(recall for _, recall, _ in rows)
    for y, (name, recall, ours) in enumerate(rows):
        ax.barh(y, recall, height=0.62, color=OURS if ours else OTHERS)
        ax.text(-0.012 * reach, y, name, ha="right", va="center", fontsize=16, fontweight="bold" if ours else "normal")
        ax.text(recall + 0.012 * reach, y, f"{recall:.0%}", va="center", fontsize=16, fontweight="bold" if ours else "normal", color=INK if ours else MUTED)
    ax.set_xlim(0, reach * 1.12)
    ax.set_ylim(-0.7, len(rows) - 0.3)
    ax.axis("off")
    fig.text(0.045, 0.93, args.title, fontsize=26, fontweight="bold", va="top")
    fig.text(0.045, 0.045, args.note, fontsize=10.5, color=MUTED, va="bottom")
    fig.subplots_adjust(left=0.3, right=0.96, top=0.8, bottom=0.12)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(args.out.with_suffix(".svg"))
    fig.savefig(args.out.with_suffix(".png"), dpi=200)


if __name__ == "__main__":
    main()
