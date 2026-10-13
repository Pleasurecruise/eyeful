from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from analyze import bootstrap, difference, mcnemar
from label import distinct, kappa
from match import tally
from model import Finding, Region, Review, Subject
from reviewbench import candidate, held, key, scored, standing
from run import stopped
from subjects import added, labels, removed

PATCH = """diff --git a/app/session.py b/app/session.py
--- a/app/session.py
+++ b/app/session.py
@@ -9,7 +9,8 @@ def is_expired(session, now):
     if session is None:
         return True
 
-    return now > session.expires_at
+    limit = session.expires_at
+    return now >= limit
 
 
 def renew(session):
@@ -30,6 +31,7 @@ def close(session):
     session.closed = True
 
 
+    session.flush()
 def reopen(session):
     session.closed = False
"""


def finding(path: str, line: int, status: str = "") -> Finding:
    return Finding(path, line, "error", "t", "b", "correctness", "test", "", status, "", "", False)


def review(subject: str, findings: tuple[Finding, ...]) -> Review:
    return Review(subject, "eyeful", "panel", 1, "none", "complete", findings, 1000, 10, 1.0)


class Regions(unittest.TestCase):
    def test_removed_lines_are_numbered_before_the_patch(self) -> None:
        self.assertEqual(removed(PATCH), (Region("app/session.py", 12, 12), Region("app/session.py", 33, 33)))

    def test_added_lines_are_numbered_after_the_patch(self) -> None:
        self.assertEqual(added(PATCH), (Region("app/session.py", 12, 13), Region("app/session.py", 34, 34)))

    def test_a_finding_near_the_defect_hits_it(self) -> None:
        subject = Subject("s", "swebench", "", "/r", "b", "h", "i", removed(PATCH))
        self.assertTrue(subject.hit("app/session.py", 15))
        self.assertFalse(subject.hit("app/session.py", 20))
        self.assertFalse(subject.hit("app/other.py", 12))

    def test_django_tests_become_labels(self) -> None:
        self.assertEqual(labels(["test_a (auth.tests.Login)", "odd name"], True), ["auth.tests.Login.test_a"])
        self.assertEqual(labels(["tests/test_a.py::test_b"], False), ["tests/test_a.py::test_b"])


class Gates(unittest.TestCase):
    def test_the_gate_is_read_from_what_the_runs_showed(self) -> None:
        ran = {"status": "unverified", "repro": {}}
        failed = ["FAILED"] * 5
        for verdict, gate in (
            ({"status": "confirmed"}, ""),
            (ran, "runs"),
            (ran | {"change": ["FAILED", "PASSED"]}, "not_flaky"),
            (ran | {"change": ["ERROR"] * 5}, "fails_as_claimed"),
            (ran | {"status": "not_reproduced", "change": ["PASSED"] * 5}, "fails_as_claimed"),
            (ran | {"status": "pre_existing", "change": failed, "base": "FAILED"}, "introduced"),
            (ran | {"status": "reproduced", "change": failed, "base": "ERROR", "fixed": "FAILED"}, "fail_to_pass"),
            (ran | {"status": "reproduced", "change": failed, "base": "PASSED", "fixed": "PASSED"}, "pass_to_pass"),
            (ran | {"status": "fix_verified", "change": failed, "base": "PASSED", "fixed": "PASSED"}, ""),
        ):
            self.assertEqual(stopped(verdict), gate, verdict)


class Statistics(unittest.TestCase):
    def test_mcnemar_is_exact_and_two_sided(self) -> None:
        self.assertEqual(mcnemar(0, 0), 1.0)
        self.assertAlmostEqual(mcnemar(5, 0), 0.0625)
        self.assertAlmostEqual(mcnemar(0, 5), 0.0625)
        self.assertEqual(mcnemar(3, 3), 1.0)

    def test_bootstrap_brackets_the_estimate(self) -> None:
        estimate = bootstrap({str(i): (float(i % 2), 1.0) for i in range(40)})
        self.assertAlmostEqual(estimate.value, 0.5)
        self.assertLess(estimate.low, 0.5)
        self.assertGreater(estimate.high, 0.5)
        self.assertEqual(str(bootstrap({})), "n/a")

    def test_a_paired_difference_of_equal_arms_is_zero(self) -> None:
        arm = {str(i): (float(i % 3 == 0), 1.0) for i in range(30)}
        estimate = difference(arm, arm)
        self.assertEqual((estimate.value, estimate.low, estimate.high), (0.0, 0.0, 0.0))

    def test_kappa(self) -> None:
        first = {"a": "actionable", "b": "noise", "c": "actionable", "d": "noise"}
        self.assertEqual(kappa(first, first), (1.0, 4))
        value, n = kappa(first, {"a": "actionable", "b": "actionable", "c": "noise", "d": "noise", "e": "noise"})
        self.assertEqual((value, n), (0.0, 4))


class Labelling(unittest.TestCase):
    def test_findings_within_two_lines_are_one_item(self) -> None:
        reviews = [
            review("s", (finding("a.py", 10), finding("a.py", 12), finding("a.py", 20))),
            review("s", (finding("a.py", 11), finding("b.py", 10))),
            review("t", (finding("a.py", 10),)),
        ]
        items = distinct(reviews)
        self.assertEqual(items[("s", "a.py", 10)], items[("s", "a.py", 12)])
        self.assertEqual(items[("s", "a.py", 10)], items[("s", "a.py", 11)])
        self.assertEqual(len({i.id for i in items.values()}), 4)


