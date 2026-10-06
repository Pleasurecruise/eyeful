package workflow

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"

	"github.com/bmatcuk/doublestar/v4"
)

const minConfidence = 0.5

func (s ciState) spent() []Run      { return s.Runs }
func (s prepareState) spent() []Run { return s.Runs }
func (s planState) spent() []Run    { return s.Runs }
func (s expertState) spent() []Run  { return s.Runs }
func (s verifyState) spent() []Run  { return s.Runs }
func (baselineState) spent() []Run  { return nil }
func (s summaryState) spent() []Run { return s.Runs }

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
	return &Workflow{roster: slices.Clone(roster), log: log.With(slog.String("component", "workflow"))}, nil
}

func (w *Workflow) Admit(req Request) error {
	if req.Level != "" && !slices.Contains(Levels, req.Level) {
		return fmt.Errorf("%w: level %q", ErrInvalidRequest, req.Level)
	}
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
	st := prepareState{Triage: triage(req.Change, req.Project), Tools: []ToolResult{}, Skipped: []string{}, Runs: []Run{}}
	if err := s.Workspace.Setup(ctx); err != nil {
		st.SetupError = err.Error()
		st.Skipped = append(st.Skipped, "setup failed, so nothing was verified: "+st.SetupError)
	}
	paths := make([]string, 0, len(req.Change.Files))
	for _, f := range req.Change.Files {
		paths = append(paths, f.Path)
	}
	tools, err := s.Workspace.Tools(ctx, paths)
	if err != nil {
		st.Skipped = append(st.Skipped, "the L1 tools failed: "+err.Error())
	} else {
		st.Tools = tools
	}
	return st, nil
}

func (w *Workflow) askPlanner(ctx context.Context, s Session, l *ledger, req Request, t Triage, files map[string]File, tools []ToolResult, st *planState) error {
	reviewed := make([]string, 0, len(t.Reviewed))
	for _, e := range t.Reviewed {
		reviewed = append(reviewed, e.Path)
	}
	shown, size := []File{}, 0
	for _, f := range pick(files, reviewed) {
		if size += len(f.Diff); size > planDiffLimit {
			break
		}
		shown = append(shown, f)
	}
	task := PlanTask{
		Files: shown, Truncated: len(shown) < len(reviewed), Manifest: slices.Concat(t.Reviewed, t.Excluded), Commits: req.Change.Commits, Tools: tools,
		Experts: make([]string, 0, len(w.roster)), MaxExperts: levels[st.Level].experts, Rejected: []string{},
	}
	for _, e := range w.roster {
		task.Experts = append(task.Experts, e.Name)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if !l.allow() {
			st.Notes = append(st.Notes, "budget exhausted before the planner ran")
			break
		}
		plan, r, err := invoke(ctx, l, Run{Role: RolePlanner}, s.Agents.Plan, task)
		if err != nil {
			return err
		}
		st.Runs = append(st.Runs, r)
		reasons := []string{"the planner failed: " + r.Error}
		if r.Error == "" {
			reasons = w.check(plan, t, st.Level)
		}
		if len(reasons) == 0 {
			st.Plan, st.Source = plan, PlanFromPlanner
			break
		}
		st.Notes = append(st.Notes, fmt.Sprintf("plan %d rejected: %v", attempt, reasons))
		task.Rejected = reasons
	}
	return nil
}

func (w *Workflow) plan(ctx context.Context, s Session, l *ledger, req Request, level Level, t Triage, files map[string]File, tools []ToolResult) (planState, error) {
	st := planState{Level: level, Notes: []string{}, Runs: []Run{}}
	switch {
	case len(req.Experts) > 0:
		picks := make([]Pick, 0, len(req.Experts))
		for _, name := range req.Experts {
			picks = append(picks, Pick{Name: name, Why: "requested"})
		}
		st.Plan, st.Source = whole(t, "request", picks), PlanFromRequest
		return st, nil
	case !levels[level].planner:
		st.Plan, st.Source = w.addSignals(w.routePlan(tools), tools), PlanFromRules
		return st, nil
	case len(t.Reviewed) == 0:
		st.Plan, st.Source = w.addSignals(w.defaultPlan(t, level), tools), PlanFromRules
		return st, nil
	}
	if err := w.askPlanner(ctx, s, l, req, t, files, tools, &st); err != nil {
		return st, err
	}
	if st.Source == "" {
		st.Plan, st.Source = w.defaultPlan(t, level), PlanFromDefault
		st.Notes = append(st.Notes, fmt.Sprintf("the default experts for %s reviewed the change", level))
	}
	if up := levels[level].escalate; st.Source == PlanFromPlanner && st.Plan.Confidence < minConfidence && up != level {
		st.Level = up
		st.Notes = append(st.Notes, fmt.Sprintf("planner confidence %.2f is below %.2f; level raised to %s", st.Plan.Confidence, minConfidence, up))
	}
	st.Plan = w.addSignals(st.Plan, tools)
	return st, nil
}

