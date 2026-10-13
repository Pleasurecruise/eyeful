"""Builds the injected-defect set's specs: screens young repositories, picks real commits and puts one defect into each."""

from __future__ import annotations

import argparse
import hashlib
import json
import subprocess
from dataclasses import asdict, dataclass
from pathlib import Path

from subjects import git, regions, sh

KINDS = ("boundary error", "missing check", "wrong condition", "resource leak", "injection flaw")
PLATFORM = "linux/arm64"
TEST = "pytest -rA -p no:cacheprovider"
EXTRAS = "test,tests,testing,dev"
MIN_TESTS = 10
MIN_ADDED, MAX_ADDED = 5, 150
RECENT = 80
DIRECT = [a for name in ("HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy") for a in ("--build-arg", f"{name}=")]
BUILD_SECONDS, SUITE_SECONDS, MODEL_SECONDS = 900, 300, 300
DOCKERFILE = f"""FROM python:3.12-slim
COPY . /testbed
WORKDIR /testbed
RUN pip install --no-cache-dir pytest && (pip install --no-cache-dir -e ".[{EXTRAS}]" || pip install --no-cache-dir -e .)
"""
ASK = """You prepare a subject for a study of code review. Below are a real commit and the file it changed, as the file is after the commit.

Put exactly one defect of this type into the lines the commit added or changed: {kind}.

The defect must be the kind of slip an author makes: small, plausible in this code, and wrong in behaviour for some input a caller can give. The file must still import, and the project's existing tests should still pass. Change nothing else, and add no comment or name that points at the defect.

Reply with one JSON object and nothing else:
{{"fits": true, "old": "<text to replace, copied exactly from the file, occurring once>", "new": "<the replacement>", "summary": "<one sentence: the input that goes wrong and what happens>"}}
If no defect of this type fits these lines, reply {{"fits": false}}.

## Commit

{diff}

## File {path}

{content}
"""


@dataclass(frozen=True)
class Spec:
    id: str
    repository: str
    commit: str
    image: str
    platform: str
    test: str
    suite: str
    defect: str
    kind: str
    summary: str


def quiet(args: list[str], cwd: Path | None, seconds: int) -> subprocess.CompletedProcess[str] | None:
    try:
        return subprocess.run(args, cwd=cwd, capture_output=True, text=True, timeout=seconds)
    except subprocess.TimeoutExpired:
        return None


def suite(image: str, tree: Path) -> tuple[bool, int]:
    done = quiet(
        ["docker", "run", "--rm", "--network", "none", "--platform", PLATFORM, "-v", f"{tree}:/change:ro", image, "sh", "-c",
         "tar -C /change --exclude=.git -cf - . | tar -C /testbed -xf - && cd /testbed && pytest -q -p no:cacheprovider"],
        None, SUITE_SECONDS,
    )
    if done is None:
        return False, 0
    tail = done.stdout.strip().splitlines()[-1:] or [""]
    passed = next((int(w) for w, unit in zip(tail[0].split(), tail[0].split()[1:]) if unit.startswith("passed") and w.isdigit()), 0)
    return done.returncode == 0, passed


def screen(name: str, work: Path) -> tuple[Path, str] | None:
    repo = work / name.replace("/", "__")
    if not repo.exists():
        if quiet(["git", "clone", "--quiet", "--depth", "300", f"https://github.com/{name}.git", str(repo)], None, 300) is None:
            return None
    if not (repo / "pyproject.toml").exists() or not any(repo.glob("test*/**/test_*.py")):
        return None
    image = f"eyeful-injected:{hashlib.sha256(name.encode()).hexdigest()[:12]}"
    (repo / ".dockerignore").write_text(".git\n")
    try:
        done = subprocess.run(
            ["docker", "build", "--quiet", "--platform", PLATFORM, *DIRECT, "-t", image, "-f", "-", "."],
            cwd=repo, input=DOCKERFILE, capture_output=True, text=True, timeout=BUILD_SECONDS,
        )
    except subprocess.TimeoutExpired:
        return None
    finally:
        (repo / ".dockerignore").unlink()
    if done.returncode != 0:
        return None
    ok, passed = suite(image, repo)
    if not ok or passed < MIN_TESTS:
        sh(["docker", "rmi", "--force", image])
        return None
    return repo, image


