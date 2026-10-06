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
		Outcome: workflow.OutcomeComplete,
		Level:   workflow.LevelStandard,
		Triage:  workflow.Triage{Excluded: []workflow.Entry{{Path: "uv.lock", Class: workflow.ClassLock}}},
		Findings: []workflow.Finding{
			{ID: "0/correctness/0", Expert: "correctness", Skill: "code-review", Path: "app/session.py", Line: 11, Evidence: workflow.EvidenceReproTest, Repro: &workflow.Repro{Test: test, Oracle: workflow.OracleSpec}},
			{ID: "0/security/0", Expert: "security", Skill: "other", Category: "naming", Path: "app/login.py", Line: 3, Evidence: workflow.EvidenceArgument},
		},
		Comments: []workflow.Comment{
			{Finding: "0/correctness/0", Label: workflow.LabelIssue, Decorations: []string{"blocking", "correctness"}, Short: "Session valid at expires_at", Subject: "Valid at expires_at.", Scenario: "is_expired(100, now=100) is False", Strength: workflow.StrengthStrong, Path: "app/session.py", Line: 11, Experts: []string{"correctness", "security"}, Test: &test, Fix: []workflow.Edit{{Path: "app/session.py", Old: "now > t", New: "now >= t"}}},
			{Finding: "0/security/0", Label: workflow.LabelNitpick, Decorations: []string{"non-blocking", "security"}, Short: "Name the token", Subject: "Name the token.", Path: "app/login.py", Line: 3, Experts: []string{"security"}},
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
		"2 findings: 1 important, 1 nit.",
		"| 🔴 Important | `app/session.py:11` | Session valid at expires_at |",
		"| 🟡 Nit | `app/login.py:3` | Name the token |",
		"## 🔴 Important · correctness · CONFIRMED",
		"Failure scenario: is_expired(100, now=100) is False",
		"<details><summary>Why this was flagged</summary>",
		"failed on the change and passed with the suggested fix",
		`Expected behaviour from: spec`,
		"-now > t\n+now >= t",
		"## 🟡 Nit · security · PLAUSIBLE",
		"`uv.lock` (lock)", "Expert `security` on group 1 is absent: timeout",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in\n%s", want, md)
		}
	}
	if strings.Contains(md, "No blocking issues") {
		t.Error("a review with an important finding says it has no blocking issues")
	}
	if strings.Index(md, "🔴 Important · ") > strings.Index(md, "🟡 Nit · ") {
		t.Error("a nit is listed before an important finding")
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
		Counts   struct{ Important, Nit int } `json:"counts"`
		Findings []struct {
			File            string `json:"file"`
			Line            int    `json:"line"`
			Severity        string `json:"severity"`
			Verdict         string `json:"verdict"`
			ShortSummary    string `json:"short_summary"`
			FailureScenario string `json:"failure_scenario"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	f := out.Findings[0]
	if out.Counts.Important != 1 || out.Counts.Nit != 1 || f.File != "app/session.py" || f.Line != 11 || f.Severity != "important" || f.Verdict != "CONFIRMED" || f.ShortSummary == "" || f.FailureScenario == "" {
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
				Properties struct {
					Oracle         string   `json:"oracle"`
					CorroboratedBy []string `json:"corroboratedBy"`
				} `json:"properties"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(data, &log); err != nil {
		t.Fatal(err)
	}
	rs := log.Runs[0].Results
	if log.Version != "2.1.0" || len(rs) != 2 {
		t.Fatalf("%s", data)
	}
	if r := rs[0]; r.RuleID != "eyeful/correctness/code-review" || r.Level != "error" || r.Kind != "fail" || r.Properties.Oracle != "spec" || r.Properties.CorroboratedBy[0] != "security" {
		t.Errorf("blocking result %+v", r)
	}
	if r := rs[1]; r.RuleID != "eyeful/security/naming" || r.Level != "note" || r.Kind != "review" {
		t.Errorf("nitpick result %+v", r)
	}

	bad := result()
	bad.Comments[0].Finding = "missing"
	if _, err := report.SARIF(bad); err == nil {
		t.Error("a comment on an unknown finding was accepted")
	}
}
