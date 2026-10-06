package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pleasurecruise/eyeful/local/agents"
	"github.com/Pleasurecruise/eyeful/local/archive"
	"github.com/Pleasurecruise/eyeful/local/settings"
	"github.com/Pleasurecruise/eyeful/local/store"
	"github.com/Pleasurecruise/eyeful/local/subject"
	"github.com/Pleasurecruise/eyeful/local/worktree"
	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/project"
	"github.com/Pleasurecruise/eyeful/workflow/prompts"
	"github.com/Pleasurecruise/eyeful/workflow/report"
)

func say(w io.Writer, text string) error {
	if _, err := io.WriteString(w, text); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func Experts() ([]workflow.Expert, error) {
	set, err := prompts.Load()
	if err != nil {
		return nil, err
	}
	return set.Roster(), nil
}

func Review(ctx context.Context, o Options) (report.Output, error) {
	// TODO(app): resume an interrupted review from its run directory.
	// TODO(app): warn when the branch under review belongs to someone else.
	set, err := prompts.Load()
	if err != nil {
		return report.Output{}, err
	}
	objects, err := os.MkdirTemp("", "eyeful-snapshot-")
	if err != nil {
		return report.Output{}, fmt.Errorf("snapshot objects: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(objects); err != nil {
			o.Log.Warn("remove the snapshot", "dir", objects, "error", err)
		}
	}()
	snap, change, err := read(ctx, o, objects)
	if err != nil {
		return report.Output{}, err
	}
	if len(change.Files) == 0 {
		return report.Output{}, say(o.Out, "Nothing to review: the change is empty.\n")
	}
	dir, err := store.Dir(snap.Root)
	if err != nil {
		return report.Output{}, err
	}
	cfg, _, err := project.Read(snap.Root)
	if err != nil {
		return report.Output{}, err
	}
	provider, agent, err := connected(ctx)
	if err != nil {
		return report.Output{}, err
	}
	wf, err := workflow.New(o.Log, set.Roster())
	if err != nil {
		return report.Output{}, err
	}
	req := workflow.Request{
		Change: change, Project: cfg.Project(o.Verification == workflow.ModeExecution),
		Budget: workflow.Budget{Tokens: budget}, Verification: o.Verification, Experts: o.Experts,
	}
	if err := wf.Admit(req); err != nil {
		return report.Output{}, err
	}
	if err := say(o.Err, describe(o, agent, snap, len(change.Files))); err != nil {
		return report.Output{}, err
	}
	ws, closeTree, err := workspace(ctx, o, snap, objects, cfg)
	if err != nil {
		return report.Output{}, err
	}
	defer closeTree()
	run := filepath.Join(dir, time.Now().UTC().Format("20060102-150405"))
	skills, removeSkills, err := writeSkills(o, run)
	if err != nil {
		return report.Output{}, err
	}
	defer removeSkills()
	runner := agents.New(o.Log, agents.Config{Provider: provider, Dir: snap.Root, Skills: skills, Set: set})
	res, err := wf.Run(ctx, workflow.Session{Agents: runner, Workspace: ws, Archive: archive.New(filepath.Join(run, "stages"))}, req)
	if err != nil {
		return report.Output{}, err
	}
	return save(ctx, o, run, snap, objects, res)
}

func writeSkills(o Options, run string) (string, func(), error) {
	dir := filepath.Join(run, "skills")
	if err := prompts.WriteSkills(dir); err != nil {
		return "", nil, err
	}
	return dir, func() {
		if err := os.RemoveAll(dir); err != nil {
			o.Log.Warn("remove the skills", "dir", dir, "error", err)
		}
	}, nil
}

func read(ctx context.Context, o Options, objects string) (subject.Snapshot, workflow.Change, error) {
	snap, err := subject.Read(ctx, o.Dir, o.Range, objects)
	if err != nil {
		return snap, workflow.Change{}, err
	}
	if snap.Head != "" {
		if err := subject.CheckedOut(ctx, snap.Root, snap.Head); err != nil {
			return snap, workflow.Change{}, err
		}
	}
	files, err := workflow.SplitPatch(snap.Patch)
	if err != nil {
		return snap, workflow.Change{}, err
	}
	commits, err := subject.Commits(ctx, snap)
	return snap, workflow.Change{Files: files, Commits: commits}, err
}

func describe(o Options, agent agents.Agent, snap subject.Snapshot, files int) string {
	what := "uncommitted changes"
	if snap.Head != "" {
		what = fmt.Sprintf("%.12s against %.12s", snap.Head, snap.Base)
	}
	experts := "chosen by the planner"
	if len(o.Experts) > 0 {
		experts = strings.Join(o.Experts, ", ")
	}
	return fmt.Sprintf("Reviewing %s in %s: %d files, snapshot %.12s.\nExperts: %s. Agent: %s.\n", what, snap.Root, files, snap.Commit, experts, strings.TrimSpace(agent.Name+" "+agent.Version))
}

func workspace(ctx context.Context, o Options, snap subject.Snapshot, objects string, cfg project.Config) (workflow.Workspace, func(), error) {
	if o.Verification != workflow.ModeExecution {
		return worktree.ReadOnly{}, func() {}, say(o.Err, "Verification: "+string(o.Verification)+"; no project command runs and your working copy is not touched.\n")
	}
	tree, err := worktree.Open(ctx, o.Log, snap, objects)
	if err != nil {
		return nil, nil, err
	}
	closeTree := func() {
		if err := tree.Close(context.WithoutCancel(ctx)); err != nil {
			o.Log.Warn("remove the work directory", "dir", tree.Dir(), "error", err)
		}
	}
	if err := confirm(ctx, o, snap.Root, cfg, tree.Dir()); err != nil {
		closeTree()
		return nil, nil, err
	}
	return worktree.NewWorkspace(tree, cfg, commandTimeout), closeTree, nil
}

func confirm(ctx context.Context, o Options, root string, cfg project.Config, work string) error {
	cmds := cfg.Commands()
	if len(cmds) == 0 {
		return say(o.Err, "No commands in "+project.Path+", so nothing runs and findings stay unverified.\n")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Commands from %s, run in %s, a checkout of the snapshot that is reset after every run and removed at the end:\n", project.Path, work)
	for _, c := range cmds {
		fmt.Fprintf(&b, "  %-9s %s\n", c.Name, c.Run)
	}
	if err := say(o.Err, b.String()); err != nil {
		return err
	}
	path, err := settings.Path()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	ok, err := store.Confirmed(dir, root, cfg.Digest())
	if err != nil || ok {
		return err
	}
	yes, err := o.Confirm(ctx, work, cmds)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNotConfirmed, err)
	}
	if !yes {
		return ErrNotConfirmed
	}
	return store.Confirm(dir, root, cfg.Digest())
}

