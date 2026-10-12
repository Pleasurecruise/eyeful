"""Reviews every subject with revmux, the external reference whose findings an agent validates by reading."""

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
from model import Finding, Review, Subject, append_review, load_reviews, load_subjects


def review(binary: Path, subject: Subject, profile: str, home: Path) -> float:
    tasks = Path(tempfile.mkdtemp())
    new = subprocess.run(
        [str(binary), "new", "--task", "review", "--run", "01", "--tasks-dir", str(tasks)],
        cwd=subject.repo, capture_output=True, text=True, check=True,
    )
    Path(json.loads(new.stdout)["scope"]).write_text(
        f"Review the change that commit {subject.head} makes to {subject.base}.\n"
        f"Diff command: git diff {subject.base} {subject.head}\n"
    )
    start = time.monotonic()
    with (home / "report.json").open("w") as report, (home / "revmux.log").open("w") as log:
        subprocess.run(
            [
                str(binary), "--task", "review", "--run", "01", "--tasks-dir", str(tasks),
                "--workdir", subject.repo, "--profile", profile, "--no-tui",
            ],
            cwd=subject.repo, stdout=report, stderr=log, env=environment(Path(subject.repo), (tasks,), []), check=False,
        )
    took = time.monotonic() - start
    opened = sorted({p for t in tasks.rglob("agents/*.jsonl") for p in outside(t, (Path(subject.repo), tasks))})
    (home / "outside.json").write_text(json.dumps(opened, indent=2) + "\n")
    shutil.copytree(tasks, home / "tasks", dirs_exist_ok=True)
    shutil.rmtree(tasks)
    return took


def read(subject: Subject, profile: str, rep: int, home: Path, seconds: float) -> Review:
    try:
        raw = json.loads((home / "report.json").read_text())
    except (OSError, json.JSONDecodeError):
        return Review(subject.id, "revmux", profile, rep, "revmux", "error", (), 0, 0, seconds)
    findings = tuple(
        Finding(
            path=f["file"], line=f["line"], severity=f["severity"], title=f["title"], body=f["body"],
            source=",".join(f.get("sources", [])), evidence="argument", cwe="", status=f.get("verdict", ""),
            gate="", oracle="", blocking=f["severity"] in ("critical", "major"),
        )
        for f in raw.get("findings", [])
    )
    stats = raw.get("stats", {})
    outcome = "leaked" if json.loads((home / "outside.json").read_text()) else "complete"
    return Review(subject.id, "revmux", profile, rep, "revmux", outcome, findings, int(stats.get("tokens", 0)), 0, seconds)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--subjects", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--revmux", type=Path, required=True)
    parser.add_argument("--profile", default="claude-only")
    parser.add_argument("--reps", type=int, default=3)
    parser.add_argument("--ids", default="")
    args = parser.parse_args()
    wanted = {i for i in args.ids.split(",") if i}
    reviews = args.out / "reviews.jsonl"
    args.out.mkdir(parents=True, exist_ok=True)
    kept = {r.key: r for r in load_reviews(reviews)} if reviews.exists() else {}
    for subject in load_subjects(args.subjects).values():
        if wanted and subject.id not in wanted:
            continue
        for rep in range(1, args.reps + 1):
            home = args.out / subject.id / f"{args.profile}-{rep}"
            clock = home / "seconds"
            if not clock.exists():
                home.mkdir(parents=True, exist_ok=True)
                took = review(args.revmux, subject, args.profile, home)
                if read(subject, args.profile, rep, home, took).outcome == "error":
                    shutil.rmtree(home)
                    raise SystemExit(f"{subject.id} {rep}: revmux failed; run again to resume")
                clock.write_text(str(took))
            result = read(subject, args.profile, rep, home, float(clock.read_text()))
            print(result.key, result.outcome, len(result.findings), f"{result.seconds:.0f}s", flush=True)
            kept[result.key] = result
            reviews.write_text("")
            for one in kept.values():
                append_review(reviews, one)


if __name__ == "__main__":
    main()
