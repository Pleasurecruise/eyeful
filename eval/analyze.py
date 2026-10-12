"""Answers the research questions from the reviews: detection, noise, verifiability and cost, each with a 95% interval."""

from __future__ import annotations

import argparse
import math
import random
from collections import Counter, defaultdict
from collections.abc import Callable, Sequence
from dataclasses import dataclass
from pathlib import Path

from label import distinct, read_labels
from model import ALL_GATES, Review, Subject, Verdict, load_reviews, load_subjects

SEED = 20616304
RESAMPLES = 10_000
CONFIRMED = ("reproduced", ALL_GATES)
STRONG = ("implicit", "existing", "spec")


@dataclass(frozen=True)
class Estimate:
    value: float
    low: float
    high: float

    def __str__(self) -> str:
        if math.isnan(self.value):
            return "n/a"
        return f"{self.value:.2f} [{self.low:.2f}, {self.high:.2f}]"


def ratio(pairs: Sequence[tuple[float, float]]) -> float:
    below = sum(d for _, d in pairs)
    return sum(n for n, _ in pairs) / below if below else math.nan


def bootstrap(per_subject: dict[str, tuple[float, float]]) -> Estimate:
    rows = list(per_subject.values())
    if not rows:
        return Estimate(math.nan, math.nan, math.nan)
    rng = random.Random(SEED)
    draws = sorted(
        v for v in (ratio(rng.choices(rows, k=len(rows))) for _ in range(RESAMPLES)) if not math.isnan(v)
    )
    if not draws:
        return Estimate(math.nan, math.nan, math.nan)
    return Estimate(ratio(rows), draws[int(0.025 * len(draws))], draws[min(len(draws) - 1, int(0.975 * len(draws)))])


def difference(a: dict[str, tuple[float, float]], b: dict[str, tuple[float, float]]) -> Estimate:
    shared = sorted(set(a) & set(b))
    if not shared:
        return Estimate(math.nan, math.nan, math.nan)
    rng = random.Random(SEED)
    draws: list[float] = []
    for _ in range(RESAMPLES):
        picked = rng.choices(shared, k=len(shared))
        d = ratio([a[s] for s in picked]) - ratio([b[s] for s in picked])
        if not math.isnan(d):
            draws.append(d)
    draws.sort()
    if not draws:
        return Estimate(math.nan, math.nan, math.nan)
    value = ratio([a[s] for s in shared]) - ratio([b[s] for s in shared])
    return Estimate(value, draws[int(0.025 * len(draws))], draws[min(len(draws) - 1, int(0.975 * len(draws)))])


def mcnemar(only_a: int, only_b: int) -> float:
    n = only_a + only_b
    if n == 0:
        return 1.0
    tail = sum(math.comb(n, k) for k in range(min(only_a, only_b) + 1)) / 2**n
    return min(1.0, 2 * tail)


def by_subject(reviews: Sequence[Review], count: Callable[[Review], tuple[float, float]]) -> dict[str, tuple[float, float]]:
    sums: dict[str, tuple[float, float]] = defaultdict(lambda: (0.0, 0.0))
    for r in reviews:
        n, d = count(r)
        sums[r.subject] = (sums[r.subject][0] + n, sums[r.subject][1] + d)
    return dict(sums)


def table(header: list[str], rows: list[list[str]]) -> str:
    out = ["| " + " | ".join(header) + " |", "| " + " | ".join("---" for _ in header) + " |"]
    return "\n".join(out + ["| " + " | ".join(r) + " |" for r in rows])


