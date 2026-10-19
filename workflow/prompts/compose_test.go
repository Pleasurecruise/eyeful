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
