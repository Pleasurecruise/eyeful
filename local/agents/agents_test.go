package agents_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Pleasurecruise/eyeful/local/agents"
	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/prompts"
)

type call struct {
	Args  []string `json:"args"`
	Stdin string   `json:"stdin"`
	Dir   string   `json:"dir"`
	Key   bool     `json:"key"`
}

const report = `{"findings":[]}`

var outputs = map[string]string{
	"claude": `{"type":"result","subtype":"success","is_error":false,"result":"done","structured_output":` + report + `,"usage":{"input_tokens":100,"output_tokens":20,"cache_read_input_tokens":5},"total_cost_usd":0.25,"modelUsage":{"claude-opus-5":{}}}`,
	"codex": `{"type":"thread.started","thread_id":"t"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"i0","type":"reasoning","text":"thinking"}}
{"type":"item.completed","item":{"id":"i1","type":"agent_message","text":"` + "```json\\n" + `{\"findings\":[]}` + "\\n```" + `"}}
{"type":"turn.completed","usage":{"input_tokens":300,"cached_input_tokens":100,"output_tokens":40}}`,
	"pi": `{"type":"session","version":3,"id":"s","cwd":"/x"}
{"type":"message_end","message":{"role":"user","content":"the prompt","timestamp":1}}
{"type":"message_end","message":{"role":"assistant","content":[{"type":"toolCall","id":"c"}],"provider":"anthropic","model":"claude-sonnet-5-5","usage":{"totalTokens":50,"cost":{"total":0.01}},"stopReason":"toolUse"}}
{"type":"message_end","message":{"role":"assistant","content":[{"type":"thinking","thinking":"hm"},{"type":"text","text":"` + "```json\\n" + `{\"findings\":[]}` + "\\n```" + `"}],"provider":"anthropic","model":"claude-sonnet-5-5","usage":{"totalTokens":70,"cost":{"total":0.02}},"stopReason":"stop"}}
{"type":"agent_end","messages":[]}`,
}

