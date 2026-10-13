# /// script
# requires-python = ">=3.12"
# dependencies = ["typesafe-sdk==0.7.4"]
# ///
"""Matches eyeful's comments to ReviewBench's golden findings with Jev, as the benchmark's matcher does with a model: every comment against every golden finding of the same file. It gives the grounded scores only; comments that match nothing are not classified."""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path

from typesafe_sdk import Noul, TypeSafeClient

from reviewbench import scored

SAME = 0.5
BATCH = 150
QUESTION = "Two code review findings on the same file. Is `candidates.c{}` about the same underlying issue as `golden.g{}`, so that the minimal fix for one would also fix the other? Their wording, line ranges and framing may differ. Answer no when they are different issues at the same location."


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--candidates", type=Path, required=True, help="one configuration's candidate files, as reviewbench.py findings wrote them")
    parser.add_argument("--golden", type=Path, required=True, help="the benchmark's golden directory")
    parser.add_argument("--out", type=Path, required=True, help="the scores, in the shape of the benchmark judge's results.checkpoint.json; pull requests already there are not asked about again")
    args = parser.parse_args()
    kept = json.loads(args.out.read_text()) if args.out.exists() else {"fingerprint": "", "completed": {}, "details": {}}
    args.out.parent.mkdir(parents=True, exist_ok=True)
    with TypeSafeClient(api_key=os.environ["EYEFUL_TYPESAFE_API_KEY"]) as client:
        for path in sorted(args.candidates.glob("*.json")):
            name = path.stem
            if name in kept["completed"]:
                continue
            handed, golden = json.loads(path.read_text())["findings"], json.loads((args.golden / f"{name}.json").read_text())["findings"]
            same: dict[int, list[int]] = {}
            for file in sorted({f["file"] for f in handed}):
                ours, theirs = ([i for i, f in enumerate(side) if f["file"] == file] for side in (handed, golden))
                pairs = [(c, g) for c in ours for g in theirs]
                state = {"file": file} | {
                    side: {f"{side[0]}{i}": {"lines": [found[i]["start_line"], found[i]["end_line"]], "message": found[i]["message"]} for i in picked}
                    for side, found, picked in (("candidates", handed, ours), ("golden", golden, theirs))
                }
                for start in range(0, len(pairs), BATCH):
                    part = pairs[start : start + BATCH]
                    answer = client.system_one(state=state, questions={f"{c}_{g}": Noul(instructions=QUESTION.format(c, g)) for c, g in part})
                    kept["fingerprint"] = f"typesafe/{answer.model}"
                    for c, g in part:
                        if answer.nouls[f"{c}_{g}"].noul >= SAME:
                            same.setdefault(c, []).append(g)
            overall, details = scored(handed, golden, same)
            kept["completed"][name], kept["details"][name] = {"metrics": {"overall": overall}}, {"candidate_findings": details}
            args.out.write_text(json.dumps(kept, indent=2) + "\n")
            print(name, f"{overall['golden_covered_count']} of {overall['golden_tp_count']} golden findings, {overall['matched_count']} of {overall['candidate_count']} comments matched", flush=True)
    found, known = (sum(v["metrics"]["overall"][k] for v in kept["completed"].values()) for k in ("golden_covered_count", "golden_tp_count"))
    print(f"grounded recall {found / max(known, 1):.3f} ({found} of {known}) over {len(kept['completed'])} pull requests")


if __name__ == "__main__":
    main()
