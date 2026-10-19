"""Answers the research questions from the reviews and the benchmarks' scores: detection, noise, verifiability and cost, each with a 95% interval."""

from __future__ import annotations

import argparse
import json
import math
import random
from collections import Counter, defaultdict
from collections.abc import Callable, Sequence
from dataclasses import dataclass
from pathlib import Path

from model import ALL_GATES, SEED, Finding, Review, Subject, load_reviews, load_subjects

RESAMPLES = 10_000
CONFIRMED = ("reproduced", ALL_GATES)
AGREE = 2
CONDITIONS = ("execution", "read_only", "none")


@dataclass(frozen=True)
class Estimate:
    value: float
    low: float
    high: float

    def __str__(self) -> str:
        if math.isnan(self.value):
            return "n/a"
        return f"{self.value:.2f} [{self.low:.2f}, {self.high:.2f}]"


@dataclass(frozen=True)
class Score:
    defects: dict[str, str]
    found: frozenset[str]
    valid: frozenset[str]


def ratio(pairs: Sequence[tuple[float, float]]) -> float:
    below = sum(d for _, d in pairs)
    return sum(n for n, _ in pairs) / below if below else math.nan


def interval(value: float, draws: list[float]) -> Estimate:
    draws = sorted(d for d in draws if not math.isnan(d))
    if not draws:
        return Estimate(math.nan, math.nan, math.nan)
    return Estimate(value, draws[int(0.025 * len(draws))], draws[min(len(draws) - 1, int(0.975 * len(draws)))])


def bootstrap(per_subject: dict[str, tuple[float, float]]) -> Estimate:
    rows = list(per_subject.values())
    if not rows:
        return Estimate(math.nan, math.nan, math.nan)
    rng = random.Random(SEED)
    return interval(ratio(rows), [ratio(rng.choices(rows, k=len(rows))) for _ in range(RESAMPLES)])


def difference(a: dict[str, tuple[float, float]], b: dict[str, tuple[float, float]]) -> Estimate:
    shared = sorted(set(a) & set(b))
    if not shared:
        return Estimate(math.nan, math.nan, math.nan)
    rng = random.Random(SEED)
    draws = []
    for _ in range(RESAMPLES):
        picked = rng.choices(shared, k=len(shared))
        draws.append(ratio([a[s] for s in picked]) - ratio([b[s] for s in picked]))
    return interval(ratio([a[s] for s in shared]) - ratio([b[s] for s in shared]), draws)


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


def detected(reviews: list[Review], scores: dict[str, Score], need: int) -> dict[tuple[str, str], bool]:
    runs: dict[tuple[str, str], int] = Counter()
    known: set[tuple[str, str]] = set()
    for r in reviews:
        known |= {(r.subject, d) for d in scores[r.key].defects}
        runs.update((r.subject, d) for d in scores[r.key].found)
    return {k: runs[k] >= need for k in known}


def compare(arms: dict[str, dict[tuple[str, str], bool]], categories: dict[tuple[str, str], str]) -> str:
    rows = []
    for name, hits in arms.items():
        per_subject: dict[str, tuple[float, float]] = defaultdict(lambda: (0.0, 0.0))
        for (subject, _), hit in hits.items():
            per_subject[subject] = (per_subject[subject][0] + hit, per_subject[subject][1] + 1)
        recall = bootstrap(dict(per_subject))
        kinds = len({categories[k] for k, hit in hits.items() if hit and categories[k]})
        rows.append([name, str(len({s for s, _ in hits})), str(len(hits)), str(recall), str(kinds)])
    out = table(["Configuration", "Subjects", "Known defects", "Recall", "Categories found"], rows)
    names = list(arms)
    tests = []
    for i, a in enumerate(names):
        for b in names[i + 1 :]:
            shared = arms[a].keys() & arms[b].keys()
            only_a = sum(arms[a][k] and not arms[b][k] for k in shared)
            only_b = sum(arms[b][k] and not arms[a][k] for k in shared)
            tests.append([f"{a} vs {b}", str(len(shared)), str(only_a), str(only_b), f"{mcnemar(only_a, only_b):.3f}"])
    return out + "\n\n" + table(["Pair", "Known defects", "Only the first", "Only the second", "McNemar exact p"], tests)


