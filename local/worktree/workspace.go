package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pleasurecruise/eyeful/local/tail"
	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/project"
)

func NewWorkspace(w *Worktree, cfg project.Config, timeout time.Duration) *Workspace {
	return &Workspace{tree: w, cfg: cfg, timeout: timeout, turn: make(chan struct{}, 1)}
}

func (s *Workspace) run(ctx context.Context, command string) (bool, string, error) {
	runCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "sh", "-c", command)
	group(cmd)
	cmd.Env = append(os.Environ(), s.tree.env...)
	cmd.Dir = s.tree.dir
	cmd.WaitDelay = 5 * time.Second
	out := tail.New(outputLimit)
	cmd.Stdout, cmd.Stderr = out, out
	start := time.Now()
	err := cmd.Run()
	s.tree.log.Info("command", "run", command, "seconds", int(time.Since(start).Seconds()), "error", err)
	var exit *exec.ExitError
	switch {
	case ctx.Err() != nil:
		return false, out.String(), fmt.Errorf("%s: %w", command, ctx.Err())
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		return false, out.String() + fmt.Sprintf("\ntimed out after %s", s.timeout), nil
	case errors.As(err, &exit):
		return false, out.String(), nil
	case err != nil:
		return false, out.String(), fmt.Errorf("%s: %w", command, err)
	}
	return true, out.String(), nil
}

func (s *Workspace) Setup(ctx context.Context) error {
	release, err := s.acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	if s.setup || s.cfg.Setup == "" {
		return s.setupErr
	}
	passed, out, err := s.run(ctx, s.cfg.Setup)
	if err != nil {
		return err
	}
	s.setup = true
	if !passed {
		s.setupErr = fmt.Errorf("%w: %s", ErrSetup, tail.Last(out, 2000))
		return s.setupErr
	}
	change, err := s.tree.settle(ctx)
	s.tree.change = change
	return err
}

func (s *Workspace) CI(ctx context.Context) (workflow.Check, error) {
	if err := s.Setup(ctx); err != nil {
		if ctx.Err() != nil {
			return workflow.Check{}, err
		}
		return workflow.Check{Passed: true, Log: err.Error()}, nil
	}
	if s.cfg.Lint == "" {
		return workflow.Check{Passed: true}, nil
	}
	release, err := s.acquire(ctx)
	if err != nil {
		return workflow.Check{}, err
	}
	defer release()
	passed, out, err := s.run(ctx, s.cfg.Lint)
	if err != nil {
		return workflow.Check{}, err
	}
	return workflow.Check{Passed: passed, Log: out}, s.tree.reset(ctx)
}

func (s *Workspace) Tools(context.Context, []string) ([]workflow.ToolResult, error) {
	// TODO(worktree): run the L1 tools for the project's languages.
	return []workflow.ToolResult{}, nil
}

func (s *Workspace) acquire(ctx context.Context) (func(), error) {
	select {
	case s.turn <- struct{}{}:
		return func() { <-s.turn }, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("wait for the worktree: %w", ctx.Err())
	}
}

func (s *Workspace) Test(ctx context.Context, edits []workflow.Edit, t workflow.Target) (workflow.TestRun, error) {
	release, err := s.acquire(ctx)
	if err != nil {
		return workflow.TestRun{}, err
	}
	defer release()
	command, label := s.cfg.Test, []string{"test"}
	if len(t.Tests) > 0 {
		quoted := make([]string, len(t.Tests))
		for i, id := range t.Tests {
			quoted[i] = "'" + strings.ReplaceAll(id, "'", `'\''`) + "'"
		}
		command, label = "", t.Tests
		if s.cfg.TestOne != "" {
			command = strings.ReplaceAll(s.cfg.TestOne, project.Placeholder, strings.Join(quoted, " "))
		}
	}
	if command == "" {
		return workflow.TestRun{}, fmt.Errorf("%w: add test and test_one to %s", ErrNoCommand, project.Path)
	}
	run := workflow.TestRun{Failed: []string{}}
	err = s.apply(edits)
	if err == nil {
		var out string
		run.Passed, out, err = s.run(ctx, command)
		run.Output = tail.Last(out, 8000)
		if !run.Passed {
			run.Failed = label
		}
	}
	return run, errors.Join(err, s.tree.reset(context.WithoutCancel(ctx)))
}

func (s *Workspace) apply(edits []workflow.Edit) (err error) {
	root, err := os.OpenRoot(s.tree.dir)
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	for _, e := range edits {
		p := filepath.Clean(filepath.FromSlash(e.Path))
		if e.Path == "" || filepath.IsAbs(p) || strings.SplitN(p, string(filepath.Separator), 2)[0] == ".git" {
			return fmt.Errorf("%w: %q", ErrUnsafeTarget, e.Path)
		}
		content := e.New
		if e.Old != "" {
			data, err := root.ReadFile(p)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrInvalidEdit, err)
			}
			if n := strings.Count(string(data), e.Old); n != 1 {
				return fmt.Errorf("%w: the text to replace occurs %d times in %s", ErrInvalidEdit, n, e.Path)
			}
			content = strings.Replace(string(data), e.Old, e.New, 1)
		}
		if err := root.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidEdit, err)
		}
		if err := root.WriteFile(p, []byte(content), 0o644); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidEdit, err)
		}
	}
	return nil
}

type ReadOnly struct{}

func (ReadOnly) CI(context.Context) (workflow.Check, error) { return workflow.Check{Passed: true}, nil }
func (ReadOnly) Setup(context.Context) error                { return nil }
func (ReadOnly) Tools(context.Context, []string) ([]workflow.ToolResult, error) {
	return []workflow.ToolResult{}, nil
}
func (ReadOnly) Test(context.Context, []workflow.Edit, workflow.Target) (workflow.TestRun, error) {
	return workflow.TestRun{}, fmt.Errorf("%w: verification is not execution", ErrNoCommand)
}
