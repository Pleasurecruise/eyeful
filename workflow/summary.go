package workflow

import (
	"cmp"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	mergeDistance = 2
	shortLimit    = 60
)

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

func near(a, b Finding) bool {
	return a.Path == b.Path && a.Line-b.Line <= mergeDistance && b.Line-a.Line <= mergeDistance
}

var labelOrder = []Label{
	LabelIssue, LabelQuestion, LabelSuggestion, LabelNitpick,
}

func merge(findings []Finding, verdicts map[string]Verification) []Merged {
	kept := slices.Clone(findings)
	slices.SortStableFunc(kept, func(a, b Finding) int { return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line)) })
	var out []Merged
	for _, f := range kept {
		v := verdicts[f.ID]
		if v.Status == StatusResolved {
			continue
		}
		if n := len(out); n > 0 && near(out[n-1].Findings[len(out[n-1].Findings)-1], f) && !slices.Contains(out[n-1].Experts, f.Expert) {
			m := &out[n-1]
			m.Findings, m.Experts = append(m.Findings, f), append(m.Experts, f.Expert)
			if k := kind(v); kindRank[k] > kindRank[m.Kind] {
				m.Kind = k
			}
			m.Group = min(m.Group, f.Group)
			if v.Status != StatusRejected {
				m.Objection = ""
			}
			continue
		}
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
	lead := slices.MaxFunc(m.Findings, func(a, b Finding) int { return cmp.Compare(severityRank[a.Severity], severityRank[b.Severity]) })
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
		Finding: m.ID, Label: label, Decorations: []string{decoration}, Short: shorten(lead.Subject), Subject: lead.Subject, Discussion: lead.Discussion,
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

func shorten(subject string) string {
	subject = strings.TrimSuffix(strings.TrimSpace(subject), ".")
	if utf8.RuneCountInString(subject) <= shortLimit {
		return subject
	}
	runes := []rune(subject)[:shortLimit-1]
	if i := strings.LastIndex(string(runes), " "); i > shortLimit/2 {
		return string(runes)[:i] + "…"
	}
	return string(runes) + "…"
}
