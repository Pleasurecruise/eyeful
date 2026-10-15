from __future__ import annotations

import json
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Literal

Dataset = Literal["swebench", "crb", "reviewbench"]
Tool = Literal["eyeful", "revmux"]
Condition = Literal["execution", "read_only", "none", "revmux"]
Verdict = Literal["actionable", "noise"]

NEAR = 3
MERGE = 2
SEED = 20616304
ALL_GATES = "fix_verified"


@dataclass(frozen=True)
class Region:
    path: str
    start: int
    end: int


@dataclass(frozen=True)
class Subject:
    id: str
    dataset: Dataset
    kind: str
    repo: str
    base: str
    head: str
    image: str
    defect: tuple[Region, ...]

    @staticmethod
    def load(path: Path) -> Subject:
        raw = json.loads(path.read_text())
        raw["defect"] = tuple(Region(**r) for r in raw["defect"])
        return Subject(**raw)

    def save(self, path: Path) -> None:
        path.write_text(json.dumps(asdict(self), indent=2) + "\n")

    def hit(self, path: str, line: int) -> bool:
        return any(r.path == path and r.start - NEAR <= line <= r.end + NEAR for r in self.defect)


@dataclass(frozen=True)
class Finding:
    path: str
    line: int
    severity: str
    title: str
    body: str
    source: str
    evidence: str
    cwe: str
    status: str
    gate: str
    oracle: str
    blocking: bool


@dataclass(frozen=True)
class Review:
    subject: str
    tool: Tool
    config: str
    rep: int
    condition: Condition
    outcome: str
    findings: tuple[Finding, ...]
    tokens: int
    micro_usd: int
    seconds: float

    @property
    def key(self) -> str:
        return f"{self.subject}/{self.tool}/{self.config}/{self.rep}/{self.condition}"


def load_reviews(path: Path) -> list[Review]:
    reviews: list[Review] = []
    for line in path.read_text().splitlines():
        raw = json.loads(line)
        raw["findings"] = tuple(Finding(**f) for f in raw["findings"])
        reviews.append(Review(**raw))
    return reviews


def append_review(path: Path, review: Review) -> None:
    with path.open("a") as out:
        out.write(json.dumps(asdict(review)) + "\n")


def load_subjects(root: Path) -> dict[str, Subject]:
    return {s.id: s for s in (Subject.load(p) for p in sorted(root.glob("*/subject.json")))}