def commits(repo: Path) -> list[tuple[str, str]]:
    picked: list[tuple[str, str]] = []
    for sha in git(repo, "log", "--no-merges", f"--max-count={RECENT}", "--format=%H").split():
        stat = [row.split("\t") for row in git(repo, "show", "--numstat", "--format=", sha).strip().splitlines()]
        source = [r for r in stat if len(r) == 3 and r[2].endswith(".py") and "test" not in r[2].lower()]
        if len(stat) == 1 and len(source) == 1 and source[0][0].isdigit() and MIN_ADDED <= int(source[0][0]) <= MAX_ADDED:
            picked.append((sha, source[0][2]))
    return picked


def defect(repo: Path, sha: str, path: str, kind: str, model: str) -> tuple[str, str] | None:
    diff = git(repo, "show", "--format=%B", sha)
    content = git(repo, "show", f"{sha}:{path}")
    try:
        done = subprocess.run(
            ["claude", "-p", "--model", model, "--output-format", "json", "--no-session-persistence", "--tools", ""],
            input=ASK.format(kind=kind, diff=diff, path=path, content=content), capture_output=True, text=True, timeout=MODEL_SECONDS,
        )
        text: str = json.loads(done.stdout)["result"]
        answer = json.loads(text[text.index("{") : text.rindex("}") + 1])
    except (subprocess.TimeoutExpired, ValueError, KeyError):
        return None
    if not answer.get("fits") or content.count(answer.get("old", "")) != 1 or not answer.get("new"):
        return None
    git(repo, "checkout", "--quiet", "--force", "--detach", sha)
    (repo / path).write_text(content.replace(answer["old"], answer["new"]))
    patch = git(repo, "diff", "--", path)
    changed = {n for r in regions(git(repo, "show", "--format=", sha), "+") if r.path == path for n in range(r.start, r.end + 1)}
    inside = any(n in changed for r in regions(patch, "+") for n in range(r.start, r.end + 1))
    return (patch, str(answer.get("summary", ""))) if patch and inside else None


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repositories", type=Path, required=True, help="one owner/name per line")
    parser.add_argument("--work", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True, help="the specs, as JSON for subjects.py")
    parser.add_argument("--per-repository", type=int, default=3)
    parser.add_argument("--total", type=int, default=60)
    parser.add_argument("--model", default="sonnet")
    args = parser.parse_args()
    args.work.mkdir(parents=True, exist_ok=True)
    specs: list[Spec] = [Spec(**s) for s in json.loads(args.out.read_text())] if args.out.exists() else []
    seen = {s.repository for s in specs}
    for name in args.repositories.read_text().split():
        if len(specs) >= args.total:
            break
        if str(args.work / name.replace("/", "__")) in seen:
            continue
        screened = screen(name, args.work)
        if screened is None:
            print(name, "skipped: no passing pytest suite in a plain Python image", flush=True)
            continue
        repo, image = screened
        taken = 0
        for sha, path in commits(repo):
            if taken == args.per_repository or len(specs) >= args.total:
                break
            kind = KINDS[len(specs) % len(KINDS)]
            made = defect(repo, sha, path, kind, args.model)
            if made is None or not suite(image, repo)[0]:
                git(repo, "checkout", "--quiet", "--force", "--detach", sha)
                continue
            patch, summary = made
            identity = f"{name.replace('/', '__')}-{sha[:8]}"
            saved = args.work / f"{identity}.patch"
            saved.write_text(patch)
            specs.append(Spec(identity, str(repo), sha, image, PLATFORM, TEST, ".", str(saved), kind, summary))
            taken += 1
            args.out.write_text(json.dumps([asdict(s) for s in specs], indent=2) + "\n")
            print(identity, kind, "|", summary, flush=True)
        git(repo, "checkout", "--quiet", "--force", "--detach", "origin/HEAD")
        if taken == 0:
            print(name, "skipped: no commit took a defect", flush=True)


if __name__ == "__main__":
    main()
