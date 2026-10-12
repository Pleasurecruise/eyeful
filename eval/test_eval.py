from __future__ import annotations

import json
import unittest

from analyze import bootstrap, difference, mcnemar
from label import distinct, kappa
from match import tally
from model import Finding, Region, Review, Subject
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
    return Finding(path, line, "high", "t", "b", "correctness", "repro_test", "", status, "", "", False)


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


if __name__ == "__main__":
    unittest.main()
