package checkout

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Pleasurecruise/eyeful/internal/review"
)

var (
	shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
	refPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
)

func Clone(ctx context.Context, dir, token string, src review.Source) error {
	if err := validate(src); err != nil {
		return err
	}
	if strings.HasPrefix(src.CloneURL, "file:") {
		token = ""
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("clear checkout: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("checkout directory: %w", err)
	}
	env, cleanup, err := environ(token)
	if err != nil {
		return err
	}
	defer cleanup()
	run := func(args ...string) (string, error) {
		return git(ctx, dir, token, env, args...)
	}
	if _, err := run("init", "--quiet"); err != nil {
		return err
	}
	if _, err := run("remote", "add", "origin", src.CloneURL); err != nil {
		return err
	}
	head := fmt.Sprintf("+refs/pull/%d/head:refs/remotes/origin/pr", src.Number)
	base := "+refs/heads/" + src.Base + ":refs/remotes/origin/base"
	if _, err := run("fetch", "--no-tags", "origin", head); err != nil {
		return err
	}
	if _, err := run("fetch", "--no-tags", "origin", base); err != nil {
		return err
	}
	if _, err := run("checkout", "--detach", "refs/remotes/origin/pr"); err != nil {
		return err
	}
	out, err := run("rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if got := strings.TrimSpace(out); got != src.HeadSHA {
		return fmt.Errorf("checkout is %.12s, want %.12s", got, src.HeadSHA)
	}
	return nil
}

func Run(ctx context.Context, dir string, args ...string) (string, error) {
	env, cleanup, err := environ("")
	if err != nil {
		return "", err
	}
	defer cleanup()
	return git(ctx, dir, "", env, args...)
}

func validate(src review.Source) error {
	if src.Number < 1 || !shaPattern.MatchString(src.HeadSHA) || !shaPattern.MatchString(src.BaseSHA) || !safeRef(src.Base) || !safeURL(src.CloneURL) {
		return fmt.Errorf("refused checkout of %s#%d", src.Repo, src.Number)
	}
	return nil
}

func safeRef(value string) bool {
	if value == "" || len(value) > 255 || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.HasSuffix(value, ".lock") {
		return false
	}
	if strings.Contains(value, "..") || strings.Contains(value, "//") || strings.Contains(value, "@{") {
		return false
	}
	return refPattern.MatchString(value)
}

func safeURL(value string) bool {
	u, err := url.Parse(value)
	if err != nil || u.User != nil {
		return false
	}
	switch u.Scheme {
	case "https":
		return u.Host != ""
	case "file":
		return u.Host == "" && strings.HasPrefix(u.Path, "/")
	default:
		return false
	}
}

func environ(token string) ([]string, func(), error) {
	home, err := os.MkdirTemp("", "eyeful-git-")
	if err != nil {
		return nil, nil, fmt.Errorf("git home: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(home) }
	config := filepath.Join(home, "gitconfig")
	if err := os.WriteFile(config, nil, 0o600); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("git config: %w", err)
	}
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		switch {
		case name == "GITHUB_TOKEN", name == "GH_TOKEN", name == "GH_ENTERPRISE_TOKEN":
			continue
		case strings.HasPrefix(name, "EYEFUL_"), strings.HasPrefix(name, "GIT_"):
			continue
		case name == "HOME", name == "XDG_CONFIG_HOME", name == "XDG_DATA_HOME":
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		"HOME="+home,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+config,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
		"LC_ALL=C",
		"GIT_PAGER=cat",
	)
	if token == "" {
		return env, cleanup, nil
	}
	path := filepath.Join(home, "askpass")
	script := "#!/bin/sh\nprintf '%s\\n' \"$EYEFUL_ASKPASS_TOKEN\"\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("git askpass: %w", err)
	}
	return append(env, "GIT_ASKPASS="+path, "EYEFUL_ASKPASS_TOKEN="+token), cleanup, nil
}

func git(ctx context.Context, dir, token string, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "credential.helper=", "-c", "protocol.file.allow=always", "-c", "safe.directory=*"}, args...)...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.WaitDelay = 5 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		detail := redact(strings.TrimSpace(stderr.String()), token)
		return "", fmt.Errorf("git %s: %w: %s", redact(strings.Join(args, " "), token), err, detail)
	}
	return stdout.String(), nil
}

func redact(text, secret string) string {
	if secret == "" {
		return text
	}
	return strings.ReplaceAll(text, secret, "[redacted]")
}
