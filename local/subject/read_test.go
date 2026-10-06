package subject_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Pleasurecruise/eyeful/local/subject"
)

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRead(t *testing.T) {
	ctx := context.Background()

	t.Run("changes against HEAD", func(t *testing.T) {
		dir := t.TempDir()
		run(t, dir, "init", "-q")
		write(t, dir, ".gitignore", "ignored/\n")
		write(t, dir, "kept.txt", "one\ntwo\n")
		write(t, dir, "staged.txt", "a\n")
		run(t, dir, "add", ".")
		run(t, dir, "commit", "-qm", "init")
		write(t, dir, "kept.txt", "one\nthree\n")
		write(t, dir, "staged.txt", "a\nb\n")
		run(t, dir, "add", "staged.txt")
		write(t, dir, "deep/a/b/c/new.go", "package c\n")
		write(t, dir, "ignored/secret.txt", "x\n")
		index := run(t, dir, "ls-files", "--stage")
		count := run(t, dir, "count-objects")

		objects := t.TempDir()
		s, err := subject.Read(ctx, filepath.Join(dir, "deep"), subject.Range{}, objects)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"+three", "+b", "new file mode", "deep/a/b/c/new.go", "+package c"} {
			if !strings.Contains(s.Patch, want) {
				t.Errorf("patch lacks %q:\n%s", want, s.Patch)
			}
		}
		if strings.Contains(s.Patch, "secret") {
			t.Errorf("patch includes an ignored file:\n%s", s.Patch)
		}
		if s.Base == subject.EmptyTree || len(s.Base) != 40 {
			t.Errorf("base = %q, want the HEAD commit", s.Base)
		}
		if got := run(t, dir, "ls-files", "--stage"); got != index {
			t.Errorf("index changed:\n%s\nwant\n%s", got, index)
		}
		show := exec.CommandContext(t.Context(), "git", "show", s.Commit+":deep/a/b/c/new.go")
		show.Dir, show.Env = dir, append(os.Environ(), "GIT_OBJECT_DIRECTORY="+objects)
		if got, err := show.Output(); err != nil || string(got) != "package c\n" {
			t.Errorf("the snapshot commit lacks the untracked file: %q, %v", got, err)
		}
		if err := os.RemoveAll(objects); err != nil {
			t.Fatal(err)
		}
		if exec.CommandContext(t.Context(), "git", "-C", dir, "cat-file", "-e", s.Commit).Run() == nil {
			t.Error("the snapshot commit is left in the repository")
		}
		if count != run(t, dir, "count-objects") {
			t.Errorf("the repository gained objects: %s", run(t, dir, "count-objects"))
		}
		again, err := subject.Read(ctx, dir, subject.Range{}, t.TempDir())
		if err != nil || again.Commit != s.Commit {
			t.Errorf("the same changes gave snapshot %q, then %q (%v)", s.Commit, again.Commit, err)
		}
		write(t, dir, "kept.txt", "one\nfour\n")
		if later, err := subject.Read(ctx, dir, subject.Range{}, t.TempDir()); err != nil || later.Commit == s.Commit {
			t.Errorf("an edit kept snapshot %q (%v)", later.Commit, err)
		}
	})

	t.Run("no commits", func(t *testing.T) {
		dir := t.TempDir()
		run(t, dir, "init", "-q")
		write(t, dir, "first.txt", "hello\n")
		s, err := subject.Read(ctx, dir, subject.Range{}, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if s.Base != subject.EmptyTree || !strings.Contains(s.Patch, "+hello") {
			t.Fatalf("base = %q, patch:\n%s", s.Base, s.Patch)
		}
		if got := run(t, dir, "ls-files"); got != "" {
			t.Errorf("index gained files: %q", got)
		}
	})

	t.Run("clean", func(t *testing.T) {
		dir := t.TempDir()
		run(t, dir, "init", "-q")
		write(t, dir, "a.txt", "a\n")
		run(t, dir, "add", ".")
		run(t, dir, "commit", "-qm", "init")
		s, err := subject.Read(ctx, dir, subject.Range{}, t.TempDir())
		if err != nil || s.Patch != "" {
			t.Fatalf("patch = %q, err = %v", s.Patch, err)
		}
	})

	t.Run("not a repository", func(t *testing.T) {
		_, err := subject.Read(ctx, t.TempDir(), subject.Range{}, t.TempDir())
		if !errors.Is(err, subject.ErrNotRepository) {
			t.Fatalf("err = %v, want ErrNotRepository", err)
		}
	})
}

