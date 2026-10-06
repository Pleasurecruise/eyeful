package subject

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Pleasurecruise/eyeful/local/git"
)

func CheckedOut(ctx context.Context, root, head string) error {
	out, err := git.Run(ctx, root, nil, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != head {
		return fmt.Errorf("%w: check out %.12s first", ErrNotCheckedOut, head)
	}
	status, err := git.Run(ctx, root, nil, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("%w: %s has uncommitted changes that are not part of %.12s", ErrNotCheckedOut, root, head)
	}
	return nil
}

func Changed(ctx context.Context, s Snapshot, r Range, objects string) (bool, error) {
	now, err := Read(ctx, s.Root, r, objects)
	switch {
	case err != nil:
		return false, err
	case now.Commit != s.Commit:
		return true, nil
	case s.Head == "":
		return false, nil
	}
	err = CheckedOut(ctx, s.Root, s.Head)
	if errors.Is(err, ErrNotCheckedOut) {
		return true, nil
	}
	return false, err
}

func Commits(ctx context.Context, s Snapshot) ([]string, error) {
	commits := []string{}
	if s.Base == EmptyTree {
		return commits, nil
	}
	tip := s.Head
	if tip == "" {
		tip = "HEAD"
	}
	out, err := git.Run(ctx, s.Root, nil, "log", "-z", "--format=%B", fmt.Sprintf("--max-count=%d", maxCommits), s.Base+".."+tip)
	if err != nil {
		return nil, err
	}
	for m := range strings.SplitSeq(out, "\x00") {
		if m = strings.TrimSpace(m); m != "" {
			commits = append(commits, m)
		}
	}
	return commits, nil
}
