package workflow

import (
	"context"
	"fmt"
	"slices"
	"strings"
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

func SkillsFor(skills []Skill, files []string) []string {
	out := []string{}
	for _, s := range skills {
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
	for _, r := range f.Related {
		if !slices.Contains(g.Files, r.Path) || r.Line < 1 || (r.Path == f.Path && r.Line == f.Line) {
			bad(fmt.Sprintf("related place %s:%d is not another line of a file of this group", r.Path, r.Line))
		}
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
	return reasons
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
		if reasons := Validate(task, report); len(reasons) > 0 && attempt == 0 {
			task.Rejected = reasons
			continue
		}
		for i, f := range report.Findings {
			if bad := checkFinding(task, f, i); len(bad) > 0 {
				st.Dropped = append(st.Dropped, fmt.Sprintf("%s handed in a finding that eyeful still could not accept on the second attempt, so it was dropped: %s", task.Expert.Name, strings.Join(bad, "; ")))
				continue
			}
			st.Findings = append(st.Findings, f)
		}
		if report.Loaded != nil {
			st.Unread = slices.DeleteFunc(slices.Clone(task.Skills), func(s string) bool { return slices.Contains(report.Loaded, s) })
		}
		return st, nil
	}
	return st, nil
}

func (w *Workflow) experts(ctx context.Context, s Session, l *ledger, plan Plan, from int, level Level, files map[string]File, tools []ToolResult, broken *TestRun, req Request) ([]Finding, Uncertainty, error) {
	var seats []*seat
	for gi := from; gi < len(plan.Groups); gi++ {
		for _, p := range plan.Groups[gi].Experts {
			i := slices.IndexFunc(seats, func(st *seat) bool { return st.pick.Name == p.Name })
			if i < 0 {
				seats = append(seats, &seat{pick: p})
				i = len(seats) - 1
			}
			seats[i].groups = append(seats[i].groups, gi)
		}
	}
	scoped := Plan{Summary: plan.Summary, Groups: plan.Groups[from:], Skipped: plan.Skipped, Confidence: plan.Confidence}
	var wg sync.WaitGroup
	turns := make(chan struct{}, maxParallel)
	var earlier []Claim
	for _, st := range seats {
		wg.Go(func() {
			select {
			case turns <- struct{}{}:
			case <-ctx.Done():
				st.err = fmt.Errorf("wait to start %s: %w", st.pick.Name, ctx.Err())
				return
			}
			defer func() { <-turns }()
			task := w.panelTask(st.pick, plan, st.groups, level, files, tools)
			task.TestOne, task.Earlier = req.Project.TestOne, slices.Clone(earlier)
			if !task.Expert.Blind {
				task.Plan = scoped
			}
			if slices.Contains(task.Expert.Evidence, EvidenceReproTest) {
				task.Regression = broken
			}
			key := fmt.Sprintf("expert/%d/%s", st.groups[0], st.pick.Name)
			st.state, st.err = checkpoint(ctx, s.Archive, l, key, func() (expertState, error) { return w.review(ctx, s, l, task) })
		})
		if req.Dependent {
			wg.Wait()
			earlier = append(earlier, st.state.Findings...)
		}
	}
	wg.Wait()
	return gather(seats, plan)
}

func gather(seats []*seat, plan Plan) ([]Finding, Uncertainty, error) {
	findings := []Finding{}
	u := Uncertainty{Absent: []Absence{}, Unread: []Unread{}, Notes: []string{}}
	for _, st := range seats {
		group := st.groups[0]
		if st.err != nil {
			return nil, Uncertainty{}, st.err
		}
		if st.state.Absent != "" {
			u.Absent = append(u.Absent, Absence{Expert: st.pick.Name, Group: group, Reason: st.state.Absent})
		}
		u.Notes = append(u.Notes, st.state.Dropped...)
		for _, skill := range st.state.Unread {
			u.Unread = append(u.Unread, Unread{Expert: st.pick.Name, Group: group, Skill: skill})
		}
		for i, c := range st.state.Findings {
			f := Finding{ID: fmt.Sprintf("%d/%s/%d", group, st.pick.Name, i), Expert: st.pick.Name, Group: group, Claim: c}
			if gi := slices.IndexFunc(st.groups, func(gi int) bool { return slices.Contains(plan.Groups[gi].Files, c.Path) }); gi >= 0 {
				f.Group = st.groups[gi]
			}
			findings = append(findings, f)
		}
	}
	return findings, u, nil
}

func (w *Workflow) panelTask(p Pick, plan Plan, groups []int, level Level, files map[string]File, tools []ToolResult) ReviewTask {
	e, _ := w.expert(p.Name)
	first := plan.Groups[groups[0]]
	g := Group{Scope: first.Scope, Category: first.Category, Summary: first.Summary, Files: []string{}, Experts: []Pick{p}}
	if e.Blind {
		g.Summary, g.Experts = "", []Pick{{Name: p.Name}}
	}
	for _, gi := range groups {
		g.Files = append(g.Files, plan.Groups[gi].Files...)
		g.Core = g.Core || plan.Groups[gi].Core
	}
	e.Evidence = slices.DeleteFunc(slices.Clone(e.Evidence), func(k EvidenceKind) bool {
		tool, backed := backing[k]
		return backed && !slices.ContainsFunc(tools, func(r ToolResult) bool { return r.Tool == tool })
	})
	tier := e.Tier
	if t := levels[level].tier; t != "" {
		tier = t
	}
	return ReviewTask{Expert: e, Tier: tier, Group: g, Skills: SkillsFor(e.Skills, g.Files), Files: pick(files, g.Files), Tools: hitsFor(tools, g.Files), Rejected: []string{}}
}