func TestRanges(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	write(t, dir, "a.txt", "one\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-qm", "init")
	run(t, dir, "checkout", "-qb", "feature")
	write(t, dir, "a.txt", "one\ntwo\n")
	run(t, dir, "commit", "-qam", "two")
	run(t, dir, "checkout", "-q", "main")
	write(t, dir, "main.txt", "later on main\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-qm", "main moves on")
	run(t, dir, "checkout", "-q", "feature")
	write(t, dir, "wip.txt", "uncommitted\n")

	tests := []struct {
		name     string
		r        subject.Range
		want     []string
		notWant  []string
		wantHead bool
	}{
		{"branch and uncommitted", subject.Range{Base: "main"}, []string{"+two", "+uncommitted"}, []string{"later on main"}, false},
		{"branch only, not checked out", subject.Range{Base: "main", Head: "feature"}, []string{"+two"}, []string{"uncommitted", "later on main"}, true},
		{"uncommitted only", subject.Range{}, []string{"+uncommitted"}, []string{"+two"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := subject.Read(ctx, dir, tt.r, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tt.want {
				if !strings.Contains(s.Patch, w) {
					t.Errorf("patch lacks %q:\n%s", w, s.Patch)
				}
			}
			for _, w := range tt.notWant {
				if strings.Contains(s.Patch, w) {
					t.Errorf("patch includes %q:\n%s", w, s.Patch)
				}
			}
			if (s.Head != "") != tt.wantHead {
				t.Errorf("head = %q", s.Head)
			}
		})
	}

	if _, err := subject.Read(ctx, dir, subject.Range{Base: "nope"}, t.TempDir()); !errors.Is(err, subject.ErrUnknownRef) {
		t.Errorf("unknown base: err = %v", err)
	}
	if _, err := subject.Read(ctx, dir, subject.Range{Base: "--output=/tmp/x"}, t.TempDir()); !errors.Is(err, subject.ErrUnknownRef) {
		t.Errorf("option-like base: err = %v", err)
	}
	run(t, dir, "checkout", "-q", "--orphan", "lonely")
	run(t, dir, "commit", "-qm", "orphan")
	if _, err := subject.Read(ctx, dir, subject.Range{Base: "main"}, t.TempDir()); !errors.Is(err, subject.ErrNoMergeBase) {
		t.Errorf("orphan branch: err = %v", err)
	}
}

func TestRacyClean(t *testing.T) {
	var dir string
	for attempt := 0; ; attempt++ {
		dir = t.TempDir()
		run(t, dir, "init", "-q", "-b", "main")
		write(t, dir, "a.txt", "one\n")
		run(t, dir, "add", ".")
		run(t, dir, "commit", "-q", "-m", "init")
		write(t, dir, "a.txt", "two\n")
		index, err := os.Stat(filepath.Join(dir, ".git", "index"))
		if err != nil {
			t.Fatal(err)
		}
		file, err := os.Stat(filepath.Join(dir, "a.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if index.ModTime().Unix() == file.ModTime().Unix() {
			time.Sleep(time.Until(file.ModTime().Truncate(time.Second).Add(1100 * time.Millisecond)))
			break
		}
		if attempt == 5 {
			t.Skip("could not write the edit in the same second as the index")
		}
	}
	snap, err := subject.Read(t.Context(), dir, subject.Range{}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(snap.Patch, "+two") {
		t.Fatalf("a same-size edit made in the index's second was missed: %q", snap.Patch)
	}
}

func TestCheckedOut(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	write(t, dir, "a.txt", "a\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-qm", "init")
	run(t, dir, "checkout", "-qb", "other")
	write(t, dir, "o.txt", "o\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-qm", "other")
	run(t, dir, "checkout", "-q", "main")
	s, err := subject.Read(t.Context(), dir, subject.Range{Head: "other"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := subject.CheckedOut(t.Context(), dir, s.Head); !errors.Is(err, subject.ErrNotCheckedOut) {
		t.Fatalf("another branch: %v", err)
	}
	run(t, dir, "checkout", "-q", "other")
	if err := subject.CheckedOut(t.Context(), dir, s.Head); err != nil {
		t.Fatal(err)
	}
	if changed, err := subject.Changed(t.Context(), s, subject.Range{Head: "other"}, t.TempDir()); err != nil || changed {
		t.Fatalf("an unchanged head: %v, %v", changed, err)
	}
	write(t, dir, "o.txt", "dirty\n")
	if err := subject.CheckedOut(t.Context(), dir, s.Head); !errors.Is(err, subject.ErrNotCheckedOut) {
		t.Fatalf("dirty tree: %v", err)
	}
	if changed, err := subject.Changed(t.Context(), s, subject.Range{Head: "other"}, t.TempDir()); err != nil || !changed {
		t.Fatalf("a dirty head: %v, %v", changed, err)
	}
}
