package harness

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Pleasurecruise/eyeful/internal/checkout"
	"github.com/Pleasurecruise/eyeful/internal/github"
	"github.com/Pleasurecruise/eyeful/internal/review"
	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/project"
	"github.com/Pleasurecruise/eyeful/workflow/pulls"
)

func TestExecute(t *testing.T) {
	src := sample(t)
	host := &fakeHost{actions: github.Actions{Passed: true}}
	agents := &script{}
	dir := t.TempDir()
	runner := Runner{
		Log: slog.New(slog.DiscardHandler), DataDir: dir, Experts: []string{"correctness"},
		Host: host, Clone: checkout.Clone, Agents: agents,
	}
	rev := review.Review{ID: "r_harness00000001", Source: src}
	if err := runner.Execute(t.Context(), rev); err != nil {
		t.Fatal(err)
	}
	if host.tokens != 2 || host.comments != 1 || agents.reviews != 1 || agents.testOne != "go test ./... -run "+project.Placeholder {
		t.Fatalf("tokens %d comments %d reviews %d test %q", host.tokens, host.comments, agents.reviews, agents.testOne)
	}
	if !strings.Contains(host.body, "the sandbox is not built") {
		t.Fatalf("report %s", host.body)
	}
	if _, err := os.Stat(filepath.Join(dir, rev.ID, "checkout")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("checkout remains")
	}
	if _, err := os.Stat(filepath.Join(dir, rev.ID, "published")); err != nil {
		t.Fatal(err)
	}
	if err := runner.Execute(t.Context(), rev); err != nil || host.tokens != 2 || host.comments != 1 {
		t.Fatalf("again tokens %d comments %d err %v", host.tokens, host.comments, err)
	}
}

func TestExecuteCIFailed(t *testing.T) {
	host := &fakeHost{actions: github.Actions{Passed: false, Log: "job failed"}}
	agents := &script{}
	dir := t.TempDir()
	runner := Runner{
		Log: slog.New(slog.DiscardHandler), DataDir: dir, Experts: []string{"correctness"},
		Host: host, Clone: checkout.Clone, Agents: agents,
	}
	rev := review.Review{ID: "r_cifailed0000001", Source: sample(t)}
	if err := runner.Execute(t.Context(), rev); err != nil {
		t.Fatal(err)
	}
	marker, err := os.ReadFile(filepath.Join(dir, rev.ID, "published"))
	if err != nil {
		t.Fatal(err)
	}
	if string(marker) != "ci_failed\n" || agents.reviews != 0 || agents.diagnoses != 1 || !strings.Contains(host.body, "boom") {
		t.Fatalf("marker %q reviews %d diagnoses %d report %s", marker, agents.reviews, agents.diagnoses, host.body)
	}
}

func TestExecuteCancelled(t *testing.T) {
	host := &fakeHost{actions: github.Actions{Passed: true}}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	runner := Runner{Log: slog.New(slog.DiscardHandler), DataDir: t.TempDir(), Host: host, Clone: checkout.Clone, Agents: &script{}}
	err := runner.Execute(ctx, review.Review{ID: "r_cancelled000001", Source: review.Source{Repo: "acme/eyeful", Base: "main", Head: "feature"}})
	if !errors.Is(err, context.Canceled) || host.tokens != 0 {
		t.Fatalf("%v tokens %d", err, host.tokens)
	}
}

func TestNotes(t *testing.T) {
	notes := notesFrom([]workflow.Comment{
		{
			Label: workflow.LabelIssue, Decorations: []string{"blocking", "correctness"},
			Subject: "A session is still valid at exactly expires_at.", Discussion: "is_expired compares with >.",
			Path: "app/session.py", Line: 11,
			Fix: []workflow.Edit{{Path: "app/session.py", Old: "now > t", New: "now >= t"}},
		},
		{Path: "", Line: 4, Subject: "skipped"},
		{Path: "a.go", Line: 0, Subject: "skipped"},
	})
	if len(notes) != 1 || notes[0].Path != "app/session.py" || notes[0].Line != 11 || !strings.Contains(notes[0].Body, "issue (blocking, correctness):") || !strings.Contains(notes[0].Body, "```suggestion\nnow >= t\n```") {
		t.Fatalf("%+v", notes)
	}
}

type fakeHost struct {
	actions  github.Actions
	tokens   int
	comments int
	body     string
}

func (h *fakeHost) Token(context.Context, int64) (string, error) {
	h.tokens++
	return "ghs_test", nil
}

func (h *fakeHost) Wait(context.Context, string, review.Source) (github.Actions, error) {
	return h.actions, nil
}

func (h *fakeHost) Description(context.Context, string, review.Source) (string, error) {
	return "Expire sessions on the boundary", nil
}

func (h *fakeHost) Publish(_ context.Context, _ string, _ review.Source, markdown string, notes []github.Note) error {
	h.comments++
	h.body = markdown
	if len(notes) > 0 {
		h.comments += len(notes)
	}
	return nil
}

type script struct {
	reviews   int
	diagnoses int
	testOne   string
}

func (s *script) Diagnose(context.Context, workflow.DiagnoseTask) (workflow.Diagnosis, workflow.Usage, error) {
	s.diagnoses++
	return workflow.Diagnosis{Failures: []workflow.Failure{{Workflow: "ci", Job: "test", Step: "run", Cause: "boom", Fix: "fix it"}}}, workflow.Usage{}, nil
}

func (s *script) Plan(context.Context, workflow.PlanTask) (pulls.Analysis, workflow.Usage, error) {
	return pulls.Analysis{}, workflow.Usage{}, nil
}

func (s *script) Review(_ context.Context, task workflow.ReviewTask) (workflow.Report, workflow.Usage, error) {
	s.reviews++
	s.testOne = task.TestOne
	return workflow.Report{Findings: []workflow.Claim{}}, workflow.Usage{}, nil
}

func (s *script) Fix(context.Context, workflow.FixTask) ([]workflow.Edit, workflow.Usage, error) {
	return []workflow.Edit{}, workflow.Usage{}, nil
}

func (s *script) Retest(context.Context, workflow.RetestTask) (workflow.Repro, workflow.Usage, error) {
	return workflow.Repro{}, workflow.Usage{}, nil
}

func (s *script) Judge(context.Context, workflow.JudgeTask) ([]workflow.Judgement, workflow.Usage, error) {
	return []workflow.Judgement{}, workflow.Usage{}, nil
}

func sample(t *testing.T) review.Source {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=eyeful", "GIT_AUTHOR_EMAIL=eyeful@localhost",
			"GIT_COMMITTER_NAME=eyeful", "GIT_COMMITTER_EMAIL=eyeful@localhost",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/base\n\ngo 1.22\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", "go.mod")
	run("commit", "-m", "base")
	base := run("rev-parse", "HEAD")
	run("checkout", "-b", "feature")
	if err := os.Remove(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "changed.txt"), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-m", "head")
	head := run("rev-parse", "HEAD")
	run("update-ref", "refs/pull/7/head", head)
	return review.Source{
		Repo: "acme/eyeful", Base: "main", Head: "feature", Number: 7,
		BaseSHA: base, HeadSHA: head, CloneURL: (&url.URL{Scheme: "file", Path: dir}).String(),
		InstallationID: 9,
	}
}
