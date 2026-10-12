package subject_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/Pleasurecruise/eyeful/local/subject"
)

func TestCommits(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	for i, message := range []string{"init", "feat: add a\n\nWhy a.", "fix: b"} {
		write(t, dir, "f.txt", strconv.Itoa(i))
		run(t, dir, "add", ".")
		run(t, dir, "commit", "-q", "-m", message)
		if i == 0 {
			run(t, dir, "switch", "-q", "-c", "feature")
		}
	}
	write(t, dir, "f.txt", "uncommitted")

	for _, c := range []struct {
		name string
		r    subject.Range
		want []string
	}{
		{"uncommitted", subject.Range{}, []string{}},
		{"branch with its uncommitted work", subject.Range{Base: "main"}, []string{"fix: b", "feat: add a\n\nWhy a."}},
		{"commit", subject.Range{Base: "HEAD~1", Head: "HEAD"}, []string{"fix: b"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, err := subject.Read(t.Context(), dir, c.r, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			got, err := subject.Commits(t.Context(), s)
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprintf("%q", got) != fmt.Sprintf("%q", c.want) {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}

	empty := t.TempDir()
	run(t, empty, "init", "-q")
	write(t, empty, "a.txt", "a")
	s, err := subject.Read(t.Context(), empty, subject.Range{}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got, err := subject.Commits(t.Context(), s); err != nil || len(got) != 0 {
		t.Fatalf("new repository: %q, %v", got, err)
	}
}

func TestHistory(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	write(t, dir, "f.txt", "one\ntwo\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-q", "-m", "feat: write one and two")
	write(t, dir, "f.txt", "one\nthree\n")
	write(t, dir, "new.txt", "fresh\n")
	s, err := subject.Read(t.Context(), dir, subject.Range{}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	history, err := subject.History(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## f.txt", "feat: write one and two", "Who last changed the lines replaced at @@ -1,2 +", "2) two"} {
		if !strings.Contains(history, want) {
			t.Errorf("history lacks %q:\n%s", want, history)
		}
	}
	if strings.Contains(history, "new.txt") {
		t.Errorf("a file the change adds has no history:\n%s", history)
	}
}
