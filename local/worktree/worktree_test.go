package worktree_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Pleasurecruise/eyeful/local/subject"
	"github.com/Pleasurecruise/eyeful/local/worktree"
	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/project"
)

func sh(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t", "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func repo(t *testing.T) (string, string) {
	t.Helper()
	root, outside := t.TempDir(), t.TempDir()
	sh(t, root, "git", "init", "-q", "-b", "main")
	write(t, filepath.Join(root, "app.txt"), "one\n")
	sh(t, root, "git", "add", ".")
	sh(t, root, "git", "commit", "-q", "-m", "init")
	write(t, filepath.Join(root, ".git/hooks/post-checkout"), "#!/bin/sh\ntouch "+filepath.Join(outside, "hook-ran")+"\n")
	write(t, filepath.Join(root, "app.txt"), "two\n")
	write(t, filepath.Join(root, "new.txt"), "new\n")
	sh(t, root, "git", "add", ".")
	sh(t, root, "git", "commit", "-q", "-m", "change")
	return root, outside
}

func TestWorktree(t *testing.T) {
	ctx := context.Background()
	root, outside := repo(t)
	objects := t.TempDir()
	snap, err := subject.Read(ctx, root, subject.Range{Head: "HEAD", Base: "HEAD~1"}, objects)
	if err != nil {
		t.Fatal(err)
	}
	w, err := worktree.Open(ctx, slog.New(slog.DiscardHandler), snap, objects)
	if err != nil {
		t.Fatal(err)
	}
	if w.Dir() == snap.Root || !exists(filepath.Join(w.Dir(), "new.txt")) {
		t.Fatalf("dir %q is not a checkout of the change", w.Dir())
	}
	if exists(filepath.Join(outside, "hook-ran")) {
		t.Fatal("a repository hook ran while opening the repository")
	}

	cfg := project.Config{
		Setup:   "touch setup-ran",
		Lint:    "echo formatted > app.txt; false",
		Test:    "grep -q fixed app.txt",
		TestOne: "test -f repro.txt && echo {test} >> ran.txt && grep -q fixed app.txt",
	}
	ws := worktree.NewWorkspace(w, cfg, 2*time.Second)
	check, err := ws.CI(ctx)
	if err != nil || check.Passed || !exists(filepath.Join(w.Dir(), "setup-ran")) {
		t.Fatalf("ci %+v, %v", check, err)
	}
	if data, _ := os.ReadFile(filepath.Join(w.Dir(), "app.txt")); string(data) != "two\n" {
		t.Fatalf("lint changes survived: %q", data)
	}

	repro := workflow.Edit{Path: "repro.txt", New: "x"}
	evil := "x'; touch " + filepath.Join(outside, "pwned") + "; echo '"
	run, err := ws.Test(ctx, []workflow.Edit{repro}, workflow.Target{Tests: []string{evil}})
	if err != nil || run.Passed || run.Failed[0] != evil {
		t.Fatalf("repro on the change: %+v, %v", run, err)
	}
	if exists(filepath.Join(outside, "pwned")) {
		t.Fatal("a test identifier ran as shell")
	}
	if exists(filepath.Join(w.Dir(), "repro.txt")) || exists(filepath.Join(w.Dir(), "ran.txt")) {
		t.Fatal("a test run left files behind")
	}
	fix := workflow.Edit{Path: "app.txt", Old: "two", New: "fixed"}
	if run, err := ws.Test(ctx, []workflow.Edit{repro, fix}, workflow.Target{Tests: []string{"t"}}); err != nil || !run.Passed {
		t.Fatalf("repro with the fix: %+v, %v", run, err)
	}
	if run, err := ws.Test(ctx, nil, workflow.Target{Touched: []string{"app.txt"}}); err != nil || run.Passed {
		t.Fatalf("suite on the change: %+v, %v", run, err)
	}

	for name, e := range map[string]workflow.Edit{
		"parent":  {Path: "../escape.txt", New: "x"},
		"git dir": {Path: ".git/hooks/pre-commit", New: "x"},
		"absent":  {Path: "app.txt", Old: "missing", New: "x"},
	} {
		if _, err := ws.Test(ctx, []workflow.Edit{e}, workflow.Target{Tests: []string{"t"}}); err == nil {
			t.Errorf("%s: edit accepted", name)
		}
	}
	if exists(filepath.Join(filepath.Dir(w.Dir()), "escape.txt")) {
		t.Fatal("an edit escaped the repository")
	}

	busy := worktree.NewWorkspace(w, project.Config{TestOne: "test ! -f busy && touch busy && sleep 0.3 && rm busy # {test}"}, 5*time.Second)
	var wg sync.WaitGroup
	results := make([]workflow.TestRun, 2)
	errs := make([]error, 2)
	for i := range 2 {
		wg.Go(func() { results[i], errs[i] = busy.Test(ctx, nil, workflow.Target{Tests: []string{"t"}}) })
	}
	wg.Wait()
	for i := range 2 {
		if errs[i] != nil || !results[i].Passed {
			t.Fatalf("concurrent run %d overlapped: %+v, %v", i, results[i], errs[i])
		}
	}

	slow := worktree.NewWorkspace(w, project.Config{Test: "sleep 5"}, 200*time.Millisecond)
	if run, err := slow.Test(ctx, nil, workflow.Target{}); err != nil || run.Passed {
		t.Fatalf("timeout: %+v, %v", run, err)
	}
	pidFile := filepath.Join(outside, "child.pid")
	orphan := worktree.NewWorkspace(w, project.Config{Test: "sleep 30 & echo $! > " + pidFile + "; wait"}, 300*time.Millisecond)
	start := time.Now()
	if run, err := orphan.Test(ctx, nil, workflow.Target{}); err != nil || run.Passed {
		t.Fatalf("timeout with a child: %+v, %v", run, err)
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("a timed-out command took %s to stop", took)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	child, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(2 * time.Second); child.Signal(syscall.Signal(0)) == nil; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("a timed-out command's child kept running")
		}
	}

	if _, err := worktree.NewWorkspace(w, project.Config{}, time.Second).Test(ctx, nil, workflow.Target{Tests: []string{"t"}}); !errors.Is(err, worktree.ErrNoCommand) {
		t.Fatalf("no command: %v", err)
	}

	if data, _ := os.ReadFile(filepath.Join(root, "app.txt")); string(data) != "two\n" {
		t.Fatalf("the user's working copy changed: %q", data)
	}
	if err := w.Close(ctx); err != nil {
		t.Fatalf("close: %v", err)
	}
	if exists(w.Dir()) || exists(filepath.Join(root, "setup-ran")) {
		t.Fatal("close left the work directory, or a command reached the repository")
	}
	if out, err := exec.CommandContext(ctx, "git", "-C", root, "worktree", "list", "--porcelain").Output(); err != nil || strings.Count(string(out), "worktree ") != 1 {
		t.Fatalf("worktrees after close %v:\n%s", err, out)
	}
}

