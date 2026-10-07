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
	listed, err := s.Review(review, "/skills")
	if err != nil || !strings.Contains(listed, "- `security-review`, `/skills/security-review/SKILL.md`: ") || strings.Contains(listed, "name: security-review") || !strings.Contains(listed, "do not use evidence `repro_test`") {
		t.Fatalf("listed %v:\n%s", err, listed)
	}
	review.TestOne = "go test -run {test}"
	onDemand, err := s.Review(review, "")
	if err != nil || strings.Contains(onDemand, "## Skills") || !strings.Contains(onDemand, "`go test -run {test}`") {
		t.Fatalf("on demand %v:\n%s", err, onDemand)
	}
	written := t.TempDir()
	if err := prompts.WriteSkills(written); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(filepath.Join(written, "security-review", "SKILL.md")); err != nil || !strings.Contains(string(body), "name: security-review") {
		t.Fatalf("written skill %v", err)
	}
	if _, err := s.Review(workflow.ReviewTask{Expert: workflow.Expert{Name: "nobody"}}, ""); err == nil {
		t.Fatal("an expert without a prompt was composed")
	}
}