func save(ctx context.Context, o Options, dir string, snap subject.Snapshot, objects string, res workflow.Result) (report.Output, error) {
	md, err := report.Markdown(res)
	if err != nil {
		return report.Output{}, err
	}
	out, err := report.Findings(res)
	if err != nil {
		return report.Output{}, err
	}
	findings, err := report.JSON(res)
	if err != nil {
		return report.Output{}, err
	}
	sarif, err := report.SARIF(res)
	if err != nil {
		return report.Output{}, err
	}
	result, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return report.Output{}, fmt.Errorf("encode result: %w", err)
	}
	snapshot, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return report.Output{}, fmt.Errorf("encode snapshot: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return report.Output{}, fmt.Errorf("run directory: %w", err)
	}
	saved := map[string][]byte{
		"snapshot.json": snapshot, "change.diff": []byte(snap.Patch),
		"review.md": []byte(md), "findings.json": findings, "results.sarif": sarif, "result.json": result,
	}
	for name, content := range saved {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			return report.Output{}, fmt.Errorf("save %s: %w", name, err)
		}
	}
	if err := say(o.Out, md); err != nil {
		return report.Output{}, err
	}
	changed, err := subject.Changed(ctx, snap, o.Range, objects)
	if err != nil {
		return report.Output{}, err
	}
	note := "\nSaved review.md, findings.json, results.sarif and result.json in " + dir + "\n"
	if changed {
		note += fmt.Sprintf("The repository changed while the review ran, so the agents may have read newer code. The findings are about snapshot %.12s; the diff is in change.diff.\n", snap.Commit)
	}
	return out, say(o.Err, note)
}
