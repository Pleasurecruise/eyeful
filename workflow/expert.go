package workflow

import (
	"context"
	"fmt"
	"slices"
	"sync"
)

// TODO(workflow): accept cve findings backed by cve_lookup once expert tools run through eyeful.
var backing = map[EvidenceKind]Tool{
	EvidenceCVE:      ToolDependencyAudit,
	EvidenceAPIBreak: ToolOpenAPIDiff,
	EvidenceCoverage: ToolCoverageDiff,
}

func Validate(t ReviewTask, r Report) []string {
	reasons := make([]string, 0, len(r.Findings))
	for i, f := range r.Findings {
		reasons = append(reasons, checkFinding(t, f, i)...)
	}
	return reasons
}

func skillsFor(e Expert, files []string) []string {
	out := []string{}
	for _, s := range e.Skills {
		if len(s.Files) == 0 || slices.ContainsFunc(files, func(f string) bool { return matchAny(s.Files, f) }) {
			out = append(out, s.Name)
		}
	}
	return out
}

func checkFinding(t ReviewTask, f Claim, i int) []string {
	g := t.Group
	var reasons []string
	bad := func(msg string) { reasons = append(reasons, fmt.Sprintf("finding %d: %s", i, msg)) }
	switch {
	case f.Skill == SkillOther && f.Category == "":
		bad(fmt.Sprintf("an %q finding needs a category", SkillOther))
	case f.Skill != SkillOther && !slices.Contains(t.Skills, f.Skill):
		bad(fmt.Sprintf("cites %q, which is neither a skill offered to you nor %q", f.Skill, SkillOther))
	}
	if !slices.Contains(g.Files, f.Path) {
		bad(fmt.Sprintf("%q is not a file of this group", f.Path))
	}
	if f.Line < 1 {
		bad("needs a line")
	}
	if f.Subject == "" {
		bad("needs a subject")
	}
	if !slices.Contains([]Severity{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow}, f.Severity) {
		bad(fmt.Sprintf("severity %q is not critical, high, medium or low", f.Severity))
	}
	for _, msg := range checkEvidence(t, f) {
		bad(msg)
	}
	return reasons
}

func checkEvidence(t ReviewTask, f Claim) []string {
	var reasons []string
	if !slices.Contains(t.Expert.Evidence, f.Evidence) {
		reasons = append(reasons, fmt.Sprintf("evidence %q is not one %s produces", f.Evidence, t.Expert.Name))
	}
	if tool, ok := backing[f.Evidence]; ok && !slices.ContainsFunc(t.Tools, func(r ToolResult) bool {
		return r.Tool == tool && slices.ContainsFunc(r.Hits, func(h Hit) bool { return h.Path == f.Path })
	}) {
		reasons = append(reasons, fmt.Sprintf("evidence %q needs a %s hit on %q", f.Evidence, tool, f.Path))
	}
	if f.Scenario == "" && (f.Evidence == EvidenceReproTest || f.Evidence == EvidenceTriggerPath) {
		reasons = append(reasons, "needs a scenario: the input or sequence that triggers it and what goes wrong")
	}
	if msg := checkRepro(f); msg != "" {
		reasons = append(reasons, msg)
	}
	return reasons
}

func checkRepro(f Claim) string {
	r := f.Repro
	switch {
	case f.Evidence != EvidenceReproTest && r != nil:
		return fmt.Sprintf("a reproduction needs evidence %q", EvidenceReproTest)
	case f.Evidence != EvidenceReproTest:
		return ""
	case r == nil || r.Run == "" || r.Test.Path == "" || r.Test.New == "":
		return fmt.Sprintf("evidence %q needs a test file and the test to run", EvidenceReproTest)
	case !slices.Contains([]Oracle{OracleImplicit, OracleExisting, OracleSpec, OracleBase, OracleAgent}, r.Oracle):
		return fmt.Sprintf("oracle %q is not implicit, existing, spec, base or agent", r.Oracle)
	case r.Oracle == OracleSpec && r.Quote == "":
		return fmt.Sprintf("oracle %q needs the quoted text", OracleSpec)
	}
	return ""
}

