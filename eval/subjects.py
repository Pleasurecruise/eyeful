"""Builds review subjects: one change that holds one known defect, in a repository whose tests run in a container."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import shlex
import subprocess
from dataclasses import dataclass
from pathlib import Path

from model import Region, Subject

PULL = re.compile(r"https://github\.com/([^/]+)/([^/]+)/pull/(\d+)")
HUNK = re.compile(r"^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@")
IDENTITY = ["-c", "user.name=eyeful", "-c", "user.email=eyeful@localhost", "-c", "commit.gpgsign=false"]
PLATFORMS = {"arm64": "linux/arm64", "x86_64": "linux/amd64"}
SUITE = 300
HISTORY = 200
ROUNDS = 4
ENTER = "export PATH=/opt/miniconda3/envs/testbed/bin:$PATH CONDA_DEFAULT_ENV=testbed CONDA_PREFIX=/opt/miniconda3/envs/testbed"


@dataclass(frozen=True)
class Runner:
    command: str
    label: bool
    enter: str = ENTER


RUNNERS: dict[str, Runner] = {
    "django/django": Runner("./tests/runtests.py --verbosity 2 --settings=test_sqlite --parallel 1", True),
    "psf/requests": Runner("pytest -rA -p no:cacheprovider", False),
    "pallets/flask": Runner("pytest -rA -p no:cacheprovider", False),
    "pytest-dev/pytest": Runner("pytest -rA -p no:cacheprovider", False),
    "pylint-dev/pylint": Runner("pytest -rA -p no:cacheprovider", False),
    "pydata/xarray": Runner("pytest -rA -p no:cacheprovider", False),
    "mwaskom/seaborn": Runner("pytest --no-header -rA -p no:cacheprovider", False),
}


def sh(args: list[str], cwd: Path | None = None, stdin: str | None = None) -> str:
    done = subprocess.run(args, cwd=cwd, input=stdin, capture_output=True, text=True)
    if done.returncode != 0:
        raise RuntimeError(f"{' '.join(args[:6])}: {done.stderr.strip() or done.stdout.strip()}")
    return done.stdout


def git(repo: Path, *args: str, stdin: str | None = None) -> str:
    return sh(["git", *IDENTITY, *args], cwd=repo, stdin=stdin)


def removed(patch: str) -> tuple[Region, ...]:
    regions: list[Region] = []
    path, line, start, end = "", 0, 0, 0
    for row in patch.splitlines():
        if row.startswith("--- "):
            path = row[4:].removeprefix("a/")
        elif m := HUNK.match(row):
            line, start, end = int(m.group(1)), 0, 0
        elif row.startswith("+++ ") or row.startswith("diff ") or row.startswith("index "):
            continue
        elif row.startswith("-"):
            start, end = start or line, line
            line += 1
        elif row.startswith("+"):
            start, end = start or line, end or line
        elif path and line:
            if start:
                regions.append(Region(path, start, end))
                start = end = 0
            line += 1
    if start:
        regions.append(Region(path, start, end))
    return tuple(dict.fromkeys(regions))


def added(patch: str) -> tuple[Region, ...]:
    regions: list[Region] = []
    path, line, start, end = "", 0, 0, 0
    for row in patch.splitlines():
        if row.startswith("+++ "):
            path = row[4:].removeprefix("b/")
        elif m := HUNK.match(row):
            if start:
                regions.append(Region(path, start, end))
            line, start, end = int(m.group(3)), 0, 0
        elif row.startswith("--- ") or row.startswith("diff ") or row.startswith("index "):
            continue
        elif row.startswith("+"):
            start, end = start or line, line
            line += 1
        elif row.startswith("-"):
            start, end = start or line, end or line
        elif path and line:
            if start:
                regions.append(Region(path, start, end))
                start = end = 0
            line += 1
    if start:
        regions.append(Region(path, start, end))
    return tuple(dict.fromkeys(regions))


def labels(tests: list[str], label: bool) -> list[str]:
    if not label:
        return tests
    named = [re.fullmatch(r"(\w+) \(([\w.]+)\)", t) for t in tests]
    return [f"{m.group(2)}.{m.group(1)}" for m in named if m]


def passing(repo: Path, candidates: list[str]) -> list[str]:
    for _ in range(ROUNDS):
        if not candidates:
            return []
        done = subprocess.run(["sh", ".eyeful/run.sh", *candidates], cwd=repo, capture_output=True, text=True)
        if done.returncode == 0:
            return candidates
        passed = set(done.stdout.splitlines())
        candidates = [t for t in candidates if f"PASSED {t}" in passed]
    return []


def commands(repo: Path, image: str, platform: str, runner: Runner, suite: list[str]) -> None:
    hidden = repo / ".eyeful"
    hidden.mkdir(exist_ok=True)
    (hidden / "inside.sh").write_text(
        "set -e\ntar -C /change --exclude=.git --exclude=.eyeful -cf - . | tar -C /testbed -xf -\n"
        f"cd /testbed\n{runner.enter}\nexec {runner.command} \"$@\"\n"
    )
    (hidden / "run.sh").write_text(
        f'exec docker run --rm --network none --platform {platform} -v "$PWD":/change:ro {image} '
        'bash /change/.eyeful/inside.sh "$@"\n'
    )
    config = "test_one: sh .eyeful/run.sh {test}\n"
    if suite:
        config += f"test: sh .eyeful/run.sh {' '.join(shlex.quote(t) for t in suite)}\n"
    (hidden / "config.yml").write_text(config)


def swebench(row: dict[str, str], out: Path, repos: Path, arch: str) -> Subject:
    name: str = row["instance_id"]
    runner = RUNNERS[row["repo"]]
    home = out / name
    repo = repos / hashlib.sha256(name.encode()).hexdigest()[:12]
    if (home / "subject.json").exists():
        return Subject.load(home / "subject.json")
    home.mkdir(parents=True, exist_ok=True)
    repos.mkdir(parents=True, exist_ok=True)
    source = f"swebench/sweb.eval.{arch}.{name.replace('__', '_1776_')}:latest"
    image = f"eyeful-eval:{hashlib.sha256(name.encode()).hexdigest()[:12]}"
    platform = PLATFORMS[arch]
    sh(["docker", "pull", "--quiet", "--platform", platform, source])
    sh(["docker", "tag", source, image])
    box = sh(["docker", "create", "--platform", platform, image]).strip()
    try:
        sh(["docker", "cp", f"{box}:/testbed", str(repo)])
    finally:
        sh(["docker", "rm", box])
    git(repo, "checkout", "--quiet", "--detach", row["base_commit"])
    git(repo, "clean", "--force", "-d", "--quiet")
    commands(repo, image, platform, runner, [])
    commands(repo, image, platform, runner, passing(repo, labels(json.loads(row["PASS_TO_PASS"])[:SUITE], runner.label)))
    git(repo, "apply", "--whitespace=nowarn", stdin=row["patch"])
    git(repo, "add", "--update")
    git(repo, "add", "--force", ".eyeful")
    git(repo, "commit", "--quiet", "--no-verify", "-m", "Prepare the release branch")
    base = git(repo, "rev-parse", "HEAD").strip()
    git(repo, "apply", "--reverse", "--whitespace=nowarn", stdin=row["patch"])
    regions = removed(row["patch"])
    touched = ", ".join(dict.fromkeys(r.path for r in regions))
    git(repo, "commit", "--quiet", "--no-verify", "--all", "-m", f"Rework {touched}")
    head = git(repo, "rev-parse", "HEAD").strip()
    subject = Subject(name, "swebench", row.get("difficulty", ""), str(repo), base, head, image, regions)
    subject.save(home / "subject.json")
    return subject


def injected(spec: dict[str, str], out: Path, repos: Path) -> Subject:
    name: str = spec["id"]
    home = out / name
    repo = repos / hashlib.sha256(name.encode()).hexdigest()[:12]
    if (home / "subject.json").exists():
        return Subject.load(home / "subject.json")
    home.mkdir(parents=True, exist_ok=True)
    repos.mkdir(parents=True, exist_ok=True)
    sh(["git", "clone", "--quiet", spec["repository"], str(repo)])
    git(repo, "checkout", "--quiet", "--detach", f"{spec['commit']}^")
    commands(repo, spec["image"], spec["platform"], Runner(spec["test"], False, "true"), spec["suite"].split())
    git(repo, "add", "--force", ".eyeful")
    git(repo, "commit", "--quiet", "--no-verify", "-m", "Add the project commands")
    base = git(repo, "rev-parse", "HEAD").strip()
    git(repo, "cherry-pick", "--no-commit", spec["commit"])
    patch = Path(spec["defect"]).read_text()
    git(repo, "apply", "--whitespace=nowarn", stdin=patch)
    message = git(repo, "log", "-1", "--format=%B", spec["commit"])
    git(repo, "commit", "--quiet", "--no-verify", "--all", "-m", message)
    head = git(repo, "rev-parse", "HEAD").strip()
    subject = Subject(name, "injected", spec["kind"], str(repo), base, head, spec["image"], added(patch))
    subject.save(home / "subject.json")
    return subject


def benchmarked(name: str, owner: str, project: str, number: str, out: Path, repos: Path) -> Subject:
    home = out / name
    repo = repos / hashlib.sha256(name.encode()).hexdigest()[:12]
    if (home / "subject.json").exists():
        return Subject.load(home / "subject.json")
    pull = json.loads(sh(["gh", "api", f"repos/{owner}/{project}/pulls/{number}"]))
    home.mkdir(parents=True, exist_ok=True)
    repo.mkdir(parents=True, exist_ok=True)
    sh(["git", "init", "--quiet"], cwd=repo)
    remote = f"https://github.com/{owner}/{project}.git"
    git(repo, "fetch", "--quiet", f"--depth={pull['commits'] + HISTORY}", remote, pull["base"]["sha"], pull["head"]["sha"])
    base = git(repo, "merge-base", pull["base"]["sha"], pull["head"]["sha"]).strip()
    git(repo, "checkout", "--quiet", "--detach", pull["head"]["sha"])
    subject = Subject(name, "crb", "", str(repo), base, pull["head"]["sha"], "", ())
    subject.save(home / "subject.json")
    return subject


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("dataset", choices=["swebench", "injected", "crb"])
    parser.add_argument("--data", type=Path, required=True, help="SWE-bench Verified rows, the injected set's specs, or one of Code Review Bench's golden comment files, as JSON")
    parser.add_argument("--out", type=Path, required=True, help="where each subject's description goes; it names the defect, so keep it away from --repos")
    parser.add_argument("--repos", type=Path, required=True, help="where the repositories the reviewers work in go, under names that say nothing about the subject; not a temporary or cache directory, which the system and cleaners empty")
    parser.add_argument("--ids", default="", help="comma-separated ids; empty builds every row the runners cover")
    parser.add_argument("--arch", default="arm64", choices=["arm64", "x86_64"])
    args = parser.parse_args()
    wanted = {i for i in args.ids.split(",") if i}
    for row in json.loads(args.data.read_text()):
        pull = PULL.fullmatch(row.get("url", ""))
        name = row.get("instance_id") or row.get("id") or "{}__{}-{}".format(*pull.groups())
        if wanted and name not in wanted:
            continue
        if args.dataset == "swebench" and row["repo"] not in RUNNERS:
            continue
        try:
            if args.dataset == "swebench":
                subject = swebench(row, args.out, args.repos, args.arch)
            elif args.dataset == "injected":
                subject = injected(row, args.out, args.repos)
            else:
                subject = benchmarked(name, *pull.groups(), args.out, args.repos)
        except RuntimeError as err:
            print(name, "skipped:", err, flush=True)
            continue
        print(subject.id, subject.base[:12], subject.head[:12], [(r.path, r.start, r.end) for r in subject.defect], flush=True)


if __name__ == "__main__":
    main()
