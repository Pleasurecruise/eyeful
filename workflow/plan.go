package workflow

import (
	"fmt"
	"slices"

	"github.com/Pleasurecruise/eyeful/workflow/pulls"
)

var crew = map[pulls.DiffCategory][]string{
	"security": {"correctness", "skeptic", "security"},
	"core":     {"correctness", "skeptic"},
	"api":      {"correctness", "skeptic"},
	"data":     {"correctness", "skeptic"},
	"cli":      {"correctness", "skeptic"},
	"ui":       {"usability", "correctness"},
	"i18n":     {"usability"},
	"tests":    {"tests"},
	"docs":     {"readability"},
	"examples": {"readability"},
	"config":   {"security"},
	"build":    {"security"},
	"scripts":  {"security", "correctness"},
	"deps":     {"security"},
}

func (w *Workflow) expert(name string) (Expert, bool) {
	i := slices.IndexFunc(w.roster, func(e Expert) bool { return e.Name == name })
	if i < 0 {
		return Expert{}, false
	}
	return w.roster[i], true
}

func (w *Workflow) staff(a pulls.Analysis, t Triage) Plan {
	p := Plan{Summary: a.OverallSummary, Groups: []Group{}, Skipped: []Pick{}, Confidence: 1}
	for _, g := range a.Groups {
		for _, leaf := range append([]pulls.DiffGroupLeaf{g.DiffGroupLeaf}, g.Children...) {
			core := leaf.Critical || slices.ContainsFunc(leaf.FilePaths, func(f string) bool { return slices.Contains(t.Risk, f) })
			names := slices.Clone(crew[leaf.Category])
			if core {
				names = append(names, "correctness", "skeptic", "security", "design")
			}
			summary := leaf.Label
			if leaf.Summary != "" {
				summary += ": " + leaf.Summary
			}
			group := Group{Category: string(leaf.Category), Summary: summary, Core: core, Files: leaf.FilePaths, Experts: []Pick{}}
			for _, name := range names {
				if _, ok := w.expert(name); ok && !slices.ContainsFunc(group.Experts, func(p Pick) bool { return p.Name == name }) {
					group.Experts = append(group.Experts, Pick{Name: name, Why: fmt.Sprintf("a %s group", leaf.Category)})
				}
			}
			p.Groups = append(p.Groups, group)
		}
	}
	return p
}

func whole(t Triage, category string, picks []Pick) Plan {
	g := Group{Category: category, Summary: "every reviewed file", Core: len(t.Risk) > 0, Experts: picks}
	for _, e := range t.Reviewed {
		g.Files = append(g.Files, e.Path)
	}
	return Plan{Groups: []Group{g}, Skipped: []Pick{}, Confidence: 1}
}
