"""Scores SWE-bench subjects with the benchmark's own harness: each fix the fix agent wrote is a prediction, and a reported finding is valid when its fix resolves the task, its FAIL_TO_PASS and PASS_TO_PASS tests passing."""

from __future__ import annotations

import argparse
import json
import subprocess
import tempfile
from pathlib import Path

from model import Subject, load_reviews, load_subjects

HARNESS = ["uv", "run", "--no-project", "--with", "swebench==5.0.2", "python", "-m", "swebench.harness.run_evaluation"]
TASK = "task"


def patch(subject: Subject, fix: list[dict]) -> str:
    with tempfile.TemporaryDirectory() as tree:
        subprocess.run(["git", "-C", subject.repo, "worktree", "add", "--quiet", "--detach", tree, subject.head], check=True)
        try:
            for edit in fix:
                path = Path(tree) / edit["path"]
                text = path.read_text() if edit["old"] else ""
                if edit["old"] and edit["old"] not in text:
                    raise RuntimeError(f"{edit['path']}: the fix's old text is not in the change")
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(text.replace(edit["old"], edit["new"], 1) if edit["old"] else edit["new"])
            subprocess.run(["git", "-C", tree, "add", "--all", "--", ".", ":!.eyeful"], check=True)
            return subprocess.run(["git", "-C", tree, "diff", "--cached"], capture_output=True, text=True, check=True).stdout
        finally:
            subprocess.run(["git", "-C", subject.repo, "worktree", "remove", "--force", tree], check=True)


def resolved(subject: Subject, fix: list[dict], run: str, data: Path, logs: Path) -> bool:
    report = logs / "logs" / "run_evaluation" / run / "eyeful" / subject.id / "report.json"
    if not report.exists():
        predictions = logs / "predictions" / f"{run}.jsonl"
        predictions.parent.mkdir(parents=True, exist_ok=True)
        predictions.write_text(json.dumps({"instance_id": subject.id, "model_name_or_path": "eyeful", "model_patch": patch(subject, fix)}) + "\n")
        subprocess.run(
            [*HARNESS, "--dataset_name", str(data.resolve()), "--instance_ids", subject.id, "--predictions_path", str(predictions.resolve()), "--run_id", run, "--max_workers", "1"],
            cwd=logs, check=True,
        )
    return bool(report.exists() and json.loads(report.read_text())[subject.id]["resolved"])


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--subjects", type=Path, required=True)
    parser.add_argument("--data", type=Path, required=True, help="the SWE-bench Verified rows the subjects were built from, as JSON; the harness reads the tasks' tests from them")
    parser.add_argument("--runs", type=Path, required=True, help="the directory run.py wrote")
    parser.add_argument("--out", type=Path, required=True, help="where the harness's logs and the scores go; a finding already judged there is not run again")
    args = parser.parse_args()
    subjects = {k: s for k, s in load_subjects(args.subjects).items() if s.dataset == "swebench"}
    args.out.mkdir(parents=True, exist_ok=True)
    fixed: dict[str, set[str]] = {}
    for path in sorted(args.runs.glob("*/*/execution/result.json")):
        name, run = path.parts[-4], path.parts[-3]
        if name not in subjects:
            continue
        config, rep = run.rsplit("-", 1)
        good = fixed.setdefault(f"{name}/eyeful/{config}/{rep}", set())
        for v in json.loads(path.read_text())["verifications"]:
            if v.get("fix") and resolved(subjects[name], v["fix"], f"{run}-{v['finding'].replace('/', '-')}", args.data, args.out):
                good.add(v["finding"])
        print(name, run, f"{len(good)} fixes resolve the task", flush=True)
    with (args.out / "scores.jsonl").open("w") as out:
        for r in load_reviews(args.runs / "reviews.jsonl"):
            if r.subject not in subjects:
                continue
            valid = sorted(f.id for f in r.findings if f.reported and f.id in fixed.get(r.run, set()))
            out.write(json.dumps({"review": r.key, "defects": {TASK: ""}, "found": [TASK] if valid else [], "valid": valid}) + "\n")


if __name__ == "__main__":
    main()
