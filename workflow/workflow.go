package workflow

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/Pleasurecruise/eyeful/workflow/pulls"
)

func (s ciState) spent() []Run      { return s.Runs }
func (s prepareState) spent() []Run { return s.Runs }
func (s planState) spent() []Run    { return s.Runs }
func (s expertState) spent() []Run  { return s.Runs }
func (s verifyState) spent() []Run  { return s.Runs }
func (s judgeState) spent() []Run   { return s.Runs }
func (suiteState) spent() []Run     { return nil }

func checkpoint[T stored](ctx context.Context, a Archive, l *ledger, key string, fn func() (T, error)) (T, error) {
	var st T
	data, ok, err := a.Load(ctx, key)
	if err != nil {
		return st, fmt.Errorf("load %s: %w", key, err)
	}
	if ok {
		if err := json.Unmarshal(data, &st); err != nil {
			return st, fmt.Errorf("decode %s: %w", key, err)
		}
		for _, r := range st.spent() {
			l.spend(r)
		}
		return st, nil
	}
	if st, err = fn(); err != nil {
		return st, err
	}
	if err := ctx.Err(); err != nil {
		return st, fmt.Errorf("%s: %w", key, err)
	}
	if data, err = json.Marshal(st); err != nil {
		return st, fmt.Errorf("encode %s: %w", key, err)
	}
	if err := a.Save(ctx, key, data); err != nil {
		return st, fmt.Errorf("save %s: %w", key, err)
	}
	return st, nil
}

func (w *Workflow) admit(ctx context.Context, a Archive, req Request) error {
	if err := w.Admit(req); err != nil {
		return err
	}
	req.Budget, req.Verification, req.Project.TestOne, req.Previous = Budget{}, "", "", Result{}
	want, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	kept, ok, err := a.Load(ctx, "request")
	switch {
	case err != nil:
		return fmt.Errorf("load request: %w", err)
	case ok && !bytes.Equal(kept, want):
		return ErrOtherRequest
	case ok:
		return nil
	}
	if err := a.Save(ctx, "request", want); err != nil {
		return fmt.Errorf("save request: %w", err)
	}
	return nil
}

