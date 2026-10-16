package harness

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pleasurecruise/eyeful/internal/checkout"
	"github.com/Pleasurecruise/eyeful/internal/github"
	"github.com/Pleasurecruise/eyeful/internal/review"
	"github.com/Pleasurecruise/eyeful/local/agents"
	"github.com/Pleasurecruise/eyeful/local/archive"
	"github.com/Pleasurecruise/eyeful/local/classify"
	"github.com/Pleasurecruise/eyeful/local/subject"
	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/project"
	"github.com/Pleasurecruise/eyeful/workflow/prompts"
	"github.com/Pleasurecruise/eyeful/workflow/report"
)

const (
	budgetTokens = 3_000_000
	idleTimeout  = 2 * time.Minute
	hardTimeout  = 20 * time.Minute
)

var errNoSandbox = errors.New("the sandbox is not built, so project commands do not run")

type Host interface {
	Token(ctx context.Context, installation int64) (string, error)
	Wait(ctx context.Context, token string, src review.Source) (github.Actions, error)
	Description(ctx context.Context, token string, src review.Source) (string, error)
	Publish(ctx context.Context, token string, src review.Source, markdown string, notes []github.Note) error
}

type Runner struct {
	Log         *slog.Logger
	DataDir     string
	Agent       string
	TypesafeKey string
	Experts     []string
	Host        Host
	Clone       func(ctx context.Context, dir, token string, src review.Source) error
	Agents      workflow.Agents
}

func (r Runner) Execute(ctx context.Context, rev review.Review) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir, err := r.directory(rev.ID)
	if err != nil {
		return err
	}
	marker := filepath.Join(dir, "published")
	switch _, err := os.Stat(marker); {
	case err == nil:
		return nil
	case !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("publication record: %w", err)
	}
	res, err := r.review(ctx, dir, rev)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	markdown, err := report.Markdown(res)
	if err != nil {
		return fmt.Errorf("report: %w", err)
	}
	token, err := r.Host.Token(ctx, rev.Source.InstallationID)
	if err != nil {
		return fmt.Errorf("installation token: %w", err)
	}
	if err := r.Host.Publish(ctx, token, rev.Source, markdown, notesFrom(res.Comments)); err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	if err := os.WriteFile(marker, []byte(res.Outcome+"\n"), 0o600); err != nil {
		return fmt.Errorf("record publication: %w", err)
	}
	r.Log.Info("published", slog.String("review", rev.ID), slog.String("repo", rev.Source.Repo), slog.String("outcome", string(res.Outcome)))
	return nil
}

func (r Runner) review(ctx context.Context, dir string, rev review.Review) (workflow.Result, error) {
	var none workflow.Result
	set, err := prompts.Load()
	if err != nil {
		return none, fmt.Errorf("prompts: %w", err)
	}
	if err := r.prepare(ctx, dir); err != nil {
		return none, err
	}
	token, err := r.Host.Token(ctx, rev.Source.InstallationID)
	if err != nil {
		return none, fmt.Errorf("installation token: %w", err)
	}
	checkoutDir := filepath.Join(dir, "checkout")
	baseDir := filepath.Join(dir, "base")
	defer r.remove(checkoutDir, baseDir)
	if err := r.Clone(ctx, checkoutDir, token, rev.Source); err != nil {
		return none, fmt.Errorf("checkout: %w", err)
	}
	actions, err := r.Host.Wait(ctx, token, rev.Source)
	if err != nil {
		return none, fmt.Errorf("actions: %w", err)
	}
	description, err := r.Host.Description(ctx, token, rev.Source)
	if err != nil {
		return none, fmt.Errorf("pull request: %w", err)
	}
	req, root, err := r.request(ctx, dir, checkoutDir, baseDir, rev.Source, description)
	if err != nil {
		return none, err
	}
	runner := r.Agents
	if runner == nil {
		home := filepath.Join(dir, "home")
		if err := os.MkdirAll(home, 0o700); err != nil {
			return none, fmt.Errorf("agent home: %w", err)
		}
		provider, ok := agents.Find(r.Agent)
		if !ok {
			return none, fmt.Errorf("agent %q is not pi, claude or codex", r.Agent)
		}
		runner = agents.New(r.Log, agents.Config{
			Provider: provider, Dir: root, Home: home, Drop: agentDrop,
			Skills: filepath.Join(dir, "skills"), Patch: filepath.Join(dir, "change.diff"),
			History: filepath.Join(dir, "history.md"), Trace: filepath.Join(dir, "agents"), Set: set,
		}, idleTimeout, hardTimeout)
	}
	wf, err := workflow.New(r.Log, set.Roster())
	if err != nil {
		return none, fmt.Errorf("workflow: %w", err)
	}
	res, err := wf.Run(ctx, workflow.Session{
		Agents: runner, Classifier: classify.New(r.Log, r.TypesafeKey),
		Workspace: workspace{check: workflow.Check{Passed: actions.Passed, Log: actions.Log}},
		Archive:   archive.New(filepath.Join(dir, "stages")),
	}, req)
	if err != nil {
		return none, fmt.Errorf("review: %w", err)
	}
	return res, nil
}