def detection(groups: dict[str, list[Review]], subjects: dict[str, Subject]) -> str:
    def found(r: Review) -> bool:
        return any(subjects[r.subject].hit(f.path, f.line) for f in r.findings)

    rows: list[list[str]] = []
    detected: dict[str, dict[str, bool]] = {}
    for name, reviews in groups.items():
        recall = bootstrap(by_subject(reviews, lambda r: (float(found(r)), 1.0)))
        size = bootstrap(by_subject(reviews, lambda r: (float(len(r.findings)), 1.0)))
        per_million = bootstrap(by_subject(reviews, lambda r: (float(found(r)), r.tokens / 1e6)))
        cwes = len({f.cwe for r in reviews for f in r.findings if f.cwe})
        hits: dict[str, list[bool]] = defaultdict(list)
        for r in reviews:
            hits[r.subject].append(found(r))
        detected[name] = {s: sum(h) * 2 > len(h) for s, h in hits.items()}
        rows.append([name, str(len(hits)), str(len(reviews)), str(recall), str(size), str(cwes), str(per_million)])
    out = table(["Configuration", "Subjects", "Reviews", "Recall", "Findings per review", "CWE categories", "Detections per 1M tokens"], rows)
    names = list(detected)
    tests: list[list[str]] = []
    for i, a in enumerate(names):
        for b in names[i + 1 :]:
            shared = set(detected[a]) & set(detected[b])
            only_a = sum(detected[a][s] and not detected[b][s] for s in shared)
            only_b = sum(detected[b][s] and not detected[a][s] for s in shared)
            tests.append([f"{a} vs {b}", str(len(shared)), str(only_a), str(only_b), f"{mcnemar(only_a, only_b):.3f}"])
    return out + "\n\n" + table(["Pair", "Subjects", "Only the first", "Only the second", "McNemar p"], tests)


def noise(groups: dict[str, list[Review]], subjects: dict[str, Subject], labels: dict[tuple[str, str, int], Verdict]) -> str:
    rows: list[list[str]] = []
    precision: dict[str, dict[str, tuple[float, float]]] = {}
    for name, reviews in groups.items():
        def judged(r: Review) -> tuple[float, float]:
            got = [labels.get((r.subject, f.path, f.line)) for f in r.findings]
            return float(got.count("actionable")), float(got.count("actionable") + got.count("noise"))

        def known(r: Review) -> tuple[float, float]:
            return float(sum(subjects[r.subject].hit(f.path, f.line) for f in r.findings)), float(len(r.findings))

        precision[name] = by_subject(reviews, judged)
        reported = bootstrap(by_subject(reviews, lambda r: (float(len(r.findings)), 1.0)))
        labelled = int(sum(d for _, d in precision[name].values()))
        rows.append([name, str(reported), str(bootstrap(by_subject(reviews, known))), str(labelled), str(bootstrap(precision[name]))])
    out = table(["Condition", "Reported per review", "Share on the known defect", "Labelled", "Actionable precision"], rows)
    names = list(precision)
    pairs = [
        [f"{a} − {b}", str(difference(precision[a], precision[b]))]
        for i, a in enumerate(names) for b in names[i + 1 :]
        if a.split(",")[0] == b.split(",")[0] or "revmux" in a + b
    ]
    return out + "\n\n" + table(["Paired difference in precision", "Estimate"], pairs)


