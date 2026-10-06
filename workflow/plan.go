package workflow

import (
	"fmt"
	"slices"
)

var levels = map[Level]limits{
	LevelQuick:    {experts: 2, tier: TierCheap, escalate: LevelStandard},
	LevelStandard: {experts: 4, planner: true, verify: true, escalate: LevelDeep},
	LevelDeep:     {planner: true, verify: true, full: true, escalate: LevelDeep},
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

func (w *Workflow) check(p Plan, t Triage, level Level) []string {
	var reasons []string
	assigned := map[string]bool{}
	reviewed := map[string]bool{}
	for _, e := range t.Reviewed {
		reviewed[e.Path] = true
	}
	limit := levels[level].experts
	for i, g := range p.Groups {
		if len(g.Experts) == 0 {
			reasons = append(reasons, fmt.Sprintf("group %d has no expert", i))
		}
		if limit > 0 && len(g.Experts) > limit {
			reasons = append(reasons, fmt.Sprintf("group %d uses %d experts; the %s level allows %d", i, len(g.Experts), level, limit))
		}
		seen := map[string]bool{}
		for _, e := range g.Experts {
			if _, ok := w.expert(e.Name); !ok {
				reasons = append(reasons, fmt.Sprintf("group %d: %q is not in the roster", i, e.Name))
			}
			if seen[e.Name] {
				reasons = append(reasons, fmt.Sprintf("group %d lists %q twice", i, e.Name))
			}
			seen[e.Name] = true
		}
		for _, f := range g.Files {
			if !reviewed[f] {
				reasons = append(reasons, fmt.Sprintf("group %d: %q is not a file triage kept", i, f))
			}
			assigned[f] = true
		}
	}
	for _, e := range t.Reviewed {
		if !assigned[e.Path] {
			reasons = append(reasons, fmt.Sprintf("%q belongs to no group", e.Path))
		}
	}
	if p.Confidence < 0 || p.Confidence > 1 {
		reasons = append(reasons, fmt.Sprintf("confidence %v is outside [0, 1]", p.Confidence))
	}
	return reasons
}

func whole(t Triage, category string, picks []Pick) Plan {
	p := Plan{Groups: []Group{}, Skipped: []Pick{}, Confidence: 1}
	if len(t.Reviewed) == 0 {
		return p
	}
	g := Group{Category: category, Summary: "every reviewed file", Experts: picks}
	for _, e := range t.Reviewed {
		g.Files = append(g.Files, e.Path)
	}
	p.Groups = []Group{g}
	return p
}

func (w *Workflow) defaultPlan(t Triage, level Level) Plan {
	picks := []Pick{}
	for _, e := range w.roster {
		if limit := levels[level].experts; limit > 0 && len(picks) == limit {
			break
		}
		picks = append(picks, Pick{Name: e.Name, Why: "default expert for the level"})
	}
	return whole(t, "default", picks)
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