func (r Runner) prepare(ctx context.Context, dir string) error {
	if r.Agents != nil {
		return nil
	}
	name := r.Agent
	provider, ok := agents.Find(name)
	if !ok {
		return fmt.Errorf("agent %q is not pi, claude or codex", name)
	}
	if _, err := agents.Detect(ctx, provider); err != nil {
		return fmt.Errorf("agent: %w", err)
	}
	skills := filepath.Join(dir, "skills")
	if err := os.RemoveAll(skills); err != nil {
		return fmt.Errorf("skills: %w", err)
	}
	if err := prompts.WriteSkills(skills); err != nil {
		return fmt.Errorf("skills: %w", err)
	}
	return nil
}

func (r Runner) request(ctx context.Context, dir, checkoutDir, baseDir string, src review.Source, description string) (workflow.Request, string, error) {
	var req workflow.Request
	objects, err := os.MkdirTemp("", "eyeful-snapshot-")
	if err != nil {
		return req, "", fmt.Errorf("snapshot objects: %w", err)
	}
	defer func() { _ = os.RemoveAll(objects) }()
	snap, err := subject.Read(ctx, checkoutDir, subject.Range{Base: src.BaseSHA, Head: src.HeadSHA}, objects)
	if err != nil {
		return req, "", fmt.Errorf("snapshot: %w", err)
	}
	if snap.Head != "" {
		if err := subject.CheckedOut(ctx, snap.Root, snap.Head); err != nil {
			return req, "", fmt.Errorf("checked out: %w", err)
		}
	}
	files, err := workflow.SplitPatch(snap.Patch)
	if err != nil {
		return req, "", fmt.Errorf("diff: %w", err)
	}
	commits, err := subject.Commits(ctx, snap)
	if err != nil {
		return req, "", fmt.Errorf("commits: %w", err)
	}
	if _, err := checkout.Run(ctx, snap.Root, "worktree", "add", "--detach", baseDir, src.BaseSHA); err != nil {
		return req, "", fmt.Errorf("base checkout: %w", err)
	}
	cfg, err := project.Detect(baseDir)
	if err != nil {
		return req, "", fmt.Errorf("project commands: %w", err)
	}
	skip, err := marked(ctx, snap.Root, files)
	if err != nil {
		return req, "", fmt.Errorf("linguist: %w", err)
	}
	history, err := subject.History(ctx, snap)
	if err != nil {
		return req, "", fmt.Errorf("history: %w", err)
	}
	if err := errors.Join(
		os.WriteFile(filepath.Join(dir, "change.diff"), []byte(snap.Patch), 0o600),
		os.WriteFile(filepath.Join(dir, "history.md"), []byte(history), 0o600),
	); err != nil {
		return req, "", fmt.Errorf("save the change: %w", err)
	}
	req = workflow.Request{
		Change:       workflow.Change{Files: files, Commits: commits, Description: description},
		Project:      workflow.Project{Skip: skip, TestOne: cfg.TestOne},
		Budget:       workflow.Budget{Tokens: budgetTokens},
		Verification: workflow.ModeExecution,
		Experts:      r.Experts,
	}
	return req, snap.Root, nil
}