def verifiability(reviews: list[Review], subjects: dict[str, Subject], labels: dict[tuple[str, str, int], Verdict]) -> str:
    repro = [(r, f) for r in reviews for f in r.findings if f.evidence == "repro_test"]
    everything = [(r, f) for r in reviews for f in r.findings]
    gates = Counter(f.gate or f.status for _, f in repro)
    oracles = Counter(f.oracle for _, f in repro if f.status in CONFIRMED)
    rate = bootstrap(by_subject(reviews, lambda r: (
        float(sum(f.status == ALL_GATES for f in r.findings if f.evidence == "repro_test")),
        float(sum(f.evidence == "repro_test" for f in r.findings)),
    )))
    proposed = bootstrap(by_subject(reviews, lambda r: (float(sum(f.evidence == "repro_test" for f in r.findings)), float(len(r.findings)))))
    known_loss = bootstrap(by_subject(reviews, lambda r: (
        float(sum(f.status not in CONFIRMED for f in r.findings if subjects[r.subject].hit(f.path, f.line))),
        float(sum(subjects[r.subject].hit(f.path, f.line) for f in r.findings)),
    )))
    loss = bootstrap(by_subject(reviews, lambda r: (
        float(sum(f.status not in CONFIRMED for f in r.findings if labels.get((r.subject, f.path, f.line)) == "actionable")),
        float(sum(labels.get((r.subject, f.path, f.line)) == "actionable" for f in r.findings)),
    )))
    ruled = sum(f.blocking for _, f in everything)
    unruled = sum(f.status in CONFIRMED for _, f in everything)
    strong = sum(f.status in CONFIRMED and f.oracle in STRONG for _, f in everything)
    rows = [
        ["Findings", str(len(everything))],
        ["With a reproduction test", f"{len(repro)} ({proposed})"],
        ["Verifiable rate: passed all six gates", str(rate)],
        ["Where the others stopped", ", ".join(f"{g} {n}" for g, n in gates.most_common() if g != ALL_GATES) or "nowhere"],
        ["Oracles of the confirmed findings", ", ".join(f"{o} {n}" for o, n in oracles.most_common()) or "none"],
        ["Verification loss on the known defect", str(known_loss)],
        ["Verification loss on findings labelled actionable", str(loss)],
        ["Blocking with the oracle rule", f"{ruled} ({strong} confirmed with an outside oracle)"],
        ["Blocking if any confirmed reproduction blocked", str(unruled)],
    ]
    return table(["Measure", "Value"], rows)


def cost(groups: dict[str, list[Review]]) -> str:
    rows: list[list[str]] = []
    for name, reviews in groups.items():
        tokens = bootstrap(by_subject(reviews, lambda r: (r.tokens / 1e3, 1.0)))
        money = bootstrap(by_subject(reviews, lambda r: (r.micro_usd / 1e6, 1.0)))
        seconds = bootstrap(by_subject(reviews, lambda r: (r.seconds, 1.0)))
        partial = sum(r.outcome != "complete" for r in reviews)
        rows.append([name, str(len(reviews)), str(tokens), str(money), str(seconds), str(partial)])
    return table(["Review", "Reviews", "Tokens (thousands)", "US dollars", "Seconds", "Not complete"], rows)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--subjects", type=Path, required=True)
    parser.add_argument("--reviews", type=Path, nargs="+", required=True)
    parser.add_argument("--labels", type=Path)
    parser.add_argument("--dataset", default="", help="report one dataset only")
    args = parser.parse_args()
    subjects = load_subjects(args.subjects)
    reviews = [
        r for path in args.reviews for r in load_reviews(path)
        if r.subject in subjects and r.outcome != "error" and args.dataset in ("", subjects[r.subject].dataset)
    ]
    verdicts = read_labels(args.labels) if args.labels else {}
    items = distinct(reviews)
    labels: dict[tuple[str, str, int], Verdict] = {k: verdicts[i.id] for k, i in items.items() if i.id in verdicts}
    found: dict[str, list[Review]] = defaultdict(list)
    conditions: dict[str, list[Review]] = defaultdict(list)
    for r in reviews:
        if r.tool == "revmux":
            found[f"revmux {r.config}"].append(r)
            conditions[f"revmux {r.config}"].append(r)
        else:
            conditions[f"eyeful {r.config}, {r.condition}"].append(r)
            if r.condition == "none":
                found[f"eyeful {r.config}"].append(r)
    executed = [r for r in reviews if r.condition == "execution"]
    print(f"{len({r.subject for r in reviews})} subjects, {len(reviews)} reviews, {len(verdicts)} labels.\n")
    print("## RQ1: detection\n\n" + detection(dict(found), subjects))
    print("\n## RQ2: noise\n\n" + noise(dict(conditions), subjects, labels))
    print("\n## RQ3: verifiability\n\n" + verifiability(executed, subjects, labels))
    print("\n## Cost\n\n" + cost(dict(conditions)))


if __name__ == "__main__":
    main()