func (w *Workflow) passes(ctx context.Context, s Session, l *ledger, req Request, prep prepareState, files map[string]File, res *Result) (bool, error) {
	res.Scopes = scopes(slices.SortedFunc(slices.Values(prep.Triage.Reviewed), func(a, b Entry) int { return cmp.Compare(a.Path, b.Path) }), 0)
	res.Level = chooseLevel(req.Level, prep.Triage)
	res.Plan = Plan{Groups: []Group{}, Skipped: []Pick{}, Confidence: 1}
	var unscoped []string
	for _, r := range prep.Tools {
		for _, h := range r.Hits {
			if !slices.ContainsFunc(res.Scopes, func(sc Scope) bool { return slices.Contains(sc.Files, h.Path) }) {
				unscoped = append(unscoped, h.Path)
			}
		}
	}
	level, short := res.Level, false
	for si, sc := range res.Scopes {
		sl := l.share(len(res.Scopes))
		tools := hitsFor(prep.Tools, sc.Files)
		if si == 0 {
			tools = hitsFor(prep.Tools, slices.Concat(sc.Files, unscoped))
		}
		ps, err := checkpoint(ctx, s.Archive, sl, fmt.Sprintf("plan/%d", si), func() (planState, error) {
			scoped := Triage{
				Reviewed: slices.DeleteFunc(slices.Clone(prep.Triage.Reviewed), func(e Entry) bool { return !slices.Contains(sc.Files, e.Path) }),
				Excluded: prep.Triage.Excluded,
				Risk:     slices.DeleteFunc(slices.Clone(prep.Triage.Risk), func(p string) bool { return !slices.Contains(sc.Files, p) }),
			}
			return w.plan(ctx, s, sl, req, level, scoped, files, tools)
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
		res.Plan.Confidence = min(res.Plan.Confidence, ps.Plan.Confidence)
		if slices.Index(Levels, ps.Level) > slices.Index(Levels, res.Level) {
			res.Level = ps.Level
		}
		if si == 0 || ps.Source != PlanFromPlanner {
			res.PlanSource = ps.Source
		}
		res.Uncertainty.Notes = append(res.Uncertainty.Notes, ps.Notes...)
		w.log.Info("plan", slog.String("scope", sc.Name), slog.String("level", string(ps.Level)), slog.String("source", string(ps.Source)), slog.Int("groups", len(ps.Plan.Groups)))

		findings, u, err := w.experts(ctx, s, sl, res.Plan, from, ps.Level, files, tools, req.Project.TestOne)
		if err != nil {
			return false, err
		}
		res.Findings = append(res.Findings, findings...)
		res.Uncertainty.Absent = append(res.Uncertainty.Absent, u.Absent...)
		res.Uncertainty.Unread = append(res.Uncertainty.Unread, u.Unread...)
		if sl.exhausted && !l.exhausted {
			short = true
			res.Uncertainty.Notes = append(res.Uncertainty.Notes, fmt.Sprintf("scope %s used up its share of the budget", sc.Name))
		}
	}
	return short, nil
}

func (w *Workflow) Run(ctx context.Context, s Session, req Request) (Result, error) {
	res := Result{
		Diagnosis: Diagnosis{Failures: []Failure{}}, Tools: []ToolResult{}, Findings: []Finding{}, Verifications: []Verification{},
		Comments: []Comment{}, Runs: []Run{},
		Uncertainty: Uncertainty{Absent: []Absence{}, Unread: []Unread{}, Dismissed: []Finding{}, Unverified: []Unverified{}, Skipped: []string{}, Notes: []string{}},
	}
	if err := w.Admit(req); err != nil {
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
	res.Triage, res.Tools = prep.Triage, prep.Tools
	res.Uncertainty.Skipped = append(res.Uncertainty.Skipped, prep.Skipped...)

	files := map[string]File{}
	for _, f := range req.Change.Files {
		files[f.Path] = f
	}
	// TODO(workflow): re-reviews that pass the previous round's findings to each expert.
	short, err := w.passes(ctx, s, l, req, prep, files, &res)
	if err != nil {
		return res, err
	}
	findings := res.Findings

	verdicts, err := w.verify(ctx, s, l, req, res.Level, prep.SetupError, findings, files)
	if err != nil {
		return res, err
	}
	res.Verifications = verdicts
	byFinding := res.Uncertainty.record(findings, verdicts)
	if req.Verification == ModeNone {
		res.Uncertainty.Skipped = append(res.Uncertainty.Skipped, "verification was off for this review")
	}

	merged := merge(findings, byFinding)
	sum, err := checkpoint(ctx, s.Archive, l, "summary", func() (summaryState, error) { return w.summarize(ctx, s, l, merged, byFinding) })
	if err != nil {
		return res, err
	}
	res.Comments = sum.Comments
	if sum.Fallback != "" {
		res.Uncertainty.Notes = append(res.Uncertainty.Notes, "comments were formatted by rules: "+sum.Fallback)
	}

	res.Runs = l.runs
	res.Uncertainty.BudgetExhausted = l.exhausted || short
	res.Outcome = OutcomeComplete
	if res.Uncertainty.BudgetExhausted {
		res.Outcome, res.Reason = OutcomePartial, "the budget ran out"
	}
	w.log.Info("done", slog.String("outcome", string(res.Outcome)), slog.Int("comments", len(res.Comments)))
	return res, nil
}

func (u *Uncertainty) record(findings []Finding, verdicts []Verification) map[string]Verification {
	byFinding := map[string]Verification{}
	for _, v := range verdicts {
		byFinding[v.Finding] = v
		if v.Status == StatusUnverified {
			u.Unverified = append(u.Unverified, Unverified{Finding: v.Finding, Reason: v.Reason})
		}
	}
	for _, f := range findings {
		if st := byFinding[f.ID].Status; st == StatusNotReproduced || st == StatusRejected {
			u.Dismissed = append(u.Dismissed, f)
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

func hitsFor(tools []ToolResult, paths []string) []ToolResult {
	out := []ToolResult{}
	for _, r := range tools {
		hits := []Hit{}
		for _, h := range r.Hits {
			if slices.Contains(paths, h.Path) {
				hits = append(hits, h)
			}
		}
		if len(hits) > 0 {
			out = append(out, ToolResult{Tool: r.Tool, Hits: hits})
		}
	}
	return out
}
