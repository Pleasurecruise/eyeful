"""Scores reviews against Code Review Bench's golden comments as its offline pipeline does: a model judges every reported finding against every golden comment of the pull request."""

from __future__ import annotations

import argparse
import json
import subprocess
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

from model import Review, load_reviews
from subjects import PULL

JUDGE = """You are evaluating AI code review tools.
Determine if the candidate issue matches the golden (expected) comment.

Golden Comment (the issue we're looking for):
{golden_comment}

Candidate Issue (from the tool's review):
{candidate}

Instructions:
- Determine if the candidate identifies the SAME underlying issue as the golden comment
- Accept semantic matches - different wording is fine if it's the same problem
- Focus on whether they point to the same bug, concern, or code issue

Respond with ONLY a JSON object:
{{"reasoning": "brief explanation", "match": true/false, "confidence": 0.0-1.0}}"""
PI = ["--mode", "json", "--no-session", "--no-tools", "--no-mcp", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files"]
PARALLEL = 8
MERGE = 2
ATTEMPTS = 5
SECONDS = 300


def judge(golden: str, candidate: str, flags: list[str]) -> dict:
    for _ in range(ATTEMPTS):
        done = subprocess.run(
            ["pi", *PI, *flags, JUDGE.format(golden_comment=golden, candidate=candidate)],
            stdin=subprocess.DEVNULL, capture_output=True, text=True, timeout=SECONDS,
        )
        text = ""
        for line in done.stdout.splitlines():
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                continue
            message = event.get("message") or {}
            if event.get("type") == "message_end" and message.get("role") == "assistant":
                text = "".join(b.get("text", "") for b in message.get("content", []) if b.get("type") == "text") or text
        try:
            answer = json.loads(text[text.index("{") : text.rindex("}") + 1])
        except ValueError:
            continue
        if isinstance(answer.get("match"), bool):
            return answer
    raise SystemExit(f"the judge gave no answer {ATTEMPTS} times; run again to resume")


def tally(review: Review, comments: list[dict[str, str]], kept: dict[str, dict]) -> tuple[int, int, int, int]:
    matched = [
        (f.path, f.line, [g["comment"] for g in comments if kept[json.dumps([g["comment"], f"{f.title}\n\n{f.body}"])]["match"]])
        for f in sorted(review.findings, key=lambda f: (f.path, f.line))
    ]
    places: list[bool] = []
    for i, (path, line, hits) in enumerate(matched):
        if i > 0 and matched[i - 1][0] == path and line - matched[i - 1][1] <= MERGE:
            places[-1] = places[-1] or bool(hits)
        else:
            places.append(bool(hits))
    return len({comment for _, _, hits in matched for comment in hits}), len(matched), len(places), places.count(False)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--golden", type=Path, required=True, help="the benchmark's golden_comments directory")
    parser.add_argument("--reviews", type=Path, nargs="+", required=True)
    parser.add_argument("--out", type=Path, required=True, help="every judgement, kept so that a second run asks nothing twice")
    parser.add_argument("--pi", required=True, help="the judge, as provider/model:thinking")
    args = parser.parse_args()
    model, thinking = args.pi.rsplit(":", 1)
    golden: dict[str, list[dict[str, str]]] = {}
    for file in sorted(args.golden.glob("*.json")):
        for row in json.loads(file.read_text()):
            if pull := PULL.fullmatch(row["url"]):
                golden["{}__{}-{}".format(*pull.groups())] = row["comments"]
    kept: dict[str, dict] = {k: v for k, v in json.loads(args.out.read_text()).items() if isinstance(v.get("match"), bool)} if args.out.exists() else {}
    reviews = [r for path in args.reviews for r in load_reviews(path) if r.subject in golden]
    pairs = {
        json.dumps([g["comment"], f"{f.title}\n\n{f.body}"])
        for r in reviews for g in golden[r.subject] for f in r.findings
    }
    todo = sorted(pairs - kept.keys())
    with ThreadPoolExecutor(PARALLEL) as pool:
        for key, answer in zip(todo, pool.map(lambda k: judge(*json.loads(k), ["--model", model, "--thinking", thinking]), todo)):
            kept[key] = answer
            args.out.write_text(json.dumps(kept, indent=2) + "\n")
    totals: dict[str, list[int]] = {}
    for r in reviews:
        found, findings, places, unmatched = tally(r, golden[r.subject], kept)
        print(f"{r.key}: {found} of {len(golden[r.subject])} golden comments, {findings} findings in {places} places, {unmatched} places match none, {r.tokens} tokens, {r.seconds:.0f}s")
        total = totals.setdefault(f"{r.tool}/{r.config}", [0, 0, 0, 0, 0, 0])
        for i, n in enumerate((found, len(golden[r.subject]), findings, places, r.tokens, 1)):
            total[i] += n
    for name, (tp, expected, candidates, places, tokens, count) in totals.items():
        recall = tp / max(expected, 1)
        for unit, n in (("finding", candidates), ("place", places)):
            precision = tp / max(n, 1)
            print(f"{name} per {unit}: precision {precision:.1%}, recall {recall:.1%}, F1 {2 * precision * recall / max(precision + recall, 1e-9):.1%} ({tp} of {expected} golden comments, {n} {unit}s)")
        print(f"{name}: {tokens // count} tokens a review")


if __name__ == "__main__":
    main()