func (l *ledger) allow() bool {
	if l.parent != nil && !l.parent.allow() {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if (l.budget.Tokens > 0 && l.spent.Tokens >= l.budget.Tokens) || (l.budget.MicroUSD > 0 && l.spent.MicroUSD >= l.budget.MicroUSD) {
		l.exhausted = true
	}
	return !l.exhausted
}

func (l *ledger) spend(r Run) {
	if l.parent != nil {
		l.parent.spend(r)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.spent.Tokens += r.Usage.Tokens
	l.spent.MicroUSD += r.Usage.MicroUSD
	if l.parent == nil {
		l.runs = append(l.runs, r)
	}
}

func invoke[T reply, K task](ctx context.Context, l *ledger, r Run, fn func(context.Context, K) (T, Usage, error), t K) (T, Run, error) {
	v, usage, err := fn(ctx, t)
	if ctx.Err() != nil {
		return v, r, fmt.Errorf("%s %s: %w", r.Role, r.Expert, ctx.Err())
	}
	r.Usage = usage
	if err != nil {
		r.Error = err.Error()
	}
	l.spend(r)
	return v, r, nil
}

func (l *ledger) share(n int) *ledger {
	return &ledger{budget: Budget{Tokens: l.budget.Tokens / int64(n), MicroUSD: l.budget.MicroUSD / int64(n)}, parent: l}
}

func New(log *slog.Logger, roster []Expert) (*Workflow, error) {
	seen := map[string]bool{}
	for _, e := range roster {
		switch {
		case e.Name == "" || seen[e.Name]:
			return nil, fmt.Errorf("%w: expert name %q is empty or repeated", ErrInvalidRoster, e.Name)
		case !slices.Contains([]Tier{TierCheap, TierMid, TierStrong}, e.Tier):
			return nil, fmt.Errorf("%w: %s has tier %q", ErrInvalidRoster, e.Name, e.Tier)
		case slices.ContainsFunc(e.Skills, func(s Skill) bool {
			return s.Name == "" || s.Name == SkillOther || slices.ContainsFunc(s.Files, func(g string) bool { return !doublestar.ValidatePattern(g) })
		}):
			return nil, fmt.Errorf("%w: %s has a skill without a name, named %q, or with an invalid glob", ErrInvalidRoster, e.Name, SkillOther)
		case len(e.Evidence) == 0:
			return nil, fmt.Errorf("%w: %s produces no evidence kind", ErrInvalidRoster, e.Name)
		}
		seen[e.Name] = true
	}
	core, err := pulls.Load()
	if err != nil {
		return nil, err
	}
	return &Workflow{roster: slices.Clone(roster), core: core, log: log.With(slog.String("component", "workflow"))}, nil
}

func (w *Workflow) Admit(req Request) error {
	if !slices.Contains([]Mode{ModeExecution, ModeReadOnly, ModeNone}, req.Verification) {
		return fmt.Errorf("%w: verification %q", ErrInvalidRequest, req.Verification)
	}
	if req.Budget.Tokens < 0 || req.Budget.MicroUSD < 0 || req.Budget.Tokens+req.Budget.MicroUSD == 0 {
		return fmt.Errorf("%w: the budget needs a limit in tokens, money or both", ErrInvalidRequest)
	}
	for _, g := range slices.Concat(req.Project.Skip, req.Project.Risk) {
		if !doublestar.ValidatePattern(g) {
			return fmt.Errorf("%w: glob %q", ErrInvalidRequest, g)
		}
	}
	picked := map[string]bool{}
	for _, name := range req.Experts {
		if _, ok := w.expert(name); !ok || picked[name] {
			return fmt.Errorf("%w: expert %q is not in the roster or is repeated", ErrInvalidRequest, name)
		}
		picked[name] = true
	}
	seen := map[string]bool{}
	for _, f := range req.Change.Files {
		if f.Path == "" || seen[f.Path] {
			return fmt.Errorf("%w: file path %q is empty or repeated", ErrInvalidRequest, f.Path)
		}
		seen[f.Path] = true
	}
	return nil
}

func (w *Workflow) ci(ctx context.Context, s Session, l *ledger) (ciState, error) {
	st := ciState{Diagnosis: Diagnosis{Failures: []Failure{}}, Runs: []Run{}}
	c, err := s.Workspace.CI(ctx)
	if err != nil {
		return st, fmt.Errorf("ci check: %w", err)
	}
	st.Check = c
	if c.Passed || !l.allow() {
		return st, nil
	}
	d, r, err := invoke(ctx, l, Run{Role: RoleCI}, s.Agents.Diagnose, DiagnoseTask{Log: c.Log})
	if err != nil {
		return st, err
	}
	st.Runs = append(st.Runs, r)
	if r.Error == "" {
		st.Diagnosis = d
	}
	return st, nil
}

func (w *Workflow) prepare(ctx context.Context, s Session, req Request) (prepareState, error) {
	st := prepareState{Triage: triage(req.Change, req.Project), Skipped: []string{}, Runs: []Run{}}
	if err := s.Workspace.Setup(ctx); err != nil {
		st.SetupError = err.Error()
		st.Skipped = append(st.Skipped, "setup failed, so nothing was verified: "+st.SetupError)
	}
	return st, nil
}

func (w *Workflow) askPlanner(ctx context.Context, s Session, l *ledger, req Request, t Triage, files map[string]File, st *planState) error {
	diff := pulls.DiffsPayload{Ref: pulls.SourceRef{Kind: "local"}, Title: "Changes under review", Commits: []pulls.Commit{}}
	for _, m := range req.Change.Commits {
		diff.Commits = append(diff.Commits, pulls.Commit{Message: m})
	}
	if first := cmp.Or(strings.TrimSpace(req.Change.Description), strings.Join(req.Change.Commits, "\n")); first != "" {
		diff.Title, _, _ = strings.Cut(first, "\n")
	}
	diff.Description = req.Change.Description
	var patch strings.Builder
	for _, e := range t.Reviewed {
		patch.WriteString(files[e.Path].Diff)
	}
	var err error
	if diff.Files, err = w.core.ParsePatch(patch.String()); err != nil {
		return err
	}
	task := PlanTask{Diff: diff, Rejected: []string{}}
	for attempt := 1; attempt <= 2; attempt++ {
		if !l.allow() {
			st.Notes = append(st.Notes, "budget exhausted before the planner ran")
			break
		}
		answer, r, err := invoke(ctx, l, Run{Role: RolePlanner}, s.Agents.Plan, task)
		if err != nil {
			return err
		}
		st.Runs = append(st.Runs, r)
		problem := "the planner failed: " + r.Error
		if r.Error == "" {
			if answer, problem, err = w.core.Check(diff, answer); err != nil {
				return err
			}
		}
		if problem == "" {
			st.Plan, st.Source = w.staff(answer, t), PlanFromPlanner
			break
		}
		st.Notes = append(st.Notes, fmt.Sprintf("grouping %d rejected: %s", attempt, problem))
		task.Rejected = []string{problem}
	}
	return nil
}

func (w *Workflow) plan(ctx context.Context, s Session, l *ledger, req Request, t Triage, files map[string]File) (planState, error) {
	st := planState{Plan: Plan{Groups: []Group{}, Skipped: []Pick{}, Confidence: 1}, Notes: []string{}, Runs: []Run{}}
	switch {
	case len(t.Reviewed) == 0:
		return st, nil
	case len(req.Experts) > 0:
		picks := make([]Pick, 0, len(req.Experts))
		for _, name := range req.Experts {
			picks = append(picks, Pick{Name: name, Why: "requested"})
		}
		st.Plan, st.Source = whole(t, "request", picks), PlanFromRequest
		return st, nil
	}
	if err := w.askPlanner(ctx, s, l, req, t, files, &st); err != nil {
		return st, err
	}
	if st.Source == "" {
		picks := []Pick{}
		for _, e := range w.roster {
			if !e.Solo && len(picks) < fallbackPanel {
				picks = append(picks, Pick{Name: e.Name, Why: "default expert"})
			}
		}
		st.Plan, st.Source = whole(t, "other", picks), PlanFromDefault
		st.Notes = append(st.Notes, "the default experts reviewed the change")
	}
	return st, nil
}

func (r Request) executes(setupErr string) bool {
	return r.Verification == ModeExecution && setupErr == ""
}

func (w *Workflow) tests(ctx context.Context, s Session, l *ledger, req Request, setupErr string, files map[string]File, res *Result) (suiteState, error) {
	if !req.executes(setupErr) {
		return suiteState{}, nil
	}
	st, err := checkpoint(ctx, s.Archive, l, "suite", func() (suiteState, error) {
		var st suiteState
		target := Target{Touched: slices.Sorted(maps.Keys(files))}
		var err error
		if st.Change, err = s.Workspace.Test(ctx, nil, target); err == nil && !st.Change.Passed {
			target.Base = true
			st.Base, err = s.Workspace.Test(ctx, nil, target)
		}
		if err != nil {
			st.Error = err.Error()
		}
		return st, nil
	})
	if err != nil {
		return st, err
	}
	broken := slices.DeleteFunc(slices.Clone(st.Change.Failed), func(t string) bool { return slices.Contains(st.Base.Failed, t) })
	switch {
	case st.Error != "":
		res.Uncertainty.Skipped = append(res.Uncertainty.Skipped, "the project's tests could not run before the review, so they gave the experts no lead and no fix is verified: "+st.Error)
	case len(broken) > 0:
		res.Regression = &TestRun{Failed: broken, Output: st.Change.Output}
	}
	return st, nil
}

func (w *Workflow) rerun(ctx context.Context, s Session, id string, repro *Repro) verifyState {
	v := Verification{Finding: id, Status: StatusUnverified, Repro: repro}
	seen, _, err := trial(ctx, s, repro)
	v.Change = seen
	switch {
	case err != nil:
		v.Reason = "the reproduction of the last review could not run on this change: " + err.Error()
	case len(seen) < reproRuns || seen[0] == TestError:
		v.Reason = fmt.Sprintf("the reproduction of the last review neither passes nor fails with %q every time on this change: %v", repro.Fails, seen)
	case seen[0] == TestPassed:
		v.Status, v.Reason = StatusResolved, "the reproduction failed in the last review and passes on this change"
	default:
		v.Status, v.Reason = StatusReproduced, "the reproduction of the last review still fails on this change"
		if before, err := s.Workspace.Test(ctx, repro.edits(), Target{Tests: []string{repro.Run}, Base: true}); err == nil && repro.status(before) == TestFailed {
			v.Status, v.Base, v.Reason = StatusPreExisting, TestFailed, "the reproduction of the last review now fails the same way on the base commit"
		}
	}
	return verifyState{Verification: v, Runs: []Run{}}
}

func (w *Workflow) recheck(ctx context.Context, s Session, l *ledger, req Request, setupErr string) ([]Finding, []Verification, []Claim, error) {
	var findings []Finding
	var verdicts []Verification
	var open []Claim
	if !req.executes(setupErr) || req.Project.TestOne == "" {
		return findings, verdicts, open, nil
	}
	for _, old := range req.Previous.Verifications {
		i := slices.IndexFunc(req.Previous.Findings, func(f Finding) bool { return f.ID == old.Finding })
		if i < 0 || old.Repro == nil || !slices.Contains([]Status{StatusReproduced, StatusFixed}, old.Status) || checkRepro(*old.Repro) != "" || severityRank[req.Previous.Findings[i].Severity] == 0 {
			continue
		}
		f := req.Previous.Findings[i]
		f.ID, f.Group = "previous/"+strings.TrimPrefix(f.ID, "previous/"), 0
		st, err := checkpoint(ctx, s.Archive, l, "recheck/"+f.ID, func() (verifyState, error) { return w.rerun(ctx, s, f.ID, old.Repro), nil })
		if err != nil {
			return nil, nil, nil, err
		}
		findings, verdicts = append(findings, f), append(verdicts, st.Verification)
		if st.Verification.Status == StatusReproduced {
			open = append(open, f.Claim)
		}
	}
	return findings, verdicts, open, nil
}

func (w *Workflow) passes(ctx context.Context, s Session, l *ledger, req Request, prep prepareState, files map[string]File, known []Claim, res *Result) (bool, error) {
	res.Scopes = scopes(slices.SortedFunc(slices.Values(prep.Triage.Reviewed), func(a, b Entry) int { return cmp.Compare(a.Path, b.Path) }), 0)
	res.Plan = Plan{Groups: []Group{}, Skipped: []Pick{}, Confidence: 1}
	short := false
	panel := &ledger{budget: Budget{Tokens: l.budget.Tokens * expertPercent / 100, MicroUSD: l.budget.MicroUSD * expertPercent / 100}, parent: l}
	for si, sc := range res.Scopes {
		sl := panel.share(len(res.Scopes))
		ps, err := checkpoint(ctx, s.Archive, sl, fmt.Sprintf("plan/%d", si), func() (planState, error) {
			scoped := Triage{
				Reviewed: slices.DeleteFunc(slices.Clone(prep.Triage.Reviewed), func(e Entry) bool { return !slices.Contains(sc.Files, e.Path) }),
				Excluded: prep.Triage.Excluded,
				Risk:     slices.DeleteFunc(slices.Clone(prep.Triage.Risk), func(p string) bool { return !slices.Contains(sc.Files, p) }),
			}
			return w.plan(ctx, s, sl, req, scoped, files)
		})
		if err != nil {
			return false, err
		}
		from := len(res.Plan.Groups)
		for _, g := range ps.Plan.Groups {
			g.Scope = si
			res.Plan.Groups = append(res.Plan.Groups, g)
		}
		res.Plan.Skipped = append(res.Plan.Skipped, ps.Plan.Skipped...)
		res.Plan.Summary = strings.TrimSpace(res.Plan.Summary + "\n\n" + ps.Plan.Summary)
		res.Plan.Confidence = min(res.Plan.Confidence, ps.Plan.Confidence)
		if si == 0 || ps.Source != PlanFromPlanner {
			res.PlanSource = ps.Source
		}
		res.Uncertainty.Notes = append(res.Uncertainty.Notes, ps.Notes...)
		w.log.Info("plan", slog.String("scope", sc.Name), slog.String("source", string(ps.Source)), slog.Int("groups", len(ps.Plan.Groups)))

		findings, u, err := w.experts(ctx, s, sl, res.Plan, from, files, res.Regression, known, req)
		if err != nil {
			return false, err
		}
		res.Findings = append(res.Findings, findings...)
		res.Uncertainty.Absent = append(res.Uncertainty.Absent, u.Absent...)
		res.Uncertainty.Unread = append(res.Uncertainty.Unread, u.Unread...)
		res.Uncertainty.Notes = append(res.Uncertainty.Notes, u.Notes...)
		if sl.exhausted && !l.exhausted {
			short = true
			res.Uncertainty.Notes = append(res.Uncertainty.Notes, fmt.Sprintf("scope %s used up its share of the budget", sc.Name))
		}
	}
	return short, nil
}

func (w *Workflow) Run(ctx context.Context, s Session, req Request) (Result, error) {
	res := Result{
		Diagnosis: Diagnosis{Failures: []Failure{}}, Findings: []Finding{}, Verifications: []Verification{},
		Comments: []Comment{}, Runs: []Run{},
		Uncertainty: Uncertainty{Absent: []Absence{}, Unread: []Unread{}, Unverified: []Unverified{}, Skipped: []string{}, Notes: []string{}},
	}
	if err := w.admit(ctx, s.Archive, req); err != nil {
		return res, err
	}
	if len(req.Change.Files) == 0 {
		res.Outcome, res.Reason = OutcomeNone, "the change has no files"
		return res, nil
	}
	l := &ledger{budget: req.Budget}

	ci, err := checkpoint(ctx, s.Archive, l, "ci", func() (ciState, error) { return w.ci(ctx, s, l) })
	if err != nil {
		return res, err
	}
	if !ci.Check.Passed {
		res.Outcome, res.Reason, res.Diagnosis, res.Runs = OutcomeCIFailed, "the CI check failed", ci.Diagnosis, l.runs
		return res, nil
	}

	prep, err := checkpoint(ctx, s.Archive, l, "prepare", func() (prepareState, error) { return w.prepare(ctx, s, req) })
	if err != nil {
		return res, err
	}
	res.Triage = prep.Triage
	res.Uncertainty.Skipped = append(res.Uncertainty.Skipped, prep.Skipped...)

	files := map[string]File{}
	for _, f := range req.Change.Files {
		files[f.Path] = f
	}
	suite, err := w.tests(ctx, s, l, req, prep.SetupError, files, &res)
	if err != nil {
		return res, err
	}
	// TODO(workflow): hand the last round's findings that no test showed to the judge again.
	carried, rechecked, known, err := w.recheck(ctx, s, l, req, prep.SetupError)
	if err != nil {
		return res, err
	}
	short, err := w.passes(ctx, s, l, req, prep, files, known, &res)
	if err != nil {
		return res, err
	}

	verdicts, err := w.verify(ctx, s, l, req, req.executes(prep.SetupError), suite, res.Findings, files)
	if err != nil {
		return res, err
	}
	res.Findings, res.Verifications = append(res.Findings, carried...), append(append(res.Verifications, verdicts...), rechecked...)
	byFinding := res.Uncertainty.record(res.Verifications)
	if req.Verification == ModeNone {
		res.Uncertainty.Skipped = append(res.Uncertainty.Skipped, "verification was off for this review")
	}

	merged := merge(res.Findings, byFinding)
	res.Comments = w.comments(merged, byFinding)

	res.Runs = l.runs
	res.Uncertainty.BudgetExhausted = l.exhausted || short
	res.Outcome = OutcomeComplete
	if reason := res.Uncertainty.shortfall(); reason != "" {
		res.Outcome, res.Reason = OutcomePartial, reason
	}
	w.log.Info("done", slog.String("outcome", string(res.Outcome)), slog.Int("comments", len(res.Comments)))
	return res, nil
}

func (u *Uncertainty) shortfall() string {
	switch {
	case u.BudgetExhausted:
		return "the budget ran out"
	case len(u.Absent) > 0:
		return fmt.Sprintf("%d of the experts picked did not hand in a report", len(u.Absent))
	}
	return ""
}

func (u *Uncertainty) record(verdicts []Verification) map[string]Verification {
	byFinding := map[string]Verification{}
	for _, v := range verdicts {
		byFinding[v.Finding] = v
		if v.Status == StatusUnverified {
			u.Unverified = append(u.Unverified, Unverified{Finding: v.Finding, Reason: v.Reason})
		}
	}
	return byFinding
}

func pick(files map[string]File, paths []string) []File {
	out := make([]File, 0, len(paths))
	for _, p := range paths {
		if f, ok := files[p]; ok {
			out = append(out, f)
		}
	}
	return out
}
