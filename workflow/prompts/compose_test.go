package prompts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/prompts"
)

func TestCompose(t *testing.T) {
	s, err := prompts.Load()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.Plan(workflow.PlanTask{Files: []workflow.File{{Path: "a.go", Diff: "+secret-diff\n"}}, Truncated: true})
	if err != nil || !strings.Contains(plan, "+secret-diff") || !strings.Contains(plan, "rest of the diff is left out") {
		t.Fatalf("plan %v:\n%s", err, plan)
	}
	if strings.Count(plan, "+secret-diff") != 1 {
		t.Fatal("the diff appears twice in the planner's prompt")
	}

	task := workflow.ReviewTask{Expert: workflow.Expert{Name: "security"}, Skills: []string{"security-review"}}
	listed, err := s.Review(task, "/skills")
	if err != nil || !strings.Contains(listed, "- `security-review`, `/skills/security-review/SKILL.md`: ") || strings.Contains(listed, "name: security-review") || !strings.Contains(listed, "do not use evidence `repro_test`") {
		t.Fatalf("listed %v:\n%s", err, listed)
	}
	task.TestOne = "go test -run {test}"
	onDemand, err := s.Review(task, "")
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
