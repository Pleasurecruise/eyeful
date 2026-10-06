package subject

import (
	"context"
	"strings"

	"github.com/Pleasurecruise/eyeful/local/git"
)

func Branches(ctx context.Context, dir string) ([]Branch, error) {
	out, err := git.Run(ctx, dir, nil, "for-each-ref", "--format=%(refname:short)%00%(HEAD)%00%(worktreepath)", "refs/heads")
	if err != nil {
		return nil, err
	}
	branches := []Branch{}
	for line := range strings.Lines(out) {
		fields := strings.Split(strings.TrimSuffix(line, "\n"), "\x00")
		branches = append(branches, Branch{Name: fields[0], Current: fields[1] == "*", Worktree: fields[2]})
	}
	return branches, nil
}
