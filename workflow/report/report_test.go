package report_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/report"
)

func result() workflow.Result {
	test := workflow.Edit{Path: "app/test_session.py", New: "def test_boundary(): ...\n"}
	return workflow.Result{
		Outcome:    workflow.OutcomeComplete,
		Regression: &workflow.TestRun{Failed: []string{"tests/test_login.py::test_remember_me"}},
		Triage:     workflow.Triage{Excluded: []workflow.Entry{{Path: "uv.lock", Class: workflow.ClassLock}}},
		Plan: workflow.Plan{Summary: "Sessions expire at the expiry second.", Groups: []workflow.Group{
			{Category: "ui", Summary: "Show when a session ends", Files: []string{"app/login.py"}},
			{Category: "security", Summary: "Expired sessions were accepted", Core: true, Files: []string{"app/session.py"}},
		}},
		Findings: []workflow.Finding{
			{ID: "0/correctness/0", Expert: "correctness", Skill: "code-review", Path: "app/session.py", Line: 11, Evidence: workflow.EvidenceTest},
			{ID: "0/security/0", Expert: "security", Skill: "other", Category: "naming", Path: "app/login.py", Line: 3, Evidence: workflow.EvidenceInspection},
			{ID: "0/skeptic/0", Expert: "skeptic", Skill: "code-review", Path: "app/session.py", Line: 40, Evidence: workflow.EvidenceAnalysis},
		},
		Comments: []workflow.Comment{
			{Finding: "0/correctness/0", Label: workflow.LabelIssue, Decorations: []string{"blocking", "correctness"}, Subject: "Valid at expires_at.", Scenario: "is_expired(100, now=100) is False", Severity: workflow.SeverityError, Kind: workflow.KindFail, Path: "app/session.py", Line: 11, Related: []workflow.Location{{Path: "app/login.py", Line: 7}}, Experts: []string{"correctness", "security"}, Repro: &workflow.Repro{Test: test, Run: "tests/test_session.py::test_boundary", Oracle: workflow.OracleSpecified, Quote: "a session expires at expires_at"}, Fix: []workflow.Edit{{Path: "app/session.py", Old: "now > t", New: "now >= t"}}},
			{Finding: "0/security/0", Label: workflow.LabelNitpick, Decorations: []string{"non-blocking", "security"}, Subject: "Name the token.", Discussion: "`t` says nothing;\ncall it `session`.", Severity: workflow.SeverityNote, Kind: workflow.KindReview, Path: "app/login.py", Line: 3, Experts: []string{"security"}},
			{Finding: "0/skeptic/0", Label: workflow.LabelQuestion, Decorations: []string{"non-blocking", "correctness"}, Subject: "Refresh skips the expiry check.", Severity: workflow.SeverityWarning, Kind: workflow.KindOpen, Path: "app/session.py", Line: 40, Experts: []string{"skeptic"}, Objection: "refresh is only reached after is_expired."},
		},
		Uncertainty: workflow.Uncertainty{Absent: []workflow.Absence{{Expert: "security", Group: 1, Reason: "timeout"}}},
	}
}

func TestMarkdown(t *testing.T) {
	md, err := report.Markdown(result())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"3 findings: 1 fail, 1 review, 1 open.",
		"## Tests the change breaks\n\nThese pass on the base commit and fail on the change: `tests/test_login.py::test_remember_me`.",
		"| 🔴 error | `app/session.py:11` | Valid at expires_at. |",
		"<details><summary>Nitpicks (1)</summary>\n\n- `app/login.py:3` Name the token.",
		"## 🔴 error · issue · correctness · fail",
		"Failure scenario: is_expired(100, now=100) is False",
		"<details><summary>Why this was flagged</summary>",
		"failed on the change in every run, not on the base commit, and passed with the suggested fix",
		`Expected behaviour from: specified`,
		"Also at `app/login.py:7`.",
		"-now > t\n+now >= t",
		"<details><summary>Disputed (1)</summary>",
		"- `app/session.py:40` Refresh skips the expiry check. A judge agent read the code and disputes this finding: refresh is only reached after is_expired. It is kept for you to decide.",
		"`uv.lock` (lock)", "Expert `security` is absent: timeout",
		"## What this change does\n\nSessions expire at the expiry second.",
		"| 1 (core) | security | Expired sessions were accepted | `app/session.py` |\n| 2 | ui |",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in\n%s", want, md)
		}
	}
	if strings.Contains(md, "No blocking issues") {
		t.Error("a review with a fail finding says it has no blocking issues")
	}
	if strings.Contains(md, "| `app/login.py:3` |") || strings.Contains(md, "## 🟡 note · nitpick") {
		t.Error("a nitpick is listed outside the folded section")
	}
	if strings.Contains(md, "| `app/session.py:40` |") || strings.Contains(md, "· open") {
		t.Error("a comment the judge disputes is listed outside the folded section")
	}
}

