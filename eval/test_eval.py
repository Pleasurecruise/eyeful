from __future__ import annotations

import unittest

from analyze import Score, bootstrap, detected, difference, mcnemar
from model import Finding, Review
from reviewbench import candidate, key, sample
from run import stopped
from subjects import PATCHED, labels

PATCH = """diff --git a/app/session.py b/app/session.py
--- a/app/session.py
+++ b/app/session.py
@@ -9,7 +9,8 @@ def is_expired(session, now):
-    return now > session.expires_at
+    return now >= session.expires_at
diff --git a/app/store.py b/app/store.py
--- a/app/store.py
+++ b/app/store.py
@@ -1 +1 @@
-x = 1
+x = 2
"""


def review(subject: str, rep: int, config: str = "panel") -> Review:
    finding = Finding("0/correctness/0", "a.py", 1, "error", "t", "b", "correctness", "test", "", "", "", "", False, True)
    return Review(subject, "eyeful", config, rep, "none", "complete", (finding,), 1000, 10, 1.0)


class Subjects(unittest.TestCase):
    def test_patched(self) -> None:
        self.assertEqual(PATCHED.findall(PATCH), ["app/session.py", "app/store.py"])

    def test_labels(self) -> None:
        self.assertEqual(labels(["test_a (auth.tests.Login)", "odd name"], True), ["auth.tests.Login.test_a"])
        self.assertEqual(labels(["tests/test_a.py::test_b"], False), ["tests/test_a.py::test_b"])
        self.assertEqual(labels(["t.py::test_a[x y]", "t.py::test_a[x", "t.py::test_b"], False), ["t.py::test_a[x y]", "t.py::test_b"])


class Gates(unittest.TestCase):
    def test_stopped(self) -> None:
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
    def test_mcnemar(self) -> None:
        self.assertEqual(mcnemar(0, 0), 1.0)
        self.assertAlmostEqual(mcnemar(5, 0), 0.0625)
        self.assertAlmostEqual(mcnemar(0, 5), 0.0625)
        self.assertEqual(mcnemar(3, 3), 1.0)

    def test_bootstrap(self) -> None:
        estimate = bootstrap({str(i): (float(i % 2), 1.0) for i in range(40)})
        self.assertAlmostEqual(estimate.value, 0.5)
        self.assertLess(estimate.low, 0.5)
        self.assertGreater(estimate.high, 0.5)
        self.assertEqual(str(bootstrap({})), "n/a")

    def test_difference(self) -> None:
        arm = {str(i): (float(i % 3 == 0), 1.0) for i in range(30)}
        estimate = difference(arm, arm)
        self.assertEqual((estimate.value, estimate.low, estimate.high), (0.0, 0.0, 0.0))

    def test_detected(self) -> None:
        reviews = [review("s", rep) for rep in (1, 2, 3)]
        scores = {r.key: Score({"a": "bug", "b": "bug"}, frozenset({"a"} if r.rep < 3 else {"b"}), frozenset()) for r in reviews}
        self.assertEqual(detected(reviews, scores, 2), {("s", "a"): True, ("s", "b"): False})
        self.assertEqual(detected(reviews, scores, 1), {("s", "a"): True, ("s", "b"): True})


class ReviewBench(unittest.TestCase):
    def test_candidate(self) -> None:
        row = {"repo": "https://github.com/owner/repo", "pr_number": 12, "base": "b" * 40, "head": "a1b2c3d4" + "0" * 32}
        result = {
            "comments": [
                {"path": "src/pool.ts", "line": 42, "kind": "review", "subject": "conns is read without the mutex.", "discussion": "Two requests can lose a write.", "scenario": ""},
                {"path": "src/pool.ts", "line": 7, "kind": "open", "subject": "Name the constant.", "discussion": ""},
            ],
            "runs": [{"usage": {"tokens": 900}}, {"usage": {"tokens": 100}}],
        }
        self.assertEqual(key(row), "owner_repo_12-a1b2c3d4")
        shown = candidate(row, "eyeful-panel-1-read_only", result, 1.5)
        self.assertEqual(shown["pr"], {"repo": row["repo"], "pr_number": 12, "base": row["base"], "head": row["head"]})
        self.assertEqual(shown["findings"], [{"producer": "eyeful-panel-1-read_only", "file": "src/pool.ts", "start_line": 42, "end_line": 42, "message": "conns is read without the mutex.\n\nTwo requests can lose a write."}])
        self.assertEqual(shown["usage"], {"time_in_ms": 1500, "total_tokens": 1000})

    def test_sample(self) -> None:
        rows = {f"r_{i}-0": {"language": "Go" if i < 6 else "Rust" if i < 9 else "C"} for i in range(10)}
        part = sample(rows, {"r_0-0"}, 4)
        self.assertEqual(len(part), 4)
        self.assertNotIn("r_0-0", part)
        self.assertEqual(sorted(r["language"] for r in part.values()), ["C", "Go", "Go", "Rust"])
        self.assertEqual(part, sample(dict(reversed(list(rows.items()))), {"r_0-0"}, 4))


if __name__ == "__main__":
    unittest.main()
