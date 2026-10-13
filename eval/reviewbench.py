"""Prepares ReviewBench pull requests as review subjects, and writes eyeful's comments on them in the benchmark's judging input format."""

from __future__ import annotations

import argparse
import csv
import hashlib
import json
import os
import subprocess
from pathlib import Path

from model import Subject
from subjects import HISTORY

MIRROR = "https://github.com/review-bench"
DEPTHS = (HISTORY, 3000)
POINTERS = ["-c", "filter.lfs.process=", "-c", "filter.lfs.smudge=", "-c", "filter.lfs.clean=", "-c", "filter.lfs.required=false"]
NITPICK = "nitpick"
COLUMNS = (
    "run", "config", "pr", "lines", "outcome", "findings", "comments", "nitpicks", "disputed", "tokens", "tokens_expert", "tokens_judge", "seconds",
    "handed_in", "golden", "golden_found", "matched", "unmatched_real", "unmatched_false", "grounded_recall", "grounded_precision",
)


def key(row: dict) -> str:
    return f"{row['repo'].removeprefix('https://github.com/').replace('/', '_')}_{row['pr_number']}-{row['head'][:8]}"


def fetch(repo: Path, row: dict) -> str:
    remotes = (f"{MIRROR}/{row['repo'].removeprefix('https://github.com/').replace('/', '_')}.git", f"{row['repo']}.git")
    env = os.environ | {"GIT_LFS_SKIP_SMUDGE": "1", "GIT_TERMINAL_PROMPT": "0"}
    for depth in DEPTHS:
        for remote in remotes:
            done = subprocess.run(["git", "-C", str(repo), "fetch", "--quiet", f"--depth={depth}", remote, row["base"], row["head"]], capture_output=True, env=env)
            if done.returncode == 0:
                break
        base = subprocess.run(["git", "-C", str(repo), "merge-base", row["base"], row["head"]], capture_output=True, text=True)
        if base.returncode == 0:
            (repo / ".git" / "FETCH_HEAD").unlink(missing_ok=True)
            return base.stdout.strip()
    raise RuntimeError("the base and the head share no history that could be fetched")


def subject(row: dict, out: Path, repos: Path) -> Subject:
    name = key(row)
    home = out / name
    home.mkdir(parents=True, exist_ok=True)
    (home / "description.md").write_text(f"{row['title']}\n\n{row.get('body') or ''}".strip() + "\n")
    if (home / "subject.json").exists():
        return Subject.load(home / "subject.json")
    repo = repos / hashlib.sha256(name.encode()).hexdigest()[:12]
    repo.mkdir(parents=True, exist_ok=True)
    subprocess.run(["git", "init", "--quiet"], cwd=repo, check=True)
    base = fetch(repo, row)
    subprocess.run(["git", "-C", str(repo), *POINTERS, "-c", "advice.detachedHead=false", "checkout", "--quiet", "--force", "--detach", row["head"]], check=True)
    made = Subject(name, "reviewbench", "", str(repo.resolve()), base, row["head"], "", ())
    made.save(home / "subject.json")
    return made


def held(rows: dict[str, dict], without: set[str], count: int) -> dict[str, dict]:
    rest = sorted((name for name in rows if name not in without), key=lambda name: hashlib.sha256(name.encode()).hexdigest())
    return {name: rows[name] for name in rest[:count]}


def candidate(row: dict, config: str, result: dict, seconds: float, folded: bool) -> dict:
    agent = f"eyeful-{config}"
    findings = [
        {
            "producer": agent, "file": c["path"], "start_line": c["line"], "end_line": c["line"],
            "message": "\n\n".join(part.strip() for part in (c["subject"], c.get("discussion", ""), c.get("scenario", "")) if part.strip()),
        }
        for c in result["comments"]
        if folded or (c["label"] != NITPICK and not c.get("objection"))
    ]
    return {
        "pr": {"repo": row["repo"], "pr_number": row["pr_number"], "base": row["base"], "head": row["head"]},
        "agent": agent,
        "findings": findings,
        "usage": {"time_in_ms": int(seconds * 1000), "total_tokens": sum(r["usage"]["tokens"] for r in result["runs"])},
    }


