package worktree

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Pleasurecruise/eyeful/local/git"
	"github.com/Pleasurecruise/eyeful/local/subject"
)

var safe = []string{"-c", "core.hooksPath=/dev/null", "-c", "user.name=eyeful", "-c", "user.email=eyeful@localhost"}

func (w *Worktree) git(ctx context.Context, args ...string) (string, error) {
	return git.Run(ctx, w.dir, w.env, slices.Concat(safe, args)...)
}

func Open(ctx context.Context, log *slog.Logger, snap subject.Snapshot, objects string) (*Worktree, error) {
	dir, err := os.MkdirTemp("", "eyeful-work-")
	if err != nil {
		return nil, fmt.Errorf("work directory: %w", err)
	}
	if _, err := git.Run(ctx, snap.Root, []string{"GIT_OBJECT_DIRECTORY=" + objects}, slices.Concat(safe, []string{"worktree", "add", "--detach", dir, snap.Commit})...); err != nil {
		return nil, errors.Join(err, os.RemoveAll(dir))
	}
	w := &Worktree{root: snap.Root, dir: dir, env: []string{"GIT_OBJECT_DIRECTORY=" + objects}, log: log.With(slog.String("worktree", dir))}
	if w.change, err = w.settle(ctx); err != nil {
		return nil, errors.Join(err, w.Close(context.WithoutCancel(ctx)))
	}
	return w, nil
}

func (w *Worktree) Dir() string { return w.dir }

func (w *Worktree) settle(ctx context.Context) (string, error) {
	if _, err := w.git(ctx, "add", "--all"); err != nil {
		return "", err
	}
	out, err := w.git(ctx, "write-tree")
	return strings.TrimSpace(out), err
}

func (w *Worktree) reset(ctx context.Context) (err error) {
	if _, err := w.git(ctx, "read-tree", w.change); err != nil {
		return err
	}
	if _, err := w.git(ctx, "checkout-index", "--all", "--force"); err != nil {
		return err
	}
	out, err := w.git(ctx, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(w.dir)
	if err != nil {
		return fmt.Errorf("open %s: %w", w.dir, err)
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	for p := range strings.SplitSeq(out, "\x00") {
		if p == "" {
			continue
		}
		if err := root.Remove(filepath.FromSlash(p)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", p, err)
		}
	}
	return nil
}

func (w *Worktree) Close(ctx context.Context) error {
	_, err := git.Run(ctx, w.root, w.env, slices.Concat(safe, []string{"worktree", "remove", "--force", w.dir})...)
	return err
}
