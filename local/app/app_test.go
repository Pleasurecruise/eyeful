package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Pleasurecruise/eyeful/local/app"
	"github.com/Pleasurecruise/eyeful/local/classify"
	"github.com/Pleasurecruise/eyeful/local/settings"
	"github.com/Pleasurecruise/eyeful/local/subject"
	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/project"
	"github.com/Pleasurecruise/eyeful/workflow/report"
)

var replies = map[string]string{
	"`submit_plan`":      `{"overallSummary":"Writes two to app.txt.","groups":[{"key":"app","label":"App","category":"core","filePaths":["app.txt"]}]}`,
	"`submit_report`":    `{"findings":[{"skill":"other","category":"naming","path":"app.txt","line":1,"severity":"warning","subject":"Say what changed.","discussion":"d","evidence":"inspection"}]}`,
	"`submit_judgement`": `{"judgements":[{"finding":"0/correctness/0","valid":true,"reason":"r"}]}`,
	"`submit_commit`":    `{"message":"fix: write two\n\nThe value changed."}`,
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
	if path := os.Getenv("FAKE_EDIT"); path != "" {
		if err := os.WriteFile(path, []byte("edited during the review\n"), 0o644); err != nil {
			return err
		}
	}
	for _, m := range regexp.MustCompile("`([^`]+/SKILL.md)`").FindAllStringSubmatch(string(prompt), -1) {
		if _, err := os.Stat(m[1]); err != nil {
			return fmt.Errorf("a listed skill cannot be read: %w", err)
		}
	}
	if m := regexp.MustCompile(`The full patch is at (\S+);`).FindStringSubmatch(string(prompt)); m != nil {
		if _, err := os.Stat(m[1]); err != nil {
			return fmt.Errorf("the patch the planner is pointed at cannot be read: %w", err)
		}
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

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t", "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func setup(t *testing.T, connect bool) (string, string, app.Options) {
	t.Helper()
	home, root, outside := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	bin := t.TempDir()
	if err := os.Symlink(os.Args[0], filepath.Join(bin, "claude")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_CLI", "1")
	if connect {
		path, err := settings.Path()
		if err != nil {
			t.Fatal(err)
		}
		if err := settings.Save(path, settings.Settings{Agent: "claude"}); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "app.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-q", "-m", "init")
	if err := os.WriteFile(filepath.Join(root, "app.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, outside, app.Options{
		Dir: root, Experts: []string{"correctness"}, Verification: workflow.ModeReadOnly,
		Commands: project.Config{Setup: "touch " + filepath.Join(outside, "setup-ran")},
		Confirm: func(context.Context, string, []project.Command) (bool, error) {
			return false, errors.New("no terminal")
		},
		Classifier: classify.New(slog.New(slog.DiscardHandler), "", nil),
		Out:        &strings.Builder{}, Err: &strings.Builder{}, Log: slog.New(slog.DiscardHandler),
	}
}

func status(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", "status", "--porcelain", "--untracked-files=all")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func yes(context.Context, string, []project.Command) (bool, error) { return true, nil }

func execution(t *testing.T) (string, string, app.Options) {
	t.Helper()
	root, outside, o := setup(t, true)
	git(t, root, "commit", "-qam", "change")
	o.Verification, o.Range = workflow.ModeExecution, subject.Range{Head: "HEAD", Base: "HEAD~1"}
	return root, outside, o
}

func TestReview(t *testing.T) {
	t.Run("uncommitted changes run in a checkout of their snapshot", func(t *testing.T) {
		root, outside, o := setup(t, true)
		o.Commands = project.Config{Setup: "cp app.txt " + filepath.Join(outside, "seen.txt") + "; touch setup-ran"}
		before := status(t, root)
		tmp := t.TempDir()
		t.Setenv("TMPDIR", tmp)
		objects, err := exec.CommandContext(t.Context(), "git", "-C", root, "count-objects").Output()
		if err != nil {
			t.Fatal(err)
		}
		o.Verification, o.Confirm = workflow.ModeExecution, yes
		if _, err := app.Review(t.Context(), o); err != nil {
			t.Fatal(err)
		}
		after, err := exec.CommandContext(t.Context(), "git", "-C", root, "count-objects").Output()
		if left, _ := os.ReadDir(tmp); err != nil || len(left) != 0 || string(after) != string(objects) {
			t.Fatalf("the review left %v in the temporary directory, or objects in the repository", left)
		}
		if data, _ := os.ReadFile(filepath.Join(outside, "seen.txt")); string(data) != "two\n" {
			t.Fatalf("setup saw %q, not the uncommitted change", data)
		}
		if after := status(t, root); after != before {
			t.Fatalf("the working copy changed:\n%s\n--\n%s", before, after)
		}
		runs, _ := filepath.Glob(filepath.Join(root, ".eyeful", "runs", "*", "change.diff"))
		if len(runs) != 1 {
			t.Fatalf("run files %v", runs)
		}
		if data, _ := os.ReadFile(runs[0]); !strings.Contains(string(data), "+two") {
			t.Fatalf("recorded diff:\n%s", data)
		}
		if strings.Contains(o.Err.(*strings.Builder).String(), "changed while the review ran") {
			t.Fatal("an unchanged repository was reported as changed")
		}
	})

	t.Run("files .gitattributes marks generated are not reviewed", func(t *testing.T) {
		root, _, o := setup(t, true)
		if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("app.txt linguist-generated\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		o.Run = filepath.Join(t.TempDir(), "run")
		if _, err := app.Review(t.Context(), o); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(o.Run, "result.json"))
		var res workflow.Result
		if err != nil || json.Unmarshal(data, &res) != nil {
			t.Fatal(err)
		}
		if !slices.Contains(res.Triage.Excluded, workflow.Entry{Path: "app.txt", Class: workflow.ClassSkipped, Added: 1, Deleted: 1}) {
			t.Fatalf("triage %+v", res.Triage)
		}
	})

	t.Run("an edit during the review is reported", func(t *testing.T) {
		root, _, o := setup(t, true)
		t.Setenv("FAKE_EDIT", filepath.Join(root, "app.txt"))
		if _, err := app.Review(t.Context(), o); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(o.Err.(*strings.Builder).String(), "changed while the review ran") {
			t.Fatalf("stderr:\n%s", o.Err.(*strings.Builder).String())
		}
	})

	t.Run("started from the repository with a relative directory", func(t *testing.T) {
		root, _, o := setup(t, true)
		t.Chdir(root)
		o.Dir = "."
		if _, err := app.Review(t.Context(), o); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("read-only review leaves the working copy alone", func(t *testing.T) {
		root, outside, o := setup(t, true)
		before := status(t, root)
		if _, err := app.Review(t.Context(), o); err != nil {
			t.Fatal(err)
		}
		out := o.Out.(*strings.Builder).String()
		if !strings.Contains(out, "# eyeful review") || !strings.Contains(out, "| 🟠 warning | `app.txt:1` |") {
			t.Fatalf("output:\n%s", out)
		}
		if _, err := os.Stat(filepath.Join(outside, "setup-ran")); err == nil {
			t.Fatal("a project command ran in a read-only review")
		}
		if after := status(t, root); after != before {
			t.Fatalf("the working copy changed:\n%s\n--\n%s", before, after)
		}
		if _, err := os.Stat(filepath.Join(root, ".git", "eyeful")); err == nil {
			t.Fatal("eyeful wrote into .git")
		}
		if runs, _ := filepath.Glob(filepath.Join(root, ".eyeful", "runs", "*", "results.sarif")); len(runs) != 1 {
			t.Fatalf("run files %v", runs)
		}
		if left, _ := filepath.Glob(filepath.Join(root, ".eyeful", "runs", "*", "skills")); len(left) != 0 {
			t.Fatalf("the skills were left in %v", left)
		}
	})

	t.Run("a run directory is resumed, for its own request only", func(t *testing.T) {
		root, _, o := setup(t, true)
		o.Run = filepath.Join(t.TempDir(), "run")
		first, err := app.Review(t.Context(), o)
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("FAKE_DOWN", "1")
		again, err := app.Review(t.Context(), o)
		if err != nil || len(again.Findings) != len(first.Findings) || len(first.Findings) != 1 {
			t.Fatalf("resumed review called an agent again: %+v, %v", again, err)
		}
		if err := os.WriteFile(filepath.Join(root, "app.txt"), []byte("three\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Review(t.Context(), o); !errors.Is(err, workflow.ErrOtherRequest) {
			t.Fatalf("another change reused the run directory: %v", err)
		}
	})

	t.Run("the head must be checked out", func(t *testing.T) {
		root, _, o := setup(t, true)
		git(t, root, "stash", "-q")
		git(t, root, "checkout", "-q", "-b", "other")
		if err := os.WriteFile(filepath.Join(root, "other.txt"), []byte("o\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", ".")
		git(t, root, "commit", "-q", "-m", "other")
		git(t, root, "checkout", "-q", "main")
		o.Range = subject.Range{Head: "other"}
		if _, err := app.Review(t.Context(), o); !errors.Is(err, subject.ErrNotCheckedOut) {
			t.Fatalf("%v", err)
		}
	})

	t.Run("a given command replaces only the one eyeful finds", func(t *testing.T) {
		root, _, o := setup(t, true)
		if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", ".")
		git(t, root, "commit", "-qm", "change")
		o.Verification, o.Range, o.Detect = workflow.ModeExecution, subject.Range{Head: "HEAD", Base: "HEAD~1"}, true
		var asked []project.Command
		o.Confirm = func(_ context.Context, _ string, cmds []project.Command) (bool, error) {
			asked = cmds
			return false, nil
		}
		if _, err := app.Review(t.Context(), o); !errors.Is(err, app.ErrNotConfirmed) {
			t.Fatalf("%v", err)
		}
		want := []project.Command{{Name: "setup", Run: o.Commands.Setup}, {Name: "test", Run: "go test ./..."}, {Name: "test_one", Run: "go test ./... -run {test}"}}
		if !slices.Equal(asked, want) {
			t.Fatalf("asked %v", asked)
		}
	})

	t.Run("unconfirmed commands do not run", func(t *testing.T) {
		_, outside, o := execution(t)
		if _, err := app.Review(t.Context(), o); !errors.Is(err, app.ErrNotConfirmed) {
			t.Fatalf("%v", err)
		}
		if _, err := os.Stat(filepath.Join(outside, "setup-ran")); err == nil {
			t.Fatal("setup ran before the user confirmed it")
		}
	})

	t.Run("a confirmation shipped in the repository is ignored", func(t *testing.T) {
		root, outside, o := execution(t)
		if err := os.MkdirAll(filepath.Join(root, ".eyeful", "runs"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".eyeful", "runs", "confirmed"), []byte(o.Commands.Digest()+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", "--force", ".eyeful/runs/confirmed")
		git(t, root, "commit", "-q", "-m", "pre-confirm")
		o.Range = subject.Range{Head: "HEAD", Base: "HEAD~2"}
		if _, err := app.Review(t.Context(), o); !errors.Is(err, app.ErrNotConfirmed) {
			t.Fatalf("%v", err)
		}
		if _, err := os.Stat(filepath.Join(outside, "setup-ran")); err == nil {
			t.Fatal("setup ran on a confirmation the repository shipped")
		}
	})

	t.Run("refused at the prompt", func(t *testing.T) {
		_, outside, o := execution(t)
		o.Confirm = func(context.Context, string, []project.Command) (bool, error) { return false, nil }
		if _, err := app.Review(t.Context(), o); !errors.Is(err, app.ErrNotConfirmed) {
			t.Fatalf("%v", err)
		}
		if _, err := os.Stat(filepath.Join(outside, "setup-ran")); err == nil {
			t.Fatal("setup ran after the user refused")
		}
	})

	t.Run("confirmed review", func(t *testing.T) {
		root, outside, o := execution(t)
		before := status(t, root)
		var asked []project.Command
		o.Confirm = func(_ context.Context, work string, cmds []project.Command) (bool, error) {
			if _, err := os.Stat(work); err != nil {
				t.Errorf("asked to run in %s: %v", work, err)
			}
			asked = cmds
			return true, nil
		}
		reviewed, err := app.Review(t.Context(), o)
		if err != nil {
			t.Fatal(err)
		}
		if len(asked) != 1 || asked[0].Name != "setup" {
			t.Fatalf("asked to confirm %v", asked)
		}
		out, err := report.Findings(reviewed)
		if err != nil || len(out.Findings) != 1 || out.Counts.Review != 1 {
			t.Fatalf("reviewed %+v, %v", out, err)
		}
		if _, err := os.Stat(filepath.Join(outside, "setup-ran")); err != nil {
			t.Fatal("setup did not run after confirmation")
		}
		if after := status(t, root); after != before {
			t.Fatalf("the reviewed repository changed:\n%s\n--\n%s", before, after)
		}
		if runs, _ := filepath.Glob(filepath.Join(root, ".eyeful", "runs", "*", "results.sarif")); len(runs) != 1 {
			t.Fatalf("run files %v", runs)
		}

		o.Confirm = nil
		if _, err := app.Review(t.Context(), o); err != nil {
			t.Fatalf("a confirmed config was asked again: %v", err)
		}
		saved, _ := filepath.Glob(filepath.Join(root, ".eyeful", "runs", "*", "findings.json"))
		data, err := os.ReadFile(saved[0])
		var printed struct {
			Findings []struct {
				Severity string `json:"severity"`
			} `json:"findings"`
		}
		if err != nil || json.Unmarshal(data, &printed) != nil || len(printed.Findings) != 1 || printed.Findings[0].Severity != "warning" {
			t.Fatalf("findings.json %v: %s", err, data)
		}
	})

	t.Run("the next review of a branch reruns what the last one reproduced", func(t *testing.T) {
		for name, tc := range map[string]struct {
			base     string
			resolved bool
		}{
			"the same branch with a fix on top":         {"HEAD~2", true},
			"another change that starts where it ended": {"HEAD~1", false},
		} {
			root, _, o := execution(t)
			o.Confirm = yes
			old, err := exec.CommandContext(t.Context(), "git", "-C", root, "rev-parse", "HEAD~1", "HEAD").Output()
			if err != nil {
				t.Fatal(err)
			}
			commits := strings.Fields(string(old))
			last := filepath.Join(root, ".eyeful", "runs", "20000101-000000")
			snapshot, err := json.Marshal(subject.Snapshot{Root: root, Base: commits[0], Head: commits[1]})
			if err != nil {
				t.Fatal(err)
			}
			found := workflow.Finding{ID: "0/correctness/0", Expert: "correctness"}
			found.Path, found.Line, found.Severity, found.Subject, found.Evidence = "app.txt", 1, workflow.SeverityError, "Writes two.", workflow.EvidenceTest
			result, err := json.Marshal(workflow.Result{
				Findings:      []workflow.Finding{found},
				Verifications: []workflow.Verification{{Finding: "0/correctness/0", Status: workflow.StatusReproduced, Repro: &workflow.Repro{Run: "app.txt", Fails: "still two", Oracle: workflow.OracleDerived}}},
			})
			if err := errors.Join(err, os.MkdirAll(last, 0o700), os.WriteFile(filepath.Join(last, "..", ".gitignore"), []byte("*\n"), 0o600), os.WriteFile(filepath.Join(last, "snapshot.json"), snapshot, 0o600), os.WriteFile(filepath.Join(last, "result.json"), result, 0o600)); err != nil {
				t.Fatal(err)
			}
			o.Commands = project.Config{TestOne: "grep -q three {test} || { echo still two; exit 1; }"}
			if err := os.WriteFile(filepath.Join(root, "app.txt"), []byte("three\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			git(t, root, "commit", "-qam", "fix")
			o.Range.Base = tc.base
			if _, err := app.Review(t.Context(), o); err != nil {
				t.Fatal(err)
			}
			if out := o.Out.(*strings.Builder).String(); strings.Contains(out, "## Resolved since the last review") != tc.resolved || strings.Contains(out, "`app.txt:1` Writes two.") != tc.resolved {
				t.Fatalf("%s: the last review's finding listed as resolved is not %t:\n%s", name, tc.resolved, out)
			}
		}
	})

	t.Run("no agent", func(t *testing.T) {
		_, _, o := setup(t, false)
		if _, err := app.Review(t.Context(), o); !errors.Is(err, settings.ErrNoAgent) {
			t.Fatalf("%v", err)
		}
	})

	t.Run("providers", func(t *testing.T) {
		setup(t, false)
		path, err := settings.Path()
		if err != nil {
			t.Fatal(err)
		}
		if err := settings.Save(path, settings.Settings{Agent: "pi"}); err != nil {
			t.Fatal(err)
		}
		bin := t.TempDir()
		if err := os.WriteFile(filepath.Join(bin, "codex"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin)
		ps, err := app.Providers()
		if err != nil {
			t.Fatal(err)
		}
		got := map[string][2]bool{}
		for _, p := range ps {
			got[p.Name] = [2]bool{p.Connected, p.Installed}
		}
		want := map[string][2]bool{"claude": {false, false}, "codex": {false, true}, "pi": {true, false}}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("pulls.review core groups the change and Go picks the experts", func(t *testing.T) {
		root, _, o := setup(t, true)
		o.Experts = nil
		if _, err := app.Review(t.Context(), o); err != nil {
			t.Fatal(err)
		}
		saved, err := filepath.Glob(filepath.Join(root, ".eyeful", "runs", "*", "result.json"))
		if err != nil || len(saved) != 1 {
			t.Fatalf("result.json %v: %v", saved, err)
		}
		data, err := os.ReadFile(saved[0])
		if err != nil {
			t.Fatal(err)
		}
		var res workflow.Result
		if err := json.Unmarshal(data, &res); err != nil {
			t.Fatal(err)
		}
		if res.PlanSource != workflow.PlanFromPlanner || res.Plan.Summary != "Writes two to app.txt." || len(res.Plan.Groups) != 1 || res.Plan.Groups[0].Category != "core" || res.Plan.Groups[0].Experts[0].Name != "correctness" {
			t.Fatalf("plan %s %+v, notes %v", res.PlanSource, res.Plan, res.Uncertainty.Notes)
		}
	})

	t.Run("unknown expert", func(t *testing.T) {
		_, _, o := setup(t, true)
		o.Experts = []string{"style"}
		if _, err := app.Review(t.Context(), o); !errors.Is(err, workflow.ErrInvalidRequest) {
			t.Fatalf("%v", err)
		}
	})
}

func TestCommit(t *testing.T) {
	root, _, o := setup(t, true)
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@t")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@t")
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	message, err := app.CommitMessage(t.Context(), root, o.Log)
	if err != nil || message != "fix: write two\n\nThe value changed." {
		t.Fatalf("message %q: %v", message, err)
	}
	head, err := app.Commit(t.Context(), root, message)
	if err != nil {
		t.Fatal(err)
	}
	if left := status(t, root); left != "" {
		t.Fatalf("not committed:\n%s", left)
	}
	out, err := exec.CommandContext(t.Context(), "git", "-C", root, "log", "-1", "--format=%H%n%B").Output()
	if err != nil || string(out) != head+"\n"+message+"\n\n" {
		t.Fatalf("last commit %q: %v", out, err)
	}
	if _, err := app.CommitMessage(t.Context(), root, o.Log); !errors.Is(err, app.ErrNothingToCommit) {
		t.Fatalf("%v", err)
	}
	if _, err := app.Commit(t.Context(), root, " "); !errors.Is(err, app.ErrNoMessage) {
		t.Fatalf("%v", err)
	}
}

func TestRepository(t *testing.T) {
	root, _, _ := setup(t, true)
	outside := t.TempDir()
	if dir, err := app.Repository(t.Context(), outside); err != nil || dir != outside {
		t.Fatalf("nothing remembered: %q, %v", dir, err)
	}
	if err := app.Remember(root); err != nil {
		t.Fatal(err)
	}
	if dir, err := app.Repository(t.Context(), outside); err != nil || dir != root {
		t.Fatalf("outside a repository: %q, %v", dir, err)
	}
	inside := filepath.Join(root, "sub")
	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if dir, err := app.Repository(t.Context(), inside); err != nil || dir != inside {
		t.Fatalf("inside a repository: %q, %v", dir, err)
	}
	if _, err := app.Connect(t.Context(), "claude"); err != nil {
		t.Fatal(err)
	}
	s, err := app.Current()
	if err != nil || s.Agent != "claude" || s.Repository != root {
		t.Fatalf("connect kept %+v, %v", s, err)
	}
}
