package subject

import (
	"cmp"
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

func History(ctx context.Context, s Snapshot) (string, error) {
	if s.Base == EmptyTree {
		return "", nil
	}
	var b strings.Builder
	path := ""
	for line := range strings.Lines(s.Patch) {
		if rest, ok := strings.CutPrefix(line, "--- a/"); ok {
			path = strings.TrimSpace(rest)
			log, err := git.Run(ctx, s.Root, nil, "log", fmt.Sprintf("--max-count=%d", fileCommits), "--date=short", "--format=%h %ad %an: %s", s.Base, "--", path)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&b, "## %s\n\nCommits that last touched it before this change:\n\n%s\n", path, log)
			continue
		}
		m := hunk.FindStringSubmatch(line)
		if m == nil || path == "" || m[2] == "0" || b.Len() > historyLimit {
			continue
		}
		blame, err := git.Run(ctx, s.Root, nil, "blame", "-s", "-L", m[1]+",+"+cmp.Or(m[2], "1"), s.Base, "--", path)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "Who last changed the lines replaced at %s:\n\n%s\n", strings.TrimSpace(m[0]), blame)
	}
	return b.String(), nil
}
