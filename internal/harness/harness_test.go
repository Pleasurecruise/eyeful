package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Pleasurecruise/eyeful/internal/checkout"
	"github.com/Pleasurecruise/eyeful/internal/github"
	"github.com/Pleasurecruise/eyeful/internal/review"
	"github.com/Pleasurecruise/eyeful/internal/sandbox"
	"github.com/Pleasurecruise/eyeful/local/archive"
	"github.com/Pleasurecruise/eyeful/local/classify"
	"github.com/Pleasurecruise/eyeful/local/settings"
	"github.com/Pleasurecruise/eyeful/workflow"
)

var replies = map[string]string{
	"`submit_plan`":      `{"overallSummary":"Adds changed.txt.","groups":[{"key":"app","label":"App","category":"core","filePaths":["changed.txt"]}]}`,
	"`submit_report`":    `{"findings":[{"skill":"other","category":"naming","path":"changed.txt","line":1,"severity":"warning","subject":"Say what changed.","discussion":"d","evidence":"inspection"}]}`,
	"`submit_judgement`": `{"judgements":[{"finding":"0/correctness/0","valid":true,"reason":"r"}]}`,
	"`submit_diagnosis`": `{"failures":[{"workflow":"ci","job":"test","step":"run","cause":"boom","fix":"fix it"}]}`,
}

func fakeClaude() error {
	if slices.Contains(os.Args, "--version") {
		_, err := fmt.Println("9.9.9 (fake)")
		return err
	}
	if os.Getenv("FAKE_DOWN") != "" {
		return errors.New("the agent is down")
	}
	prompt, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	for submit, reply := range replies {
		if strings.Contains(string(prompt), "call "+submit) {
			text, err := json.Marshal("```json\n" + reply + "\n```")
			if err != nil {
				return err
			}
			_, err = fmt.Printf(`{"type":"result","subtype":"success","is_error":false,"result":%s,"usage":{"input_tokens":10,"output_tokens":5},"total_cost_usd":0.001,"modelUsage":{"fake-model":{}}}`+"\n", text)
			return err
		}
	}
	return errors.New("no reply for this role")
}

