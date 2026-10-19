from __future__ import annotations

import json
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Literal

Dataset = Literal["swebench", "crb", "reviewbench"]
Tool = Literal["eyeful", "revmux"]
Condition = Literal["execution", "read_only", "none", "revmux"]

SEED = 20616304
ALL_GATES = "fix_verified"


@dataclass(frozen=True)
class Subject:
    id: str
    dataset: Dataset
    kind: str
    repo: str
    base: str
    head: str
    image: str

    @staticmethod
    def load(path: Path) -> Subject:
        return Subject(**json.loads(path.read_text()))

    def save(self, path: Path) -> None:
        path.write_text(json.dumps(asdict(self), indent=2) + "\n")


@dataclass(frozen=True)
class Finding:
    id: str
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
    reported: bool

    @property
    def text(self) -> str:
        return f"{self.title}\n\n{self.body}"


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

    @property
    def run(self) -> str:
        return f"{self.subject}/{self.tool}/{self.config}/{self.rep}"

    @property
    def name(self) -> str:
        return f"{self.tool}-{self.config}-{self.rep}-{self.condition}"


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