func (w *Workflow) review(ctx context.Context, s Session, l *ledger, task ReviewTask) (expertState, error) {
	st := expertState{Findings: []Claim{}, Unread: []string{}, Runs: []Run{}}
	for attempt := range 2 {
		if !l.allow() {
			st.Absent = "budget exhausted"
			return st, nil
		}
		report, r, err := invoke(ctx, l, Run{Role: RoleExpert, Expert: task.Expert.Name}, s.Agents.Review, task)
		if err != nil {
			return st, err
		}
		st.Runs = append(st.Runs, r)
		if r.Error != "" {
			st.Absent = r.Error
			return st, nil
		}
		reasons := Validate(task, report)
		if len(reasons) == 0 {
			st.Findings = report.Findings
			st.Unread = slices.DeleteFunc(slices.Clone(task.Skills), func(s string) bool { return slices.Contains(report.Loaded, s) })
			return st, nil
		}
		if attempt == 1 {
			st.Absent = fmt.Sprintf("no valid report after %d attempts: %v", attempt+1, reasons)
			return st, nil
		}
		task.Rejected = reasons
	}
	return st, nil
}

func (w *Workflow) experts(ctx context.Context, s Session, l *ledger, plan Plan, from int, level Level, files map[string]File, tools []ToolResult, testOne string) ([]Finding, Uncertainty, error) {
	type slot struct {
		pick   Pick
		groups []int
		st     expertState
		err    error
	}
	var slots []*slot
	for gi := from; gi < len(plan.Groups); gi++ {
		for _, p := range plan.Groups[gi].Experts {
			i := slices.IndexFunc(slots, func(sl *slot) bool { return sl.pick.Name == p.Name })
			if i < 0 {
				slots = append(slots, &slot{pick: p})
				i = len(slots) - 1
			}
			slots[i].groups = append(slots[i].groups, gi)
		}
	}
	scoped := Plan{Summary: plan.Summary, Groups: plan.Groups[from:], Skipped: plan.Skipped, Confidence: plan.Confidence}
	var wg sync.WaitGroup
	turns := make(chan struct{}, maxParallel)
	for _, sl := range slots {
		wg.Go(func() {
			select {
			case turns <- struct{}{}:
			case <-ctx.Done():
				sl.err = fmt.Errorf("wait to start %s: %w", sl.pick.Name, ctx.Err())
				return
			}
			defer func() { <-turns }()
			task := w.panelTask(sl.pick, plan, sl.groups, level, files, tools)
			task.Plan, task.TestOne = scoped, testOne
			key := fmt.Sprintf("expert/%d/%s", sl.groups[0], sl.pick.Name)
			sl.st, sl.err = checkpoint(ctx, s.Archive, l, key, func() (expertState, error) { return w.review(ctx, s, l, task) })
		})
	}
	wg.Wait()

	findings := []Finding{}
	u := Uncertainty{Absent: []Absence{}, Unread: []Unread{}, Notes: []string{}}
	for _, sl := range slots {
		group := sl.groups[0]
		if sl.err != nil {
			return nil, Uncertainty{}, sl.err
		}
		if sl.st.Absent != "" {
			u.Absent = append(u.Absent, Absence{Expert: sl.pick.Name, Group: group, Reason: sl.st.Absent})
		}
		for _, skill := range sl.st.Unread {
			u.Unread = append(u.Unread, Unread{Expert: sl.pick.Name, Group: group, Skill: skill})
		}
		for i, c := range sl.st.Findings {
			f := Finding{ID: fmt.Sprintf("%d/%s/%d", group, sl.pick.Name, i), Expert: sl.pick.Name, Group: group, Claim: c}
			if gi := slices.IndexFunc(sl.groups, func(gi int) bool { return slices.Contains(plan.Groups[gi].Files, c.Path) }); gi >= 0 {
				f.Group = sl.groups[gi]
			}
			findings = append(findings, f)
		}
	}
	if !levels[level].nits {
		kept := slices.DeleteFunc(slices.Clone(findings), func(f Finding) bool { return f.Severity == SeverityLow })
		if n := len(findings) - len(kept); n > 0 {
			u.Notes = append(u.Notes, fmt.Sprintf("%d low-severity findings were left out at %s", n, level))
		}
		findings = kept
	}
	return findings, u, nil
}

func (w *Workflow) panelTask(p Pick, plan Plan, groups []int, level Level, files map[string]File, tools []ToolResult) ReviewTask {
	e, _ := w.expert(p.Name)
	first := plan.Groups[groups[0]]
	g := Group{Scope: first.Scope, Category: first.Category, Summary: first.Summary, Files: []string{}, Experts: []Pick{p}}
	for _, gi := range groups {
		g.Files = append(g.Files, plan.Groups[gi].Files...)
		g.Core = g.Core || plan.Groups[gi].Core
	}
	tier := e.Tier
	if t := levels[level].tier; t != "" {
		tier = t
	}
	return ReviewTask{Expert: e, Level: level, Tier: tier, Group: g, Skills: skillsFor(e, g.Files), Files: pick(files, g.Files), Tools: hitsFor(tools, g.Files), Rejected: []string{}}
}
