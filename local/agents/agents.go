package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/Pleasurecruise/eyeful/local/tail"
	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/prompts"
)

func New(log *slog.Logger, cfg Config) *Agents {
	return &Agents{cfg: cfg, log: log.With(slog.String("component", "agents"), slog.String("agent", cfg.Provider.Name))}
}

func Find(name string) (Provider, bool) {
	i := slices.IndexFunc(Providers, func(p Provider) bool { return p.Name == name })
	if i < 0 {
		return Provider{}, false
	}
	return Providers[i], true
}

func Detect(ctx context.Context, p Provider) (Agent, error) {
	path, err := exec.LookPath(p.Tool)
	if err != nil {
		return Agent{}, fmt.Errorf("%w: %s is not on PATH; install it and %s", ErrNotInstalled, p.Tool, p.Login)
	}
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return Agent{}, fmt.Errorf("%s --version: %w", p.Tool, err)
	}
	return Agent{Name: p.Name, Version: strings.TrimSpace(string(out))}, nil
}

const replyNotice = `

## Reply

eyeful gives you no tools of its own, so ignore the tool names in the instructions above:

- The diffs you need are in the input. Read any other file with your read-only tools.
- Nothing runs a reproduction for you. eyeful's verifier runs the one you hand in.
- Your final message is your result. Where the instructions say to call ` + "`%s`" + `, end the message with
  one fenced ` + "```json" + ` block that holds that tool's arguments, and write nothing after it. It must
  match this JSON Schema:

` + "```json" + `
%s
` + "```" + `
`

func decode[T reply](text string) (T, error) {
	var v T
	body := strings.TrimSpace(text)
	if i := strings.LastIndex(body, "```json"); i >= 0 {
		body = body[i+len("```json"):]
		body, _, _ = strings.Cut(body, "```")
	} else if i, j := strings.Index(body, "{"), strings.LastIndex(body, "}"); i >= 0 && j > i {
		body = body[i : j+1]
	}
	dec := json.NewDecoder(strings.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, fmt.Errorf("decode reply: %w", err)
	}
	return v, nil
}

func ask[T reply](ctx context.Context, a *Agents, role workflow.Role, tier workflow.Tier, prompt string) (T, workflow.Usage, error) {
	return run[T](ctx, a, string(role), a.cfg.Set.Roles[role].Submit, tier, prompt)
}

func run[T reply](ctx context.Context, a *Agents, name, submit string, tier workflow.Tier, prompt string) (T, workflow.Usage, error) {
	var zero T
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		return zero, workflow.Usage{}, fmt.Errorf("schema for %s: %w", name, err)
	}
	data, err := json.Marshal(schema)
	if err != nil {
		return zero, workflow.Usage{}, fmt.Errorf("schema for %s: %w", name, err)
	}
	p := a.cfg.Provider
	prompt += fmt.Sprintf(replyNotice, submit, data)
	env := slices.DeleteFunc(os.Environ(), func(kv string) bool {
		return slices.ContainsFunc(p.strip, func(name string) bool { return strings.HasPrefix(kv, name+"=") })
	})
	var usage workflow.Usage
	var bad error
	for range 2 {
		cmd := exec.CommandContext(ctx, p.Tool, p.args(data, tier)...)
		cmd.Dir, cmd.Env, cmd.WaitDelay = a.cfg.Dir, env, 5*time.Second
		var stdout bytes.Buffer
		stderr := tail.New(stderrLimit)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = strings.NewReader(prompt), &stdout, stderr
		start := time.Now()
		runErr := cmd.Run()
		a.log.Info("agent run", slog.String("role", name), slog.Int("seconds", int(time.Since(start).Seconds())), slog.Any("error", runErr))
		ans, err := p.parse(stdout.Bytes())
		usage.Model = ans.usage.Model
		usage.Tokens += ans.usage.Tokens
		usage.MicroUSD += ans.usage.MicroUSD
		if err = errors.Join(err, runErr); err != nil {
			return zero, usage, fmt.Errorf("%s: %w: %s (if it is not signed in, %s)", p.Tool, err, tail.Last(strings.TrimSpace(stderr.String()), stderrLimit), p.Login)
		}
		v, err := decode[T](ans.text)
		if err == nil {
			return v, usage, nil
		}
		bad = err
		prompt += fmt.Sprintf("\n\nAn earlier reply to these instructions did not end with a JSON block that matches the schema: %v.\n", err)
	}
	return zero, usage, fmt.Errorf("%w: %w", ErrBadReply, bad)
}

func (a *Agents) Diagnose(ctx context.Context, t workflow.DiagnoseTask) (workflow.Diagnosis, workflow.Usage, error) {
	prompt, err := a.cfg.Set.Diagnose(t)
	if err != nil {
		return workflow.Diagnosis{}, workflow.Usage{}, err
	}
	return ask[workflow.Diagnosis](ctx, a, workflow.RoleCI, "", prompt)
}

func (a *Agents) Plan(ctx context.Context, t workflow.PlanTask) (workflow.Plan, workflow.Usage, error) {
	prompt, err := a.cfg.Set.Plan(t)
	if err != nil {
		return workflow.Plan{}, workflow.Usage{}, err
	}
	return ask[workflow.Plan](ctx, a, workflow.RolePlanner, "", prompt)
}

func (a *Agents) Review(ctx context.Context, t workflow.ReviewTask) (workflow.Report, workflow.Usage, error) {
	prompt, err := a.cfg.Set.Review(t, a.cfg.Skills)
	if err != nil {
		return workflow.Report{}, workflow.Usage{}, err
	}
	return ask[workflow.Report](ctx, a, workflow.RoleExpert, t.Tier, prompt)
}

func (a *Agents) Fix(ctx context.Context, t workflow.FixTask) ([]workflow.Edit, workflow.Usage, error) {
	prompt, err := a.cfg.Set.Fix(t)
	if err != nil {
		return nil, workflow.Usage{}, err
	}
	r, usage, err := ask[fixReply](ctx, a, workflow.RoleFix, "", prompt)
	return r.Edits, usage, err
}

func (a *Agents) Judge(ctx context.Context, t workflow.JudgeTask) (workflow.Judgement, workflow.Usage, error) {
	prompt, err := a.cfg.Set.Judge(t)
	if err != nil {
		return workflow.Judgement{}, workflow.Usage{}, err
	}
	return ask[workflow.Judgement](ctx, a, workflow.RoleJudge, "", prompt)
}

func (a *Agents) Summarize(ctx context.Context, t workflow.SummaryTask) ([]workflow.Draft, workflow.Usage, error) {
	prompt, err := a.cfg.Set.Summarize(t)
	if err != nil {
		return nil, workflow.Usage{}, err
	}
	r, usage, err := ask[summaryReply](ctx, a, workflow.RoleSummarizer, "", prompt)
	return r.Comments, usage, err
}

func (a *Agents) CommitMessage(ctx context.Context, patch string) (string, workflow.Usage, error) {
	skill, err := prompts.ReadSkill(commitSkill, "")
	if err != nil {
		return "", workflow.Usage{}, err
	}
	r, usage, err := run[commitReply](ctx, a, commitSkill, "submit_commit", workflow.TierCheap, skill+"\n\n## Diff\n\n"+patch)
	return r.Message, usage, err
}
