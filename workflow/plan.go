package workflow

import (
	"fmt"
	"slices"

	"github.com/Pleasurecruise/eyeful/workflow/pulls"
)

var levels = map[Level]limits{
	LevelQuick:    {experts: 2, tier: TierCheap},
	LevelStandard: {experts: 4, planner: true, verify: true},
	LevelDeep:     {experts: 4, nits: true, planner: true, verify: true, full: true},
}

var crew = map[pulls.DiffCategory][]string{
	"security": {"correctness", "security"},
	"core":     {"correctness"},
	"api":      {"correctness"},
	"data":     {"correctness"},
	"cli":      {"correctness"},
	"ui":       {"usability"},
	"i18n":     {"usability"},
	"tests":    {"tests"},
	"docs":     {"readability"},
	"examples": {"readability"},
	"config":   {"security"},
	"build":    {"security"},
	"scripts":  {"security"},
	"deps":     {"security"},
}

var Levels = []Level{LevelQuick, LevelStandard, LevelDeep}

var signals = map[Tool]string{
	ToolSecretScan:      "security",
	ToolDependencyAudit: "security",
	ToolTypecheck:       "correctness",
	ToolOpenAPIDiff:     "usability",
}

var everyLevelSignals = []Tool{ToolSecretScan, ToolDependencyAudit, ToolOpenAPIDiff}

func chooseLevel(requested Level, t Triage) Level {
	if requested != "" {
		return requested
	}
	if len(t.Risk) > 0 || size(slices.Concat(t.Reviewed, t.Excluded)) > largeChange {
		return LevelDeep
	}
	return LevelStandard
}

func (w *Workflow) expert(name string) (Expert, bool) {
	i := slices.IndexFunc(w.roster, func(e Expert) bool { return e.Name == name })
	if i < 0 {
		return Expert{}, false
	}
	return w.roster[i], true
}

func (w *Workflow) staff(a pulls.Analysis, t Triage, level Level) Plan {
	p := Plan{Summary: a.OverallSummary, Groups: []Group{}, Skipped: []Pick{}, Confidence: 1}
	for _, g := range a.Groups {
		for _, leaf := range append([]pulls.DiffGroupLeaf{g.DiffGroupLeaf}, g.Children...) {
			core := leaf.Critical || slices.ContainsFunc(leaf.FilePaths, func(f string) bool { return slices.Contains(t.Risk, f) })
			names := slices.Clone(crew[leaf.Category])
			if core {
				names = append(names, "correctness", "security")
			}
			if core && level == LevelDeep {
				names = append(names, "design")
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
	p := Plan{Groups: []Group{}, Skipped: []Pick{}, Confidence: 1}
	if len(t.Reviewed) == 0 {
		return p
	}
	g := Group{Category: category, Summary: "every reviewed file", Core: len(t.Risk) > 0, Experts: picks}
	for _, e := range t.Reviewed {
		g.Files = append(g.Files, e.Path)
	}
	p.Groups = []Group{g}
	return p
}

func (w *Workflow) defaultPlan(t Triage, level Level) Plan {
	picks := []Pick{}
	for _, e := range w.roster {
		if len(picks) == levels[level].experts {
			break
		}
		picks = append(picks, Pick{Name: e.Name, Why: "default expert for the level"})
	}
	return whole(t, "other", picks)
}

func (w *Workflow) routePlan(tools []ToolResult) Plan {
	p := Plan{Groups: []Group{}, Skipped: []Pick{}, Confidence: 1}
	for _, r := range tools {
		name, ok := signals[r.Tool]
		if !ok || len(r.Hits) == 0 {
			continue
		}
		if _, ok := w.expert(name); !ok {
			p.Skipped = append(p.Skipped, Pick{Name: name, Why: fmt.Sprintf("%s found something, but %s is not in the roster", r.Tool, name)})
			continue
		}
		i := slices.IndexFunc(p.Groups, func(g Group) bool { return g.Experts[0].Name == name })
		if i < 0 {
			if len(p.Groups) == levels[LevelQuick].experts {
				p.Skipped = append(p.Skipped, Pick{Name: name, Why: "the quick level allows two experts"})
				continue
			}
			p.Groups = append(p.Groups, Group{Category: "signal", Summary: string(r.Tool), Experts: []Pick{{Name: name, Why: string(r.Tool) + " found something"}}})
			i = len(p.Groups) - 1
		}
		for _, h := range r.Hits {
			if !slices.Contains(p.Groups[i].Files, h.Path) {
				p.Groups[i].Files = append(p.Groups[i].Files, h.Path)
			}
		}
	}
	return p
}

func (w *Workflow) addSignals(p Plan, tools []ToolResult) Plan {
	for _, r := range tools {
		name := signals[r.Tool]
		if _, ok := w.expert(name); !ok || !slices.Contains(everyLevelSignals, r.Tool) {
			continue
		}
		for _, h := range r.Hits {
			why := fmt.Sprintf("%s: %s", r.Tool, h.Rule)
			i := slices.IndexFunc(p.Groups, func(g Group) bool { return slices.Contains(g.Files, h.Path) })
			if i < 0 {
				p.Groups = append(p.Groups, Group{Category: "signal", Summary: string(r.Tool), Files: []string{h.Path}, Experts: []Pick{}})
				i = len(p.Groups) - 1
			}
			if !slices.ContainsFunc(p.Groups[i].Experts, func(e Pick) bool { return e.Name == name }) {
				p.Groups[i].Experts = append(p.Groups[i].Experts, Pick{Name: name, Why: why})
			}
		}
	}
	return p
}
