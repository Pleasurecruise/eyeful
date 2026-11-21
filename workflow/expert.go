package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
)

func Validate(t ReviewTask, r Report) [][]string {
	shown := map[string][]int{}
	for _, f := range t.Files {
		_, shown[f.Path] = Number(f.Diff)
	}
	reasons := make([][]string, len(r.Findings))
	for i, f := range r.Findings {
		reasons[i] = checkFinding(t, f, shown)
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

func checkFinding(t ReviewTask, f Claim, shown map[string][]int) []string {
	g := t.Group
	var reasons []string
	off := func(at Location) bool {
		return at.Line < 1 || (len(shown[at.Path]) > 0 && !slices.Contains(shown[at.Path], at.Line))
	}
	switch {
	case f.Skill == SkillOther && f.Category == "":
		reasons = append(reasons, fmt.Sprintf("an %q finding needs a category", SkillOther))
	case f.Skill != SkillOther && !slices.Contains(t.Skills, f.Skill):
		reasons = append(reasons, fmt.Sprintf("cites %q, which is neither a skill offered to you nor %q", f.Skill, SkillOther))
	}
	switch {
	case !slices.Contains(g.Files, f.Path):
		reasons = append(reasons, fmt.Sprintf("%q is not a file of this group", f.Path))
	case off(Location{Path: f.Path, Line: f.Line}):
		reasons = append(reasons, fmt.Sprintf("line %d is not a numbered line of the diff of %s: anchor the finding at the changed line that causes it", f.Line, f.Path))
	}
	for _, r := range f.Related {
		if !slices.Contains(g.Files, r.Path) || off(r) || (r.Path == f.Path && r.Line == f.Line) {
			reasons = append(reasons, fmt.Sprintf("related place %s:%d is not another numbered line of the diff of a file of this group", r.Path, r.Line))
		}
	}
	if f.Subject == "" {
		reasons = append(reasons, "needs a subject")
	}
	return append(reasons, checkEvidence(t, f)...)
}

func checkEvidence(t ReviewTask, f Claim) []string {
	var reasons []string
	if !slices.Contains([]Severity{SeverityError, SeverityWarning, SeverityNote}, f.Severity) {
		reasons = append(reasons, fmt.Sprintf("severity %q is not error, warning or note", f.Severity))
	}
	if !slices.Contains(t.Expert.Evidence, f.Evidence) {
		reasons = append(reasons, fmt.Sprintf("evidence %q is not one %s produces", f.Evidence, t.Expert.Name))
	}
	if f.Scenario == "" && (f.Evidence == EvidenceTest || f.Evidence == EvidenceAnalysis) {
		reasons = append(reasons, "needs a scenario: the input or sequence that triggers it and what goes wrong")
	}
	return reasons
}

func (w *Workflow) review(ctx context.Context, s Session, l *ledger, task ReviewTask) (expertState, error) {
	st := expertState{Findings: []Claim{}, Runs: []Run{}}
	for attempt := range 2 {
		report, r, err := invoke(ctx, l, Run{Role: RoleExpert, Expert: task.Expert.Name}, s.Agents.Review, task)
		if err != nil {
			return st, err
		}
		st.Runs = append(st.Runs, r)
		switch {
		case r.Error != "" && attempt == 0:
			w.log.Warn("expert absent", slog.String("expert", task.Expert.Name), slog.String("error", r.Error))
			st.Absent = r.Error
			return st, nil
		case r.Error != "":
			for _, c := range task.Draft {
				st.Dropped = append(st.Dropped, fmt.Sprintf("%s did not put right a finding eyeful sent back, so it was dropped: %s:%d %s: %s", task.Expert.Name, c.Path, c.Line, c.Subject, r.Error))
			}
			return st, nil
		}
		var draft []Claim
		var rejected []string
		for i, bad := range Validate(task, report) {
			switch f := report.Findings[i]; {
			case len(bad) == 0:
				st.Findings = append(st.Findings, f)
			case attempt == 0:
				rejected = append(rejected, fmt.Sprintf("draft %d: %s", len(draft), strings.Join(bad, "; ")))
				draft = append(draft, f)
			default:
				st.Dropped = append(st.Dropped, fmt.Sprintf("%s handed in a finding that eyeful still could not accept on the second attempt, so it was dropped: %s:%d %s: %s", task.Expert.Name, f.Path, f.Line, f.Subject, strings.Join(bad, "; ")))
			}
		}
		if len(draft) == 0 {
			return st, nil
		}
		w.log.Warn("findings sent back", slog.String("expert", task.Expert.Name), slog.Any("reasons", rejected))
		task.Draft, task.Rejected = draft, rejected
	}
	return st, nil
}

func (w *Workflow) experts(ctx context.Context, s Session, shares []*ledger, plan Plan, own []Plan, files map[string]File, broken *TestRun, known []Claim, req Request) ([]Finding, Uncertainty, error) {
	var seats []*seat
	for gi, g := range plan.Groups {
		for _, p := range g.Experts {
			i := slices.IndexFunc(seats, func(st *seat) bool { return st.pick.Name == p.Name && plan.Groups[st.groups[0]].Scope == g.Scope })
			if i < 0 {
				seats = append(seats, &seat{pick: p})
				i = len(seats) - 1
			}
			seats[i].groups = append(seats[i].groups, gi)
		}
	}
	for _, st := range seats {
		w.log.Info("expert", slog.String("name", st.pick.Name), slog.Any("groups", st.groups))
	}
	var wg sync.WaitGroup
	for _, st := range seats {
		wg.Go(func() {
			scope := plan.Groups[st.groups[0]].Scope
			task := w.panelTask(st.pick, plan, st.groups, files, known)
			task.TestOne, task.Commits, task.Description = req.Project.TestOne, req.Change.Commits, req.Change.Description
			if !task.Expert.Blind {
				task.Plan = own[scope]
			}
			if slices.Contains(task.Expert.Evidence, EvidenceTest) {
				task.Regression = broken
			}
			key := fmt.Sprintf("expert/%d/%s", st.groups[0], st.pick.Name)
			st.state, st.err = checkpoint(ctx, s.Archive, shares[scope], key, func() (expertState, error) { return w.review(ctx, s, shares[scope], task) })
		})
	}
	wg.Wait()
	return gather(seats, plan)
}

func gather(seats []*seat, plan Plan) ([]Finding, Uncertainty, error) {
	findings := []Finding{}
	u := Uncertainty{Absent: []Absence{}, Notes: []string{}}
	for _, st := range seats {
		group := st.groups[0]
		if st.err != nil {
			return nil, Uncertainty{}, st.err
		}
		if st.state.Absent != "" {
			u.Absent = append(u.Absent, Absence{Expert: st.pick.Name, Group: group, Reason: st.state.Absent})
		}
		u.Notes = append(u.Notes, st.state.Dropped...)
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

func (w *Workflow) panelTask(p Pick, plan Plan, groups []int, files map[string]File, earlier []Claim) ReviewTask {
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
	found := slices.DeleteFunc(slices.Clone(earlier), func(c Claim) bool { return !slices.Contains(g.Files, c.Path) })
	return ReviewTask{Expert: e, Tier: e.Tier, Group: g, Skills: SkillsFor(e.Skills, g.Files), Files: pick(files, g.Files), Rejected: []string{}, Earlier: found}
}