class Golden(unittest.TestCase):
    def test_findings_on_the_same_lines_are_one_place(self) -> None:
        comments = [{"comment": "expires a second late"}, {"comment": "leaks the session"}]
        found = review("s", (finding("a.py", 10), finding("a.py", 12), finding("a.py", 40), finding("b.py", 3)))
        kept = {json.dumps([c["comment"], "t\n\nb"]): {"match": c["comment"] == "expires a second late"} for c in comments}
        self.assertEqual(tally(found, comments, kept), (1, 4, 3, 0))
        kept = {key: {"match": False} for key in kept}
        self.assertEqual(tally(found, comments, kept), (0, 4, 3, 3))


class ReviewBench(unittest.TestCase):
    def test_a_review_becomes_the_benchmarks_candidate_file(self) -> None:
        row = {"repo": "https://github.com/owner/repo", "pr_number": 12, "base": "b" * 40, "head": "a1b2c3d4" + "0" * 32}
        result = {
            "comments": [
                {"path": "src/pool.ts", "line": 42, "label": "question", "subject": "conns is read without the mutex.", "discussion": "Two requests can lose a write.", "scenario": ""},
                {"path": "src/pool.ts", "line": 7, "label": "nitpick", "subject": "Name the constant."},
            ],
            "runs": [{"usage": {"tokens": 900}}, {"usage": {"tokens": 100}}],
        }
        self.assertEqual(key(row), "owner_repo_12-a1b2c3d4")
        shown = candidate(row, "panel", result, 1.5, False)
        self.assertEqual(shown["pr"], {"repo": row["repo"], "pr_number": 12, "base": row["base"], "head": row["head"]})
        self.assertEqual(shown["findings"], [{"producer": "eyeful-panel", "file": "src/pool.ts", "start_line": 42, "end_line": 42, "message": "conns is read without the mutex.\n\nTwo requests can lose a write."}])
        self.assertEqual(shown["usage"], {"time_in_ms": 1500, "total_tokens": 1000})
        self.assertEqual(len(candidate(row, "panel", result, 1.5, True)["findings"]), 2)

    def test_the_held_out_part_is_the_same_every_time_and_leaves_the_test_set_out(self) -> None:
        rows = {name: {} for name in ("a_b_1-00000000", "c_d_2-11111111", "e_f_3-22222222", "g_h_4-33333333")}
        part = held(rows, {"c_d_2-11111111"}, 2)
        self.assertEqual(len(part), 2)
        self.assertNotIn("c_d_2-11111111", part)
        self.assertEqual(list(part), list(held(dict(reversed(list(rows.items()))), {"c_d_2-11111111"}, 2)))

    def test_a_golden_finding_is_claimed_once_and_covered_by_every_match(self) -> None:
        golden = [{"tp_fp": "tp"}, {"tp_fp": "fp"}, {"tp_fp": "tp"}]
        overall, details = scored([{"file": "a"}, {"file": "a"}, {"file": "a"}], golden, {0: [0, 2], 1: [0], 2: [1]})
        self.assertEqual([d["matched_truth_label"] for d in details], ["tp", None, "fp"])
        self.assertEqual([d["matched_to_ground_truth"] for d in details], [True, False, True])
        self.assertEqual(overall, {
            "candidate_count": 3, "golden_tp_count": 2, "golden_covered_count": 2, "matched_count": 2, "matched_tp_count": 1,
            "grounded_recall": 1.0, "grounded_precision": 0.5,
        })
        self.assertEqual(scored([], golden, {})[0]["grounded_precision"], None)

    def test_a_pull_request_with_nothing_handed_in_still_counts_its_golden_findings(self) -> None:
        hit = {"matched_to_ground_truth": True, "matched_truth_label": "tp", "covered_golden_indices": [0]}
        miss = {"matched_to_ground_truth": False, "matched_truth_label": "", "covered_golden_indices": []}
        judged = {
            "completed": {name: {"metrics": {"overall": {"golden_tp_count": 2}}} for name in ("reviewed", "unreviewed")},
            "details": {"reviewed": {"candidate_findings": [miss, hit]}, "unreviewed": {"candidate_findings": []}},
        }
        with tempfile.TemporaryDirectory() as golden:
            for name in judged["completed"]:
                (Path(golden) / f"{name}.json").write_text(json.dumps({"findings": [{"tp_fp": "tp"}, {"tp_fp": "tp"}, {"tp_fp": "fp"}]}))
            whole = standing("run", "panel", judged, Path(golden), 0)
            first = standing("run", "panel", judged, Path(golden), 1)
        self.assertEqual(whole, {"run": "run", "config": "panel", "first": "all", "pulls": 2, "comments": 1.0, "grounded_recall": 0.25, "grounded_precision": 1.0})
        self.assertEqual((first["comments"], first["grounded_recall"], first["grounded_precision"]), (0.5, 0.0, ""))


if __name__ == "__main__":
    unittest.main()
