package subject_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Pleasurecruise/eyeful/local/subject"
)

func TestBranches(t *testing.T) {
	dir, other := t.TempDir(), filepath.Join(t.TempDir(), "other")
	run(t, dir, "init", "-q", "-b", "main")
	write(t, dir, "a.txt", "a\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-q", "-m", "init")
	run(t, dir, "branch", "idle")
	run(t, dir, "worktree", "add", "-q", "-b", "feature", other)

	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	elsewhere, err := filepath.EvalSymlinks(other)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		from string
		want []subject.Branch
	}{
		{dir, []subject.Branch{{Name: "feature", Worktree: elsewhere}, {Name: "idle"}, {Name: "main", Current: true, Worktree: root}}},
		{other, []subject.Branch{{Name: "feature", Current: true, Worktree: elsewhere}, {Name: "idle"}, {Name: "main", Worktree: root}}},
	} {
		got, err := subject.Branches(t.Context(), c.from)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Fatalf("from %s:\n got %v\nwant %v", c.from, got, c.want)
		}
	}
}
