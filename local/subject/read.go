package subject

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Pleasurecruise/eyeful/local/git"
)

var snapshotIdentity = []string{
	"GIT_AUTHOR_NAME=eyeful", "GIT_AUTHOR_EMAIL=eyeful@localhost", "GIT_AUTHOR_DATE=@0 +0000",
	"GIT_COMMITTER_NAME=eyeful", "GIT_COMMITTER_EMAIL=eyeful@localhost", "GIT_COMMITTER_DATE=@0 +0000",
}

func commit(ctx context.Context, root, ref string) (string, error) {
	out, err := git.Run(ctx, root, nil, "rev-parse", "--verify", "--quiet", "--end-of-options", ref+"^{commit}")
	var exit *exec.ExitError
	if ctx.Err() == nil && errors.As(err, &exit) {
		return "", fmt.Errorf("%q: %w", ref, ErrUnknownRef)
	}
	return strings.TrimSpace(out), err
}

func Read(ctx context.Context, dir string, r Range, objects string) (Snapshot, error) {
	out, err := git.Run(ctx, dir, nil, "rev-parse", "--path-format=absolute", "--show-toplevel", "--git-path", "index", "--git-path", "objects")
	if err != nil {
		return Snapshot{}, fmt.Errorf("%s: %w: %w", dir, ErrNotRepository, err)
	}
	paths := strings.Split(strings.TrimSpace(out), "\n")
	root, index := paths[0], paths[1]
	alternates := filepath.Join(objects, "info", "alternates")
	if err := errors.Join(os.MkdirAll(filepath.Dir(alternates), 0o700), os.WriteFile(alternates, []byte(paths[2]+"\n"), 0o600)); err != nil {
		return Snapshot{}, fmt.Errorf("snapshot objects: %w", err)
	}
	env := []string{"GIT_OBJECT_DIRECTORY=" + objects}

	tip := "HEAD"
	var head string
	if r.Head != "" {
		if head, err = commit(ctx, root, r.Head); err != nil {
			return Snapshot{}, err
		}
		tip = head
	}

	base := EmptyTree
	switch {
	case r.Base != "":
		from, err := commit(ctx, root, r.Base)
		if err != nil {
			return Snapshot{}, err
		}
		mb, err := git.Run(ctx, root, nil, "merge-base", from, tip)
		if err != nil {
			return Snapshot{}, fmt.Errorf("%s and %s: %w: %w", r.Base, tip, ErrNoMergeBase, err)
		}
		base = strings.TrimSpace(mb)
	case head != "":
		if base, err = commit(ctx, root, "HEAD"); err != nil {
			return Snapshot{}, err
		}
	default:
		c, err := commit(ctx, root, "HEAD")
		switch {
		case err == nil:
			base = c
		case !errors.Is(err, ErrUnknownRef):
			return Snapshot{}, err
		}
	}

	snap := Snapshot{Root: root, Base: base, Head: head, Commit: head}
	if head == "" {
		if snap.Commit, err = worktreeCommit(ctx, root, index, env); err != nil {
			return Snapshot{}, err
		}
	}
	if snap.Patch, err = git.Run(ctx, root, env, "diff", "--no-color", "--no-ext-diff", "--find-renames", base, snap.Commit); err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}

func worktreeCommit(ctx context.Context, root, index string, objects []string) (_ string, err error) {
	scratch, err := os.MkdirTemp("", "eyeful-index-")
	if err != nil {
		return "", fmt.Errorf("index copy: %w", err)
	}
	defer func() { err = errors.Join(err, os.RemoveAll(scratch)) }()
	tmpIndex := filepath.Join(scratch, "index")
	data, err := os.ReadFile(index)
	switch {
	case err == nil:
		info, err := os.Stat(index)
		if err != nil {
			return "", fmt.Errorf("read index: %w", err)
		}
		if err := os.WriteFile(tmpIndex, data, 0o600); err != nil {
			return "", fmt.Errorf("index copy: %w", err)
		}
		if err := os.Chtimes(tmpIndex, info.ModTime(), info.ModTime()); err != nil {
			return "", fmt.Errorf("index copy: %w", err)
		}
	case !errors.Is(err, os.ErrNotExist):
		return "", fmt.Errorf("read index: %w", err)
	}

	env := append([]string{"GIT_INDEX_FILE=" + tmpIndex}, objects...)
	if _, err := git.Run(ctx, root, env, "add", "--all"); err != nil {
		return "", err
	}
	tree, err := git.Run(ctx, root, env, "write-tree")
	if err != nil {
		return "", err
	}
	args := []string{"commit-tree", "--no-gpg-sign", "-m", "eyeful snapshot of uncommitted changes", strings.TrimSpace(tree)}
	parent, err := commit(ctx, root, "HEAD")
	switch {
	case err == nil:
		args = append(args, "-p", parent)
	case !errors.Is(err, ErrUnknownRef):
		return "", err
	}
	out, err := git.Run(ctx, root, slices.Concat(snapshotIdentity, objects), args...)
	return strings.TrimSpace(out), err
}