func TestMain(m *testing.M) {
	if os.Getenv("FAKE_CLI") == "1" {
		if err := fakeClaude(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func sandboxFor(t *testing.T) *sandbox.Client {
	t.Helper()
	srv := httptest.NewServer(sandbox.NewServer(slog.New(slog.DiscardHandler), filepath.Join(t.TempDir(), "sandbox"), time.Minute))
	t.Cleanup(srv.Close)
	return sandbox.NewClient(srv.URL)
}

func connect(t *testing.T) {
	t.Helper()
	home, bin := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err := os.Symlink(os.Args[0], filepath.Join(bin, "claude")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_CLI", "1")
	path, err := settings.Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := settings.Save(path, settings.Settings{Agent: "claude"}); err != nil {
		t.Fatal(err)
	}
}

func TestExecute(t *testing.T) {
	connect(t)
	src := sample(t)
	host := &fakeHost{actions: github.Actions{Passed: true}}
	runner := Runner{
		Log: slog.New(slog.DiscardHandler), Experts: []string{"correctness"}, Classifier: classify.New(slog.New(slog.DiscardHandler), "", nil), Sandbox: sandboxFor(t), Dir: t.TempDir(),
		Runs: archive.New(t.TempDir()), Host: host, Clone: checkout.Clone,
	}
	status, err := runner.Execute(t.Context(), src)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateDone || status.Outcome != workflow.OutcomeComplete || host.tokens != 1 || host.comments != 1 || host.reactions != 1 {
		t.Fatalf("status %+v tokens %d comments %d", status, host.tokens, host.comments)
	}
	if !strings.Contains(host.body, "<details><summary>Not posted (1)</summary>") || !strings.Contains(host.body, "- suggestion · `changed.txt:1`") || strings.Contains(host.body, "What this change does") {
		t.Fatalf("report %s", host.body)
	}
	again, err := runner.Execute(t.Context(), src)
	if err != nil || again.Outcome != status.Outcome || host.tokens != 1 || host.comments != 1 {
		t.Fatalf("again %+v tokens %d comments %d err %v", again, host.tokens, host.comments, err)
	}
}

func TestExecuteResumes(t *testing.T) {
	connect(t)
	src := sample(t)
	runs := archive.New(t.TempDir())
	first := &fakeHost{actions: github.Actions{Passed: true}, fail: errors.New("GitHub is down")}
	runner := Runner{
		Log: slog.New(slog.DiscardHandler), Experts: []string{"correctness"}, Classifier: classify.New(slog.New(slog.DiscardHandler), "", nil), Sandbox: sandboxFor(t), Dir: t.TempDir(),
		Runs: runs, Host: first, Clone: checkout.Clone,
	}
	if _, err := runner.Execute(t.Context(), src); err == nil {
		t.Fatal("publishing failed but the review succeeded")
	}
	t.Setenv("FAKE_DOWN", "1")
	second := &fakeHost{actions: github.Actions{Passed: true}}
	runner.Host = second
	runner.Dir = t.TempDir()
	status, err := runner.Execute(t.Context(), src)
	if err != nil || status.Outcome != workflow.OutcomeComplete || second.comments != 1 {
		t.Fatalf("a new container did not resume the saved stages: %+v %v", status, err)
	}
}

func TestExecuteCIFailed(t *testing.T) {
	connect(t)
	host := &fakeHost{actions: github.Actions{Passed: false, Log: "job failed"}}
	runner := Runner{
		Log: slog.New(slog.DiscardHandler), Experts: []string{"correctness"}, Classifier: classify.New(slog.New(slog.DiscardHandler), "", nil), Sandbox: sandboxFor(t), Dir: t.TempDir(),
		Runs: archive.New(t.TempDir()), Host: host, Clone: checkout.Clone,
	}
	status, err := runner.Execute(t.Context(), sample(t))
	if err != nil {
		t.Fatal(err)
	}
	if status.Outcome != workflow.OutcomeCIFailed || !strings.Contains(host.body, "boom") {
		t.Fatalf("status %+v report %s", status, host.body)
	}
}

func TestExecuteCancelled(t *testing.T) {
	host := &fakeHost{actions: github.Actions{Passed: true}}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	runner := Runner{Log: slog.New(slog.DiscardHandler), Dir: t.TempDir(), Runs: archive.New(t.TempDir()), Host: host, Clone: checkout.Clone}
	_, err := runner.Execute(ctx, sample(t))
	if !errors.Is(err, context.Canceled) || host.comments != 0 {
		t.Fatalf("%v comments %d", err, host.comments)
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
		{Label: workflow.LabelSuggestion, Path: "app/session.py", Line: 20, Subject: "skipped"},
		{Label: workflow.LabelIssue, Kind: workflow.KindOpen, Path: "app/session.py", Line: 30, Subject: "skipped"},
		{Path: "", Line: 4, Subject: "skipped"},
		{Path: "a.go", Line: 0, Subject: "skipped"},
	})
	if len(notes) != 1 || notes[0].Path != "app/session.py" || notes[0].Line != 11 || !strings.Contains(notes[0].Body, "issue (blocking, correctness):") || !strings.Contains(notes[0].Body, "```suggestion\nnow >= t\n```") {
		t.Fatalf("%+v", notes)
	}
}

type fakeHost struct {
	actions   github.Actions
	fail      error
	tokens    int
	reactions int
	comments  int
	body      string
}

func (h *fakeHost) Token(context.Context, int64) (string, error) {
	h.tokens++
	return "ghs_test", nil
}

func (h *fakeHost) Wait(context.Context, string, review.Source) (github.Actions, error) {
	return h.actions, nil
}

func (h *fakeHost) React(context.Context, string, review.Source) error {
	h.reactions++
	return nil
}

func (h *fakeHost) Description(context.Context, string, review.Source) (string, error) {
	return "Add changed.txt", nil
}

func (h *fakeHost) Publish(_ context.Context, _ review.Source, markdown string, notes []github.Note) error {
	if h.fail != nil {
		return h.fail
	}
	h.comments += 1 + len(notes)
	h.body = markdown
	return nil
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
