"""Scores Code Review Bench reviews with the benchmark's own judge, step 3 of its offline pipeline, and counts recall under its default Core profile."""

from __future__ import annotations

import argparse
import json
import os
import subprocess
from pathlib import Path

from model import load_reviews
from subjects import PULL

CORE = frozenset({"bug", "security", "concurrency", "data", "api", "perf", "test_gap", "doc_defect"})


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--benchmark", type=Path, required=True, help="the benchmark's offline directory")
    parser.add_argument("--reviews", type=Path, nargs="+", required=True)
    parser.add_argument("--out", type=Path, required=True, help="the judge's working directory and the scores; a review already judged there is not judged again")
    args = parser.parse_args()
    golden: dict[str, tuple[str, list[dict[str, str]]]] = {}
    for file in sorted((args.benchmark / "golden_comments").glob("*.json")):
        for row in json.loads(file.read_text()):
            if pull := PULL.fullmatch(row["url"]):
                golden["{}__{}-{}".format(*pull.groups())] = (row["url"], row["comments"])
    reviews = [r for path in args.reviews for r in load_reviews(path) if r.subject in golden and r.outcome not in ("error", "leaked")]
    data: dict[str, dict] = {}
    candidates: dict[str, dict[str, list[dict]]] = {}
    for r in reviews:
        url, comments = golden[r.subject]
        data.setdefault(url, {"pr_title": "", "golden_comments": comments, "reviews": []})["reviews"].append({"tool": r.name, "repo_name": r.subject, "pr_url": url, "review_comments": []})
        candidates.setdefault(url, {})[r.name] = [{"text": f.text, "path": f.path, "line": f.line, "source": "extracted"} for f in r.findings if f.reported]
    judged = args.out / "results" / os.environ["MARTIAN_MODEL"].strip().replace("/", "_")
    judged.mkdir(parents=True, exist_ok=True)
    (args.out / "results" / "benchmark_data.json").write_text(json.dumps(data, indent=2) + "\n")
    (judged / "candidates.json").write_text(json.dumps(candidates, indent=2) + "\n")
    subprocess.run(
        ["uv", "run", "--project", str(args.benchmark.resolve()), "python", "-m", "code_review_benchmark.step3_judge_comments", "--no-dedup"],
        cwd=args.out, env=os.environ | {"PYTHONPATH": str(args.benchmark.resolve())}, check=True,
    )
    evaluations = json.loads((judged / "evaluations.json").read_text())
    with (args.out / "scores.jsonl").open("w") as out:
        for r in reviews:
            url, comments = golden[r.subject]
            verdict = evaluations[url][r.name]
            unmatched = {fp["candidate"] for fp in verdict["false_positives"]}
            matched = {tp["golden_comment"] for tp in verdict["true_positives"]}
            defects = {str(i): g["category"] for i, g in enumerate(comments) if g["category"] in CORE}
            out.write(json.dumps({
                "review": r.key, "defects": defects,
                "found": [i for i in defects if comments[int(i)]["comment"] in matched],
                "valid": [f.id for f in r.findings if f.reported and f.text not in unmatched],
            }) + "\n")


if __name__ == "__main__":
    main()
