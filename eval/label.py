"""Turns reviews into a blind labelling sheet, and reads the labels back with their agreement."""

from __future__ import annotations

import argparse
import csv
import hashlib
import random
from dataclasses import dataclass
from pathlib import Path

from model import Review, Verdict, load_reviews

MERGE = 2
SEED = 20616304
FIELDS = ["id", "subject", "path", "line", "title", "body", "label"]


@dataclass(frozen=True)
class Item:
    id: str
    subject: str
    path: str
    line: int
    title: str
    body: str


def item_id(subject: str, path: str, line: int) -> str:
    return hashlib.sha256(f"{subject}\0{path}\0{line}".encode()).hexdigest()[:12]


def distinct(reviews: list[Review]) -> dict[tuple[str, str, int], Item]:
    items: dict[tuple[str, str, int], Item] = {}
    rows = sorted(
        ((r.subject, f.path, f.line, f.title, f.body) for r in reviews for f in r.findings),
        key=lambda row: row[:3],
    )
    anchor: tuple[str, str, int] | None = None
    for subject, path, line, title, body in rows:
        if anchor is None or anchor[:2] != (subject, path) or line - anchor[2] > MERGE:
            anchor = (subject, path, line)
            items[anchor] = Item(item_id(*anchor), subject, path, line, title, body)
        items[(subject, path, line)] = items[anchor]
    return items


def read_labels(path: Path) -> dict[str, Verdict]:
    labels: dict[str, Verdict] = {}
    with path.open(newline="") as sheet:
        for row in csv.DictReader(sheet):
            if row["label"] == "actionable":
                labels[row["id"]] = "actionable"
            elif row["label"] == "noise":
                labels[row["id"]] = "noise"
    return labels


def kappa(first: dict[str, Verdict], second: dict[str, Verdict]) -> tuple[float, int]:
    both = sorted(set(first) & set(second))
    if not both:
        return 0.0, 0
    agree = sum(first[i] == second[i] for i in both) / len(both)
    a = sum(first[i] == "actionable" for i in both) / len(both)
    b = sum(second[i] == "actionable" for i in both) / len(both)
    chance = a * b + (1 - a) * (1 - b)
    return (1.0 if chance == 1 else (agree - chance) / (1 - chance)), len(both)


def write(path: Path, items: list[Item]) -> None:
    with path.open("w", newline="") as sheet:
        out = csv.DictWriter(sheet, FIELDS)
        out.writeheader()
        for i in items:
            out.writerow({"id": i.id, "subject": i.subject, "path": i.path, "line": i.line, "title": i.title, "body": i.body, "label": ""})


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["sheet", "second", "kappa"])
    parser.add_argument("--reviews", type=Path, nargs="+", default=[])
    parser.add_argument("--labels", type=Path, help="the first pass, filled in")
    parser.add_argument("--again", type=Path, help="the second pass")
    parser.add_argument("--out", type=Path)
    parser.add_argument("--share", type=float, default=0.2)
    args = parser.parse_args()
    if args.action == "kappa":
        value, n = kappa(read_labels(args.labels), read_labels(args.again))
        print(f"Cohen's kappa {value:.3f} over {n} findings labelled twice")
        return
    reviews = [r for path in args.reviews for r in load_reviews(path)]
    items = list({i.id: i for i in distinct(reviews).values()}.values())
    random.Random(SEED).shuffle(items)
    if args.action == "second":
        items = random.Random(SEED + 1).sample(items, max(1, round(len(items) * args.share)))
    write(args.out, items)
    print(f"{len(items)} findings to label in {args.out}; the sheet hides the tool, the configuration and the verification outcome")


if __name__ == "__main__":
    main()