func (r Runner) directory(id string) (string, error) {
	if id == "" || id != filepath.Base(id) || strings.Contains(id, "..") {
		return "", fmt.Errorf("review id %q", id)
	}
	root := r.DataDir
	if root == "" {
		root = filepath.Join(os.TempDir(), "eyeful-reviews")
	}
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("review directory: %w", err)
	}
	return dir, nil
}

func (r Runner) remove(paths ...string) {
	for _, path := range paths {
		if err := os.RemoveAll(path); err != nil {
			r.Log.Warn("remove checkout", slog.String("path", path), slog.Any("error", err))
		}
	}
}

var agentDrop = []string{
	"GITHUB_TOKEN", "GH_TOKEN", "GH_ENTERPRISE_TOKEN",
	"EYEFUL_D1_API_TOKEN", "EYEFUL_WEBHOOK_SECRET", "EYEFUL_GITHUB_APP_PRIVATE_KEY",
	"EYEFUL_TYPESAFE_API_KEY", "EYEFUL_GITHUB_APP_ID",
}

type workspace struct {
	check workflow.Check
}

func (w workspace) CI(context.Context) (workflow.Check, error) { return w.check, nil }

func (w workspace) Setup(context.Context) error { return errNoSandbox }

func (w workspace) Read(context.Context, string) (string, error) {
	return "", errNoSandbox
}

func (w workspace) Test(context.Context, []workflow.Edit, workflow.Target) (workflow.TestRun, error) {
	return workflow.TestRun{}, errNoSandbox
}

func notesFrom(comments []workflow.Comment) []github.Note {
	notes := []github.Note{}
	for _, c := range comments {
		if c.Path == "" || c.Line < 1 {
			continue
		}
		notes = append(notes, github.Note{Path: c.Path, Line: c.Line, Body: noteBody(c)})
	}
	return notes
}

func noteBody(c workflow.Comment) string {
	var b strings.Builder
	b.WriteString(string(c.Label))
	if len(c.Decorations) > 0 {
		b.WriteString(" (" + strings.Join(c.Decorations, ", ") + ")")
	}
	b.WriteString(": " + c.Subject)
	if c.Discussion != "" {
		b.WriteString("\n\n" + c.Discussion)
	}
	if c.Scenario != "" {
		b.WriteString("\n\n" + c.Scenario)
	}
	if c.Objection != "" {
		b.WriteString("\n\n" + c.Objection)
	}
	if len(c.Fix) == 1 && c.Fix[0].Path == c.Path && c.Fix[0].New != "" {
		b.WriteString("\n\n```suggestion\n" + c.Fix[0].New)
		if !strings.HasSuffix(c.Fix[0].New, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n")
	}
	return b.String()
}

func marked(ctx context.Context, root string, files []workflow.File) ([]string, error) {
	paths := []string{}
	if len(files) == 0 {
		return paths, nil
	}
	args := make([]string, 0, 5+len(files))
	args = append(args, "check-attr", "-z", "linguist-generated", "linguist-vendored", "--")
	for _, f := range files {
		args = append(args, f.Path)
	}
	out, err := checkout.Run(ctx, root, args...)
	if err != nil {
		return nil, err
	}
	fields := strings.Split(out, "\x00")
	seen := map[string]bool{}
	for i := 0; i+2 < len(fields); i += 3 {
		if v := fields[i+2]; (v == "set" || v == "true") && !seen[fields[i]] {
			seen[fields[i]] = true
			paths = append(paths, fields[i])
		}
	}
	return paths, nil
}