def detection(reviews: list[Review], scores: dict[str, Score]) -> str:
    found = [r for r in reviews if r.condition in ("none", "revmux")]
    categories = {(r.subject, d): c for r in found for d, c in scores[r.key].defects.items()}
    configs = sorted({(r.tool, r.config) for r in found if r.config != "fixed"})
    arms = {f"{tool} {config}": detected([r for r in found if (r.tool, r.config) == (tool, config)], scores, AGREE) for tool, config in configs}
    single = [r for r in found if (r.tool, r.config) == ("eyeful", "single")]
    if single:
        arms["eyeful single, any of three runs"] = detected(single, scores, 1)
    out = compare(arms, categories)
    fixed = [r for r in found if r.config == "fixed"]
    if fixed:
        first = [r for r in found if r.config == "panel" and r.rep == 1 and r.subject in {f.subject for f in fixed}]
        out += "\n\nPlanner ablation, one run each:\n\n" + compare({"eyeful panel, run 1": detected(first, scores, 1), "eyeful fixed": detected(fixed, scores, 1)}, categories)
    return out


def noise(reviews: list[Review], subjects: dict[str, Subject], scores: dict[str, Score]) -> str:
    rows = []
    precision: dict[tuple[str, str], dict[str, tuple[float, float]]] = {}
    for tool, config, condition in sorted({(r.tool, r.config, r.condition) for r in reviews}):
        arm = [
            r for r in reviews if (r.tool, r.config, r.condition) == (tool, config, condition)
            and (condition != "execution" or subjects[r.subject].dataset == "swebench")
        ]
        if not arm:
            continue
        precision[(f"{tool} {config}", condition)] = by_subject(arm, lambda r: (
            float(sum(f.id in scores[r.key].valid for f in r.findings if f.reported)), float(sum(f.reported for f in r.findings)),
        ))
        reported = bootstrap(by_subject(arm, lambda r: (float(sum(f.reported for f in r.findings)), 1.0)))
        rows.append([f"{tool} {config}", condition, str(len(arm)), str(reported), str(bootstrap(precision[(f"{tool} {config}", condition)]))])
    out = table(["Configuration", "Condition", "Reviews", "Reported per review", "Precision"], rows)
    pairs = [
        [f"{a[0]}: {a[1]} − {b[1]}", str(difference(precision[a], precision[b]))]
        for a in precision for b in precision
        if a[0] == b[0] and a[1] in CONDITIONS and b[1] in CONDITIONS and CONDITIONS.index(a[1]) < CONDITIONS.index(b[1])
    ]
    return out + "\n\n" + table(["Paired difference in precision", "Estimate"], pairs)


def verifiability(reviews: list[Review], scores: dict[str, Score]) -> str:
    executed = [(r, f) for r in reviews for f in r.findings if f.evidence == "test" and f.severity != "note"]
    gates = Counter(f.gate or f.status for _, f in executed)
    oracles = Counter(f.oracle for _, f in executed if f.status in CONFIRMED)
    rate = bootstrap(by_subject(reviews, lambda r: (
        float(sum(f.status == ALL_GATES for f in r.findings if f.evidence == "test" and f.severity != "note")),
        float(sum(f.evidence == "test" and f.severity != "note" for f in r.findings)),
    )))
    loss = bootstrap(by_subject(reviews, lambda r: (
        float(sum(f.status not in CONFIRMED for f in r.findings if f.id in scores[r.key].valid)),
        float(sum(f.id in scores[r.key].valid for f in r.findings)),
    )))
    blocking = [(r, f) for r in reviews for f in r.findings if f.blocking]
    unruled = [(r, f) for r in reviews for f in r.findings if f.status in CONFIRMED]
    rows = [
        ["Findings executed", str(len(executed))],
        ["Verifiable rate: passed all six gates", str(rate)],
        ["Where the others stopped", ", ".join(f"{g} {n}" for g, n in gates.most_common() if g != ALL_GATES) or "nowhere"],
        ["Oracles of the reproduced findings", ", ".join(f"{o} {n}" for o, n in oracles.most_common()) or "none"],
        ["Verification loss: resolving findings execution does not confirm", str(loss)],
        ["Blocking under the oracle rule, and the share that resolves", f"{len(blocking)}, {share(blocking, scores)}"],
        ["Blocking if every reproduction may block, and the share that resolves", f"{len(unruled)}, {share(unruled, scores)}"],
    ]
    return table(["Measure", "Value"], rows)