func TestSummary(t *testing.T) {
	r := result()
	r.Comments[1].Subject = "Name the <T> token."
	blocks, err := report.Summary(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 3 {
		t.Fatalf("%d blocks, want the summary and one per comment not posted:\n%q", len(blocks), blocks)
	}
	for _, b := range blocks {
		if strings.Count(b, "<details>") != strings.Count(b, "</details>") || strings.Count(b, "```")%2 != 0 {
			t.Errorf("a block does not stand alone:\n%s", b)
		}
	}
	md := strings.Join(blocks, "\n\n")
	for _, want := range []string{
		"### ⚠️ 1 issue found",
		"Outcome `complete`, 3 findings: 1 reproduced, 1 to review, 1 disputed.",
		"The change breaks `tests/test_login.py::test_remember_me`.",
		"| 🔴 error | `app/session.py:11` | Valid at expires_at. |",
		"#### Not posted (2)",
		"<details><summary>🟡 note · nitpick · <code>app/login.py:3</code> Name the &lt;T&gt; token.</summary>\n",
		"`t` says nothing;\ncall it `session`.",
		"<details><summary>🟠 warning · question · <code>app/session.py:40</code> Refresh skips the expiry check.</summary>\n",
		"A judge agent read the code and disputes this finding: refresh is only reached after is_expired.",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in\n%s", want, md)
		}
	}
	for _, unwanted := range []string{"| `app/login.py:3` |", "| `app/session.py:40` |", "What this change does", "None was reproduced"} {
		if strings.Contains(md, unwanted) {
			t.Errorf("%q in\n%s", unwanted, md)
		}
	}

	clean := result()
	clean.Comments = clean.Comments[1:]
	clean.Regression = &workflow.TestRun{}
	blocks, err = report.Summary(clean)
	if err != nil {
		t.Fatal(err)
	}
	md = strings.Join(blocks, "\n\n")
	for _, want := range []string{"### ✅ No issues found", "None was reproduced by running a test.", "The project's tests pass on the base commit and fail on the change."} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in\n%s", want, md)
		}
	}
	if strings.Contains(md, "| Severity |") {
		t.Errorf("a table without issues in\n%s", md)
	}
}

func TestResolved(t *testing.T) {
	r := result()
	old := workflow.Finding{ID: "previous/0/correctness/7"}
	old.Path, old.Line, old.Subject = "app/old.py", 7, "Off by one."
	r.Findings = append(r.Findings, old)
	r.Verifications = append(r.Verifications, workflow.Verification{Finding: "previous/0/correctness/7", Status: workflow.StatusResolved, Repro: &workflow.Repro{Run: "tests/test_old.py::test_edge"}})
	md, err := report.Markdown(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "## Resolved since the last review") || !strings.Contains(md, "- `app/old.py:7` Off by one. (`tests/test_old.py::test_edge`)") {
		t.Errorf("resolved findings in\n%s", md)
	}
	if md, err := report.Markdown(result()); err != nil || strings.Contains(md, "## Resolved") {
		t.Errorf("a first review lists resolved findings, or fails: %v", err)
	}
}

func TestScopes(t *testing.T) {
	r := result()
	r.Findings[1].Group = 1
	r.Scopes = []workflow.Scope{{Name: "app", Files: []string{"app/session.py"}, Lines: 1500}, {Name: "web", Files: []string{"app/login.py"}, Lines: 900}}
	r.Plan = workflow.Plan{Groups: []workflow.Group{{Scope: 0}, {Scope: 1}}}
	md, err := report.Markdown(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"reviewed in 2 scopes", "| `app` | 1 | 1500 |", "| `web` | 1 | 900 |"} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in\n%s", want, md)
		}
	}
	if md, _ := report.Markdown(result()); strings.Contains(md, "## Scopes") {
		t.Error("a single scope is listed")
	}
}

func TestJSON(t *testing.T) {
	data, err := report.JSON(result())
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Counts   struct{ Fail, Review, Open int } `json:"counts"`
		Findings []struct {
			File            string `json:"file"`
			Line            int    `json:"line"`
			Severity        string `json:"severity"`
			Kind            string `json:"kind"`
			Summary         string `json:"summary"`
			FailureScenario string `json:"failure_scenario"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	f := out.Findings[0]
	if out.Counts.Fail != 1 || out.Counts.Review != 1 || out.Counts.Open != 1 || f.File != "app/session.py" || f.Line != 11 || f.Severity != "error" || f.Kind != "fail" || f.Summary == "" || f.FailureScenario == "" {
		t.Fatalf("%s", data)
	}
}

func TestSARIF(t *testing.T) {
	data, err := report.SARIF(result())
	if err != nil {
		t.Fatal(err)
	}
	var log struct {
		Version string `json:"version"`
		Runs    []struct {
			Results []struct {
				RuleID     string `json:"ruleId"`
				Level      string `json:"level"`
				Kind       string `json:"kind"`
				Baseline   string `json:"baselineState"`
				Properties struct {
					Oracle         string   `json:"oracle"`
					Objection      string   `json:"objection"`
					CorroboratedBy []string `json:"corroboratedBy"`
				} `json:"properties"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(data, &log); err != nil {
		t.Fatal(err)
	}
	rs := log.Runs[0].Results
	if log.Version != "2.1.0" || len(rs) != 3 {
		t.Fatalf("%s", data)
	}
	if r := rs[0]; r.RuleID != "eyeful/correctness/code-review" || r.Level != "error" || r.Kind != "fail" || r.Baseline != "new" || r.Properties.Oracle != "specified" || r.Properties.CorroboratedBy[0] != "security" {
		t.Errorf("blocking result %+v", r)
	}
	if r := rs[1]; r.RuleID != "eyeful/security/naming" || r.Level != "none" || r.Kind != "review" || r.Baseline != "" {
		t.Errorf("nitpick result %+v", r)
	}

	if r := rs[2]; r.Kind != "open" || r.Level != "none" || r.Properties.Objection == "" {
		t.Errorf("disputed result %+v", r)
	}

	bad := result()
	bad.Comments[0].Finding = "missing"
	if _, err := report.SARIF(bad); err == nil {
		t.Error("a comment on an unknown finding was accepted")
	}
}
