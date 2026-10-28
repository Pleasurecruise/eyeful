package checkout

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Pleasurecruise/eyeful/internal/review"
)

type gitCall struct {
	Args           []string `json:"args"`
	GitHub         bool     `json:"github"`
	Secret         bool     `json:"secret"`
	Ask            bool     `json:"ask"`
	TokenInArgs    bool     `json:"token_in_args"`
	ScriptOK       bool     `json:"script_ok"`
	ScriptHasToken bool     `json:"script_has_token"`
}

func TestMain(m *testing.M) {
	if os.Getenv("FAKE_GIT") == "1" {
		os.Exit(fakeGit())
	}
	os.Exit(m.Run())
}

func fakeGit() int {
	token := os.Getenv("EYEFUL_ASKPASS_TOKEN")
	if os.Getenv("FAKE_FAIL") == "1" {
		_, _ = os.Stderr.WriteString("auth failed " + token + "\n")
		return 1
	}
	_, githubToken := os.LookupEnv("GITHUB_TOKEN")
	_, secret := os.LookupEnv("EYEFUL_MODEL_KEY")
	joined := strings.Join(os.Args, "\x00")
	script, _ := os.ReadFile(os.Getenv("GIT_ASKPASS"))
	data, err := json.Marshal(gitCall{
		Args: os.Args[1:], GitHub: githubToken, Secret: secret,
		Ask: os.Getenv("GIT_ASKPASS") != "", TokenInArgs: token != "" && strings.Contains(joined, token),
		ScriptOK: strings.Contains(string(script), `printf '%s\n' "$EYEFUL_ASKPASS_TOKEN"`), ScriptHasToken: token != "" && strings.Contains(string(script), token),
	})
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error())
		return 2
	}
	f, err := os.OpenFile(os.Getenv("FAKE_OUT"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error())
		return 2
	}
	_, err = f.Write(append(data, '\n'))
	if cerr := f.Close(); err != nil || cerr != nil {
		return 2
	}
	if strings.Contains(strings.Join(os.Args, " "), "rev-parse") {
		_, _ = os.Stdout.WriteString(os.Getenv("FAKE_HEAD") + "\n")
	}
	return 0
}

func TestClone(t *testing.T) {
	src, _ := repo(t)
	dir := t.TempDir()
	if err := Clone(t.Context(), dir, "", src); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(command(t, dir, "rev-parse", "HEAD")); got != src.HeadSHA {
		t.Fatalf("HEAD %s", got)
	}
	if got := strings.TrimSpace(command(t, dir, "rev-parse", "refs/remotes/origin/base")); got != src.BaseSHA {
		t.Fatalf("base %s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestCloneRefusesCredentials(t *testing.T) {
	src := review.Source{
		Repo: "acme/eyeful", Number: 1, Base: "main",
		BaseSHA: strings.Repeat("a", 40), HeadSHA: strings.Repeat("b", 40),
		CloneURL: "https://user:ghs-secret@github.com/acme/eyeful.git",
	}
	err := Clone(t.Context(), t.TempDir(), "ghs-secret", src)
	if err == nil || strings.Contains(err.Error(), "ghs-secret") {
		t.Fatalf("err %v", err)
	}
}

func TestCloneAskpass(t *testing.T) {
	bin := t.TempDir()
	if err := os.Symlink(os.Args[0], filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "calls")
	const token = "ghs-test-token"
	t.Setenv("PATH", bin)
	t.Setenv("FAKE_GIT", "1")
	t.Setenv("FAKE_OUT", out)
	t.Setenv("FAKE_HEAD", strings.Repeat("b", 40))
	t.Setenv("GITHUB_TOKEN", "ghs-parent")
	t.Setenv("EYEFUL_MODEL_KEY", "model-key")
	src := review.Source{
		Repo: "acme/eyeful", Number: 4, Base: "main",
		BaseSHA: strings.Repeat("a", 40), HeadSHA: strings.Repeat("b", 40),
		CloneURL: "https://github.com/acme/eyeful.git",
	}
	if err := Clone(t.Context(), t.TempDir(), token, src); err != nil {
		t.Fatal(err)
	}
	calls := readCalls(t, out)
	if len(calls) < 4 || calls[0].GitHub || calls[0].Secret || calls[0].TokenInArgs || !calls[0].Ask || !calls[0].ScriptOK || calls[0].ScriptHasToken {
		t.Fatalf("calls %+v", calls)
	}
	if strings.Contains(strings.Join(calls[0].Args, " "), token) {
		t.Fatal("token in args")
	}
}

func TestCloneRedactsToken(t *testing.T) {
	bin := t.TempDir()
	if err := os.Symlink(os.Args[0], filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	const token = "ghs-leak-token"
	t.Setenv("PATH", bin)
	t.Setenv("FAKE_GIT", "1")
	t.Setenv("FAKE_FAIL", "1")
	t.Setenv("FAKE_OUT", filepath.Join(t.TempDir(), "calls"))
	src := review.Source{
		Repo: "acme/eyeful", Number: 4, Base: "main",
		BaseSHA: strings.Repeat("a", 40), HeadSHA: strings.Repeat("b", 40),
		CloneURL: "https://github.com/acme/eyeful.git",
	}
	err := Clone(t.Context(), t.TempDir(), token, src)
	if err == nil || strings.Contains(err.Error(), token) || !strings.Contains(err.Error(), "[redacted]") {
		t.Fatal("token leaked from git")
	}
}

func readCalls(t *testing.T, path string) []gitCall {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var calls []gitCall
	for line := range strings.Lines(string(data)) {
		var c gitCall
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			t.Fatal(err)
		}
		calls = append(calls, c)
	}
	return calls
}

func repo(t *testing.T) (review.Source, string) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		return command(t, dir, args...)
	}
	run("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("head\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", "keep.txt")
	run("commit", "-m", "base")
	base := run("rev-parse", "HEAD")
	run("checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", "keep.txt")
	run("commit", "-m", "head")
	head := run("rev-parse", "HEAD")
	run("update-ref", "refs/pull/7/head", head)
	return review.Source{
		Repo: "acme/eyeful", Base: "main", Head: "feature", Number: 7,
		BaseSHA: base, HeadSHA: head, CloneURL: "file://" + dir,
	}, dir
}

func command(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=eyeful", "GIT_AUTHOR_EMAIL=eyeful@localhost",
		"GIT_COMMITTER_NAME=eyeful", "GIT_COMMITTER_EMAIL=eyeful@localhost",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