func fake() error {
	name := filepath.Base(os.Args[0])
	if slices.Contains(os.Args, "--version") {
		_, err := fmt.Println(name + " 1.2.3")
		return err
	}
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	_, key := os.LookupEnv("ANTHROPIC_API_KEY")
	data, err := json.Marshal(call{Args: os.Args[1:], Stdin: string(stdin), Dir: dir, Key: key})
	if err != nil {
		return err
	}
	f, err := os.OpenFile(os.Getenv("FAKE_OUT"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return errors.Join(err, f.Close())
	}
	if err := f.Close(); err != nil {
		return err
	}
	switch os.Getenv("FAKE_MODE") {
	case "bad":
		_, err = fmt.Println(`{"type":"result","subtype":"success","is_error":false,"result":"no json here","usage":{"input_tokens":1,"output_tokens":1}}`)
	case "fail":
		fmt.Fprintln(os.Stderr, "Invalid API key")
		os.Exit(1)
	default:
		_, err = fmt.Println(outputs[name])
	}
	return err
}

func TestMain(m *testing.M) {
	if os.Getenv("FAKE_CLI") == "1" {
		if err := fake(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

const skills = "/run/skills"

func setup(t *testing.T, name, mode string) (*agents.Agents, string, string) {
	t.Helper()
	bin, repo := t.TempDir(), t.TempDir()
	if err := os.Symlink(os.Args[0], filepath.Join(bin, name)); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "calls")
	t.Setenv("PATH", bin)
	t.Setenv("FAKE_CLI", "1")
	t.Setenv("FAKE_MODE", mode)
	t.Setenv("FAKE_OUT", out)
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	set, err := prompts.Load()
	if err != nil {
		t.Fatal(err)
	}
	p, _ := agents.Find(name)
	return agents.New(slog.New(slog.DiscardHandler), agents.Config{Provider: p, Dir: repo, Skills: skills, Set: set}), out, repo
}

func calls(t *testing.T, out string) []call {
	t.Helper()
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var cs []call
	for line := range strings.Lines(string(data)) {
		var c call
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			t.Fatal(err)
		}
		cs = append(cs, c)
	}
	return cs
}

var task = workflow.ReviewTask{
	Expert: workflow.Expert{Name: "security", Evidence: []workflow.EvidenceKind{workflow.EvidenceArgument}},
	Tier:   workflow.TierStrong, Group: workflow.Group{Files: []string{"a.go"}}, Skills: []string{"security-review"},
}

func TestProviders(t *testing.T) {
	for _, tt := range []struct {
		name   string
		args   []string
		usage  workflow.Usage
		apiKey bool
	}{
		{"claude", []string{"-p", "--json-schema", "--model opus", "--tools Read,Grep,Glob", "--no-session-persistence"}, workflow.Usage{Model: "claude-opus-5", Tokens: 125, MicroUSD: 250000}, false},
		{"codex", []string{"exec", "--json", "--sandbox read-only", "--ephemeral"}, workflow.Usage{Model: "codex", Tokens: 340}, true},
		{"pi", []string{"--mode json", "--no-session", "--tools read,grep,find,ls"}, workflow.Usage{Model: "anthropic/claude-sonnet-5-5", Tokens: 120, MicroUSD: 30000}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a, out, repo := setup(t, tt.name, "ok")
			r, usage, err := a.Review(t.Context(), task)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Findings) != 0 || usage != tt.usage {
				t.Fatalf("report %+v, usage %+v", r, usage)
			}
			c := calls(t, out)[0]
			joined := strings.Join(c.Args, " ")
			for _, want := range tt.args {
				if !strings.Contains(joined, want) {
					t.Errorf("args lack %q: %s", want, joined)
				}
			}
			if !strings.Contains(c.Stdin, "- `security-review`, `"+filepath.Join(skills, "security-review", "SKILL.md")+"`: ") || strings.Contains(c.Stdin, "name: security-review") || !strings.Contains(c.Stdin, "`submit_report`") || strings.Contains(joined, "security-review") {
				t.Errorf("the prompt did not go in on stdin: args %s", joined)
			}
			if resolved, _ := filepath.EvalSymlinks(repo); c.Dir != resolved {
				t.Errorf("ran in %q, want %q", c.Dir, resolved)
			}
			if c.Key != tt.apiKey {
				t.Errorf("ANTHROPIC_API_KEY passed: %v", c.Key)
			}
		})
	}
}

func TestBadReply(t *testing.T) {
	a, out, _ := setup(t, "claude", "bad")
	if _, usage, err := a.Review(t.Context(), task); !errors.Is(err, agents.ErrBadReply) || usage.Tokens != 4 {
		t.Fatalf("usage %+v, err %v", usage, err)
	}
	cs := calls(t, out)
	if len(cs) != 2 || !strings.Contains(cs[1].Stdin, "did not end with a JSON block") {
		t.Fatalf("calls %d, retry prompt %q", len(cs), cs[len(cs)-1].Stdin)
	}
}

func TestFailure(t *testing.T) {
	a, _, _ := setup(t, "codex", "fail")
	_, _, err := a.Review(t.Context(), task)
	if err == nil || !strings.Contains(err.Error(), "Invalid API key") || !strings.Contains(err.Error(), "codex login") {
		t.Fatalf("%v", err)
	}
}

func TestDetect(t *testing.T) {
	setup(t, "pi", "ok")
	p, _ := agents.Find("pi")
	if a, err := agents.Detect(t.Context(), p); err != nil || a.Version != "pi 1.2.3" {
		t.Fatalf("%+v, %v", a, err)
	}
	c, _ := agents.Find("claude")
	if _, err := agents.Detect(t.Context(), c); !errors.Is(err, agents.ErrNotInstalled) {
		t.Fatalf("%v", err)
	}
}
