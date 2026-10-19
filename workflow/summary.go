package workflow

import (
	"cmp"
	"context"
	"fmt"
	"slices"
)

const mergeDistance = 2

var kindRank = map[Kind]int{KindFail: 2, KindReview: 1, KindOpen: 0}

func kind(v Verification) Kind {
	switch {
	case v.Status == StatusRejected:
		return KindOpen
	case v.Status != StatusReproduced && v.Status != StatusFixed:
		return KindReview
	case v.Repro.Oracle == OracleSpecified, v.Repro.Oracle == OracleImplicit, v.Repro.Oracle == OracleDerived && v.Repro.Test == Edit{}:
		return KindFail
	}
	return KindReview
}

var labelOrder = []Label{
	LabelIssue, LabelQuestion, LabelSuggestion, LabelNitpick,
}

func (w *Workflow) match(ctx context.Context, s Session, l *ledger, findings []Finding) (matchState, error) {
	st := matchState{Root: map[string]string{}, Notes: []string{}, Runs: []Run{}}
	var pairs [][2]Finding
	for i, a := range findings {
		st.Root[a.ID] = a.ID
		for _, b := range findings[i+1:] {
			if a.Path != b.Path || max(a.Line-b.Line, b.Line-a.Line) > mergeDistance {
				continue
			}
			pairs = append(pairs, [2]Finding{a, b})
		}
	}
	if len(pairs) == 0 {
		return st, nil
	}
	same, r, err := invoke(ctx, l, Run{Role: RoleClassifier}, s.Classifier.Same, pairs)
	if err != nil {
		return st, err
	}
	st.Runs = append(st.Runs, r)
	switch {
	case r.Error != "":
		st.Notes = append(st.Notes, "findings on the same lines were not merged: "+r.Error)
	case len(same) != len(pairs):
		st.Notes = append(st.Notes, fmt.Sprintf("findings on the same lines were not merged: %d answers for %d pairs", len(same), len(pairs)))
	default:
		for i, p := range pairs {
			if same[i] < sameProblem {
				continue
			}
			to, from := st.Root[p[0].ID], st.Root[p[1].ID]
			for id, root := range st.Root {
				if root == from {
					st.Root[id] = to
				}
			}
		}
	}
	return st, nil
}

func merge(findings []Finding, verdicts map[string]Verification, root map[string]string) []Merged {
	kept := slices.Clone(findings)
	slices.SortStableFunc(kept, func(a, b Finding) int { return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line)) })
	var out []Merged
	at := map[string]int{}
	for _, f := range kept {
		v := verdicts[f.ID]
		if v.Status == StatusResolved {
			continue
		}
		if i, ok := at[root[f.ID]]; ok {
			m := &out[i]
			m.Findings = append(m.Findings, f)
			if !slices.Contains(m.Experts, f.Expert) {
				m.Experts = append(m.Experts, f.Expert)
			}
			if k := kind(v); kindRank[k] > kindRank[m.Kind] {
				m.Kind = k
			}
			m.Group = min(m.Group, f.Group)
			if v.Status != StatusRejected {
				m.Objection = ""
			}
			continue
		}
		at[root[f.ID]] = len(out)
		m := Merged{ID: f.ID, Path: f.Path, Line: f.Line, Findings: []Finding{f}, Experts: []string{f.Expert}, Kind: kind(v), Group: f.Group}
		if v.Status == StatusRejected {
			m.Objection = v.Reason
		}
		out = append(out, m)
	}
	for i := range out {
		slices.Sort(out[i].Experts)
	}
	return out
}

func (w *Workflow) comment(m Merged, verdicts map[string]Verification) Comment {
	lead := slices.MaxFunc(m.Findings, func(a, b Finding) int {
		return cmp.Or(cmp.Compare(kindRank[kind(verdicts[a.ID])], kindRank[kind(verdicts[b.ID])]), cmp.Compare(severityRank[a.Severity], severityRank[b.Severity]))
	})
	label, decoration := LabelSuggestion, "non-blocking"
	switch {
	case !slices.ContainsFunc(m.Findings, func(f Finding) bool { return f.Severity != SeverityNote }):
		label = LabelNitpick
	case m.Kind == KindFail:
		label, decoration = LabelIssue, "blocking"
	case slices.ContainsFunc(m.Findings, func(f Finding) bool { return f.Evidence != EvidenceInspection }):
		label = LabelQuestion
	}
	c := Comment{
		Finding: m.ID, Label: label, Decorations: []string{decoration}, Subject: lead.Subject, Discussion: lead.Discussion,
		Path: m.Path, Line: m.Line, Experts: m.Experts, Severity: lead.Severity, Kind: m.Kind, Objection: m.Objection,
	}
	for _, name := range m.Experts {
		if e, ok := w.expert(name); ok && e.Area != "" && !slices.Contains(c.Decorations, e.Area) {
			c.Decorations = append(c.Decorations, e.Area)
		}
	}
	for _, f := range m.Findings {
		v := verdicts[f.ID]
		c.Scenario = cmp.Or(c.Scenario, f.Scenario)
		for _, r := range f.Related {
			if !slices.Contains(c.Related, r) {
				c.Related = append(c.Related, r)
			}
		}
		if slices.Contains([]Status{StatusReproduced, StatusFixed, StatusPreExisting}, v.Status) {
			c.Repro = cmp.Or(c.Repro, v.Repro)
		}
		if c.Fix == nil && v.Status == StatusFixed {
			c.Fix = v.Fix
		}
	}
	return c
}

func (w *Workflow) comments(merged []Merged, verdicts map[string]Verification) []Comment {
	out := make([]Comment, 0, len(merged))
	for _, m := range merged {
		out = append(out, w.comment(m, verdicts))
	}
	folded := func(c Comment) int {
		switch {
		case c.Label == LabelNitpick:
			return 2
		case c.Objection != "":
			return 1
		}
		return 0
	}
	group := map[string]int{}
	for _, m := range merged {
		group[m.ID] = m.Group
	}
	slices.SortStableFunc(out, func(a, b Comment) int {
		return cmp.Or(
			cmp.Compare(folded(a), folded(b)),
			-cmp.Compare(kindRank[a.Kind], kindRank[b.Kind]),
			-cmp.Compare(len(a.Experts), len(b.Experts)),
			cmp.Compare(slices.Index(labelOrder, a.Label), slices.Index(labelOrder, b.Label)),
			cmp.Compare(group[a.Finding], group[b.Finding]),
			cmp.Compare(a.Path, b.Path),
			cmp.Compare(a.Line, b.Line),
		)
	})
	return out
}
