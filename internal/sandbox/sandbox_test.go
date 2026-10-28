package sandbox

import (
	"errors"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Pleasurecruise/eyeful/internal/review"
	"github.com/Pleasurecruise/eyeful/workflow"
)

func TestSandbox(t *testing.T) {
	remote := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = remote
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t", "GIT_CONFIG_GLOBAL=/dev/null")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(remote, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	write("go.mod", "module example.com/sum\n\ngo 1.22\n")
	write("sum.go", "package sum\n\nfunc Add(a, b int) int { return a + b }\n")
	write("sum_test.go", "package sum\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal(Add(1, 2))\n\t}\n}\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")
	base := run("rev-parse", "HEAD")
	run("checkout", "-q", "-b", "feature")
	write("README", "sum\n")
	run("add", ".")
	run("commit", "-q", "-m", "head")
	head := run("rev-parse", "HEAD")

	srv := httptest.NewServer(NewServer(slog.New(slog.DiscardHandler), filepath.Join(t.TempDir(), "sandbox"), time.Minute))
	defer srv.Close()
	c := NewClient(srv.URL)
	if err := c.Setup(t.Context()); err == nil || !strings.Contains(err.Error(), ErrNotOpen.Error()) {
		t.Fatalf("setup before open: %v", err)
	}
	src := review.Source{Repo: "acme/sum", Base: "main", Head: "feature", Number: 7, BaseSHA: base, HeadSHA: head, CloneURL: (&url.URL{Scheme: "file", Path: remote}).String()}
	for range 2 {
		cfg, err := c.Open(t.Context(), src)
		if err != nil || cfg.Test != "go test ./..." {
			t.Fatalf("open: %+v %v", cfg, err)
		}
	}
	if err := c.Setup(t.Context()); err != nil {
		t.Fatal(err)
	}
	if content, err := c.Read(t.Context(), "README"); err != nil || content != "sum\n" {
		t.Fatalf("read %q: %v", content, err)
	}
	if run, err := c.Test(t.Context(), nil, workflow.Target{}); err != nil || !run.Passed {
		t.Fatalf("suite %+v: %v", run, err)
	}
	broken := []workflow.Edit{{Path: "sum.go", Old: "a + b", New: "a - b"}}
	if run, err := c.Test(t.Context(), broken, workflow.Target{Tests: []string{"TestAdd"}}); err != nil || run.Passed || !strings.Contains(run.Output, "FAIL") {
		t.Fatalf("broken %+v: %v", run, err)
	}
	if content, err := c.Read(t.Context(), "sum.go"); err != nil || !strings.Contains(content, "a + b") {
		t.Fatalf("the edit outlived its test run: %q %v", content, err)
	}
	if _, err := c.Read(t.Context(), "../outside"); err == nil || errors.Is(err, ErrNotOpen) {
		t.Fatalf("read outside the repository: %v", err)
	}
}
