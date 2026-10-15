package app

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Pleasurecruise/eyeful/local/agents"
	"github.com/Pleasurecruise/eyeful/local/archive"
	"github.com/Pleasurecruise/eyeful/local/classify"
	"github.com/Pleasurecruise/eyeful/local/git"
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
	snap, req, cfg, err := request(ctx, o, objects)
	if err != nil {
		return report.Output{}, err
	}
	if len(req.Change.Files) == 0 {
		return report.Output{}, say(o.Out, "Nothing to review: the change is empty.\n")
	}
	provider, agent, err := connected(ctx)
	if err != nil {
		return report.Output{}, err
	}
	wf, err := workflow.New(o.Log, set.Roster())
	if err != nil {
		return report.Output{}, err
	}
	if req.Previous, err = previous(ctx, o, snap); err != nil {
		return report.Output{}, err
	}
	if err := wf.Admit(req); err != nil {
		return report.Output{}, err
	}
	if err := say(o.Err, describe(o, agent, snap, len(req.Change.Files))); err != nil {
		return report.Output{}, err
	}
	ws, closeTree, err := workspace(ctx, o, snap, objects, cfg)
	if err != nil {
		return report.Output{}, err
	}
	defer closeTree()
	run, skills, patch, removeSkills, err := prepare(ctx, o, snap)
	if err != nil {
		return report.Output{}, err
	}
	defer removeSkills()
	runner := agents.New(o.Log, agents.Config{Provider: provider, Dir: snap.Root, Skills: skills, Patch: patch, History: filepath.Join(filepath.Dir(patch), historyFile), Trace: filepath.Join(run, "agents"), Set: set}, idleTimeout, hardTimeout)
	res, err := wf.Run(ctx, workflow.Session{Agents: runner, Classifier: classify.New(o.Log, o.TypesafeKey), Workspace: ws, Archive: archive.New(filepath.Join(run, "stages"))}, req)
	if err != nil {
		return report.Output{}, err
	}
	return save(ctx, o, run, snap, objects, res)
}

func previous(ctx context.Context, o Options, snap subject.Snapshot) (_ workflow.Result, err error) {
	var last workflow.Result
	if o.Run != "" || o.Verification != workflow.ModeExecution {
		return last, nil
	}
	dir, err := store.Dir(snap.Root)
	if err != nil {
		return last, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return last, fmt.Errorf("open earlier reviews: %w", err)
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	runs, err := os.ReadDir(dir)
	if err != nil {
		return last, fmt.Errorf("list earlier reviews: %w", err)
	}
	for _, run := range slices.Backward(runs) {
		var old subject.Snapshot
		kept, err := root.ReadFile(filepath.Join(run.Name(), "snapshot.json"))
		if err != nil || json.Unmarshal(kept, &old) != nil || old.Base != snap.Base || (old.Head == "") != (snap.Head == "") {
			continue
		}
		if snap.Head != "" {
			if _, err := git.Run(ctx, snap.Root, nil, "merge-base", "--is-ancestor", old.Head, snap.Head); err != nil {
				continue
			}
		}
		result, err := root.ReadFile(filepath.Join(run.Name(), "result.json"))
		if err != nil {
			continue
		}
		var res workflow.Result
		if err := json.Unmarshal(result, &res); err != nil {
			return last, fmt.Errorf("read the last review in %s: %w", run.Name(), err)
		}
		last.Findings, last.Verifications = res.Findings, res.Verifications
		return last, nil
	}
	return last, nil
}

func prepare(ctx context.Context, o Options, snap subject.Snapshot) (string, string, string, func(), error) {
	run, err := filepath.Abs(o.Run)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("run directory: %w", err)
	}
	if o.Run == "" {
		dir, err := store.Dir(snap.Root)
		if err != nil {
			return "", "", "", nil, err
		}
		run = filepath.Join(dir, time.Now().UTC().Format("20060102-150405"))
	}
	snapshot, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return "", "", "", nil, fmt.Errorf("encode snapshot: %w", err)
	}
	if err := errors.Join(os.MkdirAll(run, 0o700), os.WriteFile(filepath.Join(run, "snapshot.json"), snapshot, 0o600)); err != nil {
		return "", "", "", nil, fmt.Errorf("save snapshot.json: %w", err)
	}
	shared, err := os.MkdirTemp("", "eyeful-agent-")
	if err != nil {
		return "", "", "", nil, fmt.Errorf("agent files: %w", err)
	}
	dir := filepath.Join(shared, "skills")
	if err := prompts.WriteSkills(dir); err != nil {
		return "", "", "", nil, errors.Join(err, os.RemoveAll(shared))
	}
	patch := filepath.Join(shared, "change.diff")
	if err := errors.Join(os.WriteFile(patch, []byte(snap.Patch), 0o600), os.WriteFile(filepath.Join(run, "change.diff"), []byte(snap.Patch), 0o600)); err != nil {
		return "", "", "", nil, errors.Join(fmt.Errorf("save change.diff: %w", err), os.RemoveAll(shared))
	}
	history, err := subject.History(ctx, snap)
	if err == nil {
		err = os.WriteFile(filepath.Join(shared, historyFile), []byte(history), 0o600)
	}
	if err != nil {
		return "", "", "", nil, errors.Join(fmt.Errorf("save %s: %w", historyFile, err), os.RemoveAll(shared))
	}
	return run, dir, patch, func() {
		if err := os.RemoveAll(shared); err != nil {
			o.Log.Warn("remove the agent files", "dir", shared, "error", err)
		}
	}, nil
}

