package workflow

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	mergeDistance = 2
	shortLimit    = 60
)

var oracleStrength = map[Oracle]Strength{
	OracleImplicit: StrengthStrong,
	OracleExisting: StrengthStrong,
	OracleSpec:     StrengthStrong,
	OracleBase:     StrengthMedium,
	OracleAgent:    StrengthWeak,
}

var kindStrength = map[EvidenceKind]Strength{
	EvidenceCVE:         StrengthStrong,
	EvidenceAPIBreak:    StrengthStrong,
	EvidenceCoverage:    StrengthStrong,
	EvidenceTool:        StrengthMedium,
	EvidenceE2E:         StrengthMedium,
	EvidenceTriggerPath: StrengthMedium,
	EvidenceArgument:    StrengthWeak,
	EvidenceReproTest:   StrengthWeak,
}

func strength(f Finding, v Verification) Strength {
	if f.Repro != nil && (v.Status == StatusReproduced || v.Status == StatusFixed) {
		return oracleStrength[f.Repro.Oracle]
	}
	return kindStrength[f.Evidence]
}

func near(a, b Finding) bool {
	return a.Path == b.Path && a.Line-b.Line <= mergeDistance && b.Line-a.Line <= mergeDistance
}

var labelOrder = []Label{
	LabelIssue, LabelTodo, LabelChore, LabelSuggestion, LabelQuestion, LabelThought, LabelNote, LabelNitpick, LabelPraise,
}

func merge(findings []Finding, verdicts map[string]Verification) []Merged {
	kept := slices.Clone(findings)
	slices.SortStableFunc(kept, func(a, b Finding) int { return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line)) })
	var out []Merged
	for _, f := range kept {
		v := verdicts[f.ID]
		if v.Status == StatusNotReproduced || v.Status == StatusRejected {
			continue
		}
		if n := len(out); n > 0 && near(out[n-1].Findings[len(out[n-1].Findings)-1], f) {
			m := &out[n-1]
			m.Findings = append(m.Findings, f)
			m.Strength = max(m.Strength, strength(f, v))
			m.Group = min(m.Group, f.Group)
			if !slices.Contains(m.Experts, f.Expert) {
				m.Experts = append(m.Experts, f.Expert)
			}
			continue
		}
		out = append(out, Merged{ID: f.ID, Path: f.Path, Line: f.Line, Findings: []Finding{f}, Experts: []string{f.Expert}, Strength: strength(f, v), Group: f.Group})
	}
	for i := range out {
		slices.Sort(out[i].Experts)
	}
	return out
}

func (w *Workflow) comment(m Merged, d Draft, verdicts map[string]Verification) Comment {
	blocking := "non-blocking"
	if d.Label == LabelIssue && m.Strength == StrengthStrong {
		blocking = "blocking"
	}
	c := Comment{
		Finding: m.ID, Label: d.Label, Decorations: []string{blocking}, Short: d.Short, Subject: d.Subject, Discussion: d.Discussion,
		Path: m.Path, Line: m.Line, Experts: m.Experts, Strength: m.Strength,
	}
	for _, name := range m.Experts {
		if e, ok := w.expert(name); ok && e.Area != "" && !slices.Contains(c.Decorations, e.Area) {
			c.Decorations = append(c.Decorations, e.Area)
		}
	}
	for _, f := range m.Findings {
		v := verdicts[f.ID]
		c.Scenario = cmp.Or(c.Scenario, f.Scenario)
		if c.Test == nil && f.Repro != nil && (v.Status == StatusReproduced || v.Status == StatusFixed) {
			c.Test = &f.Repro.Test
		}
		if c.Fix == nil && v.Status == StatusFixed {
			c.Fix = v.Fix
		}
	}
	return c
}

func (w *Workflow) fallback(merged []Merged, verdicts map[string]Verification) []Comment {
	out := make([]Comment, 0, len(merged))
	for _, m := range merged {
		label := LabelQuestion
		if m.Strength == StrengthStrong {
			label = LabelIssue
		}
		f := m.Findings[0]
		out = append(out, w.comment(m, Draft{Finding: m.ID, Label: label, Short: shorten(f.Subject), Subject: f.Subject, Discussion: f.Discussion}, verdicts))
	}
	slices.SortStableFunc(out, func(a, b Comment) int { return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line)) })
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

func checkDrafts(merged []Merged, drafts []Draft) []string {
	var reasons []string
	count := map[string]int{}
	for _, d := range drafts {
		count[d.Finding]++
		i := slices.IndexFunc(merged, func(m Merged) bool { return m.ID == d.Finding })
		switch {
		case i < 0:
			reasons = append(reasons, fmt.Sprintf("%q is not a finding", d.Finding))
		case !slices.Contains(labelOrder, d.Label):
			reasons = append(reasons, fmt.Sprintf("%s: %q is not a Conventional Comments label", d.Finding, d.Label))
		case d.Label == LabelIssue && merged[i].Strength != StrengthStrong:
			reasons = append(reasons, d.Finding+": issue needs strong evidence")
		case d.Subject == "":
			reasons = append(reasons, d.Finding+": no subject")
		case d.Short == "" || utf8.RuneCountInString(d.Short) > shortLimit:
			reasons = append(reasons, fmt.Sprintf("%s: short must be 1 to %d characters", d.Finding, shortLimit))
		}
	}
	for _, m := range merged {
		if count[m.ID] != 1 {
			reasons = append(reasons, fmt.Sprintf("%s needs exactly one comment, got %d", m.ID, count[m.ID]))
		}
	}
	return reasons
}

func (w *Workflow) summarize(ctx context.Context, s Session, l *ledger, merged []Merged, verdicts map[string]Verification) (summaryState, error) {
	st := summaryState{Comments: []Comment{}, Runs: []Run{}}
	if len(merged) == 0 {
		return st, nil
	}
	if !l.allow() {
		st.Comments, st.Fallback = w.fallback(merged, verdicts), "budget exhausted before the summarizer ran"
		return st, nil
	}
	drafts, r, err := invoke(ctx, l, Run{Role: RoleSummarizer}, s.Agents.Summarize, SummaryTask{Findings: merged})
	if err != nil {
		return st, err
	}
	st.Runs = append(st.Runs, r)
	if r.Error != "" {
		st.Comments, st.Fallback = w.fallback(merged, verdicts), "the summarizer failed: "+r.Error
		return st, nil
	}
	if reasons := checkDrafts(merged, drafts); len(reasons) > 0 {
		st.Comments, st.Fallback = w.fallback(merged, verdicts), fmt.Sprintf("the summary was rejected: %v", reasons)
		return st, nil
	}
	byID := map[string]Merged{}
	for _, m := range merged {
		byID[m.ID] = m
	}
	for _, d := range drafts {
		st.Comments = append(st.Comments, w.comment(byID[d.Finding], d, verdicts))
	}
	slices.SortStableFunc(st.Comments, func(a, b Comment) int {
		ma, mb := byID[a.Finding], byID[b.Finding]
		return cmp.Or(
			cmp.Compare(slices.Index(labelOrder, a.Label), slices.Index(labelOrder, b.Label)),
			-cmp.Compare(a.Strength, b.Strength),
			-cmp.Compare(len(a.Experts), len(b.Experts)),
			cmp.Compare(ma.Group, mb.Group),
			cmp.Compare(a.Path, b.Path),
			cmp.Compare(a.Line, b.Line),
		)
	})
	return st, nil
}