def table(label: str, runs: Path, scores: Path, subjects: Path, rows: dict[str, dict]) -> list[dict]:
    out = []
    for path in sorted(runs.rglob("execution/result.json")):
        name, config = path.parts[-4], path.parts[-3].rsplit("-", 1)[0]
        if name not in rows:
            continue
        result = json.loads(path.read_text())
        clock = path.parent / "seconds"
        made = Subject.load(subjects / name / "subject.json")
        stat = subprocess.run(["git", "-C", made.repo, "diff", "--shortstat", made.base, made.head], capture_output=True, text=True, check=True).stdout
        spent = {role: sum(r["usage"]["tokens"] for r in result["runs"] if r["role"] == role) for role in ("expert", "judge")}
        row = {
            "run": label, "config": config, "pr": name, "lines": sum(int(part.split()[0]) for part in stat.split(",")[1:]), "outcome": result["outcome"], "findings": len(result["findings"]), "comments": len(result["comments"]),
            "nitpicks": sum(c["label"] == NITPICK for c in result["comments"]), "disputed": sum(bool(c.get("objection")) for c in result["comments"]),
            "tokens": sum(r["usage"]["tokens"] for r in result["runs"]), "tokens_expert": spent["expert"], "tokens_judge": spent["judge"],
            "seconds": round(float(clock.read_text())) if clock.exists() else "",
        }
        judged = scores / config / "results.checkpoint.json"
        score = json.loads(judged.read_text())["completed"].get(name) if judged.exists() else None
        if score:
            m = score["metrics"]["overall"]
            row |= {
                "handed_in": m["candidate_count"], "golden": m["golden_tp_count"], "golden_found": m["golden_covered_count"], "matched": m["matched_tp_count"],
                "unmatched_real": m["unmatched_tp_count"], "unmatched_false": m["unmatched_fp_count"],
                "grounded_recall": m["grounded_recall"], "grounded_precision": m["grounded_precision"],
            }
        out.append(row)
    return out