func TestEmptyRepository(t *testing.T) {
	root := t.TempDir()
	sh(t, root, "git", "init", "-q")
	write(t, filepath.Join(root, "a.txt"), "a\n")
	objects := t.TempDir()
	snap, err := subject.Read(t.Context(), root, subject.Range{}, objects)
	if err != nil {
		t.Fatal(err)
	}
	w, err := worktree.Open(t.Context(), slog.New(slog.DiscardHandler), snap, objects)
	if err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(w.Dir(), "a.txt")) {
		t.Fatal("the checkout lacks the uncommitted file")
	}
	if err := w.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestUncommitted(t *testing.T) {
	ctx := t.Context()
	root, _ := repo(t)
	write(t, filepath.Join(root, "app.txt"), "three\n")
	objects := t.TempDir()
	snap, err := subject.Read(ctx, root, subject.Range{}, objects)
	if err != nil {
		t.Fatal(err)
	}
	w, err := worktree.Open(ctx, slog.New(slog.DiscardHandler), snap, objects)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "app.txt"), "edited meanwhile\n")
	ws := worktree.NewWorkspace(w, project.Config{Test: "grep -q three app.txt"}, time.Second)
	if run, err := ws.Test(ctx, nil, workflow.Target{}); err != nil || !run.Passed {
		t.Fatalf("the checkout does not hold the snapshot: %+v, %v", run, err)
	}
	if err := w.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "app.txt")); string(data) != "edited meanwhile\n" {
		t.Fatalf("the user's edit was overwritten: %q", data)
	}
}
