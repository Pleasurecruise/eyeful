package workflow_test

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Pleasurecruise/eyeful/workflow"
)

func TestAdmit(t *testing.T) {
	w, err := workflow.New(slog.New(slog.DiscardHandler), roster)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Admit(request()); err != nil {
		t.Fatalf("a valid request: %v", err)
	}
	for name, edit := range map[string]func(*workflow.Request){
		"an unknown verification": func(r *workflow.Request) { r.Verification = "maybe" },
		"no budget":               func(r *workflow.Request) { r.Budget = workflow.Budget{} },
		"a negative budget":       func(r *workflow.Request) { r.Budget.Tokens = -1 },
		"an unknown expert":       func(r *workflow.Request) { r.Experts = []string{"oracle"} },
		"a repeated expert":       func(r *workflow.Request) { r.Experts = []string{"security", "security"} },
		"an empty path":           func(r *workflow.Request) { r.Change.Files[0].Path = "" },
		"a repeated path":         func(r *workflow.Request) { r.Change.Files[1].Path = r.Change.Files[0].Path },
	} {
		req := request()
		edit(&req)
		if err := w.Admit(req); !errors.Is(err, workflow.ErrInvalidRequest) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

type faulty struct {
	archive
	load, corrupt, save string
}

func (f *faulty) Load(ctx context.Context, key string) ([]byte, bool, error) {
	switch key {
	case f.load:
		return nil, false, errors.New("disk gone")
	case f.corrupt:
		return []byte("{"), true, nil
	}
	return f.archive.Load(ctx, key)
}

func (f *faulty) Save(ctx context.Context, key string, data []byte) error {
	if f.save != "" && strings.HasPrefix(key, f.save) {
		return errors.New("disk full")
	}
	return f.archive.Save(ctx, key, data)
}

func failing(t *testing.T, a *agents, ws *workspace, ar workflow.Archive) error {
	t.Helper()
	w, err := workflow.New(slog.New(slog.DiscardHandler), roster)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := w.Run(t.Context(), workflow.Session{Agents: a, Classifier: a, Workspace: ws, Archive: ar}, request())
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("the review kept running after a stage failed")
	}
	return nil
}

func TestArchive(t *testing.T) {
	for want, ar := range map[string]*faulty{
		"load ci: disk gone":      {load: "ci"},
		"decode ci":               {corrupt: "ci"},
		"save prepare: disk full": {save: "prepare"},
		"save plan/0: disk full":  {save: "plan/0"},
		"save match: disk full":   {save: "match"},
	} {
		err := failing(t, base(), &workspace{ci: workflow.Check{Passed: true}}, ar)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", want, err)
		}
	}
}

func TestCancel(t *testing.T) {
	t.Run("a failed plan stops the suite", func(t *testing.T) {
		ws := &workspace{ci: workflow.Check{Passed: true}, stall: func(t workflow.Target) bool { return len(t.Tests) == 0 }}
		err := failing(t, base(), ws, &faulty{save: "plan/0"})
		if err == nil || !strings.Contains(err.Error(), "save plan/0") {
			t.Fatalf("err %v", err)
		}
	})

	t.Run("a failed judge stops the reproductions", func(t *testing.T) {
		a := base()
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
			if t.Expert.Name != "correctness" {
				return answers(), nil
			}
			read := finding("app/login.py", 40, workflow.EvidenceInspection)
			return answers(finding("app/session.py", 10, workflow.EvidenceTest), read), nil
		}
		ws := &workspace{ci: workflow.Check{Passed: true}, stall: func(t workflow.Target) bool { return len(t.Tests) > 0 }}
		err := failing(t, a, ws, &faulty{save: "judge/"})
		if err == nil || !strings.Contains(err.Error(), "save judge/") {
			t.Fatalf("err %v", err)
		}
		if a.n(workflow.RoleFix) != 0 {
			t.Fatalf("a fix was written after the review failed")
		}
	})
}

func TestValidate(t *testing.T) {
	task := workflow.ReviewTask{
		Expert: workflow.Expert{Name: "correctness", Evidence: []workflow.EvidenceKind{workflow.EvidenceTest, workflow.EvidenceInspection}},
		Group:  workflow.Group{Files: []string{"app/session.py"}},
		Skills: []string{"code-review"},
		Files:  []workflow.File{{Path: "app/session.py", Diff: gitDiff("app/session.py", "bug")}},
	}
	good := finding("app/session.py", 10, workflow.EvidenceTest)
	cases := map[string]func(*workflow.Claim){
		"needs a category":                func(c *workflow.Claim) { c.Category = "" },
		`cites "golang-naming"`:           func(c *workflow.Claim) { c.Skill = "golang-naming" },
		"is not a file of this group":     func(c *workflow.Claim) { c.Path = "app/login.py" },
		"line 200 is not a numbered line": func(c *workflow.Claim) { c.Line = 200 },
		"related place app/session.py:10": func(c *workflow.Claim) { c.Related = []workflow.Location{{Path: "app/session.py", Line: 10}} },
		"needs a subject":                 func(c *workflow.Claim) { c.Subject = "" },
		`severity "low" is not`:           func(c *workflow.Claim) { c.Severity = "low" },
		`evidence "analysis" is not one`:  func(c *workflow.Claim) { c.Evidence = workflow.EvidenceAnalysis },
		"needs a scenario":                func(c *workflow.Claim) { c.Scenario = "" },
	}
	claims := make([]workflow.Claim, 0, 1+len(cases))
	claims = append(claims, good)
	wants := make([]string, 0, len(cases))
	for want, edit := range cases {
		c := good
		edit(&c)
		claims, wants = append(claims, c), append(wants, want)
	}
	reasons := workflow.Validate(task, workflow.Report{Findings: claims})
	if len(reasons[0]) != 0 {
		t.Fatalf("a valid finding was rejected: %v", reasons[0])
	}
	for i, want := range wants {
		if got := reasons[i+1]; !slices.ContainsFunc(got, func(r string) bool { return strings.Contains(r, want) }) {
			t.Errorf("%s: %v", want, got)
		}
	}
}