func request(ctx context.Context, o Options, objects string) (subject.Snapshot, workflow.Request, project.Config, error) {
	var req workflow.Request
	snap, err := subject.Read(ctx, o.Dir, o.Range, objects)
	if err != nil {
		return snap, req, project.Config{}, err
	}
	if snap.Head != "" {
		if err := subject.CheckedOut(ctx, snap.Root, snap.Head); err != nil {
			return snap, req, project.Config{}, err
		}
	}
	files, err := workflow.SplitPatch(snap.Patch)
	if err != nil {
		return snap, req, project.Config{}, err
	}
	commits, err := subject.Commits(ctx, snap)
	if err != nil {
		return snap, req, project.Config{}, err
	}
	cfg := o.Commands
	if o.Detect {
		found, err := project.Detect(snap.Root)
		if err != nil {
			return snap, req, cfg, err
		}
		cfg = project.Config{Setup: cmp.Or(cfg.Setup, found.Setup), Lint: cmp.Or(cfg.Lint, found.Lint), Test: cmp.Or(cfg.Test, found.Test), TestOne: cmp.Or(cfg.TestOne, found.TestOne)}
	}
	req = workflow.Request{
		Change: workflow.Change{Files: files, Commits: commits, Description: o.Description},
		Budget: workflow.Budget{Tokens: budget}, Verification: o.Verification, Experts: o.Experts,
	}
	if o.Verification == workflow.ModeExecution {
		req.Project.TestOne = cfg.TestOne
	}
	req.Project.Skip, err = marked(ctx, snap.Root, files)
	return snap, req, cfg, err
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
		return say(o.Err, "No test command was found for this project, so nothing runs and a judge reads every finding.\n")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Commands for this project, run in %s, a checkout of the snapshot that is reset after every run and removed at the end:\n", work)
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
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return report.Output{}, fmt.Errorf("run directory: %w", err)
	}
	saved := map[string][]byte{
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

func marked(ctx context.Context, root string, files []workflow.File) ([]string, error) {
	paths := []string{}
	if len(files) == 0 {
		return paths, nil
	}
	args := append(make([]string, 0, 5+len(files)), "check-attr", "-z", "linguist-generated", "linguist-vendored", "--")
	for _, f := range files {
		args = append(args, f.Path)
	}
	out, err := git.Run(ctx, root, nil, args...)
	if err != nil {
		return nil, err
	}
	fields := strings.Split(out, "\x00")
	for i := 0; i+2 < len(fields); i += 3 {
		if v := fields[i+2]; (v == "set" || v == "true") && !slices.Contains(paths, fields[i]) {
			paths = append(paths, fields[i])
		}
	}
	return paths, nil
}