def share(pairs: list[tuple[Review, Finding]], scores: dict[str, Score]) -> str:
    return f"{sum(f.id in scores[r.key].valid for r, f in pairs) / len(pairs):.2f}" if pairs else "n/a"


def cost(reviews: list[Review], left_out: Counter[str]) -> str:
    rows = []
    for tool, config, condition in sorted({(r.tool, r.config, r.condition) for r in reviews}):
        arm = [r for r in reviews if (r.tool, r.config, r.condition) == (tool, config, condition)]
        tokens = bootstrap(by_subject(arm, lambda r: (r.tokens / 1e3, 1.0)))
        money = bootstrap(by_subject(arm, lambda r: (r.micro_usd / 1e6, 1.0)))
        seconds = bootstrap(by_subject(arm, lambda r: (r.seconds, 1.0)))
        partial = sum(r.outcome != "complete" for r in arm)
        rows.append([f"{tool} {config}", condition, str(len(arm)), str(left_out[f"{tool} {config} {condition}"]), str(tokens), str(money), str(seconds), str(partial)])
    return table(["Configuration", "Condition", "Reviews", "Left out: read outside", "Tokens (thousands)", "US dollars", "Seconds", "Partial"], rows)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--subjects", type=Path, required=True)
    parser.add_argument("--reviews", type=Path, nargs="+", required=True)
    parser.add_argument("--scores", type=Path, nargs="+", required=True, help="the scores resolve.py, match.py and reviewbench.py scores wrote")
    parser.add_argument("--dataset", required=True, choices=["swebench", "crb", "reviewbench"])
    args = parser.parse_args()
    subjects = {k: s for k, s in load_subjects(args.subjects).items() if s.dataset == args.dataset}
    everything = [r for path in args.reviews for r in load_reviews(path) if r.subject in subjects and r.outcome != "error"]
    left_out = Counter(f"{r.tool} {r.config} {r.condition}" for r in everything if r.outcome == "leaked")
    scores = {
        raw["review"]: Score(raw["defects"], frozenset(raw["found"]), frozenset(raw["valid"]))
        for path in args.scores for raw in map(json.loads, path.read_text().splitlines())
    }
    reviews = [r for r in everything if r.outcome != "leaked"]
    missing = [r.key for r in reviews if r.key not in scores]
    if missing:
        raise SystemExit(f"{len(missing)} reviews have no score, such as {missing[0]}; run the dataset's scorer first")
    print(f"{args.dataset}: {len(subjects)} subjects, {len(reviews)} reviews, {sum(left_out.values())} left out because their agents read outside the repository.\n")
    print("## RQ1: detection\n\n" + detection(reviews, scores))
    print("\n## RQ2: noise\n\n" + noise(reviews, subjects, scores))
    if args.dataset == "swebench":
        print("\n## RQ3: verifiability\n\n" + verifiability([r for r in reviews if r.condition == "execution" and r.config != "fixed"], scores))
    print("\n## Cost\n\n" + cost(reviews, left_out))


if __name__ == "__main__":
    main()
