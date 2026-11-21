package prompts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/prompts"
	"github.com/Pleasurecruise/eyeful/workflow/pulls"
)

func TestCompose(t *testing.T) {
	s, err := prompts.Load()
	if err != nil {
		t.Fatal(err)
	}
	files := []pulls.FileChange{{Path: "a.go", Status: "modified", Additions: 1, Sha: "1", Hunks: []pulls.DiffHunk{{Header: "@@ -1 +1 @@", OldStart: 1, OldLines: 1, NewStart: 1, NewLines: 1, Patch: "+secret-diff"}}}}
	task := workflow.PlanTask{Diff: pulls.DiffsPayload{Ref: pulls.SourceRef{Kind: "local"}, Title: "t", Files: files}, Rejected: []string{"Missing paths: b.go"}}
	plan, schema, err := s.Plan(task, "/run/change.diff")
	if err != nil || !strings.Contains(string(schema), `"overallSummary"`) || !strings.Contains(plan, "Group by intent") || !strings.Contains(plan, "/run/change.diff") || !strings.Contains(plan, "+secret-diff") || !strings.HasSuffix(plan, "rejected: Missing paths: b.go") {
		t.Fatalf("plan %v:\n%s", err, plan)
	}

	review := workflow.ReviewTask{Expert: workflow.Expert{Name: "security"}, Skills: []string{"security-review"}}
	listed, err := s.Review(review, "/skills", "/run/history.md")
	skill, _ := s.Skill("security-review")
	if err != nil || !strings.Contains(listed, "### Skill `security-review`\n\nDirectory: `/skills/security-review`\n\n"+skill.Body) || skill.Body == "" || strings.Contains(listed, "name: security-review") || !strings.Contains(listed, "do not use evidence `test`") {
		t.Fatalf("listed %v:\n%s", err, listed)
	}
	if !strings.Contains(listed, "## History") || !strings.Contains(listed, "`/run/history.md`") {
		t.Fatalf("an expert is not told where the history is:\n%s", listed)
	}
	review.Files = []workflow.File{{Path: "a.go", Diff: "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -7 +7 @@\n-old\n+new\n"}}
	review.Draft, review.Rejected = []workflow.Claim{{Path: "a.go", Line: 9}}, []string{"draft 0: line 9 is not a numbered line"}
	repair, err := s.Review(review, "/skills", "")
	if err != nil || strings.Contains(repair, skill.Body) || !strings.Contains(repair, "The skills you may cite: "+strings.Join(review.Skills, ", ")) || !strings.Contains(repair, `\t-old\n7\t+new`) || !strings.Contains(repair, "Do not review the change again") || !strings.Contains(repair, "draft 0: line 9") || review.Files[0].Diff[0] != 'd' || !strings.HasSuffix(review.Files[0].Diff, "+new\n") || strings.Contains(review.Files[0].Diff, "\t") {
		t.Fatalf("repair %v:\n%s", err, repair)
	}
	review.Draft, review.Rejected = nil, nil
	review.TestOne = "go test -run {test}"
	onDemand, err := s.Review(review, "", "")
	if err != nil || strings.Contains(onDemand, "## Skills") || strings.Contains(onDemand, "## History") || strings.Contains(onDemand, "do not use evidence `test`") {
		t.Fatalf("on demand %v:\n%s", err, onDemand)
	}
	retest := workflow.RetestTask{Finding: workflow.Finding{Claim: workflow.Claim{Path: "app/session.py"}}}
	offered, err := s.Retest(retest, "/skills")
	if err != nil || !strings.Contains(offered, "### Skill `python-testing-patterns`\n\nDirectory: `/skills/python-testing-patterns`") || strings.Contains(offered, "golang-testing") {
		t.Fatalf("retest %v:\n%s", err, offered)
	}
	retest.Finding.Path = "README.md"
	if bare, err := s.Retest(retest, "/skills"); err != nil || strings.Contains(bare, "## Skills") {
		t.Fatalf("a finding in a file no testing skill covers %v:\n%s", err, bare)
	}
	written := t.TempDir()
	if err := prompts.WriteSkills(written); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(filepath.Join(written, "security-review", "SKILL.md")); err != nil || !strings.Contains(string(body), "name: security-review") {
		t.Fatalf("written skill %v", err)
	}
	if _, err := s.Review(workflow.ReviewTask{Expert: workflow.Expert{Name: "nobody"}}, "", ""); err == nil {
		t.Fatal("an expert without a prompt was composed")
	}
}

func TestRoles(t *testing.T) {
	s, err := prompts.Load()
	if err != nil {
		t.Fatal(err)
	}
	diagnose, err := s.Diagnose(workflow.DiagnoseTask{Log: "FAIL: TestLogin"})
	if err != nil || !strings.Contains(diagnose, "## Input") || !strings.Contains(diagnose, `"log": "FAIL: TestLogin"`) || strings.Contains(diagnose, "## History") {
		t.Fatalf("diagnose %v:\n%s", err, diagnose)
	}
	f := workflow.Finding{ID: "0/correctness/0", Path: "app/session.py", Line: 3, Subject: "valid at expires_at"}
	fix, err := s.Fix(workflow.FixTask{Finding: f, Repro: workflow.Repro{Run: "t.py::test_boundary"}}, "/run/history.md")
	if err != nil || !strings.Contains(fix, "`/run/history.md`") || !strings.Contains(fix, `"run": "t.py::test_boundary"`) || strings.Contains(fix, "## Skills") {
		t.Fatalf("fix %v:\n%s", err, fix)
	}
	judge, err := s.Judge(workflow.JudgeTask{Findings: []workflow.Finding{f}, Description: "Sessions expire at expires_at."}, "")
	if err != nil || !strings.Contains(judge, `"id": "0/correctness/0"`) || !strings.Contains(judge, "Sessions expire at expires_at.") || strings.Contains(judge, "## History") || strings.Contains(judge, "## Skills") {
		t.Fatalf("judge %v:\n%s", err, judge)
	}
}