def standing(label: str, config: str, judged: dict, golden: Path, first: int) -> dict:
    pulls = comments = known = matched = right = found = 0
    for name, score in judged["completed"].items():
        handed = judged["details"][name]["candidate_findings"]
        shown = handed[:first] if first else handed
        hits = [c for c in shown if c["matched_to_ground_truth"]]
        accepted = {i for i, f in enumerate(json.loads((golden / f"{name}.json").read_text())["findings"]) if f["tp_fp"] == "tp"}
        covered = {i for c in hits for i in c["covered_golden_indices"]} & accepted
        pulls, comments, known = pulls + 1, comments + len(shown), known + score["metrics"]["overall"]["golden_tp_count"]
        matched, right, found = matched + len(hits), right + sum(c["matched_truth_label"] == "tp" for c in hits), found + len(covered)
    return {
        "run": label, "config": config, "first": first or "all", "pulls": pulls, "comments": round(comments / pulls, 1),
        "grounded_recall": round(found / known, 3), "grounded_precision": round(right / matched, 3) if matched else "",
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("step", choices=["subjects", "check", "findings", "table", "standing"])
    parser.add_argument("--set", type=Path, required=True, help="the benchmark's corpus/test/test.json for the 25 test pull requests, or corpus/manifest.json for all 219")
    parser.add_argument("--without", type=Path, help="a set file whose pull requests are left out, the test set when a held-out part is drawn")
    parser.add_argument("--held", type=int, default=0, help="keep only this many pull requests of the set, the same ones every time: those whose names hash lowest")
    parser.add_argument("--subjects", type=Path, help="subjects, check and table: where each subject's description is")
    parser.add_argument("--repos", type=Path, help="subjects: where the repositories the reviewers work in go, under names that say nothing about the subject")
    parser.add_argument("--runs", type=Path, help="findings: a directory of run.py outputs, searched for result.json")
    parser.add_argument("--out", type=Path, help="findings: where the candidate files go, one directory per configuration; table: the CSV file, one row per review, which a second run is appended to; standing: the CSV file, one row per number of comments counted, which a second run is appended to")
    parser.add_argument("--scores", type=Path, help="table: the directory the benchmark's judge wrote, one directory per configuration")
    parser.add_argument("--golden", type=Path, help="standing: the benchmark's golden directory, which says which of its findings are accepted")
    parser.add_argument("--first", default="0", help="standing: how many of a review's comments count, in the order of its report, as a comma-separated list; 0 is all of them")
    parser.add_argument("--label", default="", help="table: what the run is called in the table, such as the commit the binary was built from")
    parser.add_argument("--folded", action="store_true", help="findings: also hand in the nitpicks and the comments the judge disputes, which eyeful's report folds away")
    args = parser.parse_args()
    rows = {key(r): r for r in json.loads(args.set.read_text())}
    if args.held:
        rows = held(rows, {key(r) for r in json.loads(args.without.read_text())} if args.without else set(), args.held)
    if args.step == "subjects":
        for row in rows.values():
            try:
                made = subject(row, args.subjects, args.repos)
            except (RuntimeError, subprocess.CalledProcessError) as err:
                print(key(row), "skipped:", err, flush=True)
                continue
            print(made.id, made.base[:12], made.head[:12], flush=True)
        return
    if args.step == "check":
        wrong = 0
        for name, row in rows.items():
            path = args.subjects / name / "subject.json"
            problem = "no subject"
            if path.exists():
                made = Subject.load(path)
                head = subprocess.run(["git", "-C", made.repo, "rev-parse", "HEAD"], capture_output=True, text=True).stdout.strip()
                base = subprocess.run(["git", "-C", made.repo, "merge-base", row["base"], row["head"]], capture_output=True, text=True).stdout.strip()
                dirty = subprocess.run(["git", "-C", made.repo, "status", "--porcelain"], capture_output=True, text=True).stdout.strip()
                problem = next((p for p, bad in (("the checkout is not the benchmark's head", head != row["head"] or made.head != row["head"]), ("the base is not the merge base of the benchmark's base and head", base != made.base), ("the checkout has changes", bool(dirty))) if bad), "")
            if problem:
                wrong += 1
                print(name, problem, flush=True)
        print(f"{len(rows) - wrong} of {len(rows)} subjects match the benchmark", flush=True)
        raise SystemExit(wrong > 0)
    if args.step == "table":
        fresh = not args.out.exists()
        args.out.parent.mkdir(parents=True, exist_ok=True)
        with args.out.open("a", newline="") as file:
            writer = csv.DictWriter(file, COLUMNS, restval="")
            if fresh:
                writer.writeheader()
            writer.writerows(table(args.label, args.runs, args.scores, args.subjects, rows))
        return
    if args.step == "standing":
        fresh = not args.out.exists()
        args.out.parent.mkdir(parents=True, exist_ok=True)
        with args.out.open("a", newline="") as file:
            writer = csv.DictWriter(file, ("run", "config", "first", "pulls", "comments", "grounded_recall", "grounded_precision"))
            if fresh:
                writer.writeheader()
            for judged in sorted(args.scores.glob("*/results.checkpoint.json")):
                writer.writerows(standing(args.label, judged.parent.name, json.loads(judged.read_text()), args.golden, int(first)) for first in args.first.split(","))
        return
    done: dict[str, set[str]] = {}
    for path in sorted(args.runs.rglob("execution/result.json")):
        name, config = path.parts[-4], path.parts[-3].rsplit("-", 1)[0]
        result = json.loads(path.read_text())
        if name not in rows:
            continue
        done.setdefault(config, set())
        if result["outcome"] not in ("complete", "partial") or any(r.get("error") for r in result["runs"]):
            continue
        clock = path.parent / "seconds"
        made = candidate(rows[name], config, result, float(clock.read_text()) if clock.exists() else 0.0, args.folded)
        dest = args.out / config / f"{name}.json"
        dest.parent.mkdir(parents=True, exist_ok=True)
        dest.write_text(json.dumps(made, indent=2) + "\n")
        done[config].add(name)
        print(name, config, len(made["findings"]), flush=True)
    for config, names in done.items():
        for name in rows.keys() - names:
            dest = args.out / config / f"{name}.json"
            dest.parent.mkdir(parents=True, exist_ok=True)
            dest.write_text(json.dumps(candidate(rows[name], config, {"comments": [], "runs": []}, 0.0, args.folded), indent=2) + "\n")
            print(name, config, "no finished review: handed in with no findings", flush=True)

if __name__ == "__main__":
    main()
