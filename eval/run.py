"""Reviews every subject with eyeful: each configuration finds once, and the three verification conditions share those findings."""

from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import tempfile
import time
from pathlib import Path

from audit import outside
from confine import environment
from model import Condition, Finding, Review, Subject, append_review, load_reviews, load_subjects

CONFIGS: dict[str, list[str]] = {
    "panel": [],
    "single": ["generalist"],
    "fixed": ["correctness", "security"],
}
CONDITIONS: tuple[Condition, ...] = ("execution", "read_only", "none")
MERGE = 2
ATTEMPTS = 3


def review(binary: Path, subject: Subject, config: str, condition: Condition, run: Path, flags: list[str], description: Path) -> float:
    command = [
        str(binary), "review", *CONFIGS[config], "--base", subject.base, "--head", subject.head,
        "--verification", condition, "--yes", *(["--description", str(description)] if description.exists() else []), "--run", str(run),
    ]
    run.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp())
    if (run / "stages").exists():
        shutil.copytree(run / "stages", work / "stages")
    command[-1] = str(work)
    start = time.monotonic()
    with (run / "eyeful.log").open("a") as log:
        subprocess.run(command, cwd=subject.repo, stdout=log, stderr=log, env=environment(Path(subject.repo), (), flags), check=False)
    took = time.monotonic() - start
    opened = sorted({p for t in (work / "agents").glob("*.jsonl") for p in outside(t, (Path(subject.repo),))})
    (work / "outside.json").write_text(json.dumps(opened, indent=2) + "\n")
    shutil.copytree(work, run, dirs_exist_ok=True)
    shutil.rmtree(work)
    return took


def stopped(verdict: dict) -> str:
    change, base, fixed = verdict.get("change", []), verdict.get("base", ""), verdict.get("fixed", "")
    if "repro" not in verdict or verdict["status"] == "fix_verified":
        return ""
    if not change:
        return "runs"
    if len(set(change)) > 1:
        return "not_flaky"
    if change[0] != "FAILED":
        return "fails_as_claimed"
    if base in ("", "FAILED"):
        return "introduced"
    return "pass_to_pass" if fixed == "PASSED" else "fail_to_pass"


def read(subject: Subject, config: str, rep: int, condition: Condition, run: Path, seconds: float) -> Review:
    result = run / "result.json"
    if not result.exists():
        return Review(subject.id, "eyeful", config, rep, condition, "error", (), 0, 0, seconds)
    raw = json.loads(result.read_text())
    verdicts = {v["finding"]: v for v in raw["verifications"]}
    blocking = [(c["path"], c["line"]) for c in raw["comments"] if "blocking" in c["decorations"]]
    findings = tuple(
        Finding(
            path=f["path"], line=f["line"], severity=f["severity"], title=f["subject"], body=f["discussion"],
            source=f["expert"], evidence=f["evidence"], cwe=f.get("cwe", ""),
            status=verdicts.get(f["id"], {}).get("status", ""), gate=stopped(verdicts.get(f["id"], {})),
            oracle=(verdicts.get(f["id"], {}).get("repro") or f.get("repro") or {}).get("oracle", ""),
            blocking=any(p == f["path"] and abs(n - f["line"]) <= MERGE for p, n in blocking),
        )
        for f in raw["findings"]
    )
    tokens = sum(r["usage"]["tokens"] for r in raw["runs"])
    cost = sum(r["usage"]["micro_usd"] for r in raw["runs"])
    outcome = "error" if any(r.get("error") for r in raw["runs"]) else raw["outcome"]
    if json.loads((run / "outside.json").read_text()):
        outcome = "leaked"
    return Review(subject.id, "eyeful", config, rep, condition, outcome, findings, tokens, cost, seconds)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--subjects", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--eyeful", type=Path, required=True, help="the eyeful binary under test")
    parser.add_argument("--configs", default="panel,single")
    parser.add_argument("--reps", type=int, default=3)
    parser.add_argument("--conditions", default=",".join(CONDITIONS), help="execution always runs first: it is where the findings come from")
    parser.add_argument("--ids", default="")
    parser.add_argument("--pi", default="", help="the model and thinking level every pi call uses, as provider/model:thinking; pi's own settings decide otherwise")
    args = parser.parse_args()
    wanted = {i for i in args.ids.split(",") if i}
    reviews = args.out / "reviews.jsonl"
    args.out.mkdir(parents=True, exist_ok=True)
    flags: list[str] = []
    if args.pi:
        model, thinking = args.pi.rsplit(":", 1)
        flags = ["--model", model, "--thinking", thinking]
        (args.out / "agent.json").write_text(json.dumps({"pi": {"model": model, "thinking": thinking}}, indent=2) + "\n")
    kept = {r.key: r for r in load_reviews(reviews)} if reviews.exists() else {}
    for subject in load_subjects(args.subjects).values():
        if wanted and subject.id not in wanted:
            continue
        for config in args.configs.split(","):
            for rep in range(1, args.reps + 1):
                home = args.out / subject.id / f"{config}-{rep}"
                found = 0.0
                for condition in CONDITIONS:
                    if condition not in args.conditions.split(","):
                        continue
                    run = home / condition
                    clock = run / "seconds"
                    if condition != "execution" and not (run / "stages").exists() and (home / "execution" / "stages").exists():
                        shutil.copytree(home / "execution" / "stages", run / "stages")
                    for attempt in range(1, ATTEMPTS + 1):
                        if clock.exists():
                            break
                        took = review(args.eyeful, subject, config, condition, run, flags, args.subjects / subject.id / "description.md")
                        if read(subject, config, rep, condition, run, took).outcome != "error":
                            clock.write_text(str(took))
                            break
                        failed = home / f"{condition}-failed-{attempt}"
                        shutil.rmtree(failed, ignore_errors=True)
                        run.rename(failed)
                        if attempt == ATTEMPTS:
                            raise SystemExit(f"{subject.id} {config} {rep} {condition}: the review or one of its agents failed {ATTEMPTS} times; see {failed}")
                        if condition != "execution":
                            shutil.copytree(home / "execution" / "stages", run / "stages")
                    seconds = float(clock.read_text())
                    if condition == "execution":
                        staged = [p.stat().st_mtime for p in (run / "stages" / "expert").rglob("*.json")]
                        first = (run / "snapshot.json").stat().st_mtime if (run / "snapshot.json").exists() else 0.0
                        found = max(staged, default=first) - first
                    else:
                        seconds += found
                    result = read(subject, config, rep, condition, run, seconds)
                    print(result.key, result.outcome, len(result.findings), f"{seconds:.0f}s", flush=True)
                    kept[result.key] = result
                    reviews.write_text("")
                    for one in kept.values():
                        append_review(reviews, one)


if __name__ == "__main__":
    main()
